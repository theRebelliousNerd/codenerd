//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// =============================================================================
// MOCKS FOR BOUNDARY TESTING
// =============================================================================

// mockLLMClientWithControls allows precise timing control over LLM completion
type mockLLMClientWithControls struct {
	mu          sync.Mutex
	responses   []types.LLMToolResponse
	callCount   int32
	blockChan   chan struct{} // blocks generation until closed
	errToReturn error
	simulateOOM bool
	isPiggyback bool // If true, implements PiggybackToolProvider
}

func (m *mockLLMClientWithControls) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	atomic.AddInt32(&m.callCount, 1)

	if m.blockChan != nil {
		select {
		case <-m.blockChan:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.errToReturn != nil {
		return "", m.errToReturn
	}

	if m.simulateOOM {
		// Attempting to simulate large allocation (careful not to actually crash test runner)
		_ = make([]byte, 500*1024*1024) // 500MB
	}

	if len(m.responses) == 0 {
		return "Default mock response", nil
	}

	resp := m.responses[0]
	if len(m.responses) > 1 {
		m.responses = m.responses[1:]
	}

	return resp.Text, nil
}

func (m *mockLLMClientWithControls) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	atomic.AddInt32(&m.callCount, 1)

	if m.blockChan != nil {
		select {
		case <-m.blockChan:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.errToReturn != nil {
		return nil, m.errToReturn
	}

	if len(m.responses) == 0 {
		return &types.LLMToolResponse{Text: "Default tool resp"}, nil
	}

	resp := m.responses[0]
	if len(m.responses) > 1 {
		m.responses = m.responses[1:]
	}

	return &types.LLMToolResponse{
		Text:      resp.Text,
		ToolCalls: resp.ToolCalls,
	}, nil
}

func (m *mockLLMClientWithControls) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return m.CompleteWithTools(ctx, systemPrompt, "", tools)
}

func (m *mockLLMClientWithControls) ShouldUsePiggybackTools() bool {
	return m.isPiggyback
}

func (m *mockLLMClientWithControls) GetModel() string { return "mock-model" }

func (m *mockLLMClientWithControls) Complete(ctx context.Context, prompt string) (string, error) {
	return m.CompleteWithSystem(ctx, "", prompt)
}

// mockJITCompilerLLM
type mockJITCompilerLLM struct{}

func (m *mockJITCompilerLLM) Compile(ctx context.Context, compCtx *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{Prompt: "system prompt"}, nil
}

// mockConfigFactoryLLM
type mockConfigFactoryLLM struct{}

func (m *mockConfigFactoryLLM) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	return &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"mock_tool", "slow_tool", "panic_tool", "echo_tool"},
	}, nil
}

// =============================================================================
// TEST SUITE CONFIGURATION
// =============================================================================

// registerSchedMockToolsOnce guards the global mock_tool registration below.
// buildToolDefinitions filters the config grant against tools.Global(), so a
// granted-but-unregistered tool is never offered to the model and the executor
// degrades to the text-only path with zero executions. Registering mock_tool
// keeps the tool-loop tests on the loop they intend to pin.
var registerSchedMockToolsOnce sync.Once

func registerSchedMockTools() {
	registerSchedMockToolsOnce.Do(func() {
		// Ignore the error: sibling e2e files register their own mock_tool
		// with a success handler, and every assertion in this file depends
		// on success, not on which handler won the race.
		_ = tools.Global().Register(&tools.Tool{Name: "mock_tool", Effect: tools.EffectRead, Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
			return "sched-mock-ok", nil
		}})
	})
}

func setupTestExecutorLLM(t *testing.T, llmClient core.LLMClient, maxConcurrent int) (*session.Executor, *core.APIScheduler) {
	t.Helper()
	registerSchedMockTools()

	// Reset scheduler global state implicitly by creating a fresh one
	cfg := core.DefaultAPISchedulerConfig()
	cfg.MaxConcurrentAPICalls = maxConcurrent
	cfg.SlotAcquireTimeout = 2 * time.Second
	scheduler := core.NewAPIScheduler(cfg)
	scheduler.RegisterShard("test-shard", "test")

	scheduledLLM := &core.ScheduledLLMCall{
		Scheduler: scheduler,
		ShardID:   "test-shard",
		Client:    llmClient,
	}

	executor := session.NewExecutor(newMockKernelLLM(), nil, scheduledLLM, &mockJITCompilerLLM{}, &mockConfigFactoryLLM{}, nil)

	// Tool execution resolves against tools.Global(): the suite registers
	// mock_tool there once (registerSchedMockTools) because the executor
	// offers only granted-AND-registered tools to the model.
	return executor, scheduler
}

