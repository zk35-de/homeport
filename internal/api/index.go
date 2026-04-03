package api

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zk35-de/homeport/internal/db"
	"github.com/zk35-de/homeport/internal/i18n"
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
}

type IndexData struct {
	i18n.Translator
	Categories   []db.Category
	Pages        []db.Page
	Profile      string
	ProfileName  string
	SearchAction string
	Prefs        *db.UserPreferences
	Profiles     []db.Profile
}

func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
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
		slog.Error("fetching categories", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	prefs, err := db.GetUserPreferences(profileObj.Slug)
	if err != nil {
		slog.Error("GetUserPreferences", "profile", profileObj.Slug, "err", err)
	}
	if prefs == nil {
		prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
	}

	allProfiles, err := db.GetProfiles()
	if err != nil {
		slog.Error("GetProfiles", "err", err)
	}
	pages, err := db.GetPages(profileObj.Slug)
	if err != nil {
		slog.Error("GetPages", "profile", profileObj.Slug, "err", err)
	}

	data := IndexData{
		Translator:   i18n.NewTranslator(prefs.Language),
		Categories:   categories,
		Pages:        pages,
		Profile:      profileObj.Slug,
		ProfileName:  profileObj.Name,
		SearchAction: prefs.SearchEngine,
		Prefs:        prefs,
		Profiles:     allProfiles,
	}

	if err := s.IndexTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		slog.Error("executing template", "err", err)
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
