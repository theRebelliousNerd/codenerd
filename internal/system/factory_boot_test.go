package system

import (
	"github.com/google/mangle/analysis"

	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestBootCortexWithConfig_NoLLMConfigured(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("Failed to create .nerd dir: %v", err)
	}

	t.Chdir(workspace)
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
		Workspace:          workspace,
		UserConfigOverride: userCfg,
	})
	if err != nil {
		t.Fatalf("expected boot to succeed without LLM for non-LLM commands: %v", err)
	}
	defer cortex.Close()

	_, err = cortex.LLMClient.CompleteWithSystem(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected LLM calls to fail when no LLM client is configured")
	}
	if !strings.Contains(err.Error(), "no LLM client configured") {
		t.Fatalf("unexpected LLM error: %v", err)
	}
}

func (m *MockSystemKernel) GetProgramInfo() *analysis.ProgramInfo { return nil }

func (m *MockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, forceJSON bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	go func() {
		defer close(contentChan)
		defer close(errorChan)
		res, err := m.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()
	return contentChan, errorChan
}
