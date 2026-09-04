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

func TestBootKernelSingleShardWhenPerShardFactsOff(t *testing.T) {
	cortex := bootKernelForShardModeTest(t, false)
	defer cortex.Close()

	ck, ok := cortex.Kernel.(*core.CortexKernel)
	if !ok {
		t.Fatalf("expected *core.CortexKernel, got %T", cortex.Kernel)
	}
	domains := ck.ShardDomains()
	if len(domains) != 1 {
		t.Fatalf("expected exactly one shard with per_shard_facts off, got %d (%v)", len(domains), domains)
	}
	if domains[0] != "cortex" {
		t.Fatalf("expected single catch-all shard %q, got %q", "cortex", domains[0])
	}
}

func TestBootKernelDomainShardsWhenPerShardFactsOn(t *testing.T) {
	cortex := bootKernelForShardModeTest(t, true)
	defer cortex.Close()

	ck, ok := cortex.Kernel.(*core.CortexKernel)
	if !ok {
		t.Fatalf("expected *core.CortexKernel, got %T", cortex.Kernel)
	}
	want := len(shards.DefaultShardPredicateManifests())
	if got := len(ck.ShardDomains()); got != want {
		t.Fatalf("expected %d domain shards with per_shard_facts on, got %d (%v)", want, got, ck.ShardDomains())
	}
}
