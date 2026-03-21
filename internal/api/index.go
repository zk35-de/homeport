package api

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"git.zk35.de/secalpha/homeport/internal/db"
)

// isImgURL returns true if the icon value is a URL (should be rendered as <img>).
func isImgURL(s string) bool {
	return strings.HasPrefix(s, "http") || strings.HasPrefix(s, "/api/favicon")
}

// hexToRGB converts "#rrggbb" to "r,g,b" for use in CSS rgb() or rgba().
func hexToRGB(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "99,102,241"
	}
	r, err1 := strconv.ParseInt(hex[0:2], 16, 64)
	g, err2 := strconv.ParseInt(hex[2:4], 16, 64)
	b, err3 := strconv.ParseInt(hex[4:6], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return "99,102,241"
	}
	return fmt.Sprintf("%d,%d,%d", r, g, b)
}

var tmplFuncs = template.FuncMap{
	"isImgURL": isImgURL,
	"hexToRGB": hexToRGB,
}

// Separate template sets per page to avoid {{define "content"}} conflicts.
var IndexTmpl *template.Template
var ManageTmpl *template.Template

func InitTemplates(fs embed.FS) {
	var err error

	// Index templates: base + index.html + partials
	IndexTmpl, err = template.New("").Funcs(tmplFuncs).ParseFS(fs,
		"templates/base.html",
		"templates/index.html",
		"templates/partials/*.html",
	)
	if err != nil {
		log.Fatalf("Error parsing index templates: %v", err)
	}

	// Manage templates: base + manage.html + partials
	ManageTmpl, err = template.New("").Funcs(tmplFuncs).ParseFS(fs,
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
	Prefs        *db.UserPreferences
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

	prefs, _ := db.GetUserPreferences(profile)
	if prefs == nil {
		prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
	}

	data := IndexData{
		Categories:   categories,
		Widgets:      widgets,
		Profile:      profile,
		SearchAction: db.GetSearchEngine(profile),
		Prefs:        prefs,
	}

	if err := IndexTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
