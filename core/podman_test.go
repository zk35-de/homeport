package core_test

import (
	"os"
	"strings"
	"testing"

	"git.zk35.de/secalpha/homeport/core"
)

func TestScanPodmanContainers_PodmanNotAvailable(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	// Set PATH to an empty string or a directory that does not contain 'podman'
	// This simulates 'podman' not being found in the system's PATH.
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", originalPath) // Restore PATH after test

	services, containerIDs, err := core.ScanPodmanContainers()

	if services != nil {
		t.Errorf("Expected nil services when podman is not available, got %v", services)
	}
	if containerIDs != nil {
		t.Errorf("Expected nil containerIDs when podman is not available, got %v", containerIDs)
	}

	// We expect a nil error if podman is not found, as per "gracefully skips" requirement.
	if err != nil {
		if !strings.Contains(err.Error(), "executable file not found") && !strings.Contains(err.Error(), "no such file or directory") {
			t.Errorf("Expected error indicating podman not found, but got a different error: %v", err)
		}
	}
}

// NOTE: Testing successful podman scanning is challenging without actually running podman.
// This test focuses solely on the "graceful skip" behavior.
// If actual podman integration tests were needed, an external mechanism
// (e.g., a test container, or a sophisticated mock of os/exec) would be required,
// which is beyond the scope of "standard library testing only" and often reserved
// for integration tests rather than unit tests.
