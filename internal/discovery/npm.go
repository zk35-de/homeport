package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// NPMSource discovers services from Nginx Proxy Manager.
type NPMSource struct {
	URL   string // e.g. http://npm.local:81
	Token string // Bearer token (identity:secret, fetched on first use)

	// identity/secret for token refresh
	Identity string
	Secret   string

	bearerToken string
	tokenExpiry time.Time
	client      *http.Client
}

func NewNPMSource(url, identity, secret string) *NPMSource {
	return &NPMSource{
		URL:      strings.TrimRight(url, "/"),
		Identity: identity,
		Secret:   secret,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (n *NPMSource) Type() string { return "npm" }

func (n *NPMSource) fetchToken() error {
	body, _ := json.Marshal(map[string]string{
		"identity": n.Identity,
		"secret":   n.Secret,
	})
	resp, err := n.client.Post(n.URL+"/api/tokens", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("npm token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("npm token: unexpected status %d", resp.StatusCode)
	}
	var result struct {
		Token   string `json:"token"`
		Expires string `json:"expires"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("npm token decode: %w", err)
	}
	n.bearerToken = result.Token
	if result.Expires != "" {
		if t, err := time.Parse(time.RFC3339, result.Expires); err == nil {
			n.tokenExpiry = t
		}
	}
	if n.tokenExpiry.IsZero() {
		n.tokenExpiry = time.Now().Add(23 * time.Hour)
	}
	return nil
}

func (n *NPMSource) ensureToken() error {
	if n.bearerToken == "" || time.Now().After(n.tokenExpiry.Add(-5*time.Minute)) {
		return n.fetchToken()
	}
	return nil
}

func (n *NPMSource) Fetch() ([]DiscoveredService, error) {
	if err := n.ensureToken(); err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("GET", n.URL+"/api/nginx/proxy-hosts?expand=certificate,owner", nil)
	req.Header.Set("Authorization", "Bearer "+n.bearerToken)
	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("npm fetch proxy-hosts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm proxy-hosts: status %d", resp.StatusCode)
	}

	var hosts []struct {
		ID          int      `json:"id"`
		DomainNames []string `json:"domain_names"`
		ForwardHost string   `json:"forward_host"`
		ForwardPort int      `json:"forward_port"`
		Meta        struct {
			NginxOnline bool `json:"nginx_online"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&hosts); err != nil {
		return nil, fmt.Errorf("npm decode proxy-hosts: %w", err)
	}

	var services []DiscoveredService
	for _, h := range hosts {
		if len(h.DomainNames) == 0 {
			continue
		}
		domain := h.DomainNames[0]
		serviceURL := "https://" + domain

		services = append(services, DiscoveredService{
			ExternalID:  strconv.Itoa(h.ID),
			Name:        domain,
			URL:         serviceURL,
			Description: fmt.Sprintf("NPM → %s:%d", h.ForwardHost, h.ForwardPort),
		})
	}
	return services, nil
}
