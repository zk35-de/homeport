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
	"golang.org/x/term"

	"github.com/zk35-de/homeport/assets"
	"github.com/zk35-de/homeport/core"
	"github.com/zk35-de/homeport/internal/api"
	"github.com/zk35-de/homeport/internal/backup"
	"github.com/zk35-de/homeport/internal/config"
	"github.com/zk35-de/homeport/internal/discovery"

	"github.com/zk35-de/homeport/internal/db"
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
		fmt.Fprintf(os.Stderr, "Neues Passwort für '%s': ", profile)
		pwBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fehler beim Lesen des Passworts: %v\n", err)
			os.Exit(1)
		}
		pw := string(pwBytes)
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

	// Create Server with config and init templates
	srv := api.New(cfg)
	srv.InitTemplates(assets.FS)

	// Start Background Tasks
	srv.StartStatusChecker()
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
		content, err := assets.FS.ReadFile("static/sw.js")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprintf(w, "const APP_VERSION = %q;\n", api.AppVersion)
		w.Write(content)
	}))

	// Auth routes
	r.Get("/login", srv.HandleLogin)
	r.Post("/login", srv.HandleLogin)
	r.Post("/logout", srv.HandleLogout)

	// HTML Routes – / = default profile, /{slug} = profile by slug
	r.Get("/", srv.HandleIndex)

	r.Get("/status/stream", srv.HandleStatusStream)

	// Service click-tracking redirect
	r.Get("/r/{id}", api.HandleServiceRedirect)

	r.Route("/manage", func(r chi.Router) {
		// User-level: any authenticated user can manage their own content.
		// RequireAuth (global middleware) already ensures a valid session.
		r.Get("/", srv.HandleManage)
		r.Get("/category-options", srv.HandleCategoryOptions)
		r.Get("/profile-options", srv.HandleProfileOptions)

		r.Post("/category", srv.HandleAddCategory)
		r.Get("/category/{id}/edit", srv.HandleGetCategory)
		r.Patch("/category/{id}", srv.HandleUpdateCategory)
		r.Delete("/category/{id}", srv.HandleDeleteCategory)
		r.Post("/category/{id}/span/{span}", srv.HandleUpdateCategorySpan)
		r.Post("/category/{id}/sortmode/{mode}", srv.HandleSetCategorySortMode)
		r.Get("/category/{id}/visibility", srv.HandleGetCategoryVisibility)
		r.Post("/category/{id}/visibility", srv.HandleSetCategoryVisibility)

		r.Post("/service", srv.HandleAddService)
		r.Get("/service/{id}/edit", srv.HandleGetService)
		r.Patch("/service/{id}", srv.HandleUpdateService)
		r.Delete("/service/{id}", srv.HandleDeleteService)

		r.Post("/sort/category/{id}/{direction}", srv.HandleSortCategory)
		r.Post("/sort/service/{id}/{direction}", srv.HandleSortService)
		r.Post("/sort/category/reorder", srv.HandleReorderCategories)
		r.Post("/sort/service/reorder", srv.HandleReorderServices)

		r.Get("/discovery", srv.HandleDiscoveryInbox)
		r.Post("/discovery/{id}/accept", srv.HandleAcceptDiscovery)
		r.Post("/discovery/{id}/ignore", srv.HandleIgnoreDiscovery)
		r.Post("/discovery/{id}/accept-cl", srv.HandleAcceptDiscoveryCL)
		r.Post("/discovery/{id}/ignore-cl", srv.HandleIgnoreDiscoveryCL)

		r.Get("/page-list", srv.HandleGetPageList)
		r.Post("/page", srv.HandleAddPage)
		r.Delete("/page/{id}", srv.HandleDeletePage)
		r.Patch("/page/{id}", srv.HandleUpdatePage)
		r.Post("/sort/page/{id}/{direction}", srv.HandleSortPage)
		r.Post("/category/{id}/page/{pageID}", srv.HandleSetCategoryPage)

		// Admin-only: system-level operations.
		r.Group(func(r chi.Router) {
			r.Use(srv.RequireAdmin)

			r.Get("/analytics", srv.HandleAnalytics)
			r.Get("/backup", srv.HandleBackupDownload)
			r.Post("/restore", srv.HandleRestore)
			r.Post("/profile/{slug}/clone", srv.HandleCloneProfile)

			r.Get("/discovery/sources", srv.HandleGetDiscoverySources)
			r.Post("/discovery/sources", srv.HandleAddDiscoverySource)
			r.Delete("/discovery/sources/{id}", srv.HandleDeleteDiscoverySource)
			r.Post("/discovery/sources/{id}/toggle", srv.HandleToggleDiscoverySource)
			r.Post("/discovery/sources/{id}/scan", srv.HandleScanDiscoverySource)

			r.Get("/auth", srv.HandleManageAuth)
			r.Post("/auth/password", srv.HandleSetPassword)
			r.Post("/auth/password/delete", srv.HandleDeletePassword)

			r.Post("/profile", srv.HandleAddProfile)
			r.Delete("/profile/{slug}", srv.HandleDeleteProfile)
			r.Post("/profile/{slug}/default", srv.HandleSetDefaultProfile)
		})
	})

	// REST API Routes
	r.Route("/api", func(r chi.Router) {
		// Health + Favicon + Search + Preferences: no Bearer auth required
		r.Get("/health", api.HandleHealth)
		r.Get("/favicon", api.HandleFavicon)
		r.Get("/search", api.HandleSearch)

		// Preferences: accessible via session cookie (browser) or Bearer token (API clients)
		r.Get("/user/preferences", api.HandleGetPreferences)
		r.Patch("/user/preferences", api.HandleSetPreferences)

		// Profile theme CSS (accent color + custom CSS as stylesheet)
		r.Get("/profile/{slug}/theme.css", api.HandleProfileThemeCSS)
	})

	// /{slug} nach allen statischen Routen – chi priorisiert diese automatisch
	r.Get("/{slug}", srv.HandleIndex)

	// Custom 404
	r.NotFound(api.Handle404)

	httpSrv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		slog.Info("server listening", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("shutdown complete")
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
