package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/zk35-de/homeport/internal/db"
	"github.com/zk35-de/homeport/internal/i18n"
)

// HandleGetPageList GET /manage/page-list?profile=...
func (s *Server) HandleGetPageList(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	s.renderPageList(w, r, profile)
}

// HandleAddPage POST /manage/page
func (s *Server) HandleAddPage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	profile := r.FormValue("profile")
	name := r.FormValue("name")
	icon := r.FormValue("icon")
	if icon == "" {
		icon = "📄"
	}
	if profile == "" || name == "" {
		http.Error(w, "profile and name required", http.StatusBadRequest)
		return
	}
	if _, err := db.AddPage(profile, name, icon); err != nil {
		slog.Error("adding page", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	s.renderPageList(w, r, profile)
}

// HandleDeletePage DELETE /manage/page/{id}
func (s *Server) HandleDeletePage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	profile := r.URL.Query().Get("profile")
	if profile == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	// IDOR fix: verify page belongs to the requested profile before deleting
	page, err := db.GetPage(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if page.Profile != profile {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := db.DeletePage(id); err != nil {
		slog.Error("deleting page", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	s.renderPageList(w, r, profile)
}

// HandleUpdatePage PATCH /manage/page/{id}
func (s *Server) HandleUpdatePage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	icon := r.FormValue("icon")
	profile := r.FormValue("profile")
	if profile == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	// IDOR fix: verify page belongs to the requested profile before updating
	page, err := db.GetPage(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if page.Profile != profile {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := db.UpdatePage(id, name, icon); err != nil {
		slog.Error("updating page", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	s.renderPageList(w, r, profile)
}

// HandleSortPage POST /manage/sort/page/{id}/{direction}
func (s *Server) HandleSortPage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	direction := chi.URLParam(r, "direction")
	profile := r.URL.Query().Get("profile")

	pages, err := db.GetPages(profile)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for i, p := range pages {
		if p.ID == id {
			if direction == "up" && i > 0 {
				prev := pages[i-1]
				db.UpdatePageSort(p.ID, prev.SortOrder)
				db.UpdatePageSort(prev.ID, p.SortOrder)
			} else if direction == "down" && i < len(pages)-1 {
				next := pages[i+1]
				db.UpdatePageSort(p.ID, next.SortOrder)
				db.UpdatePageSort(next.ID, p.SortOrder)
			}
			break
		}
	}
	s.renderPageList(w, r, profile)
}

// HandleSetCategoryPage POST /manage/category/{id}/page/{pageID}
func (s *Server) HandleSetCategoryPage(w http.ResponseWriter, r *http.Request) {
	catID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	pageID, _ := strconv.Atoi(chi.URLParam(r, "pageID"))
	if err := db.SetCategoryPage(catID, pageID); err != nil {
		slog.Error("setting category page", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	s.renderCategoryList(w, lang)
}

func (s *Server) renderPageList(w http.ResponseWriter, r *http.Request, profile string) {
	pages, err := db.GetPages(profile)
	if err != nil {
		slog.Error("fetching pages", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	data := struct {
		i18n.Translator
		Pages   []db.Page
		Profile string
	}{Translator: i18n.NewTranslator(lang), Pages: pages, Profile: profile}
	if err := s.ManageTmpl.ExecuteTemplate(w, "page_list", data); err != nil {
		slog.Error("executing page_list template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
