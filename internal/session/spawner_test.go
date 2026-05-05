package session

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestSpawner_Spawn_Success(t *testing.T) {
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)

	req := SpawnRequest{
		Name:       "test-agent",
		Task:       "do something",
		Type:       SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	agent, err := spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected agent, got nil")
	}

	if agent.GetName() != "test-agent" {
		t.Errorf("Expected name 'test-agent', got '%s'", agent.GetName())
	}

	// Verify it's in the map
	if _, ok := spawner.Get(agent.GetID()); !ok {
		t.Error("Agent not found in spawner map")
	}
}

func TestSpawner_Spawn_MaxLimit(t *testing.T) {
	cfg := DefaultSpawnerConfig()
	cfg.MaxActiveSubagents = 1

	// Mock that blocks forever
	mockLLM := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(1 * time.Second): // Wait longer than test execution
				return "Done", nil
			}
		},
	}

	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		cfg,
	)

	// Spawn 1
	req1 := SpawnRequest{Name: "agent1", Task: "task1", Type: SubAgentTypeEphemeral}
	a1, err := spawner.Spawn(context.Background(), req1)
	if err != nil {
		t.Fatalf("First spawn failed: %v", err)
	}

	// Wait for agent1 to be running
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	for {
		if a1.GetState() == SubAgentStateRunning {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("Timeout waiting for agent1 to run")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Spawn 2 (should fail)
	req2 := SpawnRequest{Name: "agent2", Task: "task2", Type: SubAgentTypeEphemeral}
	_, err = spawner.Spawn(context.Background(), req2)
	if err == nil {
		t.Error("Expected error for max limit, got nil")
	}
}

func TestSpawner_Lifecycle(t *testing.T) {
	// Setup MockLLM that blocks for a bit so we can test running state
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(50 * time.Millisecond):
				return &types.LLMToolResponse{Text: "Done"}, nil
			}
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(50 * time.Millisecond):
				return "Done", nil
			}
		},
	}

	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)

	req := SpawnRequest{Name: "agent1", Task: "task", Type: SubAgentTypeEphemeral}
	agent, err := spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Test GetByName
	t.Logf("Agent ID: %s, Name: %s", agent.GetID(), agent.GetName())
	a2, ok := spawner.GetByName("agent1")
	if !ok {
		t.Error("GetByName returned false")
		// Debug dump
		for _, a := range spawner.ListActive() {
			t.Logf("Active agent: %s (%s)", a.GetName(), a.GetID())
		}
	} else if a2 != agent {
		t.Error("GetByName returned wrong agent")
	}

	// Test Stop
	err = spawner.Stop(agent.GetID())
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Wait for it to stop/fail
	agent.Wait()

	state := agent.GetState()
	// Depending on timing, it might be Completed (if 50ms passed) or Failed/Completed (if cancelled)
	// Actually Stop() calls agent.Stop() which cancels context.
	// So LLM should return ctx.Err(), loop handles it.

	t.Logf("Final state: %v", state)
}

func TestSpawner_GenerateConfig_NilJITCompilerFallsBackToEmptyConfig(t *testing.T) {
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		nil,
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)

	cfg, err := spawner.generateConfig(context.Background(), SpawnRequest{
		Name:       "test-agent",
		IntentVerb: "/review",
	})
	if err != nil {
		t.Fatalf("generateConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected empty config, got nil")
	}
}

// TODO: TEST_GAP: Null/Empty: Verify Spawn behavior when SpawnRequest.Name is empty.
// TODO: TEST_GAP: Null/Empty: Verify Spawn behavior when SpawnRequest.Task is empty.
// TODO: TEST_GAP: Null/Empty: Verify Stop behavior when given an empty ID.
// TODO: TEST_GAP: Null/Empty: Verify loadSpecialistConfig gracefully handles an empty name.
// TODO: TEST_GAP: Null/Empty: Verify SpawnSpecialist behavior with empty name and malformed path injection (e.g. "../../../etc/passwd").

// TODO: TEST_GAP: Type Coercion: Verify Spawn behavior when an invalid integer is passed for SubAgentType.
// TODO: TEST_GAP: Type Coercion: Verify Spawn behavior when negative or massive Timeout values are supplied.
// TODO: TEST_GAP: Type Coercion: Verify determineAgentName handles unexpected Intent properties.

// TODO: TEST_GAP: User Request Extremes: Verify SpawnSpecialist handles massive config.yaml files.
// TODO: TEST_GAP: User Request Extremes: Verify performance/stability when concurrently spawning 10,000 subagents (checking limit rejection speed).

// TODO: TEST_GAP: State Conflicts: Verify TOCTOU condition where MaxActiveSubagents limit is checked and then another subagent is spawned concurrently.
// TODO: TEST_GAP: State Conflicts: Verify Stop behavior when Stop is called simultaneously from multiple goroutines for the same agent ID.
// TODO: TEST_GAP: State Conflicts: Verify thread safety of Cleanup when called concurrently with Spawn/Stop/ListActive.
// TODO: TEST_GAP: State Conflicts: Verify Spawner.GetByName predictability when multiple active subagents share a name.
// TODO: TEST_GAP: State Conflicts: Verify StopAll concurrent with Cleanup and Spawn.
// TODO: TEST_GAP: State Conflicts: Verify generateConfig falls back completely when both JIT compilation attempts fail and returns an empty AgentConfig.
