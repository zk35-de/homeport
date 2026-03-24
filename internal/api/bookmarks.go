package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
)

func renderBookmarksWidget(w http.ResponseWriter, widgetID int) {
	widget, err := db.GetWidgetByID(widgetID)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	lang := "de"
	if prefs, err := db.GetUserPreferences(widget.Profile); err == nil && prefs != nil && prefs.Language != "" {
		lang = prefs.Language
	}

	if err := IndexTmpl.ExecuteTemplate(w, "widget_bookmarks", newWidgetRender(widget, lang)); err != nil {
		log.Printf("renderBookmarksWidget error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleAddBookmark adds a link to a bookmarks widget.
// POST /api/widgets/{id}/bookmark  (form: name, url)
func HandleAddBookmark(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	link := db.BookmarkLink{
		Name: r.FormValue("name"),
		URL:  r.FormValue("url"),
	}
	if link.URL == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if link.Name == "" {
		link.Name = link.URL
	}
	if err := db.AddBookmarkLink(id, link); err != nil {
		log.Printf("AddBookmarkLink error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderBookmarksWidget(w, id)
}

// HandleDeleteBookmark removes a link by index from a bookmarks widget.
// DELETE /api/widgets/{id}/bookmark/{idx}
func HandleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	idx, err := strconv.Atoi(chi.URLParam(r, "idx"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := db.DeleteBookmarkLink(id, idx); err != nil {
		log.Printf("DeleteBookmarkLink error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderBookmarksWidget(w, id)
}
