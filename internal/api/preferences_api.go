package api

import (
	"encoding/json"
	"net/http"

	"github.com/zk35-de/homeport/internal/db"
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

// applyPrefsPatch applies a partial JSON patch to a UserPreferences struct.
func applyPrefsPatch(current *db.UserPreferences, patch map[string]string) {
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
	if v, ok := patch["custom_css"]; ok {
		current.CustomCSS = v
	}
	if v, ok := patch["background_mode"]; ok && (v == "aurora" || v == "none") {
		current.BackgroundMode = v
	}
	if v, ok := patch["aurora_color"]; ok {
		// Validation: must #RRGGBB sein (6 hex chars nach #)
		if len(v) == 7 && v[0] == '#' {
			current.AuroraColor = v
		}
	}
	if v, ok := patch["aurora_intensity"]; ok && (v == "subtle" || v == "medium" || v == "vivid") {
		current.AuroraIntensity = v
	}
	if v, ok := patch["aurora_animated"]; ok {
		current.AuroraAnimated = v == "true" || v == "1"
	}
}

// HandleSetPreferences partially updates user preferences for a profile.
// PATCH /api/user/preferences[?profile=slug&all=1]  (JSON body with optional fields)
// With ?all=1 (admin only) the patch is applied to every profile.
func HandleSetPreferences(w http.ResponseWriter, r *http.Request) {
	// Decode patch first so we can reuse it across profiles
	var patch map[string]string
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("all") == "1" {
		// Apply to all profiles – available to any authenticated user on /manage
		profiles, err := db.GetProfiles()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		for _, p := range profiles {
			current, err := db.GetUserPreferences(p.Slug)
			if err != nil {
				continue
			}
			applyPrefsPatch(current, patch)
			db.SetUserPreferences(p.Slug, *current)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	profile := resolvePrefsProfile(r)
	if profile == "" {
		http.Error(w, "no profile", http.StatusBadRequest)
		return
	}
	current, err := db.GetUserPreferences(profile)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	applyPrefsPatch(current, patch)
	if err := db.SetUserPreferences(profile, *current); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