// =============================================================================
// 1. SMOKE TESTS
// =============================================================================

// TestE2E_SchedulerSession_Smoke_HappyPath verifies a simple turn executes end-to-end
func TestE2E_SchedulerSession_Smoke_HappyPath(t *testing.T) {
	t.Parallel()

	llm := &mockLLMClientWithControls{
		responses: []types.LLMToolResponse{{Text: "Hello, world!"}},
	}
	exec, _ := setupTestExecutorLLM(t, llm, 5)

	ctx := context.Background()
	res, err := exec.ProcessWithIntent(ctx, "hello", &perception.Intent{Verb: "/general"})

	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if res.Response != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %s", res.Response)
	}
	if atomic.LoadInt32(&llm.callCount) != 1 {
		t.Errorf("expected 1 API call, got %d", llm.callCount)
	}
}

// =============================================================================
// 2. CONTRACT VIOLATION TESTS
// =============================================================================

// panickingClient implements LLMClient but panics on call
type panickingClient struct {
	core.LLMClient
}

func (p *panickingClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	panic("intentional mock panic")
}
func (p *panickingClient) GetModel() string { return "mock" }

// TestE2E_SchedulerSession_ContractViolation_SlotLeakOnPanic
// Ensures that if the LLM client panics completely, the slot is correctly released by defer.
func TestE2E_SchedulerSession_ContractViolation_SlotLeakOnPanic(t *testing.T) {
	t.Parallel()

	llm := &panickingClient{}
	scheduler := core.NewAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: time.Second})
	scheduled := &core.ScheduledLLMCall{Scheduler: scheduler, ShardID: "test", Client: llm}

	// This should panic inside the client, but ScheduledLLMCall catches it or the defer runs.
	_, err := scheduled.CompleteWithSystem(context.Background(), "sys", "user")

	if err == nil {
		t.Fatal("expected error from panic, got success")
	}

	// The critical assertion: Is the slot free?
	metrics := scheduler.GetMetrics()
	if metrics.ActiveSlots != 0 {
		t.Fatalf("Contract Violated: Slot leaked! ActiveSlots=%d", metrics.ActiveSlots)
	}
}

// TestE2E_SchedulerSession_ContractViolation_NilAgentConfig
// Scenario 10: Graceful degradation when AgentConfig is nil
func TestE2E_SchedulerSession_ContractViolation_NilAgentConfig(t *testing.T) {
	t.Parallel()

	llm := &mockLLMClientWithControls{
		responses: []types.LLMToolResponse{{
			Text:      "I will use a tool.",
			ToolCalls: []types.ToolCall{{ID: "1", Name: "mock_tool"}},
		}},
	}

	exec := session.NewExecutor(newMockKernelLLM(), nil, llm, nil, nil, nil) // no JIT/ConfigFactory

	// When using Process (no intent preset), it tries to transduce.
	// We'll bypass Transducer by using ProcessWithIntent and forcing a nil JIT context path by having nil factories.
	res, err := exec.ProcessWithIntent(context.Background(), "do it", &perception.Intent{Verb: "/general"})

	// It must degrade gracefully, NOT panic on nil factories inside
	// compileConfig/buildToolDefinitions. With an empty config, the executor
	// takes the text-only path and ignores the ungranted tool request rather
	// than executing it or failing the turn.
	if err != nil {
		t.Fatalf("Expected graceful text-only degradation with nil factories, got error: %v", err)
	}
	if res.Response != "I will use a tool." {
		t.Errorf("expected model text to pass through unchanged, got %q", res.Response)
	}
	if res.ToolCallsExecuted != 0 {
		t.Errorf("expected 0 tool executions with nil factories, got %d", res.ToolCallsExecuted)
	}
}

// =============================================================================
// 3. STATE CORRUPTION TESTS
// =============================================================================

