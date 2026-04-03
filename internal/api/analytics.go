package api

import (
	"log/slog"
	"net/http"

	"github.com/zk35-de/homeport/internal/db"
	"github.com/zk35-de/homeport/internal/i18n"
)

type AnalyticsData struct {
	i18n.Translator
	Stats         []db.ClickStat
	Profile       string
	ProfileName   string
	FilterProfile string
	Profiles      []db.Profile
	Prefs         *db.UserPreferences
}

// HandleAnalytics renders the analytics page.
// GET /manage/analytics
func (s *Server) HandleAnalytics(w http.ResponseWriter, r *http.Request) {
	filterProfile := r.URL.Query().Get("profile")
	stats, err := db.GetTopClicks(filterProfile, 25)
	if err != nil {
		slog.Error("GetTopClicks", "err", err)
		stats = nil
	}
	profiles, err := db.GetProfiles()
	if err != nil {
		slog.Error("GetProfiles", "err", err)
	}
	defaultProf, err := db.GetDefaultProfile()
	if err != nil {
		slog.Error("GetDefaultProfile", "err", err)
	}
	var prefs *db.UserPreferences
	if defaultProf != nil {
		if prefs, err = db.GetUserPreferences(defaultProf.Slug); err != nil {
			slog.Error("GetUserPreferences", "profile", defaultProf.Slug, "err", err)
		}
	}
	if prefs == nil {
		prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
	}
	var themeProfile, profileName string
	if defaultProf != nil {
		themeProfile = defaultProf.Slug
		profileName = defaultProf.Name
	}
	data := AnalyticsData{
		Translator:    i18n.NewTranslator(prefs.Language),
		Stats:         stats,
		Profile:       themeProfile,
		ProfileName:   profileName,
		FilterProfile: filterProfile,
		Profiles:      profiles,
		Prefs:         prefs,
	}
	if err := s.AnalyticsTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		slog.Error("analytics template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
