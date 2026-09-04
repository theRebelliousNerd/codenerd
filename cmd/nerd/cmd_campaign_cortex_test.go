package main

import (
	"context"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/northstar"
	"codenerd/internal/session"
	coresys "codenerd/internal/system"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// stubCampaignTaskExecutor is a no-op session.TaskExecutor so the config test
// can supply a non-nil TaskExecutor without booting any provider.
type stubCampaignTaskExecutor struct{}

func (s *stubCampaignTaskExecutor) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	return "", nil
}

func (s *stubCampaignTaskExecutor) ExecuteWithContext(ctx context.Context, req session.TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {
	return "", nil
}

func (s *stubCampaignTaskExecutor) ExecuteAsync(ctx context.Context, req session.TaskRequest) (string, error) {
	return "", nil
}

func (s *stubCampaignTaskExecutor) GetResult(taskID string) (string, bool, error) {
	return "", false, nil
}

func (s *stubCampaignTaskExecutor) WaitForResult(ctx context.Context, taskID string) (string, error) {
	return "", nil
}

// TestBuildCampaignOrchestratorConfig_CortexBacked proves the extracted builder
// carries the Cortex objects into the OrchestratorConfig without booting a
// provider, and that nil autopoiesis/MCP optionals degrade instead of panicking.
func TestBuildCampaignOrchestratorConfig_CortexBacked(t *testing.T) {
	// The builder constructs a Northstar observer backed by a shared guardian
	// holding an open SQLite file under cwd/.nerd. Reset the process-wide
	// registry before TempDir cleanup so RemoveAll does not hit a lock on
	// Windows.
	defer northstar.ResetGuardianRegistry()
	kern, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	virtualStore := core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), core.DefaultVirtualStoreConfig())
	shardMgr := coreshards.NewShardManager()
	var taskExec session.TaskExecutor = &stubCampaignTaskExecutor{}

	// Optional fields (Orchestrator, Scanner, LLM tiers, JIT, persistence)
	// are deliberately nil: ToolGenerator/OuroborosLoop/MCPStore must handle
	// that via their nil-safe accessors.
	cortex := &coresys.Cortex{
		Kernel:       kern,
		RealKernel:   kern,
		VirtualStore: virtualStore,
		ShardManager: shardMgr,
		TaskExecutor: taskExec,
	}

	cwd := t.TempDir()
	progressChan := make(chan campaign.Progress, 10)
	eventChan := make(chan campaign.OrchestratorEvent, 100)

	cfg, _ := buildCampaignOrchestratorConfig(cortex, cwd, progressChan, eventChan)

	if cfg.ToolPregenerator == nil {
		t.Fatalf("ToolPregenerator is nil: builder must always construct it (nil-safe on missing Ouroboros)")
	}
	if cfg.Kernel != cortex.Kernel {
		t.Fatalf("Kernel not from Cortex")
	}
	if cfg.VirtualStore != cortex.VirtualStore {
		t.Fatalf("VirtualStore not from Cortex")
	}
	if cfg.ShardManager != cortex.ShardManager {
		t.Fatalf("ShardManager not from Cortex")
	}
	if cfg.TaskExecutor != cortex.TaskExecutor {
		t.Fatalf("TaskExecutor not from Cortex")
	}
	if cfg.Workspace != cwd {
		t.Fatalf("Workspace = %q, want %q", cfg.Workspace, cwd)
	}
}
