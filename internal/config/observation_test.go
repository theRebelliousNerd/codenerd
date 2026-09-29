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
		got.MaxOutline != 60 || got.MaxRegionBytes != 24<<10 ||
		got.SubagentHydrateMaxLines != 200 || got.SubagentHydrateDefaultLines != 60 {
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
	got := cfg.GetObservationConfig()
	if got.MaxRegionLines != 11 || got.PadLines != 3 || got.MaxOutline != 7 || got.MaxRegionBytes != 1024 {
		t.Errorf("set section = %+v, want the set values", got)
	}
	if got.SubagentHydrateMaxLines != 200 || got.SubagentHydrateDefaultLines != 60 {
		t.Errorf("absent hydrate bounds = %+v, want the defaults", got)
	}
	cfg.Observation.SubagentHydrateMaxLines = 40
	cfg.Observation.SubagentHydrateDefaultLines = 12
	got = cfg.GetObservationConfig()
	if got != *cfg.Observation {
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
		{"negative hydrate max", func(c *ObservationConfig) { c.SubagentHydrateMaxLines = -1 }, "observation.subagent_hydrate_max_lines"},
		{"negative hydrate default", func(c *ObservationConfig) { c.SubagentHydrateDefaultLines = -1 }, "observation.subagent_hydrate_default_lines"},
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
	want := DefaultObservationConfig().Resolve()
	want.MaxRegionLines = 11
	want.PadLines = 3
	want.MaxOutline = 7
	want.MaxRegionBytes = 1024
	if got := ResolvedObservationLimits(); got != want {
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

// Absent hydrate keys install the leaf page (200/60). A set pair installs
// that pair. The refusal cases live on Check and on LoadUserConfig.
func TestLoadUserConfig_SubagentHydrateBounds(t *testing.T) {
	t.Cleanup(func() { SetObservationLimits(DefaultObservationConfig().Resolve()) })
	dir := t.TempDir()

	absent := filepath.Join(dir, "absent.json")
	if err := os.WriteFile(absent, []byte(`{"observation":{"max_outline":9}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(absent); err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	got := ResolvedObservationLimits()
	if got.SubagentHydrateMaxLines != 200 || got.SubagentHydrateDefaultLines != 60 || got.MaxOutline != 9 {
		t.Fatalf("absent hydrate keys installed %+v, want max 200, default 60, outline 9", got)
	}

	set := filepath.Join(dir, "set.json")
	body := `{"observation":{"subagent_hydrate_max_lines":17,"subagent_hydrate_default_lines":9}}`
	if err := os.WriteFile(set, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(set); err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	got = ResolvedObservationLimits()
	if got.SubagentHydrateMaxLines != 17 || got.SubagentHydrateDefaultLines != 9 {
		t.Fatalf("installed hydrate bounds = %d/%d, want 17/9", got.SubagentHydrateMaxLines, got.SubagentHydrateDefaultLines)
	}
}

func TestObservationConfig_Check_HydrateBounds(t *testing.T) {
	// An explicit 0 is an absent key: WithDefaults fills it, and Check
	// then accepts the default. A zero that is still zero after that fill
	// is the same error as a negative (the count loop's v < 1).
	zero := DefaultObservationConfig()
	zero.SubagentHydrateMaxLines = 0
	zero.SubagentHydrateDefaultLines = 0
	if p := zero.Check("observation"); len(p) != 0 {
		t.Errorf("explicit zero is the absent key, Check = %+v", p)
	}

	above := DefaultObservationConfig()
	above.SubagentHydrateMaxLines = 10
	above.SubagentHydrateDefaultLines = 11
	problems := above.Check("observation")
	if len(problems) != 1 || problems[0].Severity != SeverityError ||
		problems[0].Path != "observation.subagent_hydrate_default_lines" {
		t.Fatalf("default above max: %+v", problems)
	}

	// The max alone, set under the default page, is the same contradiction:
	// the page in force would not fit the cap.
	capped := DefaultObservationConfig()
	capped.SubagentHydrateMaxLines = 10
	problems = capped.Check("observation")
	if len(problems) != 1 || problems[0].Path != "observation.subagent_hydrate_default_lines" ||
		!strings.Contains(problems[0].Message, "subagent_hydrate_max_lines") {
		t.Fatalf("max below the default page: %+v", problems)
	}
}

func TestLoadUserConfig_RefusesBadSubagentHydrateBounds(t *testing.T) {
	cases := []struct {
		name string
		body string
		key  string
	}{
		{"negative max", `{"observation":{"subagent_hydrate_max_lines":-3}}`, "observation.subagent_hydrate_max_lines"},
		{"negative default", `{"observation":{"subagent_hydrate_default_lines":-1}}`, "observation.subagent_hydrate_default_lines"},
		{"default above max", `{"observation":{"subagent_hydrate_max_lines":10,"subagent_hydrate_default_lines":11}}`, "observation.subagent_hydrate_default_lines"},
		{"max below the default page", `{"observation":{"subagent_hydrate_max_lines":10}}`, "observation.subagent_hydrate_default_lines"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadUserConfig(path)
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Errorf("LoadUserConfig error = %v, want %s named", err, tc.key)
			}
		})
	}
}
