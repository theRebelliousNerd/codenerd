package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
)

func TestBootCortexWithConfig_Overrides(t *testing.T) {
	// 1. Setup workspace (temp dir)
	workspace := t.TempDir()

	// Create .nerd directory to satisfy any existence checks
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("Failed to create .nerd dir: %v", err)
	}

	// 2. Setup mocks
	mockKernel := &MockSystemKernel{}
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "OK", nil
		},
	}

	// mockUserConfig with safe defaults
	mockUserConfig := config.DefaultUserConfig()
	mockUserConfig.Embedding = &config.EmbeddingConfig{
		Provider: "none", // Disable embedding engine init
	}

	// 3. Create BootConfig
	bootCfg := BootConfig{
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
	}

	// 4. Call BootCortexWithConfig
	cortex, err := BootCortexWithConfig(context.Background(), bootCfg)
	if err != nil {
		t.Fatalf("BootCortexWithConfig failed: %v", err)
	}
	defer cortex.Close()

	// 5. Verify injection
	if cortex == nil {
		t.Fatal("Expected cortex, got nil")
	}

	// Check Kernel injection
	// cortex.Kernel is core.Kernel interface
	// We can't easily check identity against mockKernel because of interface wrapping/copying?
	// Actually, factory assigns `kernel = cfg.KernelOverride`.
	// And `cortex.Kernel = kernel`.
	// So `cortex.Kernel` should be `mockKernel` (pointer equality).
	if cortex.Kernel != mockKernel {
		// It might be wrapped if adapters are used, but factory assigns direct reference.
		// Wait, factory uses `sessionKernelAdapter` for SessionExecutor, but `cortex.Kernel` is raw kernel.
		t.Error("Kernel was not injected correctly")
	}

	// Check LLM Client injection
	// factory wraps llmClient in `core.NewScheduledLLMCall`.
	// So `cortex.LLMClient` != `mockLLM`.
	// But we can check if it works or if the base client was used?
	// `llmClient` logic:
	// `var llmClient perception.LLMClient = core.NewScheduledLLMCall("main", rawLLMClient)`
	// `rawLLMClient` is `baseLLMClient` (mockLLM) or wrapped tracing.
	// Since we didn't create localDB (no file), no tracing.
	// So `cortex.LLMClient` is `ScheduledLLMCall` wrapping `mockLLM`.
	// We can't unwrap easily.
	// But validation passed, so init worked.

	if cortex.Workspace != workspace {
		t.Errorf("Expected workspace '%s', got '%s'", workspace, cortex.Workspace)
	}
}
