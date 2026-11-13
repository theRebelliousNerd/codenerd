//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// MockTaskDelegator is used to simulate specific JITExecutor states and error patterns across the boundary.
type MockTaskDelegator struct {
	executeFunc func(ctx context.Context, intentVerb, task string) (string, error)
}

func (m *MockTaskDelegator) Execute(ctx context.Context, intentVerb, task string) (string, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, intentVerb, task)
	}
	return "mock success", nil
}

func waitForCompletion(t *testing.T, sm *shards.ShardManager, id string) types.ShardResult {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		res, ok := sm.GetResult(id)
		if ok {
			return res
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Timeout waiting for shard %s to complete", id)
	return types.ShardResult{}
}

func getState(sm *shards.ShardManager, id string) types.ShardState {
	agent, ok := sm.GetRunningShardByConfigName(id)
	if ok {
		return agent.GetState()
	}
	return types.ShardStateCompleted
}

// TestE2E_Boundary_Delegation_Smoke tests basic delegation from ShardManager to TaskDelegator
func TestE2E_Boundary_Delegation_Smoke(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			return "smoke result", nil
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("my_agent", types.ShardConfig{Name: "my_agent", Type: types.ShardTypeUser})

	agentID, err := sm.SpawnAsyncWithContext(ctx, "my_agent", "do task", nil)
	if err != nil {
		t.Fatalf("Failed to spawn async: %v", err)
	}

	res := waitForCompletion(t, sm, agentID)

	if res.Result != "smoke result" {
		t.Errorf("Expected 'smoke result', got %q", res.Result)
	}
	if res.Error != nil {
		t.Fatalf("Unexpected error from delegated agent: %v", res.Error)
	}

	if getState(sm, agentID) != types.ShardStateCompleted {
		t.Errorf("Expected agent state to be Completed, got %v", getState(sm, agentID))
	}
}

// TestE2E_Boundary_Delegation_ErrorPropagation tests that errors in JITExecutor correctly bubble back
func TestE2E_Boundary_Delegation_ErrorPropagation(t *testing.T) {
	sm := shards.NewShardManager()
	expectedErr := errors.New("JIT compilation failed")
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			return "", expectedErr
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("broken_agent", types.ShardConfig{Name: "broken_agent", Type: types.ShardTypeUser})

	agentID, err := sm.SpawnAsyncWithContext(ctx, "broken_agent", "do task", nil)
	if err != nil {
		t.Fatalf("Failed to spawn async: %v", err)
	}

	res := waitForCompletion(t, sm, agentID)

	if !errors.Is(res.Error, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, res.Error)
	}
}

// TestE2E_Boundary_Delegation_ContextCancellation tests temporal failure handling via context
func TestE2E_Boundary_Delegation_ContextCancellation(t *testing.T) {
	sm := shards.NewShardManager()
	started := make(chan struct{})
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			close(started)
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sm.DefineProfile("slow_agent", types.ShardConfig{Name: "slow_agent", Type: types.ShardTypeUser})
	agentID, err := sm.SpawnAsyncWithContext(ctx, "slow_agent", "do task", nil)
	if err != nil {
		t.Fatalf("Failed to spawn async: %v", err)
	}

	<-started
	cancel() // Trigger cancellation

	res := waitForCompletion(t, sm, agentID)

	if !errors.Is(res.Error, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", res.Error)
	}
}

// TestE2E_Boundary_Delegation_NilDelegator tests the initialization contract
func TestE2E_Boundary_Delegation_NilDelegator(t *testing.T) {
	sm := shards.NewShardManager()

	ctx := context.Background()
	sm.DefineProfile("some_agent", types.ShardConfig{Name: "some_agent", Type: types.ShardTypeUser})
	_, err := sm.SpawnAsyncWithContext(ctx, "some_agent", "do task", nil)

	if err == nil {
		t.Fatal("Expected error when spawning user agent without task delegator")
	}

	if !strings.Contains(err.Error(), "no factory registered and no task delegator wired") {
		t.Errorf("Expected specific missing delegator error, got: %v", err)
	}
}

// TestE2E_Boundary_Delegation_RefuseImageShard tests semantic routing contract
func TestE2E_Boundary_Delegation_RefuseImageShard(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("image_generator", types.ShardConfig{Name: "image_generator", Type: types.ShardTypeSystem})
	_, err := sm.SpawnAsyncWithContext(ctx, "image_generator", "draw a cat", nil)

	if err == nil {
		t.Fatal("Expected error when attempting to delegate an image shard")
	}

	if !strings.Contains(err.Error(), "image shard \"image_generator\" has no registered factory") ||
	   !strings.Contains(err.Error(), "cannot be delegated") {
		t.Errorf("Expected specific refusal error, got: %v", err)
	}
}

