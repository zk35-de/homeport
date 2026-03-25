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
	        r.Post("/profile/{slug}/clone", api.HandleCloneProfile)
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
		// Health + Favicon + Search + Preferences: no Bearer auth required
		r.Get("/health", api.HandleHealth)
		r.Get("/favicon", api.HandleFavicon)
		r.Get("/search", api.HandleSearch)

		// Preferences: accessible via session cookie (browser) or Bearer token (API clients)
		r.Get("/user/preferences", api.HandleGetPreferences)
		r.Patch("/user/preferences", api.HandleSetPreferences)

		// Protected API routes (Bearer token required)
		r.Group(func(r chi.Router) {
			r.Use(api.AuthMiddleware(cfg.Token))

			// Widgets
			r.Get("/widgets", api.HandleGetWidgets)
			r.Post("/widgets", api.HandleCreateWidget)
			r.Patch("/widgets/reorder", api.HandleReorderWidgets)
			r.Get("/widgets/{id}", api.HandleGetWidget)
			r.Patch("/widgets/{id}", api.HandleUpdateWidget)
			r.Delete("/widgets/{id}", api.HandleDeleteWidgetAPI)

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

func runSessionPurger() {
	for range time.Tick(24 * time.Hour) {
		if err := db.PurgeExpiredSessions(); err != nil {
			slog.Error("session purger error", "err", err)
		}
	}
}
