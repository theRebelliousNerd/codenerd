package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
)

// TestBootRegistersPlanningFactories confirms the boot path (not
// RegisterAllShardFactories alone) owns the tactile_router and
// campaign_runner factories. internal/shards/registration.go must not
// register shadow factories for these names; internal/system/factory.go
// re-registers them with boot-time collaborators (task executor, browser
// manager, JIT config).
func TestBootRegistersPlanningFactories(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("create .nerd dir: %v", err)
	}

	mockKernel := &MockSystemKernel{}
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "OK", nil
		},
	}
	mockUserConfig := config.DefaultUserConfig()
	mockUserConfig.Embedding = &config.EmbeddingConfig{
		Provider: "none",
	}

	cortex, err := BootCortexWithConfig(context.Background(), BootConfig{
		Workspace: workspace,
		APIKey:    "test-key",
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
		UserConfigOverride: mockUserConfig,
		LLMClientOverride:  mockLLM,
		KernelOverride:     mockKernel,
	})
	if err != nil {
		t.Fatalf("BootCortexWithConfig failed: %v", err)
	}
	defer cortex.Close()

	for _, name := range []string{"tactile_router", "campaign_runner", "session_planner"} {
		if !cortex.ShardManager.HasShardFactory(name) {
			t.Errorf("expected factory for %q to be registered after boot", name)
		}
	}
}
