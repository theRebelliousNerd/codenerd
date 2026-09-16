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
	callCount int64         // atomic
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

	return "mock response:" + userInput, nil
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

	return &types.LLMToolResponse{Text: "mock response:" + userInput}, nil
}

func (m *talMockLLMClient) ShouldUsePiggybackTools() bool { return false }

func (m *talMockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	ch <- "mock streaming response"
	close(ch)
	close(errCh)
	return ch, errCh
}

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

func (m *talMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	if m.failOnGenerate {
		return nil, fmt.Errorf("config factory failure")
	}
	return &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"mock_tool"},
		Policies:     []string{},
	}, nil
}

func (m *talMockConfigFactory) RegisterSpecialist(name string, config *config.EffectiveAgentRuntimeConfig) error {
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
func (m *talMockTransducer) SetPromptAssembler(pa perception.PromptAssembler) {}
func (m *talMockTransducer) SetStrategicContext(ctx string)                   {}

// =============================================================================
// SETUP
// =============================================================================

type talEnv struct {
	executor   *session.JITExecutor
	spawner    *session.Spawner
	llm        *talMockLLMClient
	vstore     *talMockVirtualStore
	cfgFactory *talMockConfigFactory
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

// talWaitForAgentState polls until an agent reaches want, or fails the test.
// Lifecycle transitions are asynchronous; polling beats a blind sleep and
// keeps the observed precondition deterministic instead of racy.
func talWaitForAgentState(t *testing.T, spawner *session.Spawner, taskID string, want session.SubAgentState, timeout time.Duration) *session.SubAgent {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		agent, ok := spawner.Get(taskID)
		if ok && agent.GetState() == want {
			return agent
		}
		if !time.Now().Before(deadline) {
			state := "missing"
			if ok {
				state = agent.GetState().String()
			}
			t.Fatalf("agent %s is %s after %v, want %s", taskID, state, timeout, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// =============================================================================
// 1. CRITICAL: ExecuteAsync Double-Run Detection
// =============================================================================

// TestE2E_TaskExecutor_ExecuteAsync_SingleRun proves that ExecuteAsync
// starts the subagent exactly once. The mock LLM echoes the task and counts
// invocations, so the result proves the agent ran and the counter proves it
// ran once.
func TestE2E_TaskExecutor_ExecuteAsync_SingleRun(t *testing.T) {
	env := setupTALEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	taskID, err := env.executor.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: "touch sentinel-file-single-run"})
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}
	if taskID == "" {
		t.Fatal("ExecuteAsync returned empty taskID")
	}

	result, err := env.executor.WaitForResult(ctx, taskID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if !strings.Contains(result, "sentinel-file-single-run") {
		t.Fatalf("completed task lost its input: %q", result)
	}

	// A duplicate Run starts asynchronously; give it time to reveal itself as
	// a second LLM call before pinning the count.
	time.Sleep(500 * time.Millisecond)

	if calls := atomic.LoadInt64(&env.llm.callCount); calls != 1 {
		t.Fatalf("ExecuteAsync caused %d LLM calls, want exactly 1", calls)
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
	taskID, err := env.executor.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: "blocked-task-lifecycle"})
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}
	talWaitForAgentState(t, env.spawner, taskID, session.SubAgentStateRunning, 5*time.Second)

	// While LLM is blocked, result should be not-done
	_, done, err = env.executor.GetResult(taskID)
	if err != nil {
		t.Fatalf("GetResult for a running task failed: %v", err)
	}
	if done {
		t.Fatal("Result should not be done while LLM is blocked")
	}

	// Unblock LLM
	close(blockCh)

	// Wait for completion
	result, err := env.executor.WaitForResult(ctx, taskID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if !strings.Contains(result, "blocked-task-lifecycle") {
		t.Fatalf("completed task lost its input: %q", result)
	}

	// Verify final state is done
	cached, done, err := env.executor.GetResult(taskID)
	if err != nil {
		t.Fatalf("GetResult after completion failed: %v", err)
	}
	if !done {
		t.Fatal("Result should be done after WaitForResult returns")
	}
	if cached != result {
		t.Fatalf("cached result %q differs from waited result %q", cached, result)
	}
}

// =============================================================================
// 3. CRITICAL: WaitForResult Cancellation → Zombie Subagent Leak
// =============================================================================

// TestE2E_TaskExecutor_WaitForResult_Cancellation_StopsAgent proves that
// cancelling WaitForResult's context stops the underlying subagent instead of
// leaving a zombie burning LLM tokens.
func TestE2E_TaskExecutor_WaitForResult_Cancellation_StopsAgent(t *testing.T) {
	blockCh := make(chan struct{})
	defer close(blockCh) // safety cleanup
	env := setupTALEnvironment(t)
	env.llm.blockCh = blockCh

	parentCtx, parentCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer parentCancel()

	taskID, err := env.executor.ExecuteAsync(parentCtx, session.TaskRequest{IntentVerb: "/fix", Task: "will block forever"})
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}
	talWaitForAgentState(t, env.spawner, taskID, session.SubAgentStateRunning, 5*time.Second)

	// Cancel WaitForResult via short timeout
	waitCtx, waitCancel := context.WithTimeout(parentCtx, 500*time.Millisecond)
	defer waitCancel()

	_, err = env.executor.WaitForResult(waitCtx, taskID)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForResult returned %v, want context deadline exceeded", err)
	}

	// WaitForResult must reap the agent it stopped waiting for.
	agent := talWaitForAgentState(t, env.spawner, taskID, session.SubAgentStateFailed, 5*time.Second)
	if _, resultErr := agent.GetResult(); !errors.Is(resultErr, context.Canceled) {
		t.Fatalf("cancelled agent result error is %v, want context canceled", resultErr)
	}
	if activeAgents := env.spawner.ListActive(); len(activeAgents) != 0 {
		t.Fatalf("%d agents remain active after WaitForResult cancellation", len(activeAgents))
	}
}