// TestE2E_Boundary_Delegation_HollowOutputPrevention ensures the legacy fake success bug is avoided
func TestE2E_Boundary_Delegation_HollowOutputPrevention(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			return "Real task completed", nil
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("real_agent", types.ShardConfig{Name: "real_agent", Type: types.ShardTypeUser})

	agentID, err := sm.SpawnAsyncWithContext(ctx, "real_agent", "do task", nil)
	if err != nil {
		t.Fatalf("Failed to spawn async: %v", err)
	}

	res := waitForCompletion(t, sm, agentID)

	if res.Result == "BaseShardAgent execution" {
		t.Fatal("CRITICAL: Received hollow output 'BaseShardAgent execution'. The fallback bug has returned.")
	}
	if res.Result != "Real task completed" {
		t.Errorf("Expected 'Real task completed', got %q", res.Result)
	}
}

// TestE2E_Boundary_Delegation_StateRaceCondition tests for race conditions on agent state
func TestE2E_Boundary_Delegation_StateRaceCondition(t *testing.T) {
	sm := shards.NewShardManager()

	var wg sync.WaitGroup
	startRace := make(chan struct{})

	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			<-startRace
			time.Sleep(10 * time.Millisecond)
			return "done", nil
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("race_agent", types.ShardConfig{Name: "race_agent", Type: types.ShardTypeUser})

	agentID, err := sm.SpawnAsyncWithContext(ctx, "race_agent", "do task", nil)
	if err != nil {
		t.Fatalf("Failed to spawn async: %v", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = getState(sm, agentID)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	close(startRace)
	wg.Wait()

	waitForCompletion(t, sm, agentID)
}

// TestE2E_Boundary_Delegation_ConcurrentFlood tests resource exhaustion
func TestE2E_Boundary_Delegation_ConcurrentFlood(t *testing.T) {
	sm := shards.NewShardManager()

	var activeCount int32

	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			atomic.AddInt32(&activeCount, 1)
			defer atomic.AddInt32(&activeCount, -1)
			time.Sleep(5 * time.Millisecond)
			return "done", nil
		},
	}
	sm.SetTaskDelegator(delegator)

	numAgents := 100
	var spawnWg sync.WaitGroup
	spawnWg.Add(numAgents)

	ctx := context.Background()

	for i := 0; i < numAgents; i++ {
		go func(id int) {
			defer spawnWg.Done()
			profileName := fmt.Sprintf("agent_%d", id)
			sm.DefineProfile(profileName, types.ShardConfig{Name: profileName, Type: types.ShardTypeUser})

			agentID, err := sm.SpawnAsyncWithContext(ctx, profileName, "do task", nil)
			if err != nil {
				t.Errorf("Spawn failed: %v", err)
				return
			}

			res := waitForCompletion(t, sm, agentID)
			if res.Error != nil {
				t.Errorf("Execution error: %v", res.Error)
			}
		}(i)
	}

	spawnWg.Wait()

	if val := atomic.LoadInt32(&activeCount); val != 0 {
		t.Errorf("Expected 0 active delegators, got %d. Goroutine leak likely.", val)
	}
}

// TestE2E_Boundary_Delegation_PanicRecovery tests if a panic in JITExecutor brings down the ShardManager
func TestE2E_Boundary_Delegation_PanicRecovery(t *testing.T) {
	sm := shards.NewShardManager()

	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			panic("simulated JITExecutor panic")
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("panic_agent", types.ShardConfig{Name: "panic_agent", Type: types.ShardTypeUser})

	agentID, err := sm.SpawnAsyncWithContext(ctx, "panic_agent", "do task", nil)
	if err != nil {
		t.Fatalf("Failed to spawn async: %v", err)
	}

	res := waitForCompletion(t, sm, agentID)

	if res.Error == nil {
		t.Fatal("Expected error from panic recovery, got nil")
	}
	if !strings.Contains(res.Error.Error(), "simulated JITExecutor panic") {
		t.Errorf("Expected panic message in error, got: %v", res.Error)
	}
}

// TestE2E_Boundary_Delegation_ZeroToolAvailability checks the tool contract
func TestE2E_Boundary_Delegation_ZeroToolAvailability(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			return "", errors.New("zero tools available for task requiring effect")
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("tool_agent", types.ShardConfig{Name: "tool_agent", Type: types.ShardTypeUser})

	agentID, err := sm.SpawnAsyncWithContext(ctx, "tool_agent", "do task", nil)
	if err != nil {
		t.Fatalf("Failed to spawn async: %v", err)
	}

	res := waitForCompletion(t, sm, agentID)

	if res.Error == nil {
		t.Fatalf("Expected error for missing tools, got nil")
	}
}

