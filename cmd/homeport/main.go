package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"git.zk35.de/secalpha/homeport/assets"
	"git.zk35.de/secalpha/homeport/core"
	"git.zk35.de/secalpha/homeport/internal/api"
	"git.zk35.de/secalpha/homeport/internal/config"
	"git.zk35.de/secalpha/homeport/internal/db"
)

func main() {
	cfg := config.Load()

	slog.Info("homeport starting", "port", cfg.Port, "db", cfg.DBPath)

	// Init DB
	if err := db.InitDB(cfg.DBPath); err != nil {
		slog.Error("failed to init db", "err", err)
		os.Exit(1)
	}

	// Init Templates (uses embedded FS from assets package)
	api.InitTemplates(assets.FS)

	// Start Background Tasks
	api.StartStatusChecker()
	go runICalFetcher()
	go runWeatherFetcher()
	go runPodmanScanner()

	// Router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	// Static Files from embedded FS
	staticFS, _ := fs.Sub(assets.FS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Handle("/sw.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFileFS(w, r, assets.FS, "static/sw.js")
	}))

	// Existing HTML Routes
	r.Get("/", api.HandleIndex)
	r.Get("/andrea", api.HandleIndex)

	r.Get("/status/stream", api.HandleStatusStream)

	// URL Shortener – public redirect (no auth)
	api.RegisterShortenerPublicRoutes(r)

	r.Route("/manage", func(r chi.Router) {
		r.Post("/clone-andrea", api.HandleCloneToAndrea)
		r.Post("/widget", api.HandleAddWidget)
		r.Delete("/widget/{id}", api.HandleDeleteWidget)
		r.Get("/", api.HandleManage)
		r.Post("/category", api.HandleAddCategory)
		r.Post("/service", api.HandleAddService)
		r.Delete("/category/{id}", api.HandleDeleteCategory)
		r.Delete("/service/{id}", api.HandleDeleteService)
		r.Get("/category/{id}/edit", api.HandleGetCategory)
		r.Patch("/category/{id}", api.HandleUpdateCategory)
		r.Post("/category/{id}/span/{span}", api.HandleUpdateCategorySpan)
		r.Get("/service/{id}/edit", api.HandleGetService)
		r.Patch("/service/{id}", api.HandleUpdateService)
		r.Post("/sort/category/{id}/{direction}", api.HandleSortCategory)
		r.Post("/sort/service/{id}/{direction}", api.HandleSortService)

		r.Get("/discovery", api.HandleDiscoveryInbox)
		r.Post("/discovery/{id}/accept", api.HandleAcceptDiscovery)
		r.Post("/discovery/{id}/ignore", api.HandleIgnoreDiscovery)

		r.Post("/settings/search", api.HandleSetSearchEngine)

		r.Post("/shorten", api.HandleManageShorten)
		r.Post("/unshorten/{code}", api.HandleManageUnshorten)
	})

	// REST API Routes
	r.Route("/api", func(r chi.Router) {
		// Health + Favicon: no auth
		r.Get("/health", api.HandleHealth)
		r.Get("/favicon", api.HandleFavicon)

		// Protected API routes
		r.Group(func(r chi.Router) {
			r.Use(api.AuthMiddleware(cfg.Token))

			// Widgets
			r.Get("/widgets", api.HandleGetWidgets)
			r.Post("/widgets", api.HandleCreateWidget)
			r.Patch("/widgets/reorder", api.HandleReorderWidgets)
			r.Get("/widgets/{id}", api.HandleGetWidget)
			r.Patch("/widgets/{id}", api.HandleUpdateWidget)
			r.Delete("/widgets/{id}", api.HandleDeleteWidgetAPI)

			// Preferences
			r.Get("/user/preferences", api.HandleGetPreferences)
			r.Patch("/user/preferences", api.HandleSetPreferences)

			// SSE Live Updates
			r.Get("/updates", api.DefaultHub.HandleUpdates)

			// URL Shortener API
			api.RegisterShortenerAPIRoutes(r)
		})
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		slog.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("shutdown complete")
}

func runICalFetcher() {
	fetchAll := func() {
		widgets, err := db.GetAllWidgets()
		if err != nil {
			slog.Error("iCal fetcher: error loading widgets", "err", err)
			return
		}
		for _, w := range widgets {
			if w.Type != "ical" {
				continue
			}
			var cfg struct{ URL string }
			if err := json.Unmarshal([]byte(w.Config), &cfg); err != nil || cfg.URL == "" {
				continue
			}
			events, err := core.FetchICalEvents(cfg.URL)
			if err != nil {
				slog.Error("iCal fetcher: widget error", "widget_id", w.ID, "err", err)
				continue
			}
			data, _ := json.Marshal(struct{ Events interface{} }{Events: events})
			if err := db.UpdateWidgetCache(w.ID, string(data)); err != nil {
				slog.Error("iCal fetcher: cache update error", "widget_id", w.ID, "err", err)
			}
		}
	}
	fetchAll()
	for range time.Tick(6 * time.Hour) {
		fetchAll()
	}
}

func runWeatherFetcher() {
	fetchAll := func() {
		widgets, err := db.GetAllWidgets()
		if err != nil {
			slog.Error("Weather fetcher: error loading widgets", "err", err)
			return
		}
		for _, w := range widgets {
			if w.Type != "weather" {
				continue
			}
			var cfg struct {
				Lat      float64 `json:"lat"`
				Lon      float64 `json:"lon"`
				CityName string  `json:"city_name"`
			}
			if err := json.Unmarshal([]byte(w.Config), &cfg); err != nil || cfg.Lat == 0 {
				continue
			}
			wd, err := core.FetchWeather(cfg.Lat, cfg.Lon)
			if err != nil {
				slog.Error("Weather fetcher: widget error", "widget_id", w.ID, "err", err)
				continue
			}
			wd.CityName = cfg.CityName
			// Store as WeatherCache (same fields, serialized as JSON)
			cache := struct {
				Temperature float64
				WeatherCode int
				Description string
				WindSpeed   float64
				Humidity    int
				IsDay       bool
				CityName    string
				Forecast    interface{}
			}{
				Temperature: wd.Temperature,
				WeatherCode: wd.WeatherCode,
				Description: wd.Description,
				WindSpeed:   wd.WindSpeed,
				Humidity:    wd.Humidity,
				IsDay:       wd.IsDay,
				CityName:    wd.CityName,
				Forecast:    wd.Forecast,
			}
			data, _ := json.Marshal(cache)
			if err := db.UpdateWidgetCache(w.ID, string(data)); err != nil {
				slog.Error("Weather fetcher: cache update error", "widget_id", w.ID, "err", err)
			}
		}
	}
	fetchAll()
	for range time.Tick(6 * time.Hour) {
		fetchAll()
	}
}

func runPodmanScanner() {
	scan := func() {
		services, containerIDs, err := core.ScanPodmanContainers()
		if err != nil {
			slog.Error("Podman scanner: error scanning containers", "err", err)
			return
		}
		if services == nil {
			return
		}
		for i, svc := range services {
			suggestedJSON, err := json.Marshal(svc)
			if err != nil {
				slog.Error("Podman scanner: error marshaling service", "name", svc.Name, "err", err)
				continue
			}
			if err = db.AddDiscoveryItem(containerIDs[i], string(suggestedJSON)); err != nil {
				slog.Error("Podman scanner: error adding discovery item", "name", svc.Name, "err", err)
			}
		}
	}
	scan()
	for range time.Tick(60 * time.Second) {
		scan()
	}
}
