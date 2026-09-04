package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/config"
)

// TestBootOuroborosToolStoreWiring pins the Cortex-factory Ouroboros/tool
// wiring that used to live only in the TUI boot
// (cmd/nerd/chat/session_shared_boot.go): ToolStore, generated-tool hydration
// from disk, and the kernel-listener + Dreamer-queue goroutines.
//
// One boot per test: two BootCortexWithConfig calls inside one test function
// hang the package (see factory_kernel_shards_test.go), so this test boots
// exactly once.
func TestBootOuroborosToolStoreWiring(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("Failed to create .nerd dir: %v", err)
	}

	// On-disk format HydrateToolsFromDisk expects: one file per tool under
	// .nerd/tools/.compiled; the file name (minus .exe) becomes the tool name
	// (see internal/core/tool_registry.go RestoreFromDisk).
	compiledDir := filepath.Join(workspace, ".nerd", "tools", ".compiled")
	if err := os.MkdirAll(compiledDir, 0755); err != nil {
		t.Fatalf("Failed to create compiled tools dir: %v", err)
	}
	const genToolName = "factory-ouroboros-probe-tool"
	if err := os.WriteFile(filepath.Join(compiledDir, genToolName), []byte("#!/bin/sh\necho probe\n"), 0755); err != nil {
		t.Fatalf("Failed to write generated tool fixture: %v", err)
	}

	mockKernel := &MockSystemKernel{}
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "OK", nil
		},
	}
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

	if cortex.ToolStore == nil {
		t.Error("Cortex.ToolStore should be non-nil after boot")
	}
	if cortex.VirtualStore == nil {
		t.Fatal("VirtualStore should be wired after boot")
	}
	registry := cortex.VirtualStore.GetToolRegistry()
	if registry == nil {
		t.Fatal("VirtualStore.GetToolRegistry() should be non-nil after boot")
	}
	if _, ok := registry.GetTool(genToolName); !ok {
		t.Errorf("generated tool %q should be present in the tool registry after boot", genToolName)
	}

	// The listener and consumer goroutines must exit on Close: assert Close
	// returns without hanging.
	done := make(chan error, 1)
	go func() {
		done <- cortex.Close()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Logf("Close returned error (non-fatal for this test): %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Cortex.Close() did not return within 5s; Ouroboros listener/consumer goroutines may be leaking")
	}
}
