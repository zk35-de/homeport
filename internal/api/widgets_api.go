package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
)

// HandleGetWidgets returns all widgets, optionally filtered by profile.
// GET /api/widgets?profile=markus
func HandleGetWidgets(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	var widgets []db.Widget
	var err error
	if profile != "" {
		widgets, err = db.GetWidgets(profile)
	} else {
		widgets, err = db.GetAllWidgets()
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if widgets == nil {
		widgets = []db.Widget{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(widgets)
}

// HandleCreateWidget creates a new widget.
// POST /api/widgets  (JSON body: {name, url, profile})
func HandleCreateWidget(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		Profile string `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if body.Profile == "" {
		body.Profile = "all"
	}
	if err := db.AddWidget(body.Name, body.URL, body.Profile); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// HandleGetWidget returns a single widget by ID.
// GET /api/widgets/{id}
func HandleGetWidget(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	widgets, err := db.GetAllWidgets()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for _, widget := range widgets {
		if widget.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(widget)
			return
		}
	}
	http.Error(w, "Not Found", http.StatusNotFound)
}

// HandleUpdateWidget partially updates a widget.
// PATCH /api/widgets/{id}  (JSON body with optional fields)
func HandleUpdateWidget(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	var body struct {
		Name    *string `json:"name"`
		Config  *string `json:"config"`
		Profile *string `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := db.UpdateWidget(id, body.Name, body.Config, body.Profile); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteWidgetAPI deletes a widget by ID.
// DELETE /api/widgets/{id}
func HandleDeleteWidgetAPI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := db.DeleteWidget(id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReorderItem is used in the reorder request body.
type ReorderItem struct {
	ID        int `json:"id"`
	SortOrder int `json:"sort_order"`
}

// HandleReorderWidgets updates sort_order for multiple widgets.
// PATCH /api/widgets/reorder  (JSON body: [{id:1,sort_order:2},...])
func HandleReorderWidgets(w http.ResponseWriter, r *http.Request) {
	var items []ReorderItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	for _, item := range items {
		if err := db.UpdateWidgetSort(item.ID, item.SortOrder); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
