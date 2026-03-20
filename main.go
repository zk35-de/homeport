package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"encoding/json"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"git.zk35.de/secalpha/homeport/core"
	"git.zk35.de/secalpha/homeport/db"
	"git.zk35.de/secalpha/homeport/handlers"
)

//go:embed templates static
var embedFS embed.FS

func main() {
	// 1. Env / Config
	port := os.Getenv("HOMEPORT_PORT")
	if port == "" {
		port = "8854"
	}
	dbPath := os.Getenv("HOMEPORT_DB")
	if dbPath == "" {
		dbPath = "./data/homeport.db"
	}

	// 2. Init DB
	if err := db.InitDB(dbPath); err != nil {
		log.Fatalf("Failed to init db: %v", err)
	}

	// 3. Init Templates
	handlers.InitTemplates(embedFS)

	// 4. Start Background Tasks
	handlers.StartStatusChecker()
	go runICalFetcher()

	// 5. Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static Files
	// We need to serve "static" folder from embedFS
	// But handlers.InitTemplates used the same embedFS
	// It's safer to use a sub-fs
	staticFS, _ := fs.Sub(embedFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Handle("/sw.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFileFS(w, r, embedFS, "static/sw.js")
	}))

	// Routes
	r.Get("/", handlers.HandleIndex)
	r.Get("/andrea", handlers.HandleIndex)

	r.Get("/status/stream", handlers.HandleStatusStream)

	r.Route("/manage", func(r chi.Router) {
		r.Post("/clone-andrea", handlers.HandleCloneToAndrea)
		r.Post("/widget", handlers.HandleAddWidget)
		r.Delete("/widget/{id}", handlers.HandleDeleteWidget)
		r.Get("/", handlers.HandleManage)
		r.Post("/category", handlers.HandleAddCategory)
		r.Post("/service", handlers.HandleAddService)
		r.Delete("/category/{id}", handlers.HandleDeleteCategory)
		r.Delete("/service/{id}", handlers.HandleDeleteService)
		r.Post("/sort/category/{id}/{direction}", handlers.HandleSortCategory)
		r.Post("/sort/service/{id}/{direction}", handlers.HandleSortService)
	})

	log.Printf("Starting server on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func runICalFetcher() {
	fetchAll := func() {
		widgets, err := db.GetAllWidgets()
		if err != nil {
			log.Printf("iCal fetcher: error loading widgets: %v", err)
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
				log.Printf("iCal fetcher: widget %d: %v", w.ID, err)
				continue
			}
			data, _ := json.Marshal(struct{ Events interface{} }{Events: events})
			if err := db.UpdateWidgetCache(w.ID, string(data)); err != nil {
				log.Printf("iCal fetcher: cache update widget %d: %v", w.ID, err)
			}
		}
	}
	fetchAll()
	for range time.Tick(6 * time.Hour) {
		fetchAll()
	}
}
