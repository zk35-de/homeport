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
	"git.zk35.de/secalpha/homeport/internal/i18n"
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
	"inc":      func(i int) int { return i + 1 },
	"newWidgetRender": func(w db.Widget, lang string) WidgetRenderData {
		return WidgetRenderData{Widget: w, Translator: i18n.NewTranslator(lang)}
	},
}

// Separate template sets per page to avoid {{define "content"}} conflicts.
var IndexTmpl *template.Template
var ManageTmpl *template.Template

func InitTemplates(fs embed.FS) {
	i18n.Load(fs)
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

	// Analytics template
	AnalyticsTmpl, err = template.New("").Funcs(tmplFuncs).ParseFS(fs,
		"templates/base.html",
		"templates/analytics.html",
		"templates/partials/*.html",
	)
	if err != nil {
		log.Fatalf("Error parsing analytics templates: %v", err)
	}

	// Login template (standalone, no base.html)
	LoginTmpl, err = template.New("").Funcs(tmplFuncs).ParseFS(fs, "templates/login.html")
	if err != nil {
		log.Fatalf("Error parsing login template: %v", err)
	}
}

type IndexData struct {
	i18n.Translator
	Categories   []db.Category
	Widgets      []db.Widget
	Pages        []db.Page
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
		Handle404(w, r)
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
	// Populate widget data from cache / DB
	for i := range widgets {
		switch widgets[i].Type {
		case "weather":
			if wc, err := db.GetWeatherCache(widgets[i].ID); err == nil && wc != nil {
				widgets[i].Weather = wc
			}
		case "rss":
			if items, err := db.GetRSSCache(widgets[i].ID); err == nil {
				widgets[i].RSSItems = items
			}
		case "todo":
			if todos, err := db.GetTodos(widgets[i].ID); err == nil {
				widgets[i].Todos = todos
			}
		case "notes":
			if content, err := db.GetNote(widgets[i].ID); err == nil {
				widgets[i].NoteContent = content
			}
		case "caldav":
			if cache, err := db.GetWidgetCache(widgets[i].ID); err == nil && cache != nil {
				widgets[i].Events = cache.Events
			}
		case "github":
			var gh struct {
				PRs    []db.GithubItem `json:"GithubPRs"`
				Issues []db.GithubItem `json:"GithubIssues"`
				User   string          `json:"GithubUser"`
			}
			if err := db.GetWidgetCacheRaw(widgets[i].ID, &gh); err == nil {
				widgets[i].GithubPRs = gh.PRs
				widgets[i].GithubIssues = gh.Issues
				widgets[i].GithubUser = gh.User
			}
		case "router":
			if rc, err := db.GetRouterCache(widgets[i].ID); err == nil && rc != nil {
				widgets[i].RouterStatus = rc
			}
		default:
			if cache, err := db.GetWidgetCache(widgets[i].ID); err == nil && cache != nil {
				widgets[i].Events = cache.Events
			}
		}
	}

	prefs, err := db.GetUserPreferences(profileObj.Slug)
	if err != nil {
		log.Printf("GetUserPreferences(%s): %v", profileObj.Slug, err)
	}
	if prefs == nil {
		prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
	}

	allProfiles, err := db.GetProfiles()
	if err != nil {
		log.Printf("GetProfiles: %v", err)
	}
	pages, err := db.GetPages(profileObj.Slug)
	if err != nil {
		log.Printf("GetPages(%s): %v", profileObj.Slug, err)
	}

	data := IndexData{
		Translator:   i18n.NewTranslator(prefs.Language),
		Categories:   categories,
		Widgets:      widgets,
		Pages:        pages,
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

func Handle404(w http.ResponseWriter, r *http.Request) {
	t := i18n.T("de")
	title := t("404.title")
	back := t("404.back")
	body := []byte(fmt.Sprintf(`<!DOCTYPE html><html lang="de"><head><meta charset="UTF-8"><title>%s – homeport</title>`+
		`<style>body{font-family:sans-serif;display:flex;flex-direction:column;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#0f0f13;color:#e2e8f0}`+
		`h1{font-size:4rem;margin:0;color:#6366f1}p{color:#94a3b8}a{color:#6366f1;text-decoration:none}</style>`+
		`</head><body><h1>404</h1><p>%s</p><p><a href="/">%s</a></p></body></html>`, title, title, back))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(http.StatusNotFound)
	w.Write(body)
}