// TestE2E_SchedulerSession_StateCorruption_HistoryRace
// Scenario 7: Race between modifying history and reading it
func TestE2E_SchedulerSession_StateCorruption_HistoryRace(t *testing.T) {
	t.Parallel()

	llm := &mockLLMClientWithControls{
		responses: []types.LLMToolResponse{{Text: "looping"}},
	}
	exec, _ := setupTestExecutorLLM(t, llm, 5)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Process turns which modifies history
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = exec.ProcessWithIntent(ctx, "turn", &perception.Intent{Verb: "/chat"})
		}
	}()

	// Goroutine 2: Read and clear history
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			h := exec.GetHistory()
			_ = len(h)
			if i%5 == 0 {
				exec.ClearHistory()
			}
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()
	// If it doesn't panic under `go test -race`, the mutex contract holds.
	// Behaviorally: history stays readable, clearing works, and the
	// executor still processes after the storm.
	h := exec.GetHistory()
	for _, m := range h {
		_ = m.Role
		_ = m.Content
	}
	exec.ClearHistory()
	if got := len(exec.GetHistory()); got != 0 {
		t.Fatalf("ClearHistory left %d items after race", got)
	}
	if _, err := exec.ProcessWithIntent(context.Background(), "after storm", &perception.Intent{Verb: "/chat"}); err != nil {
		t.Fatalf("Executor broken after history race: %v", err)
	}
}

// =============================================================================
// 4. RESOURCE EXHAUSTION TESTS
// =============================================================================

// TestE2E_SchedulerSession_ResourceExhaustion_ThunderingHerd
// Scenario 4: The 1,000 Shard Thundering Herd
func TestE2E_SchedulerSession_ResourceExhaustion_ThunderingHerd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping resource exhaustion in short mode")
	}

	// 1000 tasks, but only 5 slots.
	const numTasks = 1000
	const maxSlots = 5

	llm := &mockLLMClientWithControls{
		// Don't block, just return fast to test queue churn
		responses: []types.LLMToolResponse{{Text: "done"}},
	}

	exec, scheduler := setupTestExecutorLLM(t, llm, maxSlots)

	var wg sync.WaitGroup
	startCh := make(chan struct{})

	// Pre-spawn 1000 goroutines waiting at the gate
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-startCh // Wait for the gate to open
			_, _ = exec.ProcessWithIntent(context.Background(), "task", &perception.Intent{Verb: "/general"})
		}(i)
	}

	// Release the herd
	close(startCh)

	// Give it a moment to queue up
	time.Sleep(50 * time.Millisecond)

	metrics := scheduler.GetMetrics()
	if metrics.ActiveSlots > maxSlots {
		t.Errorf("Resource Exhaustion Violated: Active slots %d > Max %d", metrics.ActiveSlots, maxSlots)
	}

	// It shouldn't deadlock. Wait for all to finish.
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// Success
	case <-time.After(15 * time.Second):
		t.Fatal("Thundering herd deadlocked or took too long")
	}

	finalMetrics := scheduler.GetMetrics()
	if finalMetrics.ActiveSlots != 0 || finalMetrics.WaitingShards != 0 {
		t.Errorf("Leaked resources after herd completed: active=%d, waiting=%d",
			finalMetrics.ActiveSlots, finalMetrics.WaitingShards)
	}
}

// TestE2E_SchedulerSession_ResourceExhaustion_InfiniteToolLoop
// Scenario 2: MaxToolIterations limit must break infinite tool requests
func TestE2E_SchedulerSession_ResourceExhaustion_InfiniteToolLoop(t *testing.T) {
	t.Parallel()

	// A client that *always* returns a tool call
	llm := &mockLLMClientWithControls{}
	for i := 0; i < 20; i++ {
		llm.responses = append(llm.responses, types.LLMToolResponse{
			Text:      "I need more tools",
			ToolCalls: []types.ToolCall{{ID: fmt.Sprintf("call_%d", i), Name: "unknown_tool"}},
		})
	}

	exec, _ := setupTestExecutorLLM(t, llm, 5)

	// Pin the iteration ceiling on the executor itself: the "5" in the setup
	// helper is scheduler slots, not the tool-loop cap (which defaults to 8).
	execCfg := session.DefaultExecutorConfig()
	execCfg.MaxToolIterations = 5
	exec.SetConfig(execCfg)
	res, err := exec.ProcessWithIntent(context.Background(), "start loop", &perception.Intent{Verb: "/general"})

	// It should NOT run forever. It runs the 5 configured iterations, then the
	// forced-final path runs the one pending batch (so a late verification
	// call is not lost) and generates the conclusion: 1 initial + 5
	// follow-ups + 1 final = 7 LLM calls, 5 + 1 = 6 executions. Bounded,
	// with exactly one post-ceiling batch.
	if err != nil {
		// It's acceptable for the executor to return the accumulated error
		// when budget is blown.
	}

	if atomic.LoadInt32(&llm.callCount) > 7 {
		t.Errorf("Executor failed to cap tool iterations, called LLM %d times", llm.callCount)
	}
	if res != nil && res.ToolCallsExecuted > 6 {
		t.Errorf("Executed too many tool calls: %d", res.ToolCallsExecuted)
	}
}

