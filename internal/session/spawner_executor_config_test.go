package session

import (
	"context"
	"testing"
	"time"
)

func TestSpawner_Spawn_InheritsExecutorConfig(t *testing.T) {
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)

	// A non-default executor config. It carries no tool-call or round
	// ceilings — there are none — so what has to survive the spawn is the
	// workspace and the wall-clock constraints.
	cfg := DefaultExecutorConfig()
	cfg.RepairMaxAttempts = 24
	cfg.ToolTimeout = 120 * time.Second
	cfg.WorkspaceRoot = "/tmp/test-workspace"

	spawner.SetExecutorConfig(&cfg)

	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name:       "budget-agent",
		Task:       "do budgeting work",
		Type:       SubAgentTypeEphemeral,
		IntentVerb: "/implement",
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if agent == nil || agent.executor == nil {
		t.Fatal("expected non-nil agent and executor")
	}

	got := agent.executor.config
	if got.RepairMaxAttempts != 24 {
		t.Errorf("RepairMaxAttempts = %d, want 24 (inherited from spawner)", got.RepairMaxAttempts)
	}
	if got.ToolTimeout != 120*time.Second {
		t.Errorf("ToolTimeout = %v, want 120s (inherited from spawner)", got.ToolTimeout)
	}
	if got.WorkspaceRoot != "/tmp/test-workspace" {
		t.Errorf("WorkspaceRoot = %q, want %q", got.WorkspaceRoot, "/tmp/test-workspace")
	}
}

func TestSpawner_Spawn_DefaultExecutorConfig_WhenUnconfigured(t *testing.T) {
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)
	// Intentionally do NOT call SetExecutorConfig.

	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name:       "default-budget-agent",
		Task:       "do default work",
		Type:       SubAgentTypeEphemeral,
		IntentVerb: "/review",
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if agent == nil || agent.executor == nil {
		t.Fatal("expected non-nil agent and executor")
	}

	want := DefaultExecutorConfig()
	got := agent.executor.config
	if got.ToolTimeout != want.ToolTimeout {
		t.Errorf("ToolTimeout = %v, want default %v when spawner has no config", got.ToolTimeout, want.ToolTimeout)
	}
	if got.RepairMaxAttempts != want.RepairMaxAttempts {
		t.Errorf("RepairMaxAttempts = %d, want default %d", got.RepairMaxAttempts, want.RepairMaxAttempts)
	}
	if got.FinalAnswerReserve != want.FinalAnswerReserve {
		t.Errorf("FinalAnswerReserve = %v, want default %v", got.FinalAnswerReserve, want.FinalAnswerReserve)
	}
}

func TestSpawner_SetExecutorConfig_NilIsNoop(t *testing.T) {
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)

	// Nil must not panic and must leave the spawner in the unconfigured state.
	spawner.SetExecutorConfig(nil)
	if got := spawner.currentExecutorConfig(); got != nil {
		t.Errorf("currentExecutorConfig = %v, want nil after SetExecutorConfig(nil)", got)
	}

	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name:       "nil-config-agent",
		Task:       "task",
		Type:       SubAgentTypeEphemeral,
		IntentVerb: "/general",
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if agent.executor.config.ToolTimeout != DefaultExecutorConfig().ToolTimeout {
		t.Errorf("ToolTimeout = %v, want default %v after nil config", agent.executor.config.ToolTimeout, DefaultExecutorConfig().ToolTimeout)
	}
}
