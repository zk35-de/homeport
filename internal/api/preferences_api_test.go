package api

import (
	"testing"

	"github.com/zk35-de/homeport/internal/db"
)

func TestApplyPrefsPatch(t *testing.T) {
	current := &db.UserPreferences{
		Theme:           "dark",
		AccentColor:     "#6366f1",
		AuroraColor:     "#6366f1",
		AuroraIntensity: "medium",
		AuroraAnimated:  false,
	}

	patch := map[string]string{
		"aurora_color":     "#ff0000",
		"aurora_intensity": "vivid",
		"aurora_animated":  "true",
	}

	applyPrefsPatch(current, patch)

	if current.AuroraColor != "#ff0000" {
		t.Errorf("expected aurora_color #ff0000, got %s", current.AuroraColor)
	}
	if current.AuroraIntensity != "vivid" {
		t.Errorf("expected aurora_intensity vivid, got %s", current.AuroraIntensity)
	}
	if !current.AuroraAnimated {
		t.Error("expected aurora_animated true, got false")
	}

	// Test validation
	patch = map[string]string{
		"aurora_color":     "invalid",
		"aurora_intensity": "heavy",
		"aurora_animated":  "0",
	}
	applyPrefsPatch(current, patch)

	if current.AuroraColor != "#ff0000" {
		t.Errorf("expected aurora_color to remain #ff0000, got %s", current.AuroraColor)
	}
	if current.AuroraIntensity != "vivid" {
		t.Errorf("expected aurora_intensity to remain vivid, got %s", current.AuroraIntensity)
	}
	if current.AuroraAnimated {
		t.Error("expected aurora_animated false, got true")
	}
}
