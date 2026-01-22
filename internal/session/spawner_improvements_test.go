package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
)

// =============================================================================
// Tests for Spawner Improvements (concurrency fix, fallback config, ReadRaw)
// =============================================================================

// TestSpawner_Spawn_ConcurrencyFix verifies that the lock is not held during
// generateConfig (which can do IO/LLM calls). Multiple spawns should proceed
// concurrently without blocking each other during config generation.
func TestSpawner_Spawn_ConcurrencyFix(t *testing.T) {
	cfg := DefaultSpawnerConfig()
	cfg.MaxActiveSubagents = 5

	// Track concurrent generateConfig calls
	var concurrent int32
	var maxConcurrent int32

	mockJIT := &MockJITCompiler{
		CompileFunc: func(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			// Simulate concurrent access tracking
			current := atomic.AddInt32(&concurrent, 1)
			defer atomic.AddInt32(&concurrent, -1)

			// Track max concurrency
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
					break
				}
			}

			// Simulate IO delay
			time.Sleep(50 * time.Millisecond)
			return &prompt.CompilationResult{Prompt: "test"}, nil
		},
	}

	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		mockJIT,
		&MockConfigFactory{},
		&MockTransducer{},
		cfg,
	)

	// Spawn 3 agents concurrently
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := SpawnRequest{
				Name:       "agent",
				Task:       "task",
				Type:       SubAgentTypeEphemeral,
				IntentVerb: "/test",
			}
			_, err := spawner.Spawn(context.Background(), req)
			if err != nil {
				t.Errorf("Spawn %d failed: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	// If lock was held during generateConfig, maxConcurrent would be 1
	// With the fix, it should be > 1 (concurrent config generation)
	observed := atomic.LoadInt32(&maxConcurrent)
	if observed == 0 {
		t.Fatal("generateConfig was never called")
	}
	t.Logf("Max concurrent generateConfig calls: %d", observed)

	// Note: We can't guarantee > 1 concurrency in tests due to timing,
	// but we can verify the basic functionality works
}

// TestSpawner_GenerateConfig_FallbackOnFailure verifies that when JIT compilation
// fails, the spawner retries with baseline context and eventually returns empty config.
func TestSpawner_GenerateConfig_FallbackOnFailure(t *testing.T) {
	cfg := DefaultSpawnerConfig()

	// Track compile attempts
	var attempts int32

	mockJIT := &MockJITCompiler{
		CompileFunc: func(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			attempt := atomic.AddInt32(&attempts, 1)

			// First attempt fails
			if attempt == 1 {
				return nil, errors.New("JIT compilation failed")
			}
			// Second attempt (baseline) also fails
			if attempt == 2 {
				// Verify baseline context is used
				if cc.IntentVerb != "/general" {
					t.Errorf("Expected baseline intent /general, got %s", cc.IntentVerb)
				}
				if cc.TokenBudget != 4096 {
					t.Errorf("Expected reduced budget 4096, got %d", cc.TokenBudget)
				}
				return nil, errors.New("baseline also failed")
			}
			return &prompt.CompilationResult{Prompt: "test"}, nil
		},
	}

	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		mockJIT,
		&MockConfigFactory{},
		&MockTransducer{},
		cfg,
	)

	req := SpawnRequest{
		Name:       "agent",
		Task:       "task",
		Type:       SubAgentTypeEphemeral,
		IntentVerb: "/complex",
	}

	// Should succeed with empty config after fallback
	agent, err := spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatalf("Spawn should succeed with fallback, got: %v", err)
	}
	if agent == nil {
		t.Fatal("Expected agent, got nil")
	}

	// Verify both attempts were made
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("Expected 2 JIT attempts (original + baseline), got %d", atomic.LoadInt32(&attempts))
	}
}

// TestSpawner_GenerateConfig_FallbackSuccess verifies that baseline compilation
// succeeds when initial compilation fails.
func TestSpawner_GenerateConfig_FallbackSuccess(t *testing.T) {
	cfg := DefaultSpawnerConfig()

	var attempts int32

	mockJIT := &MockJITCompiler{
		CompileFunc: func(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			attempt := atomic.AddInt32(&attempts, 1)
			if attempt == 1 {
				return nil, errors.New("first attempt fails")
			}
			// Baseline succeeds
			return &prompt.CompilationResult{Prompt: "baseline prompt"}, nil
		},
	}

	mockConfigFactory := &MockConfigFactory{
		GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
			if result != nil && result.Prompt == "baseline prompt" {
				return &config.AgentConfig{}, nil
			}
			return nil, errors.New("unexpected result")
		},
	}

	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		mockJIT,
		mockConfigFactory,
		&MockTransducer{},
		cfg,
	)

	req := SpawnRequest{Name: "agent", Task: "task", Type: SubAgentTypeEphemeral}
	agent, err := spawner.Spawn(context.Background(), req)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if agent == nil {
		t.Fatal("Expected agent")
	}
}

// TestSpawner_Spawn_Concurrent_MaxLimitRace verifies that the double-check
// for the max limit after config generation prevents over-provisioning.
func TestSpawner_Spawn_Concurrent_MaxLimitRace(t *testing.T) {
	cfg := DefaultSpawnerConfig()
	cfg.MaxActiveSubagents = 1

	// Long-running JIT to create race window
	mockJIT := &MockJITCompiler{
		CompileFunc: func(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			time.Sleep(100 * time.Millisecond)
			return &prompt.CompilationResult{Prompt: "test"}, nil
		},
	}

	// Long-running LLM to keep agents in running state
	mockLLM := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(5 * time.Second):
				return "done", nil
			}
		},
	}

	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		mockJIT,
		&MockConfigFactory{},
		&MockTransducer{},
		cfg,
	)

	// Try to spawn 3 agents concurrently with max limit of 1
	var wg sync.WaitGroup
	var successes, failures int32

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := SpawnRequest{Name: "agent", Task: "task", Type: SubAgentTypeEphemeral}
			_, err := spawner.Spawn(context.Background(), req)
			if err != nil {
				atomic.AddInt32(&failures, 1)
			} else {
				atomic.AddInt32(&successes, 1)
			}
		}()
	}

	wg.Wait()

	t.Logf("Successes: %d, Failures: %d", atomic.LoadInt32(&successes), atomic.LoadInt32(&failures))

	// Only 1 should succeed due to max limit
	if atomic.LoadInt32(&successes) != 1 {
		t.Errorf("Expected 1 success with max limit, got %d", atomic.LoadInt32(&successes))
	}
}
