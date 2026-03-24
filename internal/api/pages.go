package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
	"git.zk35.de/secalpha/homeport/internal/i18n"
)

// HandleGetPageList GET /manage/page-list?profile=...
func HandleGetPageList(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	renderPageList(w, r, profile)
}

// HandleAddPage POST /manage/page
func HandleAddPage(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("Error adding page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderPageList(w, r, profile)
}

// HandleDeletePage DELETE /manage/page/{id}
func HandleDeletePage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	profile := r.URL.Query().Get("profile")
	if err := db.DeletePage(id); err != nil {
		log.Printf("Error deleting page %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderPageList(w, r, profile)
}

// HandleUpdatePage PATCH /manage/page/{id}
func HandleUpdatePage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	icon := r.FormValue("icon")
	profile := r.FormValue("profile")
	if err := db.UpdatePage(id, name, icon); err != nil {
		log.Printf("Error updating page %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	renderPageList(w, r, profile)
}

// HandleSortPage POST /manage/sort/page/{id}/{direction}
func HandleSortPage(w http.ResponseWriter, r *http.Request) {
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
	renderPageList(w, r, profile)
}

// HandleSetCategoryPage POST /manage/category/{id}/page/{pageID}
func HandleSetCategoryPage(w http.ResponseWriter, r *http.Request) {
	catID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	pageID, _ := strconv.Atoi(chi.URLParam(r, "pageID"))
	if err := db.SetCategoryPage(catID, pageID); err != nil {
		log.Printf("Error setting category page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func renderPageList(w http.ResponseWriter, r *http.Request, profile string) {
	pages, err := db.GetPages(profile)
	if err != nil {
		log.Printf("Error fetching pages: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	data := struct {
		i18n.Translator
		Pages   []db.Page
		Profile string
	}{Translator: i18n.NewTranslator(lang), Pages: pages, Profile: profile}
	if err := ManageTmpl.ExecuteTemplate(w, "page_list", data); err != nil {
		log.Printf("Error executing page_list template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
