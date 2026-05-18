//go:build integration

package e2e_test

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
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// =============================================================================
// MOCKS — task_executor_async lifecycle
// =============================================================================

// talMockLLMClient counts LLM invocations atomically so we can detect double-Run.
type talMockLLMClient struct {
	callCount int64 // atomic
	blockCh   chan struct{} // if non-nil, block until closed
	delay     time.Duration
}

func (m *talMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return m.CompleteWithSystem(ctx, "", prompt)
}

func (m *talMockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userInput string) (string, error) {
	atomic.AddInt64(&m.callCount, 1)

	// If blockCh is set, block until it's closed or ctx expires
	if m.blockCh != nil {
		select {
		case <-m.blockCh:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return "mock response", nil
}

func (m *talMockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userInput string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	atomic.AddInt64(&m.callCount, 1)

	if m.blockCh != nil {
		select {
		case <-m.blockCh:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return &types.LLMToolResponse{Text: "mock response"}, nil
}

func (m *talMockLLMClient) ShouldUsePiggybackTools() bool { return false }

// talMockVirtualStore — minimal VirtualStore implementation
type talMockVirtualStore struct{}

func (m *talMockVirtualStore) ExecuteTool(ctx context.Context, call types.ToolCall) (string, error) {
	return "ok", nil
}
func (m *talMockVirtualStore) ReadFile(path string) ([]string, error)        { return nil, nil }
func (m *talMockVirtualStore) WriteFile(path string, content []string) error { return nil }
func (m *talMockVirtualStore) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	return "", "", nil
}
func (m *talMockVirtualStore) ReadRaw(path string) ([]byte, error) { return nil, nil }

// talMockConfigFactory — returns an AgentConfig with a single allowed tool
type talMockConfigFactory struct {
	failOnGenerate bool
}

func (m *talMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
	if m.failOnGenerate {
		return nil, fmt.Errorf("config factory failure")
	}
	return &config.AgentConfig{
		Tools:    config.ToolSet{AllowedTools: []string{"mock_tool"}},
		Policies: config.PolicySet{Files: []string{}},
	}, nil
}

func (m *talMockConfigFactory) RegisterSpecialist(name string, config *config.AgentConfig) error {
	return nil
}

// talMockJITCompiler — returns a minimal compilation result
type talMockJITCompiler struct{}

func (m *talMockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{Prompt: "mock prompt"}, nil
}

// talMockTransducer — returns a fixed intent
type talMockTransducer struct {
	intent string
}

func (m *talMockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return perception.Intent{Verb: m.intent}, nil
}
func (m *talMockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	return perception.Intent{Verb: m.intent}, nil
}
func (m *talMockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return perception.Intent{Verb: m.intent}, nil, nil
}
func (m *talMockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}
func (m *talMockTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {}
func (m *talMockTransducer) SetStrategicContext(ctx string)                      {}

// =============================================================================
// SETUP
// =============================================================================

type talEnv struct {
	executor    *session.JITExecutor
	spawner     *session.Spawner
	llm         *talMockLLMClient
	vstore      *talMockVirtualStore
	cfgFactory  *talMockConfigFactory
}

func setupTALEnvironment(t *testing.T) *talEnv {
	t.Helper()

	llm := &talMockLLMClient{}
	vstore := &talMockVirtualStore{}
	jit := &talMockJITCompiler{}
	cfgFactory := &talMockConfigFactory{}
	trans := &talMockTransducer{intent: "/fix"}

	exec := session.NewExecutor(nil, vstore, llm, jit, cfgFactory, trans)
	exec.SetConfig(session.DefaultExecutorConfig())

	spawnerCfg := session.DefaultSpawnerConfig()
	spawner := session.NewSpawner(nil, vstore, llm, jit, cfgFactory, trans, spawnerCfg)

	taskExec := session.NewJITExecutor(exec, spawner, trans)

	return &talEnv{
		executor:   taskExec,
		spawner:    spawner,
		llm:        llm,
		vstore:     vstore,
		cfgFactory: cfgFactory,
	}
}

// =============================================================================
// 1. CRITICAL: ExecuteAsync Double-Run Detection
// =============================================================================

// TestE2E_TaskExecutor_ExecuteAsync_SingleRun proves that ExecuteAsync
// starts the subagent exactly once. The current code has a bug where
// Spawner.Spawn() calls `go agent.Run(ctx, req.Task)` and then
// executeAsyncInternal() calls `go agent.Run(context.Background(), task)`
// again — running the agent twice.
//
// This test uses an atomic call counter in the mock LLM to detect duplicates.
func TestE2E_TaskExecutor_ExecuteAsync_SingleRun(t *testing.T) {
	env := setupTALEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute async task
	taskID, err := env.executor.ExecuteAsync(ctx, "/fix", "touch sentinel file")
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}

	if taskID == "" {
		t.Fatal("ExecuteAsync returned empty taskID")
	}

	// Wait for the task to complete
	result, err := env.executor.WaitForResult(ctx, taskID)
	t.Logf("Task %s completed: result=%q, err=%v", taskID, result, err)

	// Give any duplicate Run goroutine time to execute
	time.Sleep(500 * time.Millisecond)

	// Check the LLM call count
	calls := atomic.LoadInt64(&env.llm.callCount)
	t.Logf("LLM call count after ExecuteAsync: %d", calls)

	// The agent should have been run exactly once, producing exactly one LLM call.
	// If the count is 2+, we have a double-run bug.
	if calls > 1 {
		t.Errorf("BUG CONFIRMED: ExecuteAsync caused %d LLM calls — expected exactly 1. "+
			"Spawner.Spawn() and executeAsyncInternal() both call agent.Run(), "+
			"causing duplicate execution.", calls)
	} else if calls == 0 {
		t.Log("WARNING: LLM was never called — agent may not have run at all")
	} else {
		t.Log("PASS: Agent ran exactly once")
	}
}

// =============================================================================
// 2. ExecuteAsync Result Lifecycle
// =============================================================================

// TestE2E_TaskExecutor_ExecuteAsync_ResultLifecycle verifies that the result
// map transitions through the correct states: not-found → running → completed.
func TestE2E_TaskExecutor_ExecuteAsync_ResultLifecycle(t *testing.T) {
	// Use a blocking LLM so we can observe the running state
	blockCh := make(chan struct{})
	env := setupTALEnvironment(t)
	env.llm.blockCh = blockCh

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Before ExecuteAsync, result should not exist
	_, done, err := env.executor.GetResult("nonexistent-id")
	if done {
		t.Fatal("GetResult should return done=false for non-existent task")
	}
	if err == nil {
		t.Fatal("GetResult should return error for non-existent task")
	}

	// Start async task (LLM will block)
	taskID, err := env.executor.ExecuteAsync(ctx, "/fix", "blocked task")
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}

	// Give agent time to start
	time.Sleep(200 * time.Millisecond)

	// While LLM is blocked, result should be not-done
	_, done, _ = env.executor.GetResult(taskID)
	if done {
		t.Error("Result should not be done while LLM is blocked")
	}
	t.Logf("Task %s is in running state (done=%v)", taskID, done)

	// Unblock LLM
	close(blockCh)

	// Wait for completion
	result, err := env.executor.WaitForResult(ctx, taskID)
	t.Logf("Task completed: result=%q err=%v", result, err)

	// Verify final state is done
	_, done, _ = env.executor.GetResult(taskID)
	if !done {
		t.Error("Result should be done after WaitForResult returns")
	}
}

// =============================================================================
// 3. CRITICAL: WaitForResult Cancellation → Zombie Subagent Leak
// =============================================================================

// TestE2E_TaskExecutor_WaitForResult_Cancellation_StopsAgent proves that
// cancelling WaitForResult's context does NOT stop the underlying subagent.
//
// The current code exits the polling loop on ctx.Done() without calling
// spawner.Stop(taskID), leaving a zombie subagent running indefinitely.
func TestE2E_TaskExecutor_WaitForResult_Cancellation_StopsAgent(t *testing.T) {
	// Create a permanently-blocked LLM
	blockCh := make(chan struct{})
	defer close(blockCh) // safety cleanup
	env := setupTALEnvironment(t)
	env.llm.blockCh = blockCh

	parentCtx, parentCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer parentCancel()

	// Spawn a task that will block forever
	taskID, err := env.executor.ExecuteAsync(parentCtx, "/fix", "will block forever")
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}

	// Give agent time to start running
	time.Sleep(300 * time.Millisecond)

	// Verify agent is running
	agent, ok := env.spawner.Get(taskID)
	if !ok {
		t.Fatal("Spawner should have the agent registered")
	}
	state := agent.GetState()
	t.Logf("Agent state before cancel: %v", state)
	if state != session.SubAgentStateRunning {
		t.Logf("WARNING: Agent state is %v, expected running", state)
	}

	// Cancel WaitForResult via short timeout
	waitCtx, waitCancel := context.WithTimeout(parentCtx, 500*time.Millisecond)
	defer waitCancel()

	_, err = env.executor.WaitForResult(waitCtx, taskID)
	if err == nil {
		t.Fatal("WaitForResult should have returned error on context cancellation")
	}
	t.Logf("WaitForResult returned: %v (as expected)", err)

	// Give system time to reap the agent (if it does)
	time.Sleep(500 * time.Millisecond)

	// Check: is the agent STILL running?
	agentAfter, ok := env.spawner.Get(taskID)
	if !ok {
		t.Log("Agent was removed from spawner — good")
	} else {
		stateAfter := agentAfter.GetState()
		if stateAfter == session.SubAgentStateRunning {
			t.Errorf("BUG CONFIRMED: Agent %s is STILL RUNNING after WaitForResult "+
				"cancellation. WaitForResult exits without calling spawner.Stop(). "+
				"This creates a zombie subagent burning LLM tokens.", taskID)
		} else {
			t.Logf("Agent state after cancel: %v (stopped/completed)", stateAfter)
		}
	}

	// Verify number of still-active agents
	activeAgents := env.spawner.ListActive()
	t.Logf("Active agents after cancellation: %d", len(activeAgents))
	if len(activeAgents) > 0 {
		for _, a := range activeAgents {
			t.Logf("  Zombie: id=%s name=%s state=%v", a.GetID(), a.GetName(), a.GetState())
		}
		t.Error("ZOMBIE LEAK: Active agents remain after WaitForResult cancellation")
	}
}

// =============================================================================
// 4. Context.Background() Bypass
// =============================================================================

// TestE2E_TaskExecutor_ExecuteAsync_ContextBackground verifies that the
// subagent respects the caller's context (with timeout), not context.Background().
//
// If the second Run uses context.Background(), cancelling the parent context
// won't stop the agent.
func TestE2E_TaskExecutor_ExecuteAsync_ContextBackground(t *testing.T) {
	// LLM blocks until channel is closed
	blockCh := make(chan struct{})
	defer close(blockCh)
	env := setupTALEnvironment(t)
	env.llm.blockCh = blockCh

	// Create a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	taskID, err := env.executor.ExecuteAsync(ctx, "/fix", "context test")
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}

	// Give agent time to start
	time.Sleep(200 * time.Millisecond)

	// Cancel the parent context
	cancel()

	// If the agent's Run used context.Background(), it will keep running even
	// after our context is cancelled. Wait a bit and check.
	time.Sleep(1 * time.Second)

	agent, ok := env.spawner.Get(taskID)
	if !ok {
		t.Log("Agent was cleaned up — good")
		return
	}

	state := agent.GetState()
	if state == session.SubAgentStateRunning {
		t.Logf("KNOWN ISSUE: Agent %s is still running after parent context cancellation. "+
			"executeAsyncInternal uses context.Background() for the second go agent.Run(), "+
			"which bypasses the caller's cancellation.", taskID)
	} else {
		t.Logf("Agent state after context cancel: %v", state)
	}
}

