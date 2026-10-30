package features

import "testing"

// A flag's default is written once, in DefaultFeaturesConfig: the accessors,
// Resolved and the config template all fall back to it. Each accessor and
// the boolFlags table used to carry their own literal copy while
// DefaultFeaturesConfig, documented as "the values an accessor returns",
// was read by nothing.
func TestFeatureDefaults_ComeFromDefaultFeaturesConfig(t *testing.T) {
	saved, savedActive := defaults, active.Load()
	t.Cleanup(func() { defaults = saved; active.Store(savedActive) })
	active.Store(nil)
	for _, f := range boolFlags {
		t.Setenv(f.envVar, "")
		if f.legacyEnvVar != "" {
			t.Setenv(f.legacyEnvVar, "")
		}
	}

	if IsSystemShardsEnabled() != *DefaultFeaturesConfig().SystemShards {
		t.Fatal("IsSystemShardsEnabled does not return DefaultFeaturesConfig's default")
	}
	flipped := DefaultFeaturesConfig()
	off, on := false, true
	flipped.SystemShards, flipped.DarkMode = &off, &on
	defaults = flipped
	if IsSystemShardsEnabled() || !IsDarkModeEnabled() {
		t.Fatal("the accessors do not follow DefaultFeaturesConfig")
	}
	for _, r := range Resolved() {
		if r.Name == "dark_mode" && (!r.Default || !r.Value || r.Source != SourceDefault) {
			t.Fatalf("Resolved reports dark_mode %+v, want the default from DefaultFeaturesConfig", r)
		}
	}
}
