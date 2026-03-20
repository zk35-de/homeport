package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/db"
)

type ManageData struct {
	Categories    []db.Category
	SearchEngines map[string]string
}

func HandleManage(w http.ResponseWriter, r *http.Request) {
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := ManageData{
		Categories:    categories,
		SearchEngines: db.GetAllSearchEngines(),
	}

	if err := ManageTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func HandleSetSearchEngine(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	profile := r.FormValue("profile")
	engine := r.FormValue("engine")
	if profile != "markus" && profile != "andrea" {
		http.Error(w, "Invalid profile", http.StatusBadRequest)
		return
	}
	if engine == "" {
		http.Error(w, "Missing engine", http.StatusBadRequest)
		return
	}
	if err := db.SetSearchEngine(profile, engine); err != nil {
		log.Printf("Error setting search engine: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func HandleAddCategory(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	layout := r.FormValue("layout")
	color := r.FormValue("color")

	if err := db.AddCategory(name, layout, color); err != nil {
		log.Printf("Error adding category: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	renderCategoryList(w)
}

func HandleAddService(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	categoryID, _ := strconv.Atoi(r.FormValue("category_id"))
	name := r.FormValue("name")
	url := r.FormValue("url")
	icon := r.FormValue("icon")
	desc := r.FormValue("description")
	statusCheck := r.FormValue("status_check")
	profiles := r.Form["visibility"]

	if err := db.AddService(categoryID, name, url, icon, desc, statusCheck, profiles); err != nil {
		log.Printf("Error adding service: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	renderCategoryList(w)
}

func HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := db.DeleteCategory(id); err != nil {
		log.Printf("Error deleting category: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderCategoryList(w)
}

func HandleDeleteService(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := db.DeleteService(id); err != nil {
		log.Printf("Error deleting service: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderCategoryList(w)
}

func HandleSortCategory(w http.ResponseWriter, r *http.Request) {
	// Simple swap logic or full reorder logic.
	// For simplicity, we assume we just swap with adjacent.
	// But "Up/Down" requires knowing the adjacent ID or using sort_order.
	// Better approach: Get all categories, find index, swap sort_order with prev/next, update both.
	
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	direction := chi.URLParam(r, "direction")

	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	for i, c := range categories {
		if c.ID == id {
			if direction == "up" && i > 0 {
				prev := categories[i-1]
				db.UpdateCategorySort(c.ID, prev.SortOrder)
				db.UpdateCategorySort(prev.ID, c.SortOrder)
			} else if direction == "down" && i < len(categories)-1 {
				next := categories[i+1]
				db.UpdateCategorySort(c.ID, next.SortOrder)
				db.UpdateCategorySort(next.ID, c.SortOrder)
			}
			break
		}
	}

	renderCategoryList(w)
}

func HandleSortService(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	direction := chi.URLParam(r, "direction")

	// Need to find service and its siblings.
	// We can fetch all categories (which include services sorted) and find the service.
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	found := false
	for _, c := range categories {
		for i, s := range c.Services {
			if s.ID == id {
				if direction == "up" && i > 0 {
					prev := c.Services[i-1]
					db.UpdateServiceSort(s.ID, prev.SortOrder)
					db.UpdateServiceSort(prev.ID, s.SortOrder)
				} else if direction == "down" && i < len(c.Services)-1 {
					next := c.Services[i+1]
					db.UpdateServiceSort(s.ID, next.SortOrder)
					db.UpdateServiceSort(next.ID, s.SortOrder)
				}
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	renderCategoryList(w)
}

func HandleAddWidget(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	url := r.FormValue("url")
	profile := r.FormValue("profile")
	if profile == "" {
		profile = "all"
	}
	if err := db.AddWidget(name, url, profile); err != nil {
		log.Printf("Error adding widget: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func HandleDeleteWidget(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := db.DeleteWidget(id); err != nil {
		log.Printf("Error deleting widget: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func HandleCloneToAndrea(w http.ResponseWriter, r *http.Request) {
	added, skipped, err := db.CloneToAndrea()
	if err != nil {
		log.Printf("Error cloning services to Andrea: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	response := struct {
		Added   int `json:"added"`
		Skipped int `json:"skipped"`
	}{
		Added:   added,
		Skipped: skipped,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func renderCategoryList(w http.ResponseWriter) {
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Categories []db.Category
	}{
		Categories: categories,
	}

	if err := ManageTmpl.ExecuteTemplate(w, "category_list", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleDiscoveryInbox handles the GET request for the discovery inbox partial.
func HandleDiscoveryInbox(w http.ResponseWriter, r *http.Request) {
	renderDiscoveryInbox(w)
}

// HandleAcceptDiscovery handles accepting a discovered service.
func HandleAcceptDiscovery(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := db.AcceptDiscoveryItem(id); err != nil {
		log.Printf("Error accepting discovery item %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderDiscoveryInbox(w)
}

// HandleIgnoreDiscovery handles ignoring a discovered service.
func HandleIgnoreDiscovery(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := db.IgnoreDiscoveryItem(id); err != nil {
		log.Printf("Error ignoring discovery item %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderDiscoveryInbox(w)
}

// renderDiscoveryInbox fetches discovery items and renders the discovery_inbox partial.
func renderDiscoveryInbox(w http.ResponseWriter) {
	items, err := db.GetDiscoveryInbox()
	if err != nil {
		log.Printf("Error fetching discovery inbox items: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Items []db.DiscoveryItem
	}{
		Items: items,
	}

	if err := ManageTmpl.ExecuteTemplate(w, "discovery_inbox", data); err != nil {
		log.Printf("Error executing discovery_inbox template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