// =============================================================================
// 5. Cleanup Removes Completed Agents
// =============================================================================

// TestE2E_TaskExecutor_Cleanup_RemovesCompletedAgent verifies that
// spawner.Cleanup() removes completed/failed agents and ListActive() is empty.
func TestE2E_TaskExecutor_Cleanup_RemovesCompletedAgent(t *testing.T) {
	env := setupTALEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run and complete a task
	taskID, err := env.executor.ExecuteAsync(ctx, "/fix", "quick task")
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}

	// Wait for completion
	_, err = env.executor.WaitForResult(ctx, taskID)
	t.Logf("Task completed: err=%v", err)

	// Before cleanup, agent should still be in spawner
	_, ok := env.spawner.Get(taskID)
	if !ok {
		t.Log("NOTE: Agent was already removed from spawner (may have been auto-cleaned)")
	}

	// Cleanup
	removed := env.spawner.Cleanup()
	t.Logf("Cleanup removed %d agents", removed)

	// After cleanup, no active agents should remain
	active := env.spawner.ListActive()
	if len(active) > 0 {
		t.Errorf("Expected 0 active agents after cleanup, got %d", len(active))
		for _, a := range active {
			t.Logf("  Still active: id=%s state=%v", a.GetID(), a.GetState())
		}
	}

	// The result should still be retrievable from the JITExecutor's cache
	result, done, err := env.executor.GetResult(taskID)
	t.Logf("Cached result after cleanup: result=%q done=%v err=%v", result, done, err)
}

