package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"git.zk35.de/secalpha/homeport/internal/backup"
	"git.zk35.de/secalpha/homeport/internal/config"
	"git.zk35.de/secalpha/homeport/internal/discovery"

	"git.zk35.de/secalpha/homeport/internal/db"
)

func main() {
	// CLI subcommand: homeport passwd <profile>
	if len(os.Args) >= 2 && os.Args[1] == "passwd" {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: homeport passwd <profile>\n")
			os.Exit(1)
		}
		profile := os.Args[2]
		dbPath := os.Getenv("HOMEPORT_DB")
		if dbPath == "" {
			dbPath = "./data/homeport.db"
		}
		if err := db.InitDB(dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "DB error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Neues Passwort für '%s': ", profile)
		var pw string
		fmt.Scanln(&pw)
		if pw == "" {
			fmt.Fprintln(os.Stderr, "Leeres Passwort nicht erlaubt.")
			os.Exit(1)
		}
		if err := db.SetPassword(profile, pw); err != nil {
			fmt.Fprintf(os.Stderr, "Fehler: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Passwort für '%s' gesetzt.\n", profile)
		return
	}

	cfg := config.Load()

	slog.Info("homeport starting", "port", cfg.Port, "db", cfg.DBPath)

	// Pass config to API handlers
	api.SetConfig(cfg)

	// Init DB
	if err := db.InitDB(cfg.DBPath); err != nil {
	        slog.Error("failed to init db", "err", err)
	        os.Exit(1)
	}

	// Scheduled Backups
	if cfg.BackupInterval != "" {
	        if d, err := time.ParseDuration(cfg.BackupInterval); err == nil && d > 0 {
	                slog.Info("scheduled backups enabled", "interval", d, "dir", cfg.BackupDir, "max_keep", cfg.BackupMaxKeep)
	                backup.ScheduledBackup(cfg.DBPath, cfg.BackupDir, d, cfg.BackupMaxKeep)
	        } else if err != nil {
	                slog.Error("failed to parse backup interval", "val", cfg.BackupInterval, "err", err)
	        }
	}

	// Init Templates (uses embedded FS from assets package)

	api.InitTemplates(assets.FS)

	// Start Background Tasks
	api.StartStatusChecker()
	go runICalFetcher()
	go runWeatherFetcher()
	go runRSSFetcher()
	go runGithubFetcher()
	go runRouterFetcher()
	go runPodmanScanner()
	go runSessionPurger()

	// Discovery Scheduler
	discovery.Global.Reload()

	// Router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(api.SecurityHeaders)
	r.Use(api.CSRFMiddleware)
	r.Use(api.RequireAuth(cfg))

	// Static Files from embedded FS
	staticFS, _ := fs.Sub(assets.FS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Handle("/sw.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFileFS(w, r, assets.FS, "static/sw.js")
	}))

	// Auth routes
	r.Get("/login", api.HandleLogin)
	r.Post("/login", api.HandleLogin)
	r.Post("/logout", api.HandleLogout)

	// HTML Routes – / = default profile, /{slug} = profile by slug
	// /{slug} muss NACH allen statischen Routen stehen (chi priorisiert statische)
	r.Get("/", api.HandleIndex)

	r.Get("/status/stream", api.HandleStatusStream)

	// Service click-tracking redirect
	r.Get("/r/{id}", api.HandleServiceRedirect)

	r.Route("/manage", func(r chi.Router) {
	        r.Post("/clone-andrea", api.HandleCloneToAndrea)
	        r.Post("/widget", api.HandleAddWidget)
	        r.Delete("/widget/{id}", api.HandleDeleteWidget)
	        r.Get("/", api.HandleManage)
		r.Get("/analytics", api.HandleAnalytics)

	        // Backup & Restore
	        r.Get("/backup", api.HandleBackupDownload)
	        r.Post("/restore", api.HandleRestore)

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
		r.Post("/sort/category/reorder", api.HandleReorderCategories)
		r.Post("/sort/service/reorder", api.HandleReorderServices)

		r.Get("/discovery", api.HandleDiscoveryInbox)
		r.Post("/discovery/{id}/accept", api.HandleAcceptDiscovery)
		r.Post("/discovery/{id}/ignore", api.HandleIgnoreDiscovery)

		// Discovery Sources
		r.Get("/discovery/sources", api.HandleGetDiscoverySources)
		r.Post("/discovery/sources", api.HandleAddDiscoverySource)
		r.Delete("/discovery/sources/{id}", api.HandleDeleteDiscoverySource)
		r.Post("/discovery/sources/{id}/toggle", api.HandleToggleDiscoverySource)
		r.Post("/discovery/sources/{id}/scan", api.HandleScanDiscoverySource)

		r.Post("/settings/search", api.HandleSetSearchEngine)

		// Auth management
		r.Get("/auth", api.HandleManageAuth)
		r.Post("/auth/password", api.HandleSetPassword)
		r.Post("/auth/password/delete", api.HandleDeletePassword)

		// Profile management
		r.Post("/profile", api.HandleAddProfile)
		r.Delete("/profile/{slug}", api.HandleDeleteProfile)
		r.Post("/profile/{slug}/default", api.HandleSetDefaultProfile)
		r.Post("/category/{id}/sortmode/{mode}", api.HandleSetCategorySortMode)

		// Page management
		r.Get("/page-list", api.HandleGetPageList)
		r.Post("/page", api.HandleAddPage)
		r.Delete("/page/{id}", api.HandleDeletePage)
		r.Patch("/page/{id}", api.HandleUpdatePage)
		r.Post("/sort/page/{id}/{direction}", api.HandleSortPage)
		r.Post("/category/{id}/page/{pageID}", api.HandleSetCategoryPage)
	})

	// Todo routes (no auth – HTMX from index page)
	r.Post("/api/todos", api.HandleAddTodo)
	r.Post("/api/todos/{id}/toggle", api.HandleToggleTodo)
	r.Delete("/api/todos/{id}", api.HandleDeleteTodo)

	// Bookmark routes (HTMX from index page)
	r.Post("/api/widgets/{id}/bookmark", api.HandleAddBookmark)
	r.Delete("/api/widgets/{id}/bookmark/{idx}", api.HandleDeleteBookmark)

	// Notes route
	r.Put("/api/notes/{id}", api.HandleSaveNote)

	// REST API Routes
	r.Route("/api", func(r chi.Router) {
		// Health + Favicon + Search: no auth
		r.Get("/health", api.HandleHealth)
		r.Get("/favicon", api.HandleFavicon)
		r.Get("/search", api.HandleSearch)

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
		})
	})

	// /{slug} nach allen statischen Routen – chi priorisiert diese automatisch
	r.Get("/{slug}", api.HandleIndex)

	// Custom 404
	r.NotFound(api.Handle404)

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
			switch w.Type {
			case "ical":
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
				db.UpdateWidgetCache(w.ID, string(data))
			case "caldav":
				var cfg struct {
					URL      string `json:"url"`
					Username string `json:"username"`
					Password string `json:"password"`
				}
				if err := json.Unmarshal([]byte(w.Config), &cfg); err != nil || cfg.URL == "" {
					continue
				}
				events, err := core.FetchCalDAVEvents(cfg.URL, cfg.Username, cfg.Password)
				if err != nil {
					slog.Error("CalDAV fetcher: widget error", "widget_id", w.ID, "err", err)
					continue
				}
				data, _ := json.Marshal(struct{ Events interface{} }{Events: events})
				db.UpdateWidgetCache(w.ID, string(data))
			}
		}
	}
	fetchAll()
	for range time.Tick(6 * time.Hour) {
		fetchAll()
	}
}

