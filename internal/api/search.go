package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zk35-de/homeport/internal/db"
)

type SearchResult struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Profile     string `json:"profile"`
}

// HandleSearch searches services by name/url/description.
// GET /api/search?q=text&profile=slug
func HandleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	profile := r.URL.Query().Get("profile")
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	cats, err := db.GetCategoriesWithServices(profile)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	qLower := strings.ToLower(q)
	var results []SearchResult
	for _, cat := range cats {
		for _, svc := range cat.Services {
			if strings.Contains(strings.ToLower(svc.Name), qLower) ||
				strings.Contains(strings.ToLower(svc.URL), qLower) ||
				strings.Contains(strings.ToLower(svc.Description), qLower) {
				results = append(results, SearchResult{
					ID:          svc.ID,
					Name:        svc.Name,
					URL:         svc.URL,
					Icon:        svc.Icon,
					Description: svc.Description,
					Profile:     profile,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
