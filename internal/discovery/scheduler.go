package discovery

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/zk35-de/homeport/internal/db"
)

// OnNewDiscovery is called when a new item is added to the inbox.
// Set this to broadcast an SSE event.
var OnNewDiscovery func()

// Scheduler manages per-source goroutines.
type Scheduler struct {
	mu      sync.Mutex
	sources map[int64]chan struct{} // sourceID → stop channel
}

var Global = &Scheduler{sources: make(map[int64]chan struct{})}

// Reload reads discovery_sources from DB and starts/stops goroutines as needed.
func (s *Scheduler) Reload() {
	sources, err := db.GetDiscoverySources()
	if err != nil {
		slog.Error("discovery scheduler: failed to load sources", "err", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop goroutines for removed sources
	active := make(map[int64]bool)
	for _, src := range sources {
		active[int64(src.ID)] = true
	}
	for id, stop := range s.sources {
		if !active[id] {
			close(stop)
			delete(s.sources, id)
		}
	}

	// Start goroutines for new/enabled sources
	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		if _, running := s.sources[int64(src.ID)]; running {
			continue
		}
		stop := make(chan struct{})
		s.sources[int64(src.ID)] = stop
		go s.run(src, stop)
	}
}

// ScanNow triggers an immediate scan for the given source ID (async).
func (s *Scheduler) ScanNow(sourceID int) {
	sources, err := db.GetDiscoverySources()
	if err != nil {
		slog.Error("discovery: ScanNow failed to load sources", "err", err)
		return
	}
	for _, src := range sources {
		if src.ID == sourceID {
			go scanSource(src)
			return
		}
	}
	slog.Warn("discovery: ScanNow: source not found", "id", sourceID)
}

func (s *Scheduler) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, stop := range s.sources {
		close(stop)
		delete(s.sources, id)
	}
}

func (s *Scheduler) run(src db.DiscoverySource, stop chan struct{}) {
	interval := time.Duration(src.Interval) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("discovery: source started", "id", src.ID, "type", src.Type, "name", src.Name, "interval", interval)

	// Scan immediately on start
	scanSource(src)

	for {
		select {
		case <-stop:
			slog.Info("discovery: source stopped", "id", src.ID, "name", src.Name)
			return
		case <-ticker.C:
			scanSource(src)
		}
	}
}

func scanSource(src db.DiscoverySource) {
	var backend Source
	switch src.Type {
	case "npm":
		// token field stores "identity:secret"
		identity, secret, _ := splitIdentitySecret(src.Token)
		backend = NewNPMSource(src.URL, identity, secret)
	case "docker":
		backend = NewDockerSource(src.URL)
	case "traefik":
		backend = NewTraefikSource(src.URL, src.Token)
	default:
		slog.Warn("discovery: unknown source type", "type", src.Type)
		return
	}

	services, err := backend.Fetch()
	if err != nil {
		slog.Error("discovery: fetch failed", "source", src.Name, "err", err)
		return
	}

	newItems := 0
	for _, svc := range services {
		externalID := fmt.Sprintf("%d:%s", src.ID, svc.ExternalID)
		suggested := db.SuggestedService{
			Name:        svc.Name,
			URL:         svc.URL,
			Description: svc.Description,
			Icon:        svc.Icon,
		}
		suggestedJSON, _ := json.Marshal(suggested)
		added, err := db.AddDiscoveryItemExt(externalID, string(suggestedJSON), src.ID)
		if err != nil {
			slog.Error("discovery: failed to add item", "name", svc.Name, "err", err)
			continue
		}
		if added {
			newItems++
		}
	}

	if newItems > 0 {
		slog.Info("discovery: new items found", "source", src.Name, "count", newItems)
		if OnNewDiscovery != nil {
			OnNewDiscovery()
		}
	}
}

func splitIdentitySecret(token string) (identity, secret string, ok bool) {
	for i, c := range token {
		if c == ':' {
			return token[:i], token[i+1:], true
		}
	}
	return token, "", false
}
