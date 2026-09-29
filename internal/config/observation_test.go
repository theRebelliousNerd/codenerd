package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The observation bounds are tunables owned by .nerd/config.json. These pin
// their defaults, the Get overlay and the Check errors; the call-site test
// (VirtualStore.handleReadFile) lives in internal/core.
func TestDefaultObservationConfig(t *testing.T) {
	got := DefaultObservationConfig()
	if got.MaxRegionLines != 400 || got.PadLines != 8 ||
		got.MaxOutline != 60 || got.MaxRegionBytes != 24<<10 {
		t.Errorf("defaults = %+v", got)
	}
}

func TestGetObservationConfig_Overlay(t *testing.T) {
	var nilCfg *UserConfig
	if got := nilCfg.GetObservationConfig(); got != DefaultObservationConfig() {
		t.Errorf("nil config = %+v, want defaults", got)
	}
	if got := (&UserConfig{}).GetObservationConfig(); got != DefaultObservationConfig() {
		t.Errorf("absent section = %+v, want defaults", got)
	}
	cfg := &UserConfig{Observation: &ObservationConfig{
		MaxRegionLines: 11, PadLines: 3, MaxOutline: 7, MaxRegionBytes: 1024,
	}}
	if got := cfg.GetObservationConfig(); got != *cfg.Observation {
		t.Errorf("set section = %+v, want the set values", got)
	}
}

func TestObservationConfig_Check(t *testing.T) {
	if p := DefaultObservationConfig().Check("observation"); len(p) != 0 {
		t.Errorf("defaults fail their own check: %+v", p)
	}
	cases := []struct {
		name string
		set  func(*ObservationConfig)
		path string
	}{
		{"negative lines", func(c *ObservationConfig) { c.MaxRegionLines = -1 }, "observation.max_region_lines"},
		{"negative pad", func(c *ObservationConfig) { c.PadLines = -1 }, "observation.pad_lines"},
		{"negative outline", func(c *ObservationConfig) { c.MaxOutline = -1 }, "observation.max_outline"},
		{"negative bytes", func(c *ObservationConfig) { c.MaxRegionBytes = -1 }, "observation.max_region_bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := DefaultObservationConfig()
			tc.set(&c)
			problems := c.Check("observation")
			if len(problems) != 1 || problems[0].Severity != SeverityError || problems[0].Path != tc.path {
				t.Errorf("problems = %+v, want one error at %s", problems, tc.path)
			}
		})
	}
}

// A set observation section reaches the installed policy through a real
// load; a missing file installs the defaults.
func TestLoadUserConfig_InstallsObservationLimits(t *testing.T) {
	t.Cleanup(func() { SetObservationLimits(DefaultObservationConfig().Resolve()) })
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{"observation":{"max_region_lines":11,"pad_lines":3,` +
		`"max_outline":7,"max_region_bytes":1024}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(path); err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	got, want := ResolvedObservationLimits(), ObservationLimits{MaxRegionLines: 11, PadLines: 3, MaxOutline: 7, MaxRegionBytes: 1024}
	if got != want {
		t.Errorf("installed limits = %+v, want %+v", got, want)
	}
	if _, err := LoadUserConfig(filepath.Join(dir, "missing.json")); err != nil {
		t.Fatalf("LoadUserConfig (missing): %v", err)
	}
	if got := ResolvedObservationLimits(); got != DefaultObservationConfig().Resolve() {
		t.Errorf("missing file installed %+v, want defaults", got)
	}
}

// A contradictory observation section refuses the file with the key named.
func TestLoadUserConfig_RefusesBadObservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"observation":{"max_outline":-2}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUserConfig(path)
	if err == nil || !strings.Contains(err.Error(), "observation.max_outline") {
		t.Errorf("LoadUserConfig error = %v, want observation.max_outline named", err)
	}
}
