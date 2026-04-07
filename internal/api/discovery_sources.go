package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zk35-de/homeport/internal/db"
	"github.com/zk35-de/homeport/internal/discovery"
	"github.com/zk35-de/homeport/internal/i18n"
)

// HandleGetDiscoverySources GET /manage/discovery/sources – returns rendered source list partial
func (s *Server) HandleGetDiscoverySources(w http.ResponseWriter, r *http.Request) {
	s.renderDiscoverySources(w, r)
}

// HandleAddDiscoverySource POST /manage/discovery/sources
func (s *Server) HandleAddDiscoverySource(w http.ResponseWriter, r *http.Request) {
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
		slog.Error("AddDiscoverySource", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	discovery.Global.Reload()
	s.renderDiscoverySources(w, r)
}

// HandleDeleteDiscoverySource DELETE /manage/discovery/sources/{id}
func (s *Server) HandleDeleteDiscoverySource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := db.DeleteDiscoverySource(id); err != nil {
		slog.Error("DeleteDiscoverySource", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	discovery.Global.Reload()
	s.renderDiscoverySources(w, r)
}

// HandleToggleDiscoverySource POST /manage/discovery/sources/{id}/toggle
func (s *Server) HandleToggleDiscoverySource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	sources, err := db.GetDiscoverySources()
	if err != nil {
		slog.Error("GetDiscoverySources", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for _, src := range sources {
		if src.ID == id {
			if err := db.SetDiscoverySourceEnabled(id, !src.Enabled); err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			break
		}
	}
	discovery.Global.Reload()
	s.renderDiscoverySources(w, r)
}

// HandleScanDiscoverySource POST /manage/discovery/sources/{id}/scan – manual immediate scan
func (s *Server) HandleScanDiscoverySource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	discovery.Global.ScanNow(id)
	s.renderDiscoverySources(w, r)
}

func (s *Server) renderDiscoverySources(w http.ResponseWriter, r *http.Request) {
	sources, err := db.GetDiscoverySources()
	if err != nil {
		slog.Error("GetDiscoverySources", "err", err)
	}
	lang := GetLang(r)
	data := struct {
		i18n.Translator
		Sources []db.DiscoverySource
	}{Translator: i18n.NewTranslator(lang), Sources: sources}
	if err := s.ManageTmpl.ExecuteTemplate(w, "discovery_sources", data); err != nil {
		slog.Error("discovery_sources template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
