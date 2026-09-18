package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A config still carrying features.diff_eval must fail to load by name. The
// strict decoder alone would say "unknown field", which an operator reads as a
// typo; the key was removed because the evaluation path behind it was unsound
// (Docs/journeys/impl/S23-differential-path.md), and the message has to say so.
func TestLoadUserConfig_RejectsRemovedDiffEvalKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"provider":"gemini","features":{"diff_eval":true}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadUserConfig(path)
	if err == nil {
		t.Fatal("LoadUserConfig accepted a config with the removed features.diff_eval key")
	}
	if !strings.Contains(err.Error(), "features.diff_eval") {
		t.Errorf("error does not name the key: %v", err)
	}
	if !strings.Contains(err.Error(), "no longer a supported key") {
		t.Errorf("error does not say the key was removed: %v", err)
	}
}

// The rejection must be specific to removed keys: a config with a live
// features block still loads.
func TestLoadUserConfig_AcceptsLiveFeatureKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"provider":"gemini","features":{"provenance":true}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig rejected a live features block: %v", err)
	}
	if cfg.Features == nil || cfg.Features.Provenance == nil || !*cfg.Features.Provenance {
		t.Fatalf("features.provenance did not round-trip: %+v", cfg.Features)
	}
}
