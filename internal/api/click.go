package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
)

// HandleServiceRedirect tracks a click and redirects to the service URL.
// GET /r/{id}?p={profile}
func HandleServiceRedirect(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	profile := r.URL.Query().Get("p")

	url, err := db.GetServiceURL(id)
	if err != nil || url == "" {
		http.NotFound(w, r)
		return
	}

	// Prevent open redirect to non-http(s) URLs (e.g. javascript:, data:)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		http.NotFound(w, r)
		return
	}

	if profile != "" {
		if err := db.RecordClick(id, profile); err != nil {
			log.Printf("click track error service=%d profile=%s: %v", id, profile, err)
		}
	}

	http.Redirect(w, r, url, http.StatusFound)
}

// HandleSetCategorySortMode sets the sort mode for a category.
// POST /manage/category/{id}/sortmode/{mode}
func HandleSetCategorySortMode(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	mode := chi.URLParam(r, "mode")
	if err := db.SetCategorySortMode(id, mode); err != nil {
		log.Printf("SetCategorySortMode error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Re-render category list
	HandleCategoryList(w, r)
}

// HandleCategoryList renders the category_list partial (shared helper).
func HandleCategoryList(w http.ResponseWriter, r *http.Request) {
	cats, err := db.GetCategoriesWithServices("")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	profiles, err := db.GetProfiles()
	if err != nil {
		log.Printf("GetProfiles: %v", err)
	}
	data := struct {
		Categories []db.Category
		Profiles   []db.Profile
	}{cats, profiles}
	if err := ManageTmpl.ExecuteTemplate(w, "category_list", data); err != nil {
		log.Printf("category_list render error: %v", err)
	}
}
