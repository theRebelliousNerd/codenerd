package session

import (
	"context"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

func newSessionWiringExecutor() *Executor {
	return NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
}

func TestExecutor_SetOuroborosRegistry_NilIsSafe(t *testing.T) {
	e := newSessionWiringExecutor()

	// Must not panic; a nil registry clears the slot.
	e.SetOuroborosRegistry(nil)
	if e.OuroborosRegistry() != nil {
		t.Fatal("OuroborosRegistry() should be nil after SetOuroborosRegistry(nil)")
	}

	registry := core.NewToolRegistry(t.TempDir())
	e.SetOuroborosRegistry(registry)
	if e.OuroborosRegistry() != registry {
		t.Fatal("OuroborosRegistry() should return the configured registry")
	}

	// Setting nil again clears rather than panicking.
	e.SetOuroborosRegistry(nil)
	if e.OuroborosRegistry() != nil {
		t.Fatal("OuroborosRegistry() should be nil after clearing with SetOuroborosRegistry(nil)")
	}
}

func TestExecutor_CloneForTask_InheritsSessionID(t *testing.T) {
	parent := newSessionWiringExecutor()
	parent.SetSessionID("session-parent-1")
	parent.SetHistory([]perception.ConversationTurn{
		{Role: "user", Content: "do something"},
		{Role: "assistant", Content: "done"},
	})
	registry := core.NewToolRegistry(t.TempDir())
	parent.SetOuroborosRegistry(registry)

	clone := parent.CloneForTask()
	if clone == nil {
		t.Fatal("CloneForTask returned nil")
	}
	if got := clone.SessionID(); got != "session-parent-1" {
		t.Errorf("clone SessionID() = %q, want %q", got, "session-parent-1")
	}
	if clone.OuroborosRegistry() != registry {
		t.Error("clone should inherit the parent's Ouroboros registry")
	}
	// History isolation is preserved: the clone starts empty even though the
	// parent has history, and the parent keeps its own.
	if got := len(clone.GetHistory()); got != 0 {
		t.Errorf("clone history length = %d, want 0 (isolation)", got)
	}
	if got := len(parent.GetHistory()); got != 2 {
		t.Errorf("parent history length = %d, want 2 (untouched)", got)
	}
}

func TestSpawner_ForwardsSessionIDAndRegistry(t *testing.T) {
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)
	spawner.SetSessionID("session-spawn-1")
	registry := core.NewToolRegistry(t.TempDir())
	spawner.SetOuroborosRegistry(registry)

	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name:       "wiring-agent",
		Task:       "do wiring work",
		Type:       SubAgentTypeEphemeral,
		IntentVerb: "/implement",
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if agent == nil || agent.executor == nil {
		t.Fatal("expected non-nil agent and executor")
	}
	if got := agent.executor.SessionID(); got != "session-spawn-1" {
		t.Errorf("subagent executor SessionID() = %q, want %q", got, "session-spawn-1")
	}
	if agent.executor.OuroborosRegistry() != registry {
		t.Error("subagent executor should inherit the spawner's Ouroboros registry")
	}
}

func TestSpawner_SetOuroborosRegistry_NilIsSafe(t *testing.T) {
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)
	// Must not panic; subagents then run without generated tools.
	spawner.SetOuroborosRegistry(nil)

	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name:       "nil-registry-agent",
		Task:       "do work without generated tools",
		Type:       SubAgentTypeEphemeral,
		IntentVerb: "/general",
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if agent.executor.OuroborosRegistry() != nil {
		t.Error("subagent executor registry should be nil when spawner has none")
	}
}
