package api

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
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
	ProfileName  string        // NEU: Anzeigename für <title> etc.
	SearchAction string
	Prefs        *db.UserPreferences
	Profiles     []db.Profile  // NEU: für Nav-Links
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	// Slug aus URL-Param (chi) oder URL-Pfad als Fallback (Tests ohne chi-Kontext)
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			slug = path
		}
	}

	var profileObj *db.Profile
	var err error
	if slug == "" {
		profileObj, err = db.GetDefaultProfile()
	} else {
		profileObj, err = db.GetProfileBySlug(slug)
	}
	if err != nil || profileObj == nil {
		http.NotFound(w, r)
		return
	}

	categories, err := db.GetCategoriesWithServices(profileObj.Slug)
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	widgets, err := db.GetWidgets(profileObj.Slug)
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

	prefs, _ := db.GetUserPreferences(profileObj.Slug)
	if prefs == nil {
		prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
	}

	allProfiles, _ := db.GetProfiles()

	data := IndexData{
		Categories:   categories,
		Widgets:      widgets,
		Profile:      profileObj.Slug,
		ProfileName:  profileObj.Name,
		SearchAction: db.GetSearchEngine(profileObj.Slug),
		Prefs:        prefs,
		Profiles:     allProfiles,
	}

	if err := IndexTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
