package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// TraefikSource discovers services from a Traefik instance via its REST API.
type TraefikSource struct {
	URL    string // e.g. http://traefik.local:8080
	client *http.Client
	// BasicAuth credentials (optional), stored as "user:password"
	user     string
	password string
}

var hostRe = regexp.MustCompile("Host\\(`([^`]+)`\\)")

func NewTraefikSource(url, token string) *TraefikSource {
	user, password, _ := splitIdentitySecret(token)
	return &TraefikSource{
		URL:      strings.TrimRight(url, "/"),
		user:     user,
		password: password,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TraefikSource) Type() string { return "traefik" }

func (t *TraefikSource) Fetch() ([]DiscoveredService, error) {
	req, err := http.NewRequest("GET", t.URL+"/api/http/routers", nil)
	if err != nil {
		return nil, err
	}
	if t.user != "" {
		req.SetBasicAuth(t.user, t.password)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("traefik fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("traefik routers: status %d", resp.StatusCode)
	}

	var routers []struct {
		Name        string   `json:"name"`
		Rule        string   `json:"rule"`
		EntryPoints []string `json:"entryPoints"`
		Status      string   `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&routers); err != nil {
		return nil, fmt.Errorf("traefik decode: %w", err)
	}

	var services []DiscoveredService
	for _, r := range routers {
		if r.Status == "disabled" {
			continue
		}
		// Extract all Host() values from rule
		matches := hostRe.FindAllStringSubmatch(r.Rule, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			domain := m[1]
			scheme := "http"
			for _, ep := range r.EntryPoints {
				if strings.Contains(ep, "https") || strings.Contains(ep, "websecure") {
					scheme = "https"
					break
				}
			}
			services = append(services, DiscoveredService{
				ExternalID:  r.Name + ":" + domain,
				Name:        domain,
				URL:         scheme + "://" + domain,
				Description: "Traefik → " + r.Name,
			})
		}
	}
	return services, nil
}