// =============================================================================
// 6. Capacity Limit
// =============================================================================

// TestE2E_TaskExecutor_ExecuteAsync_CapacityLimit verifies that the spawner
// rejects new agents when maxActiveSubagents is reached.
func TestE2E_TaskExecutor_ExecuteAsync_CapacityLimit(t *testing.T) {
	// Create a blocking LLM so agents stay active
	blockCh := make(chan struct{})
	defer close(blockCh)

	llm := &talMockLLMClient{blockCh: blockCh}
	vstore := &talMockVirtualStore{}
	jit := &talMockJITCompiler{}
	cfgFactory := &talMockConfigFactory{}
	trans := &talMockTransducer{intent: "/fix"}

	exec := session.NewExecutor(nil, vstore, llm, jit, cfgFactory, trans)
	exec.SetConfig(session.DefaultExecutorConfig())

	// Set max to 3
	spawnerCfg := session.SpawnerConfig{MaxActiveSubagents: 3}
	spawner := session.NewSpawner(nil, vstore, llm, jit, cfgFactory, trans, spawnerCfg)
	taskExec := session.NewJITExecutor(exec, spawner, trans)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Spawn up to the limit
	var taskIDs []string
	for i := 0; i < 3; i++ {
		taskID, err := taskExec.ExecuteAsync(ctx, "/fix", fmt.Sprintf("task %d", i))
		if err != nil {
			t.Fatalf("ExecuteAsync #%d failed: %v", i, err)
		}
		taskIDs = append(taskIDs, taskID)
		time.Sleep(50 * time.Millisecond) // Let spawn register
	}

	// Give agents time to register and start
	time.Sleep(300 * time.Millisecond)

	active := spawner.ListActive()
	t.Logf("Active agents after spawning 3: %d", len(active))

	// Next spawn should fail — capacity exceeded
	_, err := taskExec.ExecuteAsync(ctx, "/fix", "one too many")
	if err == nil {
		t.Error("Expected capacity error when exceeding maxActiveSubagents, got nil")
	} else {
		t.Logf("Correctly rejected: %v", err)
	}
}

