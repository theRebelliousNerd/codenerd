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

// The keys that put a wall clock on a run were removed on 2026-09-19 (Steve:
// "there should not be timeouts like that... some agentic runs are like hours
// long"). A config still setting one must fail to load, naming the section,
// the key and what to do -- a user who believes a 30-minute ceiling still
// protects them is worse off than one whose config will not start.
func TestLoadUserConfig_RejectsRemovedRunClocks(t *testing.T) {
	cases := map[string]string{
		"core_limits.max_session_duration_min":           `{"core_limits": {"max_concurrent_shards": 4, "max_session_duration_min": 120}}`,
		"llm_timeouts.shard_execution_timeout":           `{"llm_timeouts": {"shard_execution_timeout": "30m"}}`,
		"llm_timeouts.ooda_loop_timeout":                 `{"llm_timeouts": {"ooda_loop_timeout": "30m"}}`,
		"llm_timeouts.campaign_phase_timeout":            `{"llm_timeouts": {"campaign_phase_timeout": "30m"}}`,
		"llm_timeouts.document_processing_timeout":       `{"llm_timeouts": {"document_processing_timeout": "20m"}}`,
		"llm_timeouts.ouroboros_timeout":                 `{"llm_timeouts": {"ouroboros_timeout": "10m"}}`,
		"shard_profiles.coder.max_execution_time_sec":    `{"shard_profiles": {"coder": {"max_retries": 3, "max_execution_time_sec": 600}}}`,
		"shard_profiles.reviewer.max_execution_time_sec": `{"shard_profiles": {"reviewer": {"max_execution_time_sec": 900}}}`,
		"default_shard.max_execution_time_sec":           `{"default_shard": {"max_execution_time_sec": 300}}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadUserConfig(writeCoreLimitsTestConfig(t, content))
			if err == nil {
				t.Fatalf("%s still loads; a removed run clock must not be silently ignored", name)
			}
			for _, want := range []string{name, "no longer a supported key", "delete the key"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error does not contain %q: %v", want, err)
				}
			}
		})
	}

	t.Run("every one in one file is named at once", func(t *testing.T) {
		_, err := LoadUserConfig(writeCoreLimitsTestConfig(t, `{
			"core_limits": {"max_concurrent_shards": 4, "max_session_duration_min": 120},
			"llm_timeouts": {"per_call_timeout": "10m", "ooda_loop_timeout": "30m", "shard_execution_timeout": "30m"},
			"shard_profiles": {"coder": {"max_execution_time_sec": 600}, "tester": {"max_execution_time_sec": 300}},
			"default_shard": {"max_execution_time_sec": 300}
		}`))
		if err == nil {
			t.Fatal("a config setting six removed keys still loads")
		}
		for _, key := range []string{
			"core_limits.max_session_duration_min", "llm_timeouts.ooda_loop_timeout",
			"llm_timeouts.shard_execution_timeout", "shard_profiles.coder.max_execution_time_sec",
			"shard_profiles.tester.max_execution_time_sec", "default_shard.max_execution_time_sec",
		} {
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error does not name %s, so the user must fix them one boot at a time: %v", key, err)
			}
		}
	})

	t.Run("the request bounds that stay still load", func(t *testing.T) {
		cfg, err := LoadUserConfig(writeCoreLimitsTestConfig(t,
			`{"llm_timeouts": {"per_call_timeout": "7m", "articulation_timeout": "4m", "follow_up_timeout": "3m"}}`))
		if err != nil {
			t.Fatalf("a config of live request bounds was refused: %v", err)
		}
		if cfg.LLMTimeouts == nil || cfg.LLMTimeouts.PerCallTimeout != "7m" {
			t.Fatalf("llm_timeouts.per_call_timeout did not load: %+v", cfg.LLMTimeouts)
		}
	})
}