// =============================================================================
// 5. TEMPORAL FAILURE TESTS
// =============================================================================

// TestE2E_SchedulerSession_Temporal_AcquireTimeout
// Verifies that a context timeout while waiting in queue propagates correctly and doesn't leak.
func TestE2E_SchedulerSession_Temporal_AcquireTimeout(t *testing.T) {
	t.Parallel()

	llm := &mockLLMClientWithControls{
		blockChan: make(chan struct{}), // block all calls indefinitely
	}
	// Max 1 slot
	exec, scheduler := setupTestExecutorLLM(t, llm, 1)

	// Task 1: Acquires the slot and blocks forever
	go exec.ProcessWithIntent(context.Background(), "task1", &perception.Intent{Verb: "/general"})

	time.Sleep(50 * time.Millisecond) // let task 1 take the slot

	// Task 2: Will queue up. We give it a short context timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := exec.ProcessWithIntent(ctx, "task2", &perception.Intent{Verb: "/general"})
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Task 2 expected to fail due to context timeout")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "context error") && !strings.Contains(err.Error(), "canceled") {
		t.Errorf("Expected context error, got: %v", err)
	}
	if duration > 500*time.Millisecond {
		t.Errorf("Executor didn't respect context cancellation quickly enough: took %v", duration)
	}

	// Verify Task 2 is removed from queue
	metrics := scheduler.GetMetrics()
	if metrics.WaitingForSlot > 0 {
		t.Errorf("Scheduler leaked waiting task: %d still waiting", metrics.WaitingForSlot)
	}
}

// TestE2E_SchedulerSession_Temporal_TOCTOU_CancelRace
// Scenario 1: The Context Cancellation TOCTOU (Time-Of-Check to Time-Of-Use)
func TestE2E_SchedulerSession_Temporal_TOCTOU_CancelRace(t *testing.T) {
	t.Parallel()

	// Direct test of the scheduler fallback logic
	scheduler := core.NewAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})
	scheduler.RegisterShard("taskA", "test")
	scheduler.RegisterShard("taskB", "test")
	scheduler.RegisterShard("taskC", "test")

	// 1. Task A takes the slot
	err := scheduler.AcquireAPISlot(context.Background(), "taskA")
	if err != nil {
		t.Fatal(err)
	}

	// 2. Task B queues up with a cancelable context
	ctxB, cancelB := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		_ = scheduler.AcquireAPISlot(ctxB, "taskB")
		// We don't care if it succeeded or failed, only that it doesn't corrupt state.
	}()

	time.Sleep(50 * time.Millisecond) // Let Task B enter waitQueue

	// 3. The Exploit: Release slot and cancel B simultaneously
	wg.Add(1)
	go func() {
		defer wg.Done()
		scheduler.ReleaseAPISlot("taskA") // This pushes to w.ch
	}()
	cancelB() // This closes ctxB.Done()

	wg.Wait()

	// 4. Verify no leaked slot.
	// Cleanup: if Task B grabbed it via fallback, release it.
	stateB, ok := scheduler.GetShardState("taskB")
	if ok && stateB.Phase == core.PhaseExecutingAPI {
		scheduler.ReleaseAPISlot("taskB")
	}

	// Try to acquire slot with short timeout to prove it's free
	ctxC, cancelC := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelC()

	err = scheduler.AcquireAPISlot(ctxC, "taskC")
	if err != nil {
		t.Fatalf("TOCTOU Race failed: API slot was leaked! Error: %v", err)
	}
	scheduler.ReleaseAPISlot("taskC")
}

