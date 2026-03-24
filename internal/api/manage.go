package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
	"git.zk35.de/secalpha/homeport/internal/i18n"
)

type ManageData struct {
	i18n.Translator
	Categories    []db.Category
	SearchEngines map[string]string
	Prefs         *db.UserPreferences
	Profiles      []db.Profile
	Pages         []db.Page
	Widgets       []db.Widget
	Profile       string // für base.html (.Profile)
	ProfileName   string // für base.html <title>
}

func HandleManage(w http.ResponseWriter, r *http.Request) {
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	profiles, err := db.GetProfiles()
	if err != nil {
		log.Printf("GetProfiles: %v", err)
	}

	var prefs *db.UserPreferences
	var pages []db.Page
	defaultSlug := ""
	profileName := ""
	if def, defErr := db.GetDefaultProfile(); defErr != nil {
		log.Printf("GetDefaultProfile: %v", defErr)
	} else if def != nil {
		defaultSlug = def.Slug
		profileName = def.Name
		if prefs, err = db.GetUserPreferences(def.Slug); err != nil {
			log.Printf("GetUserPreferences(%s): %v", def.Slug, err)
		}
		if pages, err = db.GetPages(def.Slug); err != nil {
			log.Printf("GetPages(%s): %v", def.Slug, err)
		}
	}
	if prefs == nil {
		prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
	}
	widgets, err := db.GetAllWidgets()
	if err != nil {
		log.Printf("GetAllWidgets: %v", err)
	}

	data := ManageData{
		Translator:    i18n.NewTranslator(prefs.Language),
		Categories:    categories,
		SearchEngines: db.GetAllSearchEngines(),
		Prefs:         prefs,
		Profiles:      profiles,
		Pages:         pages,
		Widgets:       widgets,
		Profile:       defaultSlug,
		ProfileName:   profileName,
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
	if prof, _ := db.GetProfileBySlug(profile); prof == nil {
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

	if _, err := db.AddCategory(name, layout, color); err != nil {
		log.Printf("Error adding category: %v", err)
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
	profiles := r.Form["visibility"]

	if err := db.AddService(categoryID, name, url, icon, desc, statusCheck, profiles); err != nil {
		log.Printf("Error adding service: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := db.DeleteCategory(id); err != nil {
		log.Printf("Error deleting category: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleDeleteService(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := db.DeleteService(id); err != nil {
		log.Printf("Error deleting service: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleGetService(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
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
		log.Printf("GetProfiles: %v", err)
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
		log.Printf("Error executing service_edit_form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func HandleUpdateService(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	url := r.FormValue("url")
	icon := r.FormValue("icon")
	desc := r.FormValue("description")
	statusCheck := r.FormValue("status_check")
	profiles := r.Form["visibility"]

	if err := db.UpdateService(id, name, url, icon, desc, statusCheck, profiles); err != nil {
		log.Printf("Error updating service %d: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	lang := GetLang(r)
	renderCategoryList(w, lang)
}

func HandleGetCategory(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
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
		log.Printf("Error executing category_edit_form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func HandleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	layout := r.FormValue("layout")
	color := r.FormValue("color")

	if err := db.UpdateCategory(id, name, layout, color); err != nil {
		log.Printf("Error updating category %d: %v", id, err)
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
		log.Printf("Error updating category span %d: %v", id, err)
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

func HandleAddWidget(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	widgetType := r.FormValue("widget_type")
	profile := r.FormValue("profile")
	if profile == "" {
		profile = "all"
	}

	var err error
	switch widgetType {
	case "clock":
		mode := r.FormValue("clock_mode")
		if mode == "" {
			mode = "digital"
		}
		timezone := r.FormValue("clock_timezone")
		if timezone == "" {
			timezone = "Europe/Berlin"
		}
		countdown := r.FormValue("clock_countdown")
		config := `{"mode":"` + mode + `","timezone":"` + timezone + `","show_date":true,"show_seconds":true,"countdown":"` + countdown + `"}`
		err = db.AddWidgetTyped(name, "clock", config, profile)
	case "weather":
		lat := r.FormValue("weather_lat")
		lon := r.FormValue("weather_lon")
		city := r.FormValue("weather_city")
		config, _ := json.Marshal(map[string]interface{}{"lat": lat, "lon": lon, "city_name": city})
		err = db.AddWidgetTyped(name, "weather", string(config), profile)
	case "rss":
		feedURL := r.FormValue("rss_url")
		maxStr := r.FormValue("rss_max")
		if maxStr == "" {
			maxStr = "10"
		}
		config, _ := json.Marshal(map[string]interface{}{"url": feedURL, "max": maxStr})
		err = db.AddWidgetTyped(name, "rss", string(config), profile)
	case "todo":
		err = db.AddWidgetTyped(name, "todo", `{}`, profile)
	case "bookmarks":
		err = db.AddWidgetTyped(name, "bookmarks", `{"layout":"grid","links":[]}`, profile)
	case "notes":
		err = db.AddWidgetTyped(name, "notes", `{}`, profile)
	case "caldav":
		calURL := r.FormValue("caldav_url")
		calUser := r.FormValue("caldav_username")
		calPass := r.FormValue("caldav_password")
		config, _ := json.Marshal(map[string]string{"url": calURL, "username": calUser, "password": calPass})
		err = db.AddWidgetTyped(name, "caldav", string(config), profile)
	case "github":
		ghToken := r.FormValue("github_token")
		showPRs := r.FormValue("github_show_prs") != ""
		showIssues := r.FormValue("github_show_issues") != ""
		config, _ := json.Marshal(map[string]interface{}{"token": ghToken, "show_prs": showPRs, "show_issues": showIssues})
		err = db.AddWidgetTyped(name, "github", string(config), profile)
	case "router":
		routerType := r.FormValue("router_type")
		if routerType == "" {
			routerType = "fritzbox"
		}
		routerURL := r.FormValue("router_url")
		routerPassword := r.FormValue("router_password")
		config, _ := json.Marshal(map[string]string{
			"router_type":     routerType,
			"router_url":      routerURL,
			"router_password": routerPassword,
		})
		err = db.AddWidgetTyped(name, "router", string(config), profile)
	default:
		// ical (legacy default)
		url := r.FormValue("url")
		err = db.AddWidget(name, url, profile)
	}

	if err != nil {
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

func renderCategoryList(w http.ResponseWriter, lang string) {
	categories, err := db.GetCategoriesWithServices("")
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
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
		log.Printf("Error executing template: %v", err)
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
		log.Printf("Error accepting discovery item %d: %v", id, err)
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
		log.Printf("Error ignoring discovery item %d: %v", id, err)
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
		log.Printf("Error fetching discovery inbox items: %v", err)
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
		log.Printf("Error executing discovery_inbox template: %v", err)
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
		log.Printf("Error executing profile_list: %v", err)
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
		log.Printf("Error adding profile: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderProfileList(w, lang)
}

func HandleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if err := db.DeleteProfile(slug); err != nil {
		log.Printf("Error deleting profile %s: %v", slug, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	lang := GetLang(r)
	renderProfileList(w, lang)
}

func HandleSetDefaultProfile(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if err := db.SetDefaultProfile(slug); err != nil {
		log.Printf("Error setting default profile %s: %v", slug, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	lang := GetLang(r)
	renderProfileList(w, lang)
}
