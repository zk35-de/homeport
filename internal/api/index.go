package api

import (
	"embed"
	"html/template"
	"log"
	"net/http"

	"git.zk35.de/secalpha/homeport/internal/db"
)

// Separate template sets per page to avoid {{define "content"}} conflicts.
var IndexTmpl *template.Template
var ManageTmpl *template.Template

func InitTemplates(fs embed.FS) {
	var err error

	// Index templates: base + index.html + partials
	IndexTmpl, err = template.ParseFS(fs,
		"templates/base.html",
		"templates/index.html",
		"templates/partials/*.html",
	)
	if err != nil {
		log.Fatalf("Error parsing index templates: %v", err)
	}

	// Manage templates: base + manage.html + partials
	ManageTmpl, err = template.ParseFS(fs,
		"templates/base.html",
		"templates/manage.html",
		"templates/partials/*.html",
	)
	if err != nil {
		log.Fatalf("Error parsing manage templates: %v", err)
	}
}

type IndexData struct {
	Categories   []db.Category
	Widgets      []db.Widget
	Profile      string
	SearchAction string
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	profile := "markus"
	if r.URL.Path == "/andrea" {
		profile = "andrea"
	}

	categories, err := db.GetCategoriesWithServices(profile)
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	widgets, err := db.GetWidgets(profile)
	if err != nil {
		log.Printf("Error fetching widgets: %v", err)
		widgets = nil
	}
	// Populate widget data from cache
	for i := range widgets {
		switch widgets[i].Type {
		case "weather":
			if wc, err := db.GetWeatherCache(widgets[i].ID); err == nil && wc != nil {
				widgets[i].Weather = wc
			}
		default:
			if cache, err := db.GetWidgetCache(widgets[i].ID); err == nil && cache != nil {
				widgets[i].Events = cache.Events
			}
		}
	}

	data := IndexData{
		Categories:   categories,
		Widgets:      widgets,
		Profile:      profile,
		SearchAction: db.GetSearchEngine(profile),
	}

	if err := IndexTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