// =============================================================================
// 6. CASCADING FAILURE TESTS
// =============================================================================

// TestE2E_SchedulerSession_Cascading_PiggybackMalformed
// Scenario 8: Semantic Failure -> Malformed Piggyback survives
func TestE2E_SchedulerSession_Cascading_PiggybackMalformed(t *testing.T) {
	t.Parallel()

	malformedJSON := `{"control_packet": {"intent_classification": {"category": "broken"` // Missing braces

	llm := &mockLLMClientWithControls{
		responses:   []types.LLMToolResponse{{Text: malformedJSON}},
		isPiggyback: true,
	}

	exec, _ := setupTestExecutorLLM(t, llm, 5)

	res, err := exec.ProcessWithIntent(context.Background(), "test", &perception.Intent{Verb: "/general"})

	if err != nil {
		t.Fatalf("Cascading failure: Malformed Piggyback crashed the executor: %v", err)
	}

	// The executor must swallow the parse error and degrade gracefully: an
	// unterminated envelope is indistinguishable from truncation, so the
	// emitter answers with the truncation notice instead of raw garbage.
	if !strings.Contains(res.Response, "cut off") {
		t.Errorf("Expected truncation-notice fallback, got: %s", res.Response)
	}
}

// =============================================================================
// 7. RECOVERY TESTS
// =============================================================================

// TestE2E_SchedulerSession_Recovery_DynamicReconfig
// Scenario 5: Rapid Dynamic Reconfiguration
func TestE2E_SchedulerSession_Recovery_DynamicReconfig(t *testing.T) {
	t.Parallel()

	scheduler := core.NewAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 2, SlotAcquireTimeout: 5 * time.Second})

	// Enrollment is the fail-closed gate: unregistered shards cannot hold
	// slots, so register before acquiring.
	for i := 1; i <= 5; i++ {
		scheduler.RegisterShard(fmt.Sprintf("t%d", i), "test")
	}

	// Acquire initial 2
	if err := scheduler.AcquireAPISlot(context.Background(), "t1"); err != nil {
		t.Fatalf("t1 acquire failed: %v", err)
	}
	if err := scheduler.AcquireAPISlot(context.Background(), "t2"); err != nil {
		t.Fatalf("t2 acquire failed: %v", err)
	}

	// Queue up 3
	for i := 3; i <= 5; i++ {
		go scheduler.AcquireAPISlot(context.Background(), fmt.Sprintf("t%d", i))
	}

	time.Sleep(50 * time.Millisecond)

	// Dynamically expand to 10
	scheduler.UpdateMaxConcurrentAPICalls(10)

	// The 3 queued should immediately wake up.
	time.Sleep(50 * time.Millisecond)

	metrics := scheduler.GetMetrics()
	if metrics.ActiveSlots != 5 {
		t.Errorf("Expected 5 active slots after expansion, got %d", metrics.ActiveSlots)
	}
	if metrics.WaitingForSlot != 0 {
		t.Errorf("Expected 0 waiters, got %d", metrics.WaitingForSlot)
	}

	// Release all
	for i := 1; i <= 5; i++ {
		scheduler.ReleaseAPISlot(fmt.Sprintf("t%d", i))
	}

	// Shrink back to 1
	scheduler.UpdateMaxConcurrentAPICalls(1)
	metrics2 := scheduler.GetMetrics()
	if metrics2.MaxSlots != 1 {
		t.Errorf("Expected MaxSlots=1, got %d", metrics2.MaxSlots)
	}
}

// =============================================================================
// 8. PARTIAL PIPELINE FAILURE TESTS
// =============================================================================