// TestE2E_Boundary_Delegation_OutOfOrderCompletion checks asynchronous handling
func TestE2E_Boundary_Delegation_OutOfOrderCompletion(t *testing.T) {
	sm := shards.NewShardManager()

	startedFirst := make(chan struct{})
	finishFirst := make(chan struct{})

	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			if task == "first" {
				close(startedFirst)
				<-finishFirst // intentionally hang
				return "first done", nil
			}
			return "second done", nil
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("order_agent", types.ShardConfig{Name: "order_agent", Type: types.ShardTypeUser})

	id1, _ := sm.SpawnAsyncWithContext(ctx, "order_agent", "first", nil)
	<-startedFirst // ensure first is running

	id2, _ := sm.SpawnAsyncWithContext(ctx, "order_agent", "second", nil)

	res2 := waitForCompletion(t, sm, id2)
	if res2.Result != "second done" {
		t.Errorf("Expected second done, got %v", res2.Result)
	}

	close(finishFirst) // let first finish
	res1 := waitForCompletion(t, sm, id1)
	if res1.Result != "first done" {
		t.Errorf("Expected first done, got %v", res1.Result)
	}
}

// TestE2E_Boundary_Delegation_DoubleCancellation checks reentrancy safety
func TestE2E_Boundary_Delegation_DoubleCancellation(t *testing.T) {
	sm := shards.NewShardManager()
	started := make(chan struct{})
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			close(started)
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx, cancel := context.WithCancel(context.Background())

	sm.DefineProfile("cancel_agent", types.ShardConfig{Name: "cancel_agent", Type: types.ShardTypeUser})
	agentID, _ := sm.SpawnAsyncWithContext(ctx, "cancel_agent", "do task", nil)

	<-started
	cancel() // first
	cancel() // second

	res := waitForCompletion(t, sm, agentID)
	if !errors.Is(res.Error, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", res.Error)
	}
}

// TestE2E_Boundary_Delegation_Recovery checks behavior after an error
func TestE2E_Boundary_Delegation_Recovery(t *testing.T) {
	sm := shards.NewShardManager()

	attempt := 0
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			attempt++
			if attempt == 1 {
				return "", errors.New("transient API failure")
			}
			return "recovered success", nil
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("recovery_agent", types.ShardConfig{Name: "recovery_agent", Type: types.ShardTypeUser})

	id1, _ := sm.SpawnAsyncWithContext(ctx, "recovery_agent", "do task", nil)
	res1 := waitForCompletion(t, sm, id1)
	if res1.Error == nil {
		t.Fatalf("Expected error on first attempt")
	}

	id2, _ := sm.SpawnAsyncWithContext(ctx, "recovery_agent", "do task", nil)
	res2 := waitForCompletion(t, sm, id2)
	if res2.Result != "recovered success" {
		t.Errorf("Expected recovered success, got %v", res2.Result)
	}
}

// TestE2E_Boundary_Delegation_PanicResultMarshalling checks transition boundary safety
func TestE2E_Boundary_Delegation_PanicResultMarshalling(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			panic("panic during marshalling")
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("marshaling_agent", types.ShardConfig{Name: "marshaling_agent", Type: types.ShardTypeUser})

	agentID, _ := sm.SpawnAsyncWithContext(ctx, "marshaling_agent", "do task", nil)
	res := waitForCompletion(t, sm, agentID)

	if res.Error == nil {
		t.Fatalf("Expected error from panic recovery, got nil")
	}
	if !strings.Contains(res.Error.Error(), "panic during marshalling") {
		t.Errorf("Expected specific panic error, got: %v", res.Error)
	}
}

// TestE2E_Boundary_Delegation_MalformedResult checks resilient parsing
func TestE2E_Boundary_Delegation_MalformedResult(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			return "{ invalid_json: [}", nil
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("malformed_agent", types.ShardConfig{Name: "malformed_agent", Type: types.ShardTypeUser})

	agentID, _ := sm.SpawnAsyncWithContext(ctx, "malformed_agent", "do task", nil)
	res := waitForCompletion(t, sm, agentID)

	if res.Result != "{ invalid_json: [}" {
		t.Errorf("Expected raw malformed string, got %v", res.Result)
	}
}

// TestE2E_Boundary_Delegation_EmptyTask checks boundary tolerance for empty input
func TestE2E_Boundary_Delegation_EmptyTask(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			if task == "" {
				return "", errors.New("empty task")
			}
			return "success", nil
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("empty_agent", types.ShardConfig{Name: "empty_agent", Type: types.ShardTypeUser})

	agentID, _ := sm.SpawnAsyncWithContext(ctx, "empty_agent", "", nil)
	res := waitForCompletion(t, sm, agentID)

	if res.Error == nil {
		t.Fatalf("Expected error for empty task")
	}
}

