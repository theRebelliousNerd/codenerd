package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LoadUserConfig must reject explicitly invalid core_limits at load time
// instead of letting them misbehave at runtime. Never touches the
// repository's .nerd/config.json: every case uses t.TempDir().
func writeCoreLimitsTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestLoadUserConfig_RejectsExplicitZeroShards(t *testing.T) {
	path := writeCoreLimitsTestConfig(t, `{"core_limits": {"max_concurrent_shards": 0}}`)
	_, err := LoadUserConfig(path)
	if err == nil {
		t.Fatal("expected error for max_concurrent_shards=0")
	}
	if !strings.Contains(err.Error(), "core_limits") {
		t.Fatalf("error should name core_limits, got: %v", err)
	}
}

func TestLoadUserConfig_MissingCoreLimitsLoadsDefaults(t *testing.T) {
	path := writeCoreLimitsTestConfig(t, `{"provider": "ollama"}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig error: %v", err)
	}
	limits := cfg.GetCoreLimits()
	def := DefaultCoreLimits()
	names := []string{
		"max_total_memory_mb",
		"max_concurrent_shards",
		"max_facts_in_kernel",
		"max_derived_facts_limit",
	}
	got := []int{
		limits.MaxTotalMemoryMB,
		limits.MaxConcurrentShards,
		limits.MaxFactsInKernel,
		limits.MaxDerivedFactsLimit,
	}
	want := []int{
		def.MaxTotalMemoryMB,
		def.MaxConcurrentShards,
		def.MaxFactsInKernel,
		def.MaxDerivedFactsLimit,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %d, want default %d", names[i], got[i], want[i])
		}
	}
}

func TestLoadUserConfig_ValidExplicitLimitsLoadUnchanged(t *testing.T) {
	path := writeCoreLimitsTestConfig(t, `{"core_limits": {
		"max_total_memory_mb": 8192,
		"max_concurrent_shards": 4,
		"max_concurrent_api_calls": 3,
		"max_facts_in_kernel": 50000,
		"max_derived_facts_limit": 20000
	}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig error: %v", err)
	}
	limits := cfg.GetCoreLimits()
	names := []string{
		"max_total_memory_mb",
		"max_concurrent_shards",
		"max_concurrent_api_calls",
		"max_facts_in_kernel",
		"max_derived_facts_limit",
	}
	got := []int{
		limits.MaxTotalMemoryMB,
		limits.MaxConcurrentShards,
		limits.MaxConcurrentAPICalls,
		limits.MaxFactsInKernel,
		limits.MaxDerivedFactsLimit,
	}
	want := []int{8192, 4, 3, 50000, 20000}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %d, want %d", names[i], got[i], want[i])
		}
	}
}

// The six tool-budget keys were deleted on 2026-09-18 with the count ceilings
// they configured. A config that still sets one must fail to load, and the
// failure must name the key and say why it is gone — the strict decoder's own
// "unknown field" names the key but leaves the user guessing whether it was
// renamed, moved or removed, and a user who believes they still have a
// ceiling is worse off than one whose config will not start.
func TestRemovedConfigKeys_FailLoudly(t *testing.T) {
	cases := map[string]string{
		"max_tool_calls":                `25`,
		"max_tool_iterations":           `6`,
		"adaptive_tool_budget":          `false`,
		"tool_iteration_extension_size": `4`,
		"max_tool_iteration_extensions": `1`,
		"tool_loop_repeat_threshold":    `3`,
	}
	for key, value := range cases {
		t.Run(key, func(t *testing.T) {
			path := writeCoreLimitsTestConfig(t,
				`{"core_limits": {"max_concurrent_shards": 4, "`+key+`": `+value+`}}`)
			_, err := LoadUserConfig(path)
			if err == nil {
				t.Fatalf("core_limits.%s still loads; a removed key must not be silently ignored", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Fatalf("error does not name the removed key %q: %v", key, err)
			}
			if !strings.Contains(err.Error(), "no longer a supported key") {
				t.Fatalf("error does not say the key was removed: %v", err)
			}
			if !strings.Contains(err.Error(), "delete the key") {
				t.Fatalf("error does not say what to do about it: %v", err)
			}
		})
	}

	t.Run("all six at once are all named", func(t *testing.T) {
		fields := make([]string, 0, len(cases))
		for key, value := range cases {
			fields = append(fields, `"`+key+`": `+value)
		}
		path := writeCoreLimitsTestConfig(t, `{"core_limits": {`+strings.Join(fields, ", ")+`}}`)
		_, err := LoadUserConfig(path)
		if err == nil {
			t.Fatal("a config of nothing but removed keys still loads")
		}
		for key := range cases {
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error does not name %q, so the user must fix them one boot at a time: %v", key, err)
			}
		}
	})

	t.Run("the replacement for the repeat threshold is named", func(t *testing.T) {
		path := writeCoreLimitsTestConfig(t, `{"core_limits": {"tool_loop_repeat_threshold": 3}}`)
		_, err := LoadUserConfig(path)
		if err == nil {
			t.Fatal("tool_loop_repeat_threshold still loads")
		}
		if !strings.Contains(err.Error(), "working_repeat_threshold") {
			t.Fatalf("the key that became a policy fact must say where it went: %v", err)
		}
	})
}
