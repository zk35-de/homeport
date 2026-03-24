package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
	"git.zk35.de/secalpha/homeport/internal/discovery"
	"git.zk35.de/secalpha/homeport/internal/i18n"
)

// HandleGetDiscoverySources GET /manage/discovery/sources – returns rendered source list partial
func HandleGetDiscoverySources(w http.ResponseWriter, r *http.Request) {
	renderDiscoverySources(w, r)
}

// HandleAddDiscoverySource POST /manage/discovery/sources
func HandleAddDiscoverySource(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	typ := r.FormValue("type")
	name := r.FormValue("name")
	url := r.FormValue("url")
	token := r.FormValue("token")
	intervalStr := r.FormValue("interval")
	interval, _ := strconv.Atoi(intervalStr)
	if interval <= 0 {
		interval = 60
	}
	if typ == "" || name == "" || url == "" {
		http.Error(w, "type, name, url required", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		http.Error(w, "URL must start with http:// or https://", http.StatusBadRequest)
		return
	}
	if len(token) > 512 {
		http.Error(w, "token too long", http.StatusBadRequest)
		return
	}
	if _, err := db.AddDiscoverySource(typ, name, url, token, interval); err != nil {
		log.Printf("AddDiscoverySource error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	discovery.Global.Reload()
	renderDiscoverySources(w, r)
}

// HandleDeleteDiscoverySource DELETE /manage/discovery/sources/{id}
func HandleDeleteDiscoverySource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := db.DeleteDiscoverySource(id); err != nil {
		log.Printf("DeleteDiscoverySource error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	discovery.Global.Reload()
	renderDiscoverySources(w, r)
}

// HandleToggleDiscoverySource POST /manage/discovery/sources/{id}/toggle
func HandleToggleDiscoverySource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	sources, err := db.GetDiscoverySources()
	if err != nil {
		log.Printf("GetDiscoverySources: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for _, s := range sources {
		if s.ID == id {
			if err := db.SetDiscoverySourceEnabled(id, !s.Enabled); err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			break
		}
	}
	discovery.Global.Reload()
	renderDiscoverySources(w, r)
}

// HandleScanDiscoverySource POST /manage/discovery/sources/{id}/scan – manual immediate scan
func HandleScanDiscoverySource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	sources, err := db.GetDiscoverySources()
	if err != nil {
		log.Printf("GetDiscoverySources: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for _, s := range sources {
		if s.ID == id {
			// Reload triggers an immediate scan via new goroutine restart
			// For a real immediate scan we'd need a trigger channel, but
			// simply stopping+restarting is sufficient for manual use.
			_ = db.SetDiscoverySourceEnabled(id, true)
			discovery.Global.Reload()
			break
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func renderDiscoverySources(w http.ResponseWriter, r *http.Request) {
	sources, err := db.GetDiscoverySources()
	if err != nil {
		log.Printf("GetDiscoverySources: %v", err)
	}
	lang := GetLang(r)
	data := struct {
		Sources []db.DiscoverySource
		T       func(string) string
	}{Sources: sources, T: i18n.T(lang)}
	if err := ManageTmpl.ExecuteTemplate(w, "discovery_sources", data); err != nil {
		log.Printf("discovery_sources template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