func runGithubFetcher() {
	fetchAll := func() {
		widgets, err := db.GetAllWidgets()
		if err != nil {
			slog.Error("GitHub fetcher: error loading widgets", "err", err)
			return
		}
		for _, w := range widgets {
			if w.Type != "github" {
				continue
			}
			var cfg struct {
				Token      string `json:"token"`
				ShowPRs    bool   `json:"show_prs"`
				ShowIssues bool   `json:"show_issues"`
			}
			if err := json.Unmarshal([]byte(w.Config), &cfg); err != nil || cfg.Token == "" {
				continue
			}
			ghData, err := core.FetchGithubData(cfg.Token, cfg.ShowPRs, cfg.ShowIssues)
			if err != nil {
				slog.Error("GitHub fetcher: widget error", "widget_id", w.ID, "err", err)
				continue
			}
			type ghItem struct {
				Title  string `json:"title"`
				URL    string `json:"url"`
				Number int    `json:"number"`
				Repo   string `json:"repo"`
			}
			prs := make([]ghItem, len(ghData.PRs))
			for i, p := range ghData.PRs {
				prs[i] = ghItem{Title: p.Title, URL: p.URL, Number: p.Number, Repo: p.Repo}
			}
			issues := make([]ghItem, len(ghData.Issues))
			for i, p := range ghData.Issues {
				issues[i] = ghItem{Title: p.Title, URL: p.URL, Number: p.Number, Repo: p.Repo}
			}
			data, _ := json.Marshal(struct {
				GithubPRs    []ghItem `json:"GithubPRs"`
				GithubIssues []ghItem `json:"GithubIssues"`
				GithubUser   string   `json:"GithubUser"`
			}{GithubPRs: prs, GithubIssues: issues, GithubUser: ghData.User})
			db.UpdateWidgetCache(w.ID, string(data))
		}
	}
	fetchAll()
	for range time.Tick(15 * time.Minute) {
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

func runRSSFetcher() {
	fetchAll := func() {
		widgets, err := db.GetAllWidgets()
		if err != nil {
			slog.Error("RSS fetcher: error loading widgets", "err", err)
			return
		}
		for _, w := range widgets {
			if w.Type != "rss" {
				continue
			}
			var cfg struct {
				URL string      `json:"url"`
				Max json.Number `json:"max"`
			}
			if err := json.Unmarshal([]byte(w.Config), &cfg); err != nil || cfg.URL == "" {
				continue
			}
			items, err := core.FetchRSSFeed(cfg.URL)
			if err != nil {
				slog.Error("RSS fetcher: widget error", "widget_id", w.ID, "err", err)
				continue
			}
			max, _ := cfg.Max.Int64()
			if max <= 0 || max > 50 {
				max = 10
			}
			if int64(len(items)) > max {
				items = items[:max]
			}
			// Convert core.RSSItem → db.RSSItem
			dbItems := make([]db.RSSItem, len(items))
			for i, it := range items {
				dbItems[i] = db.RSSItem{Title: it.Title, URL: it.URL, PubDate: it.PubDate}
			}
			data, _ := json.Marshal(struct {
				RSSItems []db.RSSItem `json:"RSSItems"`
			}{RSSItems: dbItems})
			if err := db.UpdateWidgetCache(w.ID, string(data)); err != nil {
				slog.Error("RSS fetcher: cache update error", "widget_id", w.ID, "err", err)
			}
		}
	}
	fetchAll()
	for range time.Tick(30 * time.Minute) {
		fetchAll()
	}
}

func runRouterFetcher() {
	fetchAll := func() {
		widgets, err := db.GetAllWidgets()
		if err != nil {
			slog.Error("Router fetcher: error loading widgets", "err", err)
			return
		}
		for _, w := range widgets {
			if w.Type != "router" {
				continue
			}
			var cfg struct {
				RouterType     string `json:"router_type"`
				RouterURL      string `json:"router_url"`
				RouterPassword string `json:"router_password"`
			}
			if err := json.Unmarshal([]byte(w.Config), &cfg); err != nil {
				continue
			}
			fetcher := core.NewRouterFetcher(cfg.RouterType, cfg.RouterURL, cfg.RouterPassword)
			rs, err := fetcher.Fetch()
			if err != nil {
				slog.Error("Router fetcher: widget error", "widget_id", w.ID, "err", err)
				continue
			}
			data, _ := json.Marshal(struct {
				DSLDownMbit  float64 `json:"DSLDownMbit"`
				DSLUpMbit    float64 `json:"DSLUpMbit"`
				DSLOnline    bool    `json:"DSLOnline"`
				LTEActive    bool    `json:"LTEActive"`
				LTESignalDBm float64 `json:"LTESignalDBm"`
				LTEBand      string  `json:"LTEBand"`
				Mode         string  `json:"Mode"`
				Online       bool    `json:"Online"`
			}{
				DSLDownMbit:  rs.DSLDownMbit,
				DSLUpMbit:    rs.DSLUpMbit,
				DSLOnline:    rs.DSLOnline,
				LTEActive:    rs.LTEActive,
				LTESignalDBm: rs.LTESignalDBm,
				LTEBand:      rs.LTEBand,
				Mode:         rs.Mode,
				Online:       rs.Online,
			})
			if err := db.UpdateWidgetCache(w.ID, string(data)); err != nil {
				slog.Error("Router fetcher: cache update error", "widget_id", w.ID, "err", err)
			}
		}
	}
	fetchAll()
	for range time.Tick(60 * time.Second) {
		fetchAll()
	}
}

func runSessionPurger() {
	for range time.Tick(24 * time.Hour) {
		if err := db.PurgeExpiredSessions(); err != nil {
			slog.Error("session purger error", "err", err)
		}
	}
}
