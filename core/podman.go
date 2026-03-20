package core

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"git.zk35.de/secalpha/homeport/internal/db"
)

// podmanContainer represents the structure of a Podman container object from 'podman ps --format json'.
type podmanContainer struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Labels map[string]string `json:"Labels"`
}

// ScanPodmanContainers reads running containers via 'podman ps --format json'
// and returns SuggestedService entries for containers with io.homeport.* labels.
// It gracefully skips if podman is not available.
func ScanPodmanContainers() ([]db.SuggestedService, []string, error) {
	cmd := exec.Command("podman", "ps", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// Check if podman command simply wasn't found
			if strings.Contains(string(exitError.Stderr), "executable file not found") || strings.Contains(err.Error(), "no such file or directory") {
				return nil, nil, nil // Graceful skip: podman not available
			}
		}
		return nil, nil, fmt.Errorf("failed to run 'podman ps': %w", err)
	}

	var containers []podmanContainer
	if err := json.Unmarshal(output, &containers); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal podman output: %w", err)
	}

	var suggestedServices []db.SuggestedService
	var containerIDs []string

	for _, c := range containers {
		// Only consider containers with a homeport name label
		if name, ok := c.Labels["io.homeport.name"]; ok && name != "" {
			service := db.SuggestedService{
				Name: name,
				URL:  c.Labels["io.homeport.url"],
				Icon: c.Labels["io.homeport.icon"],
				Description: c.Labels["io.homeport.description"],
				Category:    c.Labels["io.homeport.category"],
				Profile:     c.Labels["io.homeport.profile"],
				StatusCheck: c.Labels["io.homeport.status_check"],
			}

			// Provide defaults if not specified
			if service.URL == "" {
				// Attempt to derive from name or ID if URL is not provided
				// This is a placeholder, a more robust solution might inspect container ports or network settings
				service.URL = fmt.Sprintf("http://%s", c.Names[0]) // Use container name as a default hostname
			}
			if service.Category == "" {
				service.Category = "Discovered"
			}

			suggestedServices = append(suggestedServices, service)
			containerIDs = append(containerIDs, c.ID)
		}
	}

	return suggestedServices, containerIDs, nil
}
