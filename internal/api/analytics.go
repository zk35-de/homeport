package api

import (
	"html/template"
	"log"
	"net/http"

	"git.zk35.de/secalpha/homeport/internal/db"
)

// AnalyticsTmpl is initialized in InitTemplates.
var AnalyticsTmpl *template.Template

type AnalyticsData struct {
	Stats    []db.ClickStat
	Profile  string
	Profiles []db.Profile
	Prefs    *db.UserPreferences
}

// HandleAnalytics renders the analytics page.
// GET /manage/analytics
func HandleAnalytics(w http.ResponseWriter, r *http.Request) {
	filterProfile := r.URL.Query().Get("profile")
	stats, err := db.GetTopClicks(filterProfile, 25)
	if err != nil {
		log.Printf("GetTopClicks error: %v", err)
		stats = nil
	}
	profiles, _ := db.GetProfiles()
	defaultProf, _ := db.GetDefaultProfile()
	var prefs *db.UserPreferences
	if defaultProf != nil {
		prefs, _ = db.GetUserPreferences(defaultProf.Slug)
	}
	if prefs == nil {
		prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
	}
	data := AnalyticsData{
		Stats:    stats,
		Profile:  filterProfile,
		Profiles: profiles,
		Prefs:    prefs,
	}
	if err := AnalyticsTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Analytics template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