// =============================================================================
// 4. Context.Background() Bypass
// =============================================================================

// TestE2E_TaskExecutor_ExecuteAsync_ContextBackground verifies that the
// subagent inherits the caller's context. Cancelling the parent context must
// stop the agent; a context.Background bypass would leave it running.
func TestE2E_TaskExecutor_ExecuteAsync_ContextBackground(t *testing.T) {
	blockCh := make(chan struct{})
	defer close(blockCh)
	env := setupTALEnvironment(t)
	env.llm.blockCh = blockCh

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	taskID, err := env.executor.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: "context-propagation"})
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}
	talWaitForAgentState(t, env.spawner, taskID, session.SubAgentStateRunning, 5*time.Second)

	// Cancel the parent context
	cancel()

	agent := talWaitForAgentState(t, env.spawner, taskID, session.SubAgentStateFailed, 5*time.Second)
	if _, resultErr := agent.GetResult(); !errors.Is(resultErr, context.Canceled) {
		t.Fatalf("cancelled agent result error is %v, want context canceled", resultErr)
	}
	if activeAgents := env.spawner.ListActive(); len(activeAgents) != 0 {
		t.Fatalf("%d agents remain active after parent context cancellation", len(activeAgents))
	}
}

// =============================================================================
// 5. Cleanup Removes Completed Agents
// =============================================================================

