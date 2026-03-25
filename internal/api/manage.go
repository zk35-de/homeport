package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
	"git.zk35.de/secalpha/homeport/internal/i18n"
)

type ManageData struct {
	i18n.Translator
	Categories     []db.Category
	Prefs          *db.UserPreferences
	Profiles       []db.Profile
	Pages          []db.Page
	Profile        string
	ProfileName    string
	DefaultProfile string
}

func HandleManage(w http.ResponseWriter, r *http.Request) {
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		slog.Error("fetching categories", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	profiles, err := db.GetProfiles()
	if err != nil {
		slog.Error("GetProfiles", "err", err)
	}

	var prefs *db.UserPreferences
	var pages []db.Page
	defaultSlug := ""
	profileName := ""
	if def, defErr := db.GetDefaultProfile(); defErr != nil {
		slog.Error("GetDefaultProfile", "err", defErr)
	} else if def != nil {
		defaultSlug = def.Slug
		profileName = def.Name
		if prefs, err = db.GetUserPreferences(def.Slug); err != nil {
			slog.Error("GetUserPreferences", "profile", def.Slug, "err", err)
		}
		if pages, err = db.GetPages(def.Slug); err != nil {
			slog.Error("GetPages", "profile", def.Slug, "err", err)
		}
	}
	if prefs == nil {
		prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
	}

	data := ManageData{
		Translator:     i18n.NewTranslator(prefs.Language),
		Categories:     categories,
		Prefs:          prefs,
		Profiles:       profiles,
		Pages:          pages,
		Profile:        defaultSlug,
		ProfileName:    profileName,
		DefaultProfile: defaultSlug,
	}

	if err := ManageTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		slog.Error("executing template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}


func HandleAddCategory(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	color := r.FormValue("color")

	if _, err := db.AddCategory(name, color); err != nil {
		slog.Error("adding category", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	lang := GetLang(r)
	renderCategoryList(w, lang)
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
	noCheck := r.FormValue("no_check") == "1"
	profiles := r.Form["visibility"]

	if err := db.AddService(categoryID, name, url, icon, desc, statusCheck, noCheck, profiles); err != nil {
		slog.Error("adding service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if _, err := db.GetCategory(id); err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if err := db.DeleteCategory(id); err != nil {
		slog.Error("deleting category", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleDeleteService(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if _, err := db.GetService(id); err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if err := db.DeleteService(id); err != nil {
		slog.Error("deleting service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleGetService(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	svc, err := db.GetService(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	profiles, err := db.GetProfiles()
	if err != nil {
		slog.Error("GetProfiles", "err", err)
	}
	lang := GetLang(r)
	data := struct {
		i18n.Translator
		Service    *db.Service
		Categories []db.Category
		Profiles   []db.Profile
	}{
		Translator: i18n.NewTranslator(lang),
		Service:    svc,
		Categories: categories,
		Profiles:   profiles,
	}

	if err := ManageTmpl.ExecuteTemplate(w, "service_edit_form", data); err != nil {
		slog.Error("executing service_edit_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func HandleUpdateService(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if _, err := db.GetService(id); err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	url := r.FormValue("url")
	icon := r.FormValue("icon")
	desc := r.FormValue("description")
	statusCheck := r.FormValue("status_check")
	noCheck := r.FormValue("no_check") == "1"
	profiles := r.Form["visibility"]

	if err := db.UpdateService(id, name, url, icon, desc, statusCheck, noCheck, profiles); err != nil {
		slog.Error("updating service", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleGetCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	cat, err := db.GetCategory(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	lang := GetLang(r)
	data := struct {
		i18n.Translator
		Category *db.Category
	}{
		Translator: i18n.NewTranslator(lang),
		Category:   cat,
	}

	if err := ManageTmpl.ExecuteTemplate(w, "category_edit_form", data); err != nil {
		slog.Error("executing category_edit_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func HandleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if _, err := db.GetCategory(id); err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	color := r.FormValue("color")

	if err := db.UpdateCategory(id, name, color); err != nil {
		slog.Error("updating category", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleUpdateCategorySpan(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	span, _ := strconv.Atoi(chi.URLParam(r, "span"))
	if err := db.UpdateCategorySpan(id, span); err != nil {
		slog.Error("updating category span", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleReorderCategories(w http.ResponseWriter, r *http.Request) {
	var items []db.ReorderItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := db.ReorderCategories(items); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func HandleReorderServices(w http.ResponseWriter, r *http.Request) {
	var items []db.ReorderItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := db.ReorderServices(items); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func GetLang(r *http.Request) string {
	if def, err := db.GetDefaultProfile(); err == nil && def != nil {
		if prefs, err := db.GetUserPreferences(def.Slug); err == nil && prefs != nil && prefs.Language != "" {
			return prefs.Language
		}
	}
	return "de"
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

	lang := GetLang(r)
	renderCategoryList(w, lang)
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

	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleCloneProfile(w http.ResponseWriter, r *http.Request) {
	srcSlug := chi.URLParam(r, "slug")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	dstName := strings.TrimSpace(r.FormValue("name"))
	if dstName == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	dstSlug := strings.ToLower(strings.ReplaceAll(dstName, " ", "-"))

	// Create destination profile if it doesn't exist
	profiles, _ := db.GetProfiles()
	exists := false
	for _, p := range profiles {
		if p.Slug == dstSlug {
			exists = true
			break
		}
	}
	if !exists {
		if err := db.AddProfile(dstSlug, dstName); err != nil {
			slog.Error("HandleCloneProfile: AddProfile", "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	added, skipped, err := db.CloneServicesToProfile(srcSlug, dstSlug)
	if err != nil {
		slog.Error("HandleCloneProfile: CloneServicesToProfile", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	slog.Info("CloneProfile", "src", srcSlug, "dst", dstSlug, "added", added, "skipped", skipped)
	HandleManage(w, r)
}

func renderCategoryList(w http.ResponseWriter, lang string) {
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		slog.Error("fetching categories", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		i18n.Translator
		Categories []db.Category
	}{
		Translator: i18n.NewTranslator(lang),
		Categories: categories,
	}

	if err := ManageTmpl.ExecuteTemplate(w, "category_list", data); err != nil {
		slog.Error("executing template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleDiscoveryInbox handles the GET request for the discovery inbox partial.
func HandleDiscoveryInbox(w http.ResponseWriter, r *http.Request) {
	lang := GetLang(r)
	renderDiscoveryInbox(w, lang)
}

// HandleAcceptDiscovery handles accepting a discovered service.
func HandleAcceptDiscovery(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := db.AcceptDiscoveryItem(id); err != nil {
		slog.Error("accepting discovery item", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderDiscoveryInbox(w, lang)
}

// HandleIgnoreDiscovery handles ignoring a discovered service.
func HandleIgnoreDiscovery(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := db.IgnoreDiscoveryItem(id); err != nil {
		slog.Error("ignoring discovery item", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderDiscoveryInbox(w, lang)
}

// renderDiscoveryInbox fetches discovery items and renders the discovery_inbox partial.
func renderDiscoveryInbox(w http.ResponseWriter, lang string) {
	items, err := db.GetDiscoveryInbox()
	if err != nil {
		slog.Error("fetching discovery inbox items", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		i18n.Translator
		Items []db.DiscoveryItem
	}{
		Translator: i18n.NewTranslator(lang),
		Items:      items,
	}

	if err := ManageTmpl.ExecuteTemplate(w, "discovery_inbox", data); err != nil {
		slog.Error("executing discovery_inbox template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}


func renderProfileList(w http.ResponseWriter, lang string) {
	profiles, err := db.GetProfiles()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data := struct {
		i18n.Translator
		Profiles []db.Profile
	}{
		Translator: i18n.NewTranslator(lang),
		Profiles:   profiles,
	}
	if err := ManageTmpl.ExecuteTemplate(w, "profile_list", data); err != nil {
		slog.Error("executing profile_list", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func HandleAddProfile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	slug := r.FormValue("slug")
	if name == "" || slug == "" {
		http.Error(w, "name and slug required", http.StatusBadRequest)
		return
	}
	if err := db.AddProfile(name, slug); err != nil {
		slog.Error("adding profile", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderProfileList(w, lang)
}

func HandleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if err := db.DeleteProfile(slug); err != nil {
		slog.Error("deleting profile", "slug", slug, "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	lang := GetLang(r)
	renderProfileList(w, lang)
}

func HandleSetDefaultProfile(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if err := db.SetDefaultProfile(slug); err != nil {
		slog.Error("setting default profile", "slug", slug, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderProfileList(w, lang)
}
