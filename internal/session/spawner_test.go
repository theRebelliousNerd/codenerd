package session

import (
	"context"
	"sync"
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

	// Spawn() creates and registers but does not start execution. Production
	// callers explicitly `go agent.Run(ctx, task)`. Mirror that or the agent
	// stays in Idle and never transitions to Running.
	go a1.Run(context.Background(), req1.Task)

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

	// Spawner.Spawn() creates and registers but does not start execution —
	// production callers (e.g. JITExecutor.executeAsyncInternal) explicitly
	// start the agent with `go agent.Run(ctx, task)`. Mirror that contract
	// here, otherwise the agent stays in Idle and Wait() polls forever.
	go agent.Run(context.Background(), req.Task)

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

	// Wait for it to stop/fail (bounded so a regression doesn't hang the suite).
	done := make(chan struct{})
	go func() {
		agent.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("agent.Wait did not return within 2s after Stop; final state=%v", agent.GetState())
	}

	state := agent.GetState()
	// After Stop() the mock LLM returns ctx.Err(), so the agent finishes
	// either Completed (if it raced past the LLM call) or Failed.
	if state != SubAgentStateCompleted && state != SubAgentStateFailed {
		t.Errorf("expected Completed or Failed after Stop, got %v", state)
	}
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

// -----------------------------------------------------------------------------
// QA NEGATIVE TESTING
// -----------------------------------------------------------------------------

func TestSpawner_Spawn_EmptyName(t *testing.T) {
	spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())

	req := SpawnRequest{
		Name: "", // Empty name
		Task: "do something",
	}

	agent, err := spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatalf("Spawn should not fail with empty name, but got: %v", err)
	}

	if agent.GetName() != "" {
		t.Errorf("Expected agent name to be empty string, got '%s'", agent.GetName())
	}
}

func TestSpawner_Shutdown_ZeroAgents(t *testing.T) {
	spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())

	// Should not block or panic
	spawner.StopAll()
}

func TestSpawner_StateConflicts_ShutdownConcurrentSpawn(t *testing.T) {
	spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range 100 {
			spawner.Spawn(context.Background(), SpawnRequest{Name: "concurrent"})
		}
	}()

	go func() {
		defer wg.Done()
		for range 100 {
			spawner.StopAll()
		}
	}()

	wg.Wait()
}

// TODO: TEST_GAP: Null/Undefined/Empty - Nil context in Spawn operations causes context.WithCancel(nil) panic in SubAgent.Run()
// TODO: TEST_GAP: Null/Undefined/Empty - Empty Task string ("") behavior should be validated
// TODO: TEST_GAP: Null/Undefined/Empty - Empty/Nil configuration files for specialists
// TODO: TEST_GAP: Null/Undefined/Empty - Null JIT Compiler & Config Factory Interplay
// TODO: TEST_GAP: Type Coercion - Invalid YAML Types in Specialist Config (e.g. string for Timeout, arrays for objects)
// TODO: TEST_GAP: Type Coercion - Intent Category/Verb with unexpected characters (e.g. control chars, null bytes, emojis)
// TODO: TEST_GAP: User Request Extremes - Massive Task payloads (e.g. 50MB string)
// TODO: TEST_GAP: User Request Extremes - Sub-zero or zero Max Active Subagents behavior
// TODO: TEST_GAP: User Request Extremes - Max Config Size Boundary (1048576 vs 1048577 bytes)
// TODO: TEST_GAP: User Request Extremes - Frontier Coding Benchmark with 10,000+ concurrent agents causing lock contention
// TODO: TEST_GAP: State Conflicts - Rapid spawn and immediate context cancellation
// TODO: TEST_GAP: State Conflicts - Concurrent Cleanup() and Stop()
// TODO: TEST_GAP: State Conflicts - Zombie Agents (Leaked memory/goroutines after completion if Cleanup() is never called)

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify Spawner handles a Nil Context gracefully in Spawn operations without context.WithCancel(nil) panic in SubAgent.Run().
// TODO: TEST_GAP: [Type Coercion] Verify Spawner handles invalid YAML Types in Specialist Config (e.g. string for Timeout, arrays for objects).
// TODO: TEST_GAP: [User Request Extremes] Verify Spawner handles 10,000+ concurrent spawn requests gracefully rejecting with capacity errors.
// TODO: TEST_GAP: [State Conflicts] Verify that rapid spawn and immediate shutdown calls don't result in zombie goroutines due to race conditions.
