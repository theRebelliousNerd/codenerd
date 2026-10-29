package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/features"
	"codenerd/internal/shards"
)

// bootKernelForShardModeTest boots a full Cortex with a real kernel (no
// KernelOverride) and no provider, returning the booted Cortex. The caller
// must Close the Cortex.
func bootKernelForShardModeTest(t *testing.T, perShardFacts bool) *Cortex {
	t.Helper()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("Failed to create .nerd dir: %v", err)
	}

	// Pin the canonical env var explicitly: resolveBool gives
	// CODENERD_PER_SHARD_FACTS precedence over the active config, so an
	// ambient export would otherwise override SetActive below.
	if perShardFacts {
		t.Setenv("CODENERD_PER_SHARD_FACTS", "1")
	} else {
		t.Setenv("CODENERD_PER_SHARD_FACTS", "0")
	}
	v := perShardFacts
	features.SetActive(&features.FeaturesConfig{PerShardFacts: &v})
	t.Cleanup(func() { features.SetActive(nil) })

	// Clear provider keys so boot never depends on ambient credentials.
	for _, envVar := range []string{
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
		"GEMINI_API_KEY",
		"XAI_API_KEY",
		"ZAI_API_KEY",
		"OPENROUTER_API_KEY",
	} {
		t.Setenv(envVar, "")
	}

	userCfg := config.DefaultUserConfig()
	userCfg.Provider = ""
	userCfg.APIKey = ""
	userCfg.AnthropicAPIKey = ""
	userCfg.OpenAIAPIKey = ""
	userCfg.GeminiAPIKey = ""
	userCfg.XAIAPIKey = ""
	userCfg.ZAIAPIKey = ""
	userCfg.OpenRouterAPIKey = ""
	userCfg.Embedding = &config.EmbeddingConfig{Provider: "none"}

	cortex, err := BootCortexWithConfig(context.Background(), BootConfig{
		Workspace: workspace,
		DisableSystemShards: []string{
			"constitution_gate",
			"perception_firewall",
			"executive_policy",
			"world_model_ingestor",
			"session_planner",
			"tactile_router",
			"campaign_runner",
			"mangle_repair",
			"legislator",
		},
		UserConfigOverride: userCfg,
		LLMClientOverride:  &MockLLMClient{},
	})
	if err != nil {
		t.Fatalf("BootCortexWithConfig failed (perShardFacts=%v): %v", perShardFacts, err)
	}
	return cortex
}

// The domain split stays on regardless of per_shard_facts. Honoring "off" with
// one catch-all shard was measured live on 2026-09-04: every kernel evaluation
// went from ~0.25 s to 33–40 s and hydrating learned facts from 1.2 s to 70 s,
// because a single store puts every large fact family under every rule that
// joins it. Until single-store evaluation is affordable, the flag is recorded
// in the boot log and nothing else; this pins that both settings boot the
// manifests, so a future change to honor the flag has to come with numbers.
func TestBootKernelDomainShardsWhenPerShardFactsOff(t *testing.T) {
	assertDomainShardsBooted(t, false)
}

func TestBootKernelDomainShardsWhenPerShardFactsOn(t *testing.T) {
	assertDomainShardsBooted(t, true)
}

// One boot per test: two BootCortexWithConfig calls inside one test function
// hung the package on 2026-09-04 (boot after an explicit Close in the same
// process), which is itself worth a look but is not what this test pins.
func assertDomainShardsBooted(t *testing.T, flag bool) {
	t.Helper()
	cortex := bootKernelForShardModeTest(t, flag)
	t.Cleanup(func() { _ = cortex.Close() })
	ck, ok := cortex.Kernel.(*core.CortexKernel)
	if !ok {
		t.Fatalf("expected *core.CortexKernel, got %T", cortex.Kernel)
	}
	want := len(shards.DefaultShardPredicateManifests())
	if got := ck.ShardDomains(); len(got) != want {
		t.Fatalf("per_shard_facts=%v: expected %d domain shards, got %d (%v)", flag, want, len(got), got)
	}
}
