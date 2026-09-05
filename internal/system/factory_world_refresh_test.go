package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
)

// One boot per test: this boots exactly once.
func TestBootRefreshesWorldModel(t *testing.T) {
	workspace := t.TempDir()
	os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755)
	os.WriteFile(filepath.Join(workspace, "hello.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	mockKernel := &MockSystemKernel{}
	mockLLM := &MockLLMClient{}
	mockUserConfig := config.DefaultUserConfig()
	mockUserConfig.Embedding = &config.EmbeddingConfig{Provider: "none"}

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

	assertBootTopology(t, cortex)
}

func assertBootTopology(t *testing.T, cortex *Cortex) {
	t.Helper()
	facts, err := cortex.Kernel.Query("file_topology")
	if err != nil {
		t.Fatalf("Query file_topology: %v", err)
	}
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		if s, ok := f.Args[0].(string); ok && s == "hello.go" {
			return
		}
	}
	t.Fatalf("canonical hello.go missing in %v", facts)
}
