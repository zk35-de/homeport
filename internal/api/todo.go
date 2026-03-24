package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
)

// renderTodoWidget renders the widget_todo template for a given widget and returns it.
func renderTodoWidget(w http.ResponseWriter, widgetID int) {
	widget, err := db.GetWidgetByID(widgetID)
	if err != nil {
		log.Printf("Error fetching widget %d: %v", widgetID, err)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	todos, err := db.GetTodos(widgetID)
	if err != nil {
		log.Printf("Error fetching todos for widget %d: %v", widgetID, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	widget.Todos = todos

	lang := "de"
	if prefs, err := db.GetUserPreferences(widget.Profile); err == nil && prefs != nil && prefs.Language != "" {
		lang = prefs.Language
	}

	if err := IndexTmpl.ExecuteTemplate(w, "widget_todo", newWidgetRender(widget, lang)); err != nil {
		log.Printf("Error rendering todo widget: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getWidgetNameByID is now redundant for re-rendering since we fetch the full widget in renderTodoWidget.
// Keeping it if needed elsewhere, but updating callers of renderTodoWidget.

func HandleAddTodo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	widgetID, err := strconv.Atoi(r.FormValue("widget_id"))
	if err != nil || widgetID == 0 {
		http.Error(w, "invalid widget_id", http.StatusBadRequest)
		return
	}
	text := r.FormValue("text")
	if text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}
	dueDate := r.FormValue("due_date")
	if _, err := db.AddTodo(widgetID, text, dueDate); err != nil {
		log.Printf("Error adding todo: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderTodoWidget(w, widgetID)
}

func HandleToggleTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	widgetID, err := db.GetTodoWidgetID(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := db.ToggleTodo(id); err != nil {
		log.Printf("Error toggling todo %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderTodoWidget(w, widgetID)
}

func HandleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	widgetID, err := db.GetTodoWidgetID(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := db.DeleteTodo(id); err != nil {
		log.Printf("Error deleting todo %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderTodoWidget(w, widgetID)
}
