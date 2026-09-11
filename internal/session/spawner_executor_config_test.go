package session

import (
	"context"
	"testing"
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

	// Build a non-default budget mirroring .nerd/config.json core_limits.
	cfg := DefaultExecutorConfig()
	cfg.MaxToolIterations = 24
	cfg.MaxToolCalls = 120
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
	if got.MaxToolIterations != 24 {
		t.Errorf("MaxToolIterations = %d, want 24 (inherited from spawner)", got.MaxToolIterations)
	}
	if got.MaxToolCalls != 120 {
		t.Errorf("MaxToolCalls = %d, want 120 (inherited from spawner)", got.MaxToolCalls)
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
	if got.MaxToolIterations != want.MaxToolIterations {
		t.Errorf("MaxToolIterations = %d, want default %d when spawner has no config", got.MaxToolIterations, want.MaxToolIterations)
	}
	if got.MaxToolIterations != 8 {
		t.Errorf("MaxToolIterations = %d, want exactly 8 as DefaultExecutorConfig", got.MaxToolIterations)
	}
	if got.MaxToolCalls != want.MaxToolCalls {
		t.Errorf("MaxToolCalls = %d, want default %d", got.MaxToolCalls, want.MaxToolCalls)
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
	if agent.executor.config.MaxToolIterations != DefaultExecutorConfig().MaxToolIterations {
		t.Errorf("MaxToolIterations = %d, want default %d after nil config", agent.executor.config.MaxToolIterations, DefaultExecutorConfig().MaxToolIterations)
	}
}