// TestE2E_SchedulerSession_Partial_ToolBatchFailure
// Scenario 6: Partial Tool Batch Failure. One errors, one succeeds.
func TestE2E_SchedulerSession_Partial_ToolBatchFailure(t *testing.T) {
	t.Parallel()

	llm := &mockLLMClientWithControls{
		responses: []types.LLMToolResponse{
			{
				Text: "Running tools",
				ToolCalls: []types.ToolCall{
					{ID: "1", Name: "missing_tool"}, // will fail
					{ID: "2", Name: "mock_tool"},    // would succeed if we had modular registry injected, but here both fail config check
				},
			},
			{Text: "Final response"},
		},
	}

	exec, _ := setupTestExecutorLLM(t, llm, 5)

	res, err := exec.ProcessWithIntent(context.Background(), "test", &perception.Intent{Verb: "/general"})

	// The system must not crash. It should report the tool errors internally and continue the loop.
	if err != nil {
		t.Fatalf("Pipeline crashed on tool error: %v", err)
	}
	if res.ToolCallsExecuted != 2 {
		t.Errorf("Expected 2 tool calls attempted, got %d", res.ToolCallsExecuted)
	}
	if res.Response != "Final response" {
		t.Errorf("LLM did not recover after tool error, got: %s", res.Response)
	}
}

// =============================================================================
// 9. END-TO-END DATA INTEGRITY & MULTI-TURN ACCUMULATION
// =============================================================================

// TestE2E_SchedulerSession_DataIntegrity_MultiTurn
func TestE2E_SchedulerSession_DataIntegrity_MultiTurn(t *testing.T) {
	t.Parallel()

	llm := &mockLLMClientWithControls{
		responses: []types.LLMToolResponse{
			{Text: "Turn 1"},
			{Text: "Turn 2"},
			{Text: "Turn 3"},
		},
	}
	exec, _ := setupTestExecutorLLM(t, llm, 5)

	ctx := context.Background()

	// 1. First turn
	res1, err := exec.ProcessWithIntent(ctx, "req 1", &perception.Intent{Verb: "/general"})
	if err != nil {
		t.Fatalf("Turn 1 err: %v", err)
	}

	// 2. Second turn
	res2, err := exec.ProcessWithIntent(ctx, "req 2", &perception.Intent{Verb: "/general"})
	if err != nil {
		t.Fatalf("Turn 2 err: %v", err)
	}

	// 3. Check history accumulation (User + Assistant = 2 per turn)
	history := exec.GetHistory()
	if len(history) != 4 {
		t.Errorf("Expected 4 history items, got %d", len(history))
	}

	// 4. Verify integrity
	if res1.Response != "Turn 1" || res2.Response != "Turn 2" {
		t.Errorf("Data corruption: res1=%s res2=%s", res1.Response, res2.Response)
	}
}

// =============================================================================
// 10. ADVANCED EDGE CASES (ADDED FOR COVERAGE AND ROBUSTNESS)
// =============================================================================

// TestE2E_SchedulerSession_Cascading_LargeError_Propagation
// Scenario 16: a 100KB model-side failure must surface intact — not hang,
// not get swallowed, not truncated past recognition.
func TestE2E_SchedulerSession_Cascading_LargeError_Propagation(t *testing.T) {
	t.Parallel()

	// Create a mock LLM that requests a specific tool
	llm := &mockLLMClientWithControls{
		responses: []types.LLMToolResponse{
			{
				Text: "Fetching large file",
				ToolCalls: []types.ToolCall{
					{ID: "large_tool", Name: "mock_large_tool"},
				},
			},
			{Text: "Processed file"},
		},
	}

	exec := session.NewExecutor(newMockKernelLLM(), nil, llm, nil, nil, nil)

	largeError := fmt.Errorf("fake large error: %s", strings.Repeat("A", 100*1024))
	llm.errToReturn = largeError // The LLM fails immediately in this mock setup.

	_, err := exec.ProcessWithIntent(context.Background(), "test", &perception.Intent{Verb: "/general"})

	if err == nil {
		t.Fatalf("Expected large error, got nil")
	}
	if !strings.Contains(err.Error(), "fake large error") {
		t.Fatalf("Large error lost its marker in propagation: %.120s...", err.Error())
	}
	if len(err.Error()) < 100*1024 {
		t.Fatalf("Large error truncated in propagation: %d bytes", len(err.Error()))
	}
}

