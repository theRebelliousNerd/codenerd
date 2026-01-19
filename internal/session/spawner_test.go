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