// TestE2E_Boundary_Delegation_RepeatedSpawns checks state management robustness
func TestE2E_Boundary_Delegation_RepeatedSpawns(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			return "done " + task, nil
		},
	}
	sm.SetTaskDelegator(delegator)
	ctx := context.Background()
	sm.DefineProfile("repeat_agent", types.ShardConfig{Name: "repeat_agent", Type: types.ShardTypeUser})

	for i := 0; i < 5; i++ {
		id, _ := sm.SpawnAsyncWithContext(ctx, "repeat_agent", fmt.Sprintf("task%d", i), nil)
		res := waitForCompletion(t, sm, id)
		if res.Result != fmt.Sprintf("done task%d", i) {
			t.Errorf("Expected done task%d, got %v", i, res.Result)
		}
	}
}

// TestE2E_Boundary_Delegation_ExecutorResourceContention simulates slow network I/O
func TestE2E_Boundary_Delegation_ExecutorResourceContention(t *testing.T) {
	sm := shards.NewShardManager()
	var inProgress int32
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			atomic.AddInt32(&inProgress, 1)
			defer atomic.AddInt32(&inProgress, -1)
			// simulate slow LLM API call holding connection
			time.Sleep(50 * time.Millisecond)
			return "success", nil
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("contention_agent", types.ShardConfig{Name: "contention_agent", Type: types.ShardTypeUser})

	id1, _ := sm.SpawnAsyncWithContext(ctx, "contention_agent", "task1", nil)
	id2, _ := sm.SpawnAsyncWithContext(ctx, "contention_agent", "task2", nil)

	res1 := waitForCompletion(t, sm, id1)
	res2 := waitForCompletion(t, sm, id2)

	if res1.Error != nil || res2.Error != nil {
		t.Fatalf("Expected both to succeed despite contention")
	}
}

// TestE2E_Boundary_Delegation_JITContextTimeout ensures the executor honors context expiration
func TestE2E_Boundary_Delegation_JITContextTimeout(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return "slow result", nil
			}
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	sm.DefineProfile("timeout_agent", types.ShardConfig{Name: "timeout_agent", Type: types.ShardTypeUser})
	agentID, _ := sm.SpawnAsyncWithContext(ctx, "timeout_agent", "do task", nil)

	res := waitForCompletion(t, sm, agentID)
	if !errors.Is(res.Error, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded, got %v", res.Error)
	}
}

// TestE2E_Boundary_Delegation_PartialPipelineFailure checks intermediate state safety
func TestE2E_Boundary_Delegation_PartialPipelineFailure(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			// Simulates parsing the prompt OK, but failing midway
			return "", fmt.Errorf("tool execution failed halfway")
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("partial_agent", types.ShardConfig{Name: "partial_agent", Type: types.ShardTypeUser})

	agentID, _ := sm.SpawnAsyncWithContext(ctx, "partial_agent", "do complex task", nil)
	res := waitForCompletion(t, sm, agentID)

	if res.Error == nil {
		t.Fatalf("Expected error for partial failure")
	}
	if !strings.Contains(res.Error.Error(), "tool execution failed halfway") {
		t.Errorf("Expected specific error, got %v", res.Error)
	}
}

// TestE2E_Boundary_Delegation_MissingDependency simulates campaign missing prior outputs
func TestE2E_Boundary_Delegation_MissingDependency(t *testing.T) {
	sm := shards.NewShardManager()
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			return "", errors.New("required context / dependency missing")
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("dep_agent", types.ShardConfig{Name: "dep_agent", Type: types.ShardTypeUser})

	agentID, _ := sm.SpawnAsyncWithContext(ctx, "dep_agent", "use missing fact", nil)
	res := waitForCompletion(t, sm, agentID)

	if res.Error == nil {
		t.Fatalf("Expected error for missing dependency")
	}
}

// TestE2E_Boundary_Delegation_MassivePayload ensures boundaries don't OOM silently
func TestE2E_Boundary_Delegation_MassivePayload(t *testing.T) {
	sm := shards.NewShardManager()
	massiveString := strings.Repeat("A", 1024*1024*10) // 10MB
	delegator := &MockTaskDelegator{
		executeFunc: func(ctx context.Context, intentVerb, task string) (string, error) {
			return massiveString, nil
		},
	}
	sm.SetTaskDelegator(delegator)

	ctx := context.Background()
	sm.DefineProfile("heavy_agent", types.ShardConfig{Name: "heavy_agent", Type: types.ShardTypeUser})

	agentID, _ := sm.SpawnAsyncWithContext(ctx, "heavy_agent", "do heavy task", nil)
	res := waitForCompletion(t, sm, agentID)

	if len(res.Result) != len(massiveString) {
		t.Errorf("Expected string of length %d, got %d", len(massiveString), len(res.Result))
	}
}
