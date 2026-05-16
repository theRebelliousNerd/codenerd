package session

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// mockJITCompiler returns a successful compilation.
type mockJITCompiler struct {
	fail bool
}

func (m *mockJITCompiler) Compile(ctx context.Context, compilationCtx *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	if m.fail {
		return nil, fmt.Errorf("simulated compile failure")
	}
	return &prompt.CompilationResult{}, nil
}

// mockConfigFactory generates an empty config.
type mockConfigFactory struct{}

type sleepyLLMClient struct{
	MockLLMClient
}

func (s *sleepyLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	time.Sleep(50 * time.Millisecond)
	return &types.LLMToolResponse{Text: `{"surface_response":"done","control_packet":{}}`}, nil
}

func (s *sleepyLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	time.Sleep(50 * time.Millisecond)
	return `{"surface_response":"done","control_packet":{}}`, nil
}

func (s *sleepyLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	time.Sleep(50 * time.Millisecond)
	return `{"surface_response":"done","control_packet":{}}`, nil
}

type blockyLLMClient struct{
	MockLLMClient
	block chan struct{}
}

func (s *blockyLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	<-s.block
	return &types.LLMToolResponse{Text: `{"surface_response":"done","control_packet":{}}`}, nil
}

func (s *blockyLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	<-s.block
	return `{"surface_response":"done","control_packet":{}}`, nil
}

func (s *blockyLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	<-s.block
	return `{"surface_response":"done","control_packet":{}}`, nil
}

func (m *mockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intentVerbs ...string) (*config.AgentConfig, error) {
	return &config.AgentConfig{}, nil
}

type mockTransducerUT struct{}

func (m *mockTransducerUT) ParseIntent(ctx context.Context, p string) (perception.Intent, error) {
	return perception.Intent{}, nil
}

func (m *mockTransducerUT) ParseIntentWithGCD(ctx context.Context, p string, history []perception.ConversationTurn, something int) (perception.Intent, []string, error) {
	return perception.Intent{}, nil, nil
}

func (m *mockTransducerUT) ParseIntentWithContext(ctx context.Context, p string, history []perception.ConversationTurn) (perception.Intent, error) {
	return perception.Intent{}, nil
}

func (m *mockTransducerUT) ResolveFocus(ctx context.Context, p string, history []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}

func (m *mockTransducerUT) SetPromptAssembler(pa *articulation.PromptAssembler) {}

func (m *mockTransducerUT) SetStrategicContext(context string) {}

// -----------------------------------------------------------------------------
// Marathon 5: Session Spawner (Remediation)
// -----------------------------------------------------------------------------

func TestSpawner_MaxActiveSubagents_TOCTOU(t *testing.T) {
	// State Conflicts: Verify Spawn TOCTOU mitigation with concurrent requests
	blocker := make(chan struct{})
	defer close(blocker)

	spawner := NewSpawner(
		&MockKernel{}, &MockVirtualStore{}, &blockyLLMClient{block: blocker}, &mockJITCompiler{fail: false}, &mockConfigFactory{}, &mockTransducerUT{},
		SpawnerConfig{MaxActiveSubagents: 10},
	)

	var wg sync.WaitGroup
	var successfulSpawns int32

	// Thundering herd of 100 requests, limit is 10
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := SpawnRequest{
				Name:       fmt.Sprintf("agent-%d", idx),
				Task:       "test",
				Type:       SubAgentTypeEphemeral,
				IntentVerb: "/test",
			}
			_, err := spawner.Spawn(context.Background(), req)
			if err == nil {
				atomic.AddInt32(&successfulSpawns, 1)
			}
		}(i)
	}

	wg.Wait()

	if successfulSpawns > 10 {
		t.Errorf("expected at most 10 spawns, got %d", successfulSpawns)
	}
}

func TestSpawner_MassiveSpawn_Performance(t *testing.T) {
	// User Request Extremes: Verify performance/stability when concurrently spawning 10,000 subagents
	// Limit to a smaller bound for unit tests, e.g., 1000, and max active = 1000.
	spawner := NewSpawner(
		&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &mockJITCompiler{fail: false}, &mockConfigFactory{}, &mockTransducerUT{},
		SpawnerConfig{MaxActiveSubagents: 1000},
	)

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := SpawnRequest{
				Name:       fmt.Sprintf("agent-%d", idx),
				Task:       "test",
				Type:       SubAgentTypeEphemeral,
				IntentVerb: "/test",
			}
			_, _ = spawner.Spawn(context.Background(), req)
		}(i)
	}
	wg.Wait()

	if time.Since(start) > 5*time.Second {
		t.Errorf("spawning 1000 agents took too long: %v", time.Since(start))
	}
}

func TestSpawner_GenerateConfigFallback(t *testing.T) {
	// State Conflicts: Verify generateConfig fallback when JIT compilation fails
	spawner := NewSpawner(
		&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &mockJITCompiler{fail: true}, &mockConfigFactory{}, &mockTransducerUT{},
		SpawnerConfig{MaxActiveSubagents: 10},
	)

	req := SpawnRequest{
		Name:       "fallback-agent",
		Task:       "test",
		Type:       SubAgentTypeEphemeral,
		IntentVerb: "/test",
	}

	agent, err := spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatalf("expected spawn to succeed despite JIT failure, got %v", err)
	}
	if agent == nil {
		t.Errorf("expected fallback Agent to be non-nil")
	}
}

func TestSpawner_StopAllConcurrentWithCleanupAndSpawn(t *testing.T) {
	// State Conflicts: Verify StopAll concurrent with Cleanup and Spawn
	spawner := NewSpawner(
		&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &mockJITCompiler{fail: false}, &mockConfigFactory{}, &mockTransducerUT{},
		SpawnerConfig{MaxActiveSubagents: 100},
	)

	var wg sync.WaitGroup

	// Spawner
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			req := SpawnRequest{
				Name:       fmt.Sprintf("agent-%d", i),
				Task:       "test",
				Type:       SubAgentTypeEphemeral,
				IntentVerb: "/test",
			}
			_, _ = spawner.Spawn(context.Background(), req)
		}
	}()

	// Stopper
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			spawner.StopAll()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Cleaner
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			spawner.Cleanup()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()
	// Just ensure no panics
}

func TestSpawner_GetByNamePredictability(t *testing.T) {
	// State Conflicts: Verify Spawner.GetByName predictability
	spawner := NewSpawner(
		&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &mockJITCompiler{fail: false}, &mockConfigFactory{}, &mockTransducerUT{},
		SpawnerConfig{MaxActiveSubagents: 10},
	)

	req1 := SpawnRequest{Name: "duplicate", Task: "test", Type: SubAgentTypeEphemeral}
	agent1, _ := spawner.Spawn(context.Background(), req1)
	if agent1 != nil {} // Keep unused warning away

	// wait for agent to start
	time.Sleep(10 * time.Millisecond)

	req2 := SpawnRequest{Name: "duplicate", Task: "test", Type: SubAgentTypeEphemeral}
	_, _ = spawner.Spawn(context.Background(), req2)

	found, ok := spawner.GetByName("duplicate")
	if !ok || found == nil {
		t.Fatalf("expected to find agent by name")
	}
	// Note: since the map is unordered, we just want to ensure it doesn't crash
	// and returns an active one.
}