// TestE2E_TaskExecutor_Cleanup_RemovesCompletedAgent verifies the terminal
// lifecycle split: a retrieved result moves to the JITExecutor cache and out
// of the spawner registry, while Cleanup removes a terminal agent that was
// never retrieved.
func TestE2E_TaskExecutor_Cleanup_RemovesCompletedAgent(t *testing.T) {
	env := setupTALEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	taskID, err := env.executor.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: "quick-task-cleanup"})
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}
	result, err := env.executor.WaitForResult(ctx, taskID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if !strings.Contains(result, "quick-task-cleanup") {
		t.Fatalf("completed task lost its input: %q", result)
	}
	if _, ok := env.spawner.Get(taskID); ok {
		t.Fatal("retrieved agent remains in the spawner registry after its result was cached")
	}

	direct, err := env.spawner.Spawn(ctx, session.SpawnRequest{
		Name:       "coder",
		Task:       "direct-cleanup-marker",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/fix",
	})
	if err != nil {
		t.Fatalf("direct Spawn failed: %v", err)
	}
	direct.Run(ctx, "direct-cleanup-marker")
	talWaitForAgentState(t, env.spawner, direct.GetID(), session.SubAgentStateCompleted, 5*time.Second)

	if removed := env.spawner.Cleanup(); removed != 1 {
		t.Fatalf("Cleanup removed %d agents, want the one unretrieved terminal agent", removed)
	}
	if _, ok := env.spawner.Get(direct.GetID()); ok {
		t.Fatal("Cleanup left a completed agent in the spawner registry")
	}
	if active := env.spawner.ListActive(); len(active) != 0 {
		t.Fatalf("Expected 0 active agents after cleanup, got %d", len(active))
	}

	cached, done, err := env.executor.GetResult(taskID)
	if err != nil {
		t.Fatalf("GetResult after cleanup failed: %v", err)
	}
	if !done {
		t.Fatal("retrieved result is no longer done after cleanup")
	}
	if cached != result {
		t.Fatalf("cached result %q differs from waited result %q", cached, result)
	}
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
	taskIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		taskID, err := taskExec.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: fmt.Sprintf("capacity-task-%d", i)})
		if err != nil {
			t.Fatalf("ExecuteAsync #%d failed: %v", i, err)
		}
		taskIDs = append(taskIDs, taskID)
		talWaitForAgentState(t, spawner, taskID, session.SubAgentStateRunning, 5*time.Second)
	}
	seen := make(map[string]struct{}, len(taskIDs))
	for _, taskID := range taskIDs {
		if _, dup := seen[taskID]; dup || taskID == "" {
			t.Fatalf("spawned task IDs are not unique and non-empty: %q", taskIDs)
		}
		seen[taskID] = struct{}{}
	}
	if active := spawner.ListActive(); len(active) != 3 {
		t.Fatalf("spawner holds %d active agents, want exactly the 3-agent capacity", len(active))
	}

	// Next spawn should fail — capacity exceeded
	_, err := taskExec.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: "one too many"})
	if err == nil {
		t.Fatal("Expected capacity error when exceeding maxActiveSubagents, got nil")
	}
	if !strings.Contains(err.Error(), "max active subagents reached") {
		t.Fatalf("capacity rejection lost its contract wording: %v", err)
	}
}

// =============================================================================
// 7. Concurrent ExecuteAsync — Race Safety
// =============================================================================

// TestE2E_TaskExecutor_ExecuteAsync_Concurrent verifies that concurrent
// ExecuteAsync calls stay race-safe and return each caller's own result.
func TestE2E_TaskExecutor_ExecuteAsync_Concurrent(t *testing.T) {
	env := setupTALEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const goroutines = 5
	markers := make([]string, goroutines)
	for i := range markers {
		markers[i] = fmt.Sprintf("concurrent-task-%02d", i)
	}
	var wg sync.WaitGroup
	spawnErrs := make([]error, goroutines)
	taskIDs := make([]string, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			taskID, err := env.executor.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/fix", Task: markers[idx]})
			taskIDs[idx] = taskID
			spawnErrs[idx] = err
		}(i)
	}
	wg.Wait()

	seen := make(map[string]struct{}, goroutines)
	for i, err := range spawnErrs {
		if err != nil {
			t.Fatalf("goroutine %d failed to spawn: %v", i, err)
		}
		if taskIDs[i] == "" {
			t.Fatalf("goroutine %d returned an empty task ID", i)
		}
		if _, dup := seen[taskIDs[i]]; dup {
			t.Fatalf("duplicate task ID %q", taskIDs[i])
		}
		seen[taskIDs[i]] = struct{}{}
	}

	for i, taskID := range taskIDs {
		result, waitErr := env.executor.WaitForResult(ctx, taskID)
		if waitErr != nil {
			t.Fatalf("goroutine %d WaitForResult failed: %v", i, waitErr)
		}
		if !strings.Contains(result, markers[i]) {
			t.Fatalf("goroutine %d result lost its marker: %q", i, result)
		}
		for j, foreign := range markers {
			if j != i && strings.Contains(result, foreign) {
				t.Fatalf("goroutine %d result leaked marker %q: %q", i, foreign, result)
			}
		}
	}

	if calls := atomic.LoadInt64(&env.llm.callCount); calls != goroutines {
		t.Fatalf("concurrent ExecuteAsync caused %d LLM calls, want exactly %d", calls, goroutines)
	}
}

// ResolveAllowedTools projects the same fixture envelope before JIT selection.
func (m *talMockConfigFactory) ResolveAllowedTools(ctx context.Context, intents ...string) ([]string, error) {
	resolved, err := m.Generate(ctx, &prompt.CompilationResult{}, intents...)
	if err != nil || resolved == nil {
		return nil, err
	}
	return append([]string(nil), resolved.AllowedTools...), nil
}
