package features

import (
	"strings"
	"testing"
)

// TestConfigSchemaJSON_ShouldListEveryRecognisedKey is what keeps the published
// snippet from drifting: it is generated from the same tables the accessors
// read, and a new flag that is not documented fails here.
func TestConfigSchemaJSON_ShouldListEveryRecognisedKey(t *testing.T) {
	snippet := ConfigSchemaJSON()

	for _, key := range ConfigSchemaKeys() {
		if !strings.Contains(snippet, "\""+key+"\":") {
			t.Errorf("schema snippet omits key %q", key)
		}
	}
	if !strings.Contains(snippet, "\"features\": {") {
		t.Error("schema snippet does not show the features block it documents")
	}
}

func TestConfigSchemaJSON_ShouldNameEveryEnvVar(t *testing.T) {
	snippet := ConfigSchemaJSON()

	for _, f := range boolFlags {
		if !strings.Contains(snippet, f.envVar) {
			t.Errorf("schema snippet omits env var %s for %s", f.envVar, f.name)
		}
		if f.legacyEnvVar != "" && !strings.Contains(snippet, f.legacyEnvVar) {
			t.Errorf("schema snippet omits legacy env var %s for %s", f.legacyEnvVar, f.name)
		}
	}
	for _, f := range intFlags {
		if !strings.Contains(snippet, f.envVar) {
			t.Errorf("schema snippet omits env var %s for %s", f.envVar, f.name)
		}
	}
}

// TestConfigSchemaKeys_ShouldMatchTheJSONTags catches the drift that matters
// most: a key documented under a name .nerd/config.json does not unmarshal.
func TestConfigSchemaKeys_ShouldMatchTheJSONTags(t *testing.T) {
	// The struct tags are the authority; ConfigSchemaKeys is derived from the
	// flag tables, so agreeing here proves the tables name real config keys.
	want := map[string]bool{
		"diff_eval": true, "flight_recorder": true, "provenance": true,
		"system_shards": true, "per_shard_facts": true, "dark_mode": true,
		"skip_onboarding": true, "taxonomy_fast": true,
		"fast_scan_workers": true, "fast_ast_max_bytes": true,
	}
	got := ConfigSchemaKeys()
	if len(got) != len(want) {
		t.Fatalf("ConfigSchemaKeys() = %v (%d keys), want %d", got, len(got), len(want))
	}
	for _, key := range got {
		if !want[key] {
			t.Errorf("ConfigSchemaKeys() reports %q, which is not a features JSON tag", key)
		}
	}
}

// TestPerShardFacts_ShouldRemainOptInEvenWhenFullyEnabled pins the 2026-08-15
// audit result. ShardFactRouter dispatches a single predicate to its owning
// shard but does not evaluate joins across shards: rule evaluation happens
// inside a shard's own kernel over that shard's local facts, so a rule body
// spanning two owners silently derives nothing. Until a join coordinator
// exists, turning this on deletes cross-domain derivations rather than failing
// loudly. Flip it here only together with that coordinator.
func TestPerShardFacts_ShouldRemainOptInEvenWhenFullyEnabled(t *testing.T) {
	cfg := FullyEnabledFeaturesConfig()

	if cfg.PerShardFacts == nil {
		t.Fatal("FullyEnabledFeaturesConfig must state PerShardFacts explicitly, not leave it unset")
	}
	if *cfg.PerShardFacts {
		t.Fatal("PerShardFacts was flipped on in FullyEnabledFeaturesConfig; " +
			"ShardFactRouter still has no cross-shard join coordinator, so this " +
			"silently deletes every cross-domain derivation")
	}

	// Every other boolean is on: this is the "most modern paths" config, and
	// PerShardFacts being the single exception is the point.
	for name, p := range map[string]*bool{
		"DiffEval": cfg.DiffEval, "FlightRecorder": cfg.FlightRecorder,
		"Provenance": cfg.Provenance, "SystemShards": cfg.SystemShards,
		"DarkMode": cfg.DarkMode, "SkipOnboarding": cfg.SkipOnboarding,
		"TaxonomyFast": cfg.TaxonomyFast,
	} {
		if p == nil || !*p {
			t.Errorf("FullyEnabledFeaturesConfig().%s should be true", name)
		}
	}
}

// TestPerShardFacts_ShouldStillHonourAnExplicitOptIn: the flag ships off, but
// an operator who sets it must get the router rather than a silently ignored
// setting — the accessor is ordinary resolveBool, not a hard short-circuit.
func TestPerShardFacts_ShouldStillHonourAnExplicitOptIn(t *testing.T) {
	SetActive(nil)
	t.Cleanup(func() { SetActive(nil) })

	if IsPerShardFactsEnabled() {
		t.Fatal("PerShardFacts should default off")
	}

	t.Setenv("CODENERD_PER_SHARD_FACTS", "1")
	if !IsPerShardFactsEnabled() {
		t.Fatal("an explicit env opt-in was ignored; the accessor must not short-circuit")
	}

	t.Setenv("CODENERD_PER_SHARD_FACTS", "")
	on := true
	SetActive(&FeaturesConfig{PerShardFacts: &on})
	if !IsPerShardFactsEnabled() {
		t.Fatal("an explicit config opt-in was ignored")
	}
}