// TestE2E_SchedulerSession_Temporal_TaskExecutor_SpawnLimiter
// Scenario 17: Spawner Limits vs. Session Intent Injection
func TestE2E_SchedulerSession_Temporal_TaskExecutor_SpawnLimiter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long test")
	}

	// Create a spawner with max 2 active agents
	kernel := newMockKernelLLM()
	llm := &mockLLMClientWithControls{
		responses: []types.LLMToolResponse{{Text: "Done"}},
	}

	spawner := session.NewSpawner(kernel, nil, llm, nil, nil, nil, session.SpawnerConfig{MaxActiveSubagents: 2, TokenBudget: 4096})

	ctx := context.Background()

	// Spawn 3 agents. The 3rd should fail if it exceeds the limit.
	spawn := func(name, task string) error {
		_, err := spawner.Spawn(ctx, session.SpawnRequest{Name: name, Task: task, Type: session.SubAgentTypePersistent, IntentVerb: "/consult/" + name})
		return err
	}
	err1 := spawn("agent1", "task 1")
	err2 := spawn("agent2", "task 2")
	err3 := spawn("agent3", "task 3")

	if err1 != nil {
		t.Errorf("Agent 1 failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Agent 2 failed: %v", err2)
	}

	if err3 == nil {
		t.Errorf("Expected agent 3 to fail due to spawner limits, but it succeeded")
	} else if !strings.Contains(err3.Error(), "max active subagents reached") {
		t.Errorf("Expected limit error, got: %v", err3)
	}
}

// TestE2E_SchedulerSession_Semantic_PiggybackFallback
// Scenario 19: Piggyback Protocol Fallback Behavior
func TestE2E_SchedulerSession_Semantic_PiggybackFallback(t *testing.T) {
	t.Parallel()

	// A client that CLAIMS piggyback support. The claim must survive the
	// scheduler wrapper (ScheduledLLMCall delegates ShouldUsePiggybackTools),
	// so generation takes the structured-output path and the envelope's
	// tool_request executes via the single-turn piggyback batch, which runs
	// tools once without multi-turn continuation.
	envelope := `{"control_packet":{"reasoning_trace":"need tool data","tool_requests":[{"id":"req_1","tool_name":"mock_tool","tool_args":{},"purpose":"fallback probe","required":true}]},"surface_response":"using tools"}`
	llm := &mockLLMClientWithControls{
		isPiggyback: true,
		responses:   []types.LLMToolResponse{{Text: envelope}},
	}

	exec, _ := setupTestExecutorLLM(t, llm, 5)

	res, err := exec.ProcessWithIntent(context.Background(), "test", &perception.Intent{Verb: "/general"})

	if err != nil {
		t.Fatalf("Executor failed on piggyback fallback: %v", err)
	}

	// Piggyback batch path executes tools but does not feed them back
	if res.ToolCallsExecuted != 1 {
		t.Errorf("Expected 1 tool call to be executed, got %d", res.ToolCallsExecuted)
	}
}

