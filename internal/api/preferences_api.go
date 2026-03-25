package api

import (
	"encoding/json"
	"net/http"

	"git.zk35.de/secalpha/homeport/internal/db"
)

// resolvePrefsProfile determines the target profile for preferences endpoints.
// Priority: ?profile= query param → session cookie → default profile.
func resolvePrefsProfile(r *http.Request) string {
	if p := r.URL.Query().Get("profile"); p != "" {
		return p
	}
	if p := SessionProfile(r); p != "" {
		return p
	}
	if def, err := db.GetDefaultProfile(); err == nil && def != nil {
		return def.Slug
	}
	return ""
}

// HandleGetPreferences returns user preferences for a profile.
// GET /api/user/preferences[?profile=slug]
func HandleGetPreferences(w http.ResponseWriter, r *http.Request) {
	profile := resolvePrefsProfile(r)
	if profile == "" {
		http.Error(w, "no profile", http.StatusBadRequest)
		return
	}
	prefs, err := db.GetUserPreferences(profile)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prefs)
}

// HandleSetPreferences partially updates user preferences for a profile.
// PATCH /api/user/preferences[?profile=slug]  (JSON body with optional fields)
func HandleSetPreferences(w http.ResponseWriter, r *http.Request) {
	profile := resolvePrefsProfile(r)
	if profile == "" {
		http.Error(w, "no profile", http.StatusBadRequest)
		return
	}

	// Load current preferences as base
	current, err := db.GetUserPreferences(profile)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Decode partial update using a map so we only change provided fields
	var patch map[string]string
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if v, ok := patch["theme"]; ok {
		current.Theme = v
	}
	if v, ok := patch["accent_color"]; ok {
		current.AccentColor = v
	}
	if v, ok := patch["search_engine"]; ok {
		current.SearchEngine = v
	}

	if v, ok := patch["language"]; ok {
		current.Language = v
	}
	if v, ok := patch["layout"]; ok {
		current.Layout = v
	}
	if v, ok := patch["custom_css"]; ok {
		current.CustomCSS = v
	}

	if err := db.SetUserPreferences(profile, *current); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
