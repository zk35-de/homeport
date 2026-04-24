package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zk35-de/homeport/internal/db"
)

// HandleProfileThemeCSS serves a profile's accent color + custom CSS as a stylesheet.
// GET /api/profile/{slug}/theme.css
func HandleProfileThemeCSS(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		return
	}

	prefs, err := db.GetUserPreferences(slug)
	if err != nil || prefs == nil {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		return
	}

	var sb strings.Builder
	if prefs.AccentColor != "" {
		rgb := hexToRGB(prefs.AccentColor)
		fmt.Fprintf(&sb, ":root { --accent: %s; --accent-rgb: %s; --accent-hover: %s; }\n",
			prefs.AccentColor, rgb, prefs.AccentColor)
	}
	if prefs.AuroraColor != "" {
		fmt.Fprintf(&sb, ":root { --aurora-color: %s; }\n", prefs.AuroraColor)
	}
	if prefs.CustomCSS != "" {
		sb.WriteString(prefs.CustomCSS)
		sb.WriteString("\n")
	}

	css := sb.String()
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(css)))
	w.Write([]byte(css))
}
