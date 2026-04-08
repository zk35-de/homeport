package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zk35-de/homeport/internal/db"
	"github.com/zk35-de/homeport/internal/i18n"
)

type ManageData struct {
	i18n.Translator
	Categories     []db.Category
	DiscoveryItems []db.DiscoveryItem
	Prefs          *db.UserPreferences
	Profiles       []db.Profile
	Pages          []db.Page
	Profile        string
	ProfileName    string
	DefaultProfile string
	CSRFToken      string
	IsAdmin        bool
	ProfileColors  map[string]string // slug → aurora_color
}

// sessionContext returns the session profile slug and whether that profile is admin.
// No-auth mode: slug="" → treated as admin.
func sessionContext(r *http.Request) (slug string, isAdmin bool) {
	slug = SessionProfile(r)
	if slug == "" {
		return "", true // no-auth = admin
	}
	if a, _ := db.GetUserAuth(slug); a != nil {
		return slug, a.IsAdmin
	}
	return slug, false
}

// serviceVisibleToProfile reports whether the service with the given ID is visible to the profile.
func serviceVisibleToProfile(serviceID int, profile string) bool {
	svc, err := db.GetService(serviceID)
	if err != nil {
		return false
	}
	for _, p := range svc.VisibleTo {
		if p == profile {
			return true
		}
	}
	return false
}

func (s *Server) HandleManage(w http.ResponseWriter, r *http.Request) {
	// Determine which profile to display. Admins see the global view (all
	// services, default profile context). Non-admin users see only their own
	// profile's services.
	sessionSlug, isAdmin := sessionContext(r)

	filterProfile := ""    // empty = show all services (admin/no-auth view)
	contextSlug := ""      // profile used for prefs, pages, UI context
	if sessionSlug != "" && !isAdmin {
		filterProfile = sessionSlug
		contextSlug = sessionSlug
	}

	categories, err := db.GetCategoriesWithServices(filterProfile)
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

	// Admin / no-auth: use default profile for UI context.
	if contextSlug == "" {
		if def, defErr := db.GetDefaultProfile(); defErr != nil {
			slog.Error("GetDefaultProfile", "err", defErr)
		} else if def != nil {
			contextSlug = def.Slug
			defaultSlug = def.Slug
		}
	} else {
		// Non-admin: context is the session profile; still need the default slug.
		if def, defErr := db.GetDefaultProfile(); defErr == nil && def != nil {
			defaultSlug = def.Slug
		}
	}

	if contextSlug != "" {
		if p, pErr := db.GetProfileBySlug(contextSlug); pErr == nil && p != nil {
			profileName = p.Name
		}
		if prefs, err = db.GetUserPreferences(contextSlug); err != nil {
			slog.Error("GetUserPreferences", "profile", contextSlug, "err", err)
		}
		if pages, err = db.GetPages(contextSlug); err != nil {
			slog.Error("GetPages", "profile", contextSlug, "err", err)
		}
	}
	if prefs == nil {
		prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
	}

	discoveryItems, err := db.GetDiscoveryInbox()
	if err != nil {
		slog.Error("fetching discovery inbox", "err", err)
		discoveryItems = nil
	}

	profileColors := make(map[string]string, len(profiles))
	for _, p := range profiles {
		if pp, err := db.GetUserPreferences(p.Slug); err == nil {
			profileColors[p.Slug] = pp.AuroraColor
		} else {
			profileColors[p.Slug] = "#6366f1"
		}
	}

	data := ManageData{
		Translator:     i18n.NewTranslator(prefs.Language),
		Categories:     categories,
		DiscoveryItems: discoveryItems,
		Prefs:          prefs,
		Profiles:       profiles,
		Pages:          pages,
		Profile:        contextSlug,
		ProfileName:    profileName,
		DefaultProfile: defaultSlug,
		CSRFToken:      CSRFToken(r),
		IsAdmin:        isAdmin || sessionSlug == "", // no-auth mode = everyone is admin
		ProfileColors:  profileColors,
	}

	if err := s.ManageTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		slog.Error("executing template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) HandleAddCategory(w http.ResponseWriter, r *http.Request) {
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

	// Fire categoryAdded so #cat-select reloads its options via hx-trigger.
	// HTMX 2.x dispatches HX-Trigger events without eval() → CSP-safe.
	w.Header().Set("HX-Trigger", "categoryAdded")
	lang := GetLang(r)
	s.renderCategoryList(w, lang)
}

func (s *Server) HandleAddService(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	categoryID, _ := strconv.Atoi(r.FormValue("category_id"))
	if categoryID <= 0 {
		http.Error(w, "category required", http.StatusBadRequest)
		return
	}
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
	s.renderCategoryList(w, lang)
}

func (s *Server) HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if _, isAdmin := sessionContext(r); !isAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
	w.Header().Set("HX-Trigger", "categoryAdded")
	lang := GetLang(r)
	s.renderCategoryList(w, lang)
}

