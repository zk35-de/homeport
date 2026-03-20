package api

import (
	"net/http"
	"strings"
)

// AuthMiddleware enforces Bearer token authentication on all routes except
// the whitelist (/, /andrea, /api/health, /static/*, /sw.js).
func AuthMiddleware(token string) func(http.Handler) http.Handler {
	whitelist := []string{
		"/",
		"/andrea",
		"/api/health",
		"/sw.js",
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Exact whitelist matches
			for _, p := range whitelist {
				if path == p {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Prefix whitelist
			if strings.HasPrefix(path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			// Validate Bearer token
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
