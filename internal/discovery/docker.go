package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DockerSource discovers services from a Docker daemon via TCP.
type DockerSource struct {
	URL    string // e.g. http://192.168.2.10:2375
	client *http.Client
}

func NewDockerSource(url string) *DockerSource {
	return &DockerSource{
		URL:    strings.TrimRight(url, "/"),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *DockerSource) Type() string { return "docker" }

func (d *DockerSource) Fetch() ([]DiscoveredService, error) {
	resp, err := d.client.Get(d.URL + "/containers/json")
	if err != nil {
		return nil, fmt.Errorf("docker fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker containers/json: status %d", resp.StatusCode)
	}

	var containers []struct {
		ID    string            `json:"Id"`
		Names []string          `json:"Names"`
		Image string            `json:"Image"`
		Ports []struct {
			IP          string `json:"IP"`
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
		} `json:"Ports"`
		Labels map[string]string `json:"Labels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("docker decode: %w", err)
	}

	var services []DiscoveredService
	for _, c := range containers {
		name := c.Labels["homeport.name"]
		if name == "" {
			// Use first container name (strip leading /)
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			} else {
				name = c.Image
			}
		}

		serviceURL := c.Labels["homeport.url"]
		if serviceURL == "" {
			// Find first mapped TCP port
			for _, p := range c.Ports {
				if p.PublicPort > 0 && p.Type == "tcp" {
					host := p.IP
					if host == "" || host == "0.0.0.0" {
						host = strings.Split(strings.TrimPrefix(d.URL, "http://"), ":")[0]
						if host == "" {
							host = "localhost"
						}
					}
					serviceURL = fmt.Sprintf("http://%s:%d", host, p.PublicPort)
					break
				}
			}
		}
		if serviceURL == "" {
			continue // no reachable URL, skip
		}

		services = append(services, DiscoveredService{
			ExternalID:  c.ID[:12],
			Name:        name,
			URL:         serviceURL,
			Description: c.Labels["homeport.description"],
			Icon:        c.Labels["homeport.icon"],
		})
	}
	return services, nil
}