func (s *Server) HandleDeleteService(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	slug, isAdmin := sessionContext(r)
	if !isAdmin && !serviceVisibleToProfile(id, slug) {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
	s.renderCategoryList(w, lang)
}

func (s *Server) HandleGetService(w http.ResponseWriter, r *http.Request) {
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

	if err := s.ManageTmpl.ExecuteTemplate(w, "service_edit_form", data); err != nil {
		slog.Error("executing service_edit_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) HandleUpdateService(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	slug, isAdmin := sessionContext(r)
	if !isAdmin && !serviceVisibleToProfile(id, slug) {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
	s.renderCategoryList(w, lang)
}

func (s *Server) HandleGetCategory(w http.ResponseWriter, r *http.Request) {
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

	if err := s.ManageTmpl.ExecuteTemplate(w, "category_edit_form", data); err != nil {
		slog.Error("executing category_edit_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) HandleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if _, isAdmin := sessionContext(r); !isAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
	s.renderCategoryList(w, lang)
}

func (s *Server) HandleUpdateCategorySpan(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	span, _ := strconv.Atoi(chi.URLParam(r, "span"))
	if err := db.UpdateCategorySpan(id, span); err != nil {
		slog.Error("updating category span", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	s.renderCategoryList(w, lang)
}

func (s *Server) HandleReorderCategories(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) HandleReorderServices(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) HandleSortCategory(w http.ResponseWriter, r *http.Request) {
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
	s.renderCategoryList(w, lang)
}

func (s *Server) HandleSortService(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	direction := chi.URLParam(r, "direction")

	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	found := false
	for _, c := range categories {
		for i, sv := range c.Services {
			if sv.ID == id {
				if direction == "up" && i > 0 {
					prev := c.Services[i-1]
					db.UpdateServiceSort(sv.ID, prev.SortOrder)
					db.UpdateServiceSort(prev.ID, sv.SortOrder)
				} else if direction == "down" && i < len(c.Services)-1 {
					next := c.Services[i+1]
					db.UpdateServiceSort(sv.ID, next.SortOrder)
					db.UpdateServiceSort(next.ID, sv.SortOrder)
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
	s.renderCategoryList(w, lang)
}

func (s *Server) HandleCloneProfile(w http.ResponseWriter, r *http.Request) {
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
	s.HandleManage(w, r)
}

// HandleCategoryOptions returns <option> elements for all categories.
// Used by HTMX to refresh the category dropdown in the service-add form.
// GET /manage/category-options
func (s *Server) HandleCategoryOptions(w http.ResponseWriter, r *http.Request) {
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for _, c := range categories {
		fmt.Fprintf(w, `<option value="%d">%s</option>`, c.ID, c.Name)
	}
}

// HandleProfileOptions returns <option> elements for all profiles.
// Used by HTMX to refresh the profile dropdown in the auth password form.
// GET /manage/profile-options
func (s *Server) HandleProfileOptions(w http.ResponseWriter, r *http.Request) {
	profiles, err := db.GetProfiles()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for _, p := range profiles {
		fmt.Fprintf(w, `<option value="%s">%s</option>`, p.Slug, p.Name)
	}
}

func (s *Server) renderCategoryList(w http.ResponseWriter, lang string) {
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		slog.Error("fetching categories", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	discoveryItems, err := db.GetDiscoveryInbox()
	if err != nil {
		slog.Error("fetching discovery inbox for category list", "err", err)
		discoveryItems = nil
	}

	data := struct {
		i18n.Translator
		Categories     []db.Category
		DiscoveryItems []db.DiscoveryItem
	}{
		Translator:     i18n.NewTranslator(lang),
		Categories:     categories,
		DiscoveryItems: discoveryItems,
	}

	if err := s.ManageTmpl.ExecuteTemplate(w, "category_list", data); err != nil {
		slog.Error("executing template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// HandleGetCategoryVisibility returns the inline visibility form for a category.
func (s *Server) HandleGetCategoryVisibility(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	profiles, err := db.GetProfiles()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	data := struct {
		i18n.Translator
		CategoryID int
		Profiles   []db.Profile
	}{
		Translator: i18n.NewTranslator(lang),
		CategoryID: id,
		Profiles:   profiles,
	}
	if err := s.ManageTmpl.ExecuteTemplate(w, "category_visibility_form", data); err != nil {
		slog.Error("executing category_visibility_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleSetCategoryVisibility applies the selected profiles to all services in a category.
func (s *Server) HandleSetCategoryVisibility(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	profiles := r.Form["profiles"]
	if err := db.SetCategoryVisibility(id, profiles); err != nil {
		slog.Error("SetCategoryVisibility", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	s.renderCategoryList(w, lang)
}

// HandleAcceptDiscoveryCL accepts a discovery item and re-renders the category list (incl. discovery section).
func (s *Server) HandleAcceptDiscoveryCL(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	categoryID, _ := strconv.Atoi(r.FormValue("category_id"))
	noCheck := r.FormValue("no_check") == "1"

	if err := db.AcceptDiscoveryItem(id, categoryID, noCheck); err != nil {
		slog.Error("accepting discovery item (CL)", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	s.renderCategoryList(w, lang)
}

// HandleIgnoreDiscoveryCL ignores a discovery item and re-renders the category list.
func (s *Server) HandleIgnoreDiscoveryCL(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := db.IgnoreDiscoveryItem(id); err != nil {
		slog.Error("ignoring discovery item (CL)", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	s.renderCategoryList(w, lang)
}

// HandleDiscoveryInbox handles the GET request for the discovery inbox partial.
func (s *Server) HandleDiscoveryInbox(w http.ResponseWriter, r *http.Request) {
	lang := GetLang(r)
	s.renderDiscoveryInbox(w, lang)
}

// HandleAcceptDiscovery handles accepting a discovered service.
func (s *Server) HandleAcceptDiscovery(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	categoryID, _ := strconv.Atoi(r.FormValue("category_id"))
	noCheck := r.FormValue("no_check") == "1"

	if err := db.AcceptDiscoveryItem(id, categoryID, noCheck); err != nil {
		slog.Error("accepting discovery item", "id", id, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	s.renderDiscoveryInbox(w, lang)
}

// HandleIgnoreDiscovery handles ignoring a discovered service.
func (s *Server) HandleIgnoreDiscovery(w http.ResponseWriter, r *http.Request) {
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
	s.renderDiscoveryInbox(w, lang)
}

// renderDiscoveryInbox fetches discovery items and renders the discovery_inbox partial.
func (s *Server) renderDiscoveryInbox(w http.ResponseWriter, lang string) {
	items, err := db.GetDiscoveryInbox()
	if err != nil {
		slog.Error("fetching discovery inbox items", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		slog.Error("fetching categories for discovery inbox", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		i18n.Translator
		Items      []db.DiscoveryItem
		Categories []db.Category
	}{
		Translator: i18n.NewTranslator(lang),
		Items:      items,
		Categories: categories,
	}

	if err := s.ManageTmpl.ExecuteTemplate(w, "discovery_inbox", data); err != nil {
		slog.Error("executing discovery_inbox template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) renderProfileList(w http.ResponseWriter, lang string) {
	profiles, err := db.GetProfiles()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	profileColors := make(map[string]string, len(profiles))
	for _, p := range profiles {
		if pp, err := db.GetUserPreferences(p.Slug); err == nil {
			profileColors[p.Slug] = pp.AuroraColor
		} else {
			profileColors[p.Slug] = "#6366f1"
		}
	}
	data := struct {
		i18n.Translator
		Profiles      []db.Profile
		ProfileColors map[string]string
	}{
		Translator:    i18n.NewTranslator(lang),
		Profiles:      profiles,
		ProfileColors: profileColors,
	}
	if err := s.ManageTmpl.ExecuteTemplate(w, "profile_list", data); err != nil {
		slog.Error("executing profile_list", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) HandleAddProfile(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("HX-Trigger", "profileChanged")
	lang := GetLang(r)
	s.renderProfileList(w, lang)
}

func (s *Server) HandleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if err := db.DeleteProfile(slug); err != nil {
		slog.Error("deleting profile", "slug", slug, "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("HX-Trigger", "profileChanged")
	lang := GetLang(r)
	s.renderProfileList(w, lang)
}

func (s *Server) HandleSetDefaultProfile(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if err := db.SetDefaultProfile(slug); err != nil {
		slog.Error("setting default profile", "slug", slug, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	s.renderProfileList(w, lang)
}