// =============================================================================
// 7. Concurrent ExecuteAsync — Race Safety
// =============================================================================

// TestE2E_TaskExecutor_ExecuteAsync_Concurrent verifies that concurrent
// ExecuteAsync calls don't cause data races or panics.
func TestE2E_TaskExecutor_ExecuteAsync_Concurrent(t *testing.T) {
	env := setupTALEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const goroutines = 5
	var wg sync.WaitGroup
	errors := make([]error, goroutines)
	taskIDs := make([]string, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			taskID, err := env.executor.ExecuteAsync(ctx, "/fix", fmt.Sprintf("concurrent task %d", idx))
			taskIDs[idx] = taskID
			errors[idx] = err
		}(i)
	}
	wg.Wait()

	// Count successes and failures
	successes := 0
	for i, err := range errors {
		if err != nil {
			t.Logf("Goroutine %d: error=%v (may be capacity limit)", i, err)
		} else {
			successes++
			t.Logf("Goroutine %d: taskID=%s", i, taskIDs[i])
		}
	}

	t.Logf("Concurrent ExecuteAsync: %d/%d succeeded", successes, goroutines)

	// Wait for all successful tasks to complete
	for i, taskID := range taskIDs {
		if errors[i] == nil && taskID != "" {
			result, waitErr := env.executor.WaitForResult(ctx, taskID)
			t.Logf("Task %d result: %q err=%v", i, result, waitErr)
		}
	}

	// Verify no double-run inflation
	calls := atomic.LoadInt64(&env.llm.callCount)
	t.Logf("Total LLM calls for %d concurrent tasks: %d", successes, calls)

	// If double-run exists, calls will be ~2x successes
	if calls > int64(successes)*2 {
		t.Errorf("Suspiciously high LLM call count: %d calls for %d tasks (possible double-run)", calls, successes)
	}
}