// TestE2E_SchedulerSession_Semantic_PriorityOrdering proves the waiter list
// is priority-ordered, not FIFO: with one slot held, a low-priority waiter
// that queued FIRST must still lose the grant to a high-priority waiter
// that queued second. Same-priority waiters keep FIFO order.
func TestE2E_SchedulerSession_Semantic_PriorityOrdering(t *testing.T) {
	t.Parallel()

	scheduler := core.NewAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 10 * time.Second})
	scheduler.RegisterShardWithPriority("holder", "test", types.PriorityNormal)
	scheduler.RegisterShardWithPriority("low", "test", types.PriorityLow)
	scheduler.RegisterShardWithPriority("high", "test", types.PriorityHigh)

	if err := scheduler.AcquireAPISlot(context.Background(), "holder"); err != nil {
		t.Fatal(err)
	}

	lowAcquired := make(chan struct{})
	highAcquired := make(chan struct{})
	go func() {
		defer close(lowAcquired)
		_ = scheduler.AcquireAPISlot(context.Background(), "low")
	}()
	waitForSchedulerWaiters(t, scheduler, 1)
	go func() {
		defer close(highAcquired)
		_ = scheduler.AcquireAPISlot(context.Background(), "high")
	}()
	waitForSchedulerWaiters(t, scheduler, 2)

	// Release: the grant must jump the queue to high.
	scheduler.ReleaseAPISlot("holder")

	select {
	case <-highAcquired:
	case <-time.After(5 * time.Second):
		t.Fatal("high-priority waiter never acquired the released slot")
	}
	select {
	case <-lowAcquired:
		t.Fatal("low-priority waiter acquired ahead of a queued high-priority waiter")
	case <-time.After(100 * time.Millisecond):
		// Expected: low still waiting while high holds the slot.
	}
	scheduler.ReleaseAPISlot("high")

	select {
	case <-lowAcquired:
	case <-time.After(5 * time.Second):
		t.Fatal("low-priority waiter never acquired after high released")
	}
	scheduler.ReleaseAPISlot("low")

	// Same priority keeps FIFO: n1 queued first, so n1 is granted first.
	scheduler.RegisterShardWithPriority("n1", "test", types.PriorityNormal)
	scheduler.RegisterShardWithPriority("n2", "test", types.PriorityNormal)
	if err := scheduler.AcquireAPISlot(context.Background(), "holder"); err != nil {
		t.Fatal(err)
	}
	n1Acquired := make(chan struct{})
	n2Acquired := make(chan struct{})
	go func() {
		defer close(n1Acquired)
		_ = scheduler.AcquireAPISlot(context.Background(), "n1")
	}()
	waitForSchedulerWaiters(t, scheduler, 1)
	go func() {
		defer close(n2Acquired)
		_ = scheduler.AcquireAPISlot(context.Background(), "n2")
	}()
	waitForSchedulerWaiters(t, scheduler, 2)
	scheduler.ReleaseAPISlot("holder")

	select {
	case <-n1Acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("first-queued normal waiter never acquired")
	}
	select {
	case <-n2Acquired:
		t.Fatal("second-queued waiter acquired ahead of first-queued at equal priority")
	case <-time.After(100 * time.Millisecond):
		// Expected: n2 still waiting while n1 holds the slot.
	}
	scheduler.ReleaseAPISlot("n1")

	select {
	case <-n2Acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("second-queued waiter never acquired")
	}
	scheduler.ReleaseAPISlot("n2")

	if m := scheduler.GetMetrics(); m.ActiveSlots != 0 || m.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots: %+v", m)
	}
}

// waitForSchedulerWaiters polls until want waiters are queued (or fails).
// Queue arrival is asynchronous; polling beats a blind sleep and keeps the
// preemption windows above deterministic instead of racy.
func waitForSchedulerWaiters(t *testing.T, scheduler *core.APIScheduler, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got := scheduler.GetMetrics().WaitingForSlot; got >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d queued waiters", want)
}

// mockKernelLLM implements types.Kernel
type mockKernelLLM struct {
	mu    sync.RWMutex
	facts []types.Fact
}

func newMockKernelLLM() *mockKernelLLM {
	return &mockKernelLLM{}
}
func (m *mockKernelLLM) Assert(fact types.Fact) error                                   { return nil }
func (m *mockKernelLLM) Retract(predicate string) error                                 { return nil }
func (m *mockKernelLLM) Query(query string) ([]types.Fact, error)                       { return nil, nil }
func (m *mockKernelLLM) LoadFacts(facts []types.Fact) error                             { return nil }
func (m *mockKernelLLM) RetractFact(fact types.Fact) error                              { return nil }
func (m *mockKernelLLM) AssertBatch(facts []types.Fact) error                           { return nil }
func (m *mockKernelLLM) QueryAll() (map[string][]types.Fact, error)                     { return nil, nil }
func (m *mockKernelLLM) UpdateSystemFacts() error                                       { return nil }
func (m *mockKernelLLM) Reset()                                                         {}
func (m *mockKernelLLM) AppendPolicy(policy string)                                     {}
func (m *mockKernelLLM) RetractExactFactsBatch(facts []types.Fact) error                { return nil }
func (m *mockKernelLLM) RemoveFactsByPredicateSet(predicates map[string]struct{}) error { return nil }

func (m *mockKernelLLM) GetProgramInfo() *analysis.ProgramInfo { return nil }
func (m *mockLLMClientWithControls) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	return nil, nil
}

// ResolveAllowedTools projects the same fixture envelope before JIT selection.
func (m *mockConfigFactoryLLM) ResolveAllowedTools(ctx context.Context, intents ...string) ([]string, error) {
	resolved, err := m.Generate(ctx, &prompt.CompilationResult{}, intents...)
	if err != nil || resolved == nil {
		return nil, err
	}
	return append([]string(nil), resolved.AllowedTools...), nil
}
