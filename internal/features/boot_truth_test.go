package features_test

import (
	"encoding/json"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/features"
)

// One boot truth (GAP-FEAT-02, Docs/architecture/features/TODO.md).
//
// There are two constructors and they are not rivals:
//
//   - A boot with no features block in .nerd/config.json (or none loaded yet)
//     resolves every flag to the compile-time default: the def column of the
//     flag table, which DefaultFeaturesConfig states as a struct.
//   - `nerd init` writes FullyEnabledFeaturesConfig into the config it seeds
//     (config.DefaultUserConfig), so an initialized workspace opts in
//     explicitly and the registry reports those values as "config".
//
// The risk was never which one "wins" but that the defaults are written in
// three places -- the table's def column, DefaultFeaturesConfig, and the
// literal in each IsXxx accessor -- and nothing tied them together. These
// tests do; TestResolved_ShouldMatchAccessors ties the accessors to the table.

func clearFeatureEnv(t *testing.T) {
	t.Helper()
	for _, f := range features.Resolved() {
		t.Setenv(f.EnvVar, "")
		if f.LegacyEnvVar != "" {
			t.Setenv(f.LegacyEnvVar, "")
		}
	}
}

func asFlagMap(t *testing.T, cfg features.FeaturesConfig) map[string]*bool {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	out := map[string]*bool{}
	for k, v := range m {
		if b, ok := v.(bool); ok {
			out[k] = &b
		}
	}
	return out
}

func TestBootTruth_ANoConfigBootResolvesToDefaultFeaturesConfig(t *testing.T) {
	clearFeatureEnv(t)
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })

	defaults := asFlagMap(t, features.DefaultFeaturesConfig())
	for _, f := range features.Resolved() {
		d, ok := defaults[f.Name]
		if !ok {
			t.Errorf("DefaultFeaturesConfig leaves %s unset; it must state every compile-time default", f.Name)
			continue
		}
		if *d != f.Default || f.Value != f.Default || f.Source != features.SourceDefault {
			t.Errorf("%s: DefaultFeaturesConfig says %v, the flag table's default is %v, a no-config boot resolves %v (%s)",
				f.Name, *d, f.Default, f.Value, f.Source)
		}
	}

	// Installing the defaults explicitly is indistinguishable from installing
	// nothing: there is one answer to "what does a fresh boot run with".
	noConfig := map[string]bool{}
	for _, f := range features.Resolved() {
		noConfig[f.Name] = f.Value
	}
	d := features.DefaultFeaturesConfig()
	features.SetActive(&d)
	for _, f := range features.Resolved() {
		if f.Value != noConfig[f.Name] {
			t.Errorf("%s resolves %v with DefaultFeaturesConfig installed and %v with no config", f.Name, f.Value, noConfig[f.Name])
		}
	}
}

func TestBootTruth_InitSeedsFullyEnabledFeaturesConfig(t *testing.T) {
	seeded := config.DefaultUserConfig().Features
	if seeded == nil {
		t.Fatal("config.DefaultUserConfig writes no features block; `nerd init` would seed nothing")
	}
	got, want := asFlagMap(t, *seeded), asFlagMap(t, features.FullyEnabledFeaturesConfig())
	if len(got) != len(want) {
		t.Fatalf("the seeded features block has %d flags, FullyEnabledFeaturesConfig has %d", len(got), len(want))
	}
	for name, w := range want {
		if g, ok := got[name]; !ok || *g != *w {
			t.Errorf("the seeded config writes %s=%v, FullyEnabledFeaturesConfig says %v", name, deref(g), *w)
		}
	}
}

func deref(p *bool) any {
	if p == nil {
		return "unset"
	}
	return *p
}
