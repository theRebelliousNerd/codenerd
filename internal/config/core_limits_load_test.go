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
		"max_session_duration_min": 60,
		"max_facts_in_kernel": 50000,
		"max_derived_facts_limit": 20000,
		"max_tool_calls": 25,
		"max_tool_iterations": 6,
		"adaptive_tool_budget": false,
		"tool_iteration_extension_size": 4,
		"max_tool_iteration_extensions": 1,
		"tool_loop_repeat_threshold": 3
	}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig error: %v", err)
	}
	limits := cfg.GetCoreLimits()
	if limits.AdaptiveToolBudget == nil || *limits.AdaptiveToolBudget {
		t.Fatal("explicit adaptive_tool_budget=false was lost")
	}
	names := []string{
		"max_total_memory_mb",
		"max_concurrent_shards",
		"max_concurrent_api_calls",
		"max_session_duration_min",
		"max_facts_in_kernel",
		"max_derived_facts_limit",
		"max_tool_calls",
		"max_tool_iterations",
		"tool_iteration_extension_size",
		"max_tool_iteration_extensions",
		"tool_loop_repeat_threshold",
	}
	got := []int{
		limits.MaxTotalMemoryMB,
		limits.MaxConcurrentShards,
		limits.MaxConcurrentAPICalls,
		limits.MaxSessionDurationMin,
		limits.MaxFactsInKernel,
		limits.MaxDerivedFactsLimit,
		limits.MaxToolCalls,
		limits.MaxToolIterations,
		limits.ToolIterationExtensionSize,
		limits.MaxToolIterationExtensions,
		limits.ToolLoopRepeatThreshold,
	}
	want := []int{8192, 4, 3, 60, 50000, 20000, 25, 6, 4, 1, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %d, want %d", names[i], got[i], want[i])
		}
	}
}
