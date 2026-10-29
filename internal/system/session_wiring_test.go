package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

// bootSessionWiringCortex boots a Cortex with the same mock harness as
// TestBootCortexWithConfig_Overrides so session-wiring assertions run against
// the real factory assembly path, not a hand-wired executor.
func bootSessionWiringCortex(t *testing.T, sessionID string) *Cortex {
	t.Helper()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("Failed to create .nerd dir: %v", err)
	}

	mockKernel := &MockSystemKernel{}
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "OK", nil
		},
	}
	mockUserConfig := config.DefaultUserConfig()
	mockUserConfig.Embedding = &config.EmbeddingConfig{Provider: "none"}

	bootCfg := BootConfig{
		Workspace: workspace,
		APIKey:    "test-key",
		SessionID: sessionID,
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

	cortex, err := BootCortexWithConfig(context.Background(), bootCfg)
	if err != nil {
		t.Fatalf("BootCortexWithConfig failed: %v", err)
	}
	t.Cleanup(func() { _ = cortex.Close() })
	return cortex
}

func TestBootSessionIdentity_MintedByDefault(t *testing.T) {
	cortex := bootSessionWiringCortex(t, "")

	sid := cortex.SessionID()
	if sid == "" {
		t.Fatal("Cortex.SessionID() should be minted at boot, got empty")
	}
	if sid == "default" {
		t.Fatal("Cortex.SessionID() should never be the \"default\" fallback")
	}
	if !strings.HasPrefix(sid, "session-") {
		t.Errorf("Cortex.SessionID() = %q, want prefix %q", sid, "session-")
	}
	if cortex.SessionExecutor == nil {
		t.Fatal("SessionExecutor should be wired after boot")
	}
	if got := cortex.SessionExecutor.SessionID(); got != sid {
		t.Errorf("SessionExecutor.SessionID() = %q, want Cortex identity %q", got, sid)
	}
}

func TestBootSessionIdentity_HonorsOverride(t *testing.T) {
	cortex := bootSessionWiringCortex(t, "session-resumed-123")

	if got := cortex.SessionID(); got != "session-resumed-123" {
		t.Errorf("Cortex.SessionID() = %q, want override %q", got, "session-resumed-123")
	}
	if got := cortex.SessionExecutor.SessionID(); got != "session-resumed-123" {
		t.Errorf("SessionExecutor.SessionID() = %q, want override %q", got, "session-resumed-123")
	}
}

func TestBootOuroborosRegistry_WiredFromVirtualStore(t *testing.T) {
	cortex := bootSessionWiringCortex(t, "")

	if cortex.VirtualStore == nil {
		t.Fatal("VirtualStore should be wired after boot")
	}
	storeRegistry := cortex.VirtualStore.GetToolRegistry()
	if storeRegistry == nil {
		t.Fatal("VirtualStore.GetToolRegistry() should be non-nil after boot")
	}
	execRegistry := cortex.SessionExecutor.OuroborosRegistry()
	if execRegistry == nil {
		t.Fatal("SessionExecutor should carry the VirtualStore tool registry after boot")
	}
	if execRegistry != storeRegistry {
		t.Error("SessionExecutor registry should be the VirtualStore registry instance")
	}

	// A tool registered in the VirtualStore registry must be visible through
	// the executor's registry — the same registry buildToolCatalogForPiggyback
	// reads when assembling the tool catalog.
	const toolName = "boot-wiring-probe-tool"
	if err := storeRegistry.RegisterToolWithInfo(&core.Tool{
		Name:        toolName,
		Command:     "go",
		Description: "boot wiring regression probe",
	}); err != nil {
		t.Fatalf("register probe tool: %v", err)
	}
	found := false
	for _, tool := range execRegistry.ListTools() {
		if tool.Name == toolName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tool %q registered in VirtualStore registry not visible via executor registry", toolName)
	}
}
