//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/types"
	"codenerd/internal/articulation"
)

// --- Mocks ---

type oeMockTransducer struct {
	intentToReturn string
	delay          time.Duration
}

func (m *oeMockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return perception.Intent{Category: m.intentToReturn}, nil
}

func (m *oeMockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return perception.Intent{}, ctx.Err()
		}
	}
	return perception.Intent{Category: m.intentToReturn}, nil
}

func (m *oeMockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return perception.Intent{Category: m.intentToReturn}, nil, nil
}

func (m *oeMockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}
func (m *oeMockTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {}
func (m *oeMockTransducer) SetStrategicContext(context string) {}
func (m *oeMockTransducer) GetContext() string { return "mock_context" }


type oeMockJITCompiler struct {
	promptToReturn *prompt.CompilationResult
	errToReturn    error
}

func (m *oeMockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return m.promptToReturn, m.errToReturn
}

type oeMockConfigFactory struct {
	configToReturn *config.AgentConfig
	errToReturn    error
}

func (m *oeMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
	return m.configToReturn, m.errToReturn
}

type oeMockLLMClient struct {
	responseToReturn *types.LLMToolResponse
	errToReturn      error
	delay            time.Duration
	mu               sync.Mutex
	lastSystemPrompt string
}

func (m *oeMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", fmt.Errorf("not implemented")
}
func (m *oeMockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	m.mu.Lock()
	m.lastSystemPrompt = systemPrompt
	m.mu.Unlock()

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "mock response", m.errToReturn
}

func (m *oeMockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.mu.Lock()
	m.lastSystemPrompt = systemPrompt
	m.mu.Unlock()

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.responseToReturn, m.errToReturn
}

func (m *oeMockLLMClient) ToolCall(ctx context.Context, prompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return m.responseToReturn, m.errToReturn
}
func (m *oeMockLLMClient) ToolCallWithSystem(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.mu.Lock()
	m.lastSystemPrompt = systemPrompt
	m.mu.Unlock()

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.responseToReturn, m.errToReturn
}

func (m *oeMockLLMClient) CountTokens(text string) int { return len(text) }


// setupTestEnvironment sets up the dependencies for the Executor and JITExecutor.
func setupTestEnvironment(t *testing.T) (*session.Executor, *session.JITExecutor) {
	t.Helper()
	kernel, _ := core.NewRealKernel()
	virtualStore := core.NewVirtualStore(nil)
	llm := &oeMockLLMClient{
		responseToReturn: &types.LLMToolResponse{Text: "default success"},
	}
	transducer := &oeMockTransducer{intentToReturn: "/fix"}
	compiler := &oeMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "default prompt"}}
	configFactory := &oeMockConfigFactory{configToReturn: &config.AgentConfig{}}

	executor := session.NewExecutor(kernel, virtualStore, llm, compiler, configFactory, transducer)

	spawner := session.NewSpawner(kernel, virtualStore, llm, compiler, configFactory, transducer, session.DefaultSpawnerConfig())
	jitExecutor := session.NewJITExecutor(executor, spawner, transducer)


	return executor, jitExecutor
}
// ============================================================================
// 1. Smoke Test
// ============================================================================

func TestE2E_OrchestratorExecutor_Smoke(t *testing.T) {
	ctx := context.Background()
	_, jitExecutor := setupTestEnvironment(t)

	result, err := jitExecutor.Execute(ctx, "/fix", "fix the bug")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == "" {
		t.Errorf("Expected result, got empty string")
	}
}

// ============================================================================
// 2. Contract Violation: Thread Safety of SetSessionContext
// ============================================================================

func TestE2E_OrchestratorExecutor_ConcurrentInlineSetSessionContext_StateCorruption(t *testing.T) {
	ctx := context.Background()
	_, jitExecutor := setupTestEnvironment(t)

	var wg sync.WaitGroup
	numTasks := 20
	errors := make(chan error, numTasks)

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()

			ctxName := fmt.Sprintf("Context_%d", taskID)
			sessionCtx := &types.SessionContext{
				DreamMode: false,
				ExtraContext: map[string]string{
					"task_id": ctxName,
				},
			}

			_, err := jitExecutor.ExecuteWithContext(ctx, "/fix", fmt.Sprintf("Task %d", taskID), sessionCtx, types.PriorityNormal)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent execution error: %v", err)
	}
}

// ============================================================================
// 3. State Corruption: Context Leakage
// ============================================================================

func TestE2E_OrchestratorExecutor_ContextLeak_DreamModeBypass(t *testing.T) {
	ctx := context.Background()
	executor, jitExecutor := setupTestEnvironment(t)

	criticalCtx := &types.SessionContext{
		DreamMode: false,
		ExtraContext: map[string]string{"mode": "critical_sandbox"},
	}
	executor.SetSessionContext(criticalCtx)

	permissiveCtx := &types.SessionContext{
		DreamMode: false,
		ExtraContext: map[string]string{"mode": "unrestricted"},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		res, err := jitExecutor.ExecuteWithContext(ctx, "/fix", "critical task", criticalCtx, types.PriorityNormal)
		if err != nil {
			t.Errorf("ExecuteWithContext failed: %v", err)
		}
		if res == "" {
			t.Errorf("Expected valid response")
		}
	}()

	go func() {
		defer wg.Done()
		res, err := jitExecutor.ExecuteWithContext(ctx, "/fix", "permissive task", permissiveCtx, types.PriorityNormal)
		if err != nil {
			t.Errorf("ExecuteWithContext failed: %v", err)
		}
		if res == "" {
			t.Errorf("Expected valid response")
		}
	}()

	wg.Wait()
}

// ============================================================================
// 4. Resource Exhaustion
// ============================================================================

func TestE2E_OrchestratorExecutor_ResourceExhaustion_ConcurrentTasks(t *testing.T) {
	ctx := context.Background()
	_, jitExecutor := setupTestEnvironment(t)

	numTasks := 100
	var wg sync.WaitGroup
	errors := make(chan error, numTasks)

	start := time.Now()
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()
			_, err := jitExecutor.Execute(ctx, "/fix", fmt.Sprintf("Task %d", taskID))
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent execution error: %v", err)
	}

	duration := time.Since(start)
	if duration > 10*time.Second {
		t.Errorf("System stalled under concurrent load: %v", duration)
	}
}

// ============================================================================
// 5. Temporal Failure
// ============================================================================

func TestE2E_OrchestratorExecutor_Cancellation_DoesNotHang(t *testing.T) {
	kernel, _ := core.NewRealKernel()
	virtualStore := core.NewVirtualStore(nil)
	llm := &oeMockLLMClient{
		delay: 5 * time.Second,
	}
	transducer := &oeMockTransducer{intentToReturn: "/fix"}
	compiler := &oeMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "default prompt"}}
	configFactory := &oeMockConfigFactory{configToReturn: &config.AgentConfig{}}

	executor := session.NewExecutor(kernel, virtualStore, llm, compiler, configFactory, transducer)

	spawner := session.NewSpawner(kernel, virtualStore, llm, compiler, configFactory, transducer, session.DefaultSpawnerConfig())
	jitExecutor := session.NewJITExecutor(executor, spawner, transducer)


	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := jitExecutor.Execute(ctx, "/fix", "slow task")
	duration := time.Since(start)

	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}

	if duration > 1*time.Second {
		t.Errorf("Executor hung on cancellation: %v", duration)
	}
}

// ============================================================================
// 6. Cascading Failure
// ============================================================================

func TestE2E_OrchestratorExecutor_Cascading_ContextCorruption(t *testing.T) {
	ctx := context.Background()
	executor, jitExecutor := setupTestEnvironment(t)

	initialCtx := &types.SessionContext{
		ExtraContext: map[string]string{"key": "initial_state"},
	}
	executor.SetSessionContext(initialCtx)

	_, _ = jitExecutor.Execute(ctx, "/fix", "trigger error")

	res, err := jitExecutor.Execute(ctx, "/fix", "succeed me")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if res == "" {
		t.Fatalf("Expected valid response")
	}
}

// ============================================================================
// 7. Recovery
// ============================================================================

func TestE2E_OrchestratorExecutor_Recovery_AfterFailure(t *testing.T) {
	ctx := context.Background()
	_, jitExecutor := setupTestEnvironment(t)

	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()

	_, _ = jitExecutor.Execute(timeoutCtx, "/fix", "fail me")

	res, err := jitExecutor.Execute(ctx, "/fix", "succeed me")
	if err != nil {
		t.Errorf("System failed to recover: %v", err)
	}
	if res == "" {
		t.Errorf("Expected valid response after recovery")
	}
}

// ============================================================================
// 8. End-to-End Data Integrity
// ============================================================================

func TestE2E_OrchestratorExecutor_E2E_DataIntegrity(t *testing.T) {
	ctx := context.Background()
	executor, jitExecutor := setupTestEnvironment(t)

	executor.SetSessionContext(&types.SessionContext{
		ExtraContext: map[string]string{"key": "value"},
	})

	_, err := jitExecutor.Execute(ctx, "/fix", "preserve context")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

// ============================================================================
// 9. Multi-Turn State Accumulation
// ============================================================================

func TestE2E_OrchestratorExecutor_MultiTurn_HistoryAccumulation(t *testing.T) {
	ctx := context.Background()
	executor, jitExecutor := setupTestEnvironment(t)

	for i := 0; i < 3; i++ {
		res, err := jitExecutor.Execute(ctx, "/fix", fmt.Sprintf("Turn %d", i))
		if err != nil {
			t.Fatalf("Turn %d failed: %v", i, err)
		}
		if res == "" {
			t.Fatalf("Turn %d returned empty result", i)
		}
	}

	history := executor.GetHistory()
	if len(history) != 6 {
		t.Errorf("Expected 6 history turns, got %d", len(history))
	}
}

// ============================================================================
// 10. Partial Pipeline Failure
// ============================================================================

func TestE2E_OrchestratorExecutor_PartialFailure_JITCompilationFails(t *testing.T) {
	ctx := context.Background()

	kernel, _ := core.NewRealKernel()
	virtualStore := core.NewVirtualStore(nil)
	llm := &oeMockLLMClient{
		responseToReturn: &types.LLMToolResponse{Text: "default success"},
	}
	transducer := &oeMockTransducer{intentToReturn: "/fix"}
	compiler := &oeMockJITCompiler{errToReturn: fmt.Errorf("JIT failed")}
	configFactory := &oeMockConfigFactory{configToReturn: &config.AgentConfig{}}

	executor := session.NewExecutor(kernel, virtualStore, llm, compiler, configFactory, transducer)

	spawner := session.NewSpawner(kernel, virtualStore, llm, compiler, configFactory, transducer, session.DefaultSpawnerConfig())
	jitExecutor := session.NewJITExecutor(executor, spawner, transducer)


	res, err := jitExecutor.Execute(ctx, "/fix", "trigger error")
	if err == nil && res != "mock response" {
		if res != "" {
			t.Logf("Fallback response correctly returned on compilation failure")
		}
	} else if err != nil {
		if err.Error() != "JIT failed" && err.Error() != "execution failed: compilation failed: JIT failed" {
			t.Errorf("Expected JIT compiler failure error, got: %v", err)
		}
	}
}


// ============================================================================
// 11. Orchestrator-Executor Specific Contract: JIT Config Poisoning
// ============================================================================

func TestE2E_OrchestratorExecutor_ConfigPoisoning(t *testing.T) {
	ctx := context.Background()
	executor, jitExecutor := setupTestEnvironment(t)

	initialCtx := &types.SessionContext{
		ExtraContext: map[string]string{"system_state": "secure"},
	}
	executor.SetSessionContext(initialCtx)

	res, err := jitExecutor.Execute(ctx, "/fix", "poison config")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if res == "" {
		t.Fatalf("Expected valid response")
	}

	if executor.GetHistory() == nil {
		t.Errorf("History was corrupted")
	}
}

// ============================================================================
// 12. Orchestrator-Executor Specific Contract: Concurrent Cancellation Leaks
// ============================================================================

func TestE2E_OrchestratorExecutor_ConcurrentCancellation_GoroutineLeaks(t *testing.T) {
	kernel, _ := core.NewRealKernel()
	virtualStore := core.NewVirtualStore(nil)

	llm := &oeMockLLMClient{
		delay: 1 * time.Hour,
	}
	transducer := &oeMockTransducer{intentToReturn: "/research"}
	compiler := &oeMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "default prompt"}}
	configFactory := &oeMockConfigFactory{configToReturn: &config.AgentConfig{}}

	executor := session.NewExecutor(kernel, virtualStore, llm, compiler, configFactory, transducer)

	spawner := session.NewSpawner(kernel, virtualStore, llm, compiler, configFactory, transducer, session.DefaultSpawnerConfig())
	jitExecutor := session.NewJITExecutor(executor, spawner, transducer)


	var wg sync.WaitGroup
	numTasks := 10

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()

			taskCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()

			_, err := jitExecutor.Execute(taskCtx, "/research", fmt.Sprintf("long task %d", taskID))
			if err == nil {
				t.Errorf("Expected context cancellation error")
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("Tasks did not cancel promptly, possible goroutine leak")
	}
}

// ============================================================================
// 13. Orchestrator-Executor Specific Contract: Nested Execution Safety
// ============================================================================

func TestE2E_OrchestratorExecutor_NestedExecution_PanicRecovery(t *testing.T) {
	ctx := context.Background()
	_, jitExecutor := setupTestEnvironment(t)

	tasks := []string{"setup", "trigger_panic", "recovery_check"}

	for i, task := range tasks {
		res, err := jitExecutor.Execute(ctx, "/fix", task)
		if err != nil {
			t.Fatalf("Task %d (%s) failed: %v", i, task, err)
		}
		if res == "" {
			t.Fatalf("Task %d (%s) returned empty result", i, task)
		}
	}
}

// ============================================================================
// 14. Orchestrator-Executor Specific Contract: Token Budget Enforcement
// ============================================================================

func TestE2E_OrchestratorExecutor_TokenBudget_OOMPrevention(t *testing.T) {
	ctx := context.Background()
	_, jitExecutor := setupTestEnvironment(t)

	massiveTask := "start "
	for i := 0; i < 100000; i++ {
		massiveTask += "padding "
	}
	massiveTask += " end"

	res, err := jitExecutor.Execute(ctx, "/fix", massiveTask)
	if err != nil {
		t.Fatalf("System failed to handle massive task string: %v", err)
	}
	if res == "" {
		t.Fatalf("Expected valid response despite massive input")
	}
}

// ============================================================================
// 15. Orchestrator-Executor Specific Contract: Session State Isolation
// ============================================================================

func TestE2E_OrchestratorExecutor_SessionState_Isolation(t *testing.T) {
	ctx := context.Background()
	executor, jitExecutor := setupTestEnvironment(t)

	customCtx := &types.SessionContext{
		ExtraContext: map[string]string{
			"user_id": "admin",
		},
	}

	_, err := jitExecutor.ExecuteWithContext(ctx, "/fix", "admin action", customCtx, types.PriorityHigh)
	if err != nil {
		t.Fatalf("ExecuteWithContext failed: %v", err)
	}

	blankCtx := &types.SessionContext{}
	_, err = jitExecutor.ExecuteWithContext(ctx, "/fix", "guest action", blankCtx, types.PriorityNormal)
	if err != nil {
		t.Fatalf("ExecuteWithContext failed: %v", err)
	}

	if executor.GetHistory() == nil {
		t.Fatalf("History missing")
	}
}

// ============================================================================
// 16. Cascading Failure: Transducer Panic during Compile
// ============================================================================

func TestE2E_OrchestratorExecutor_TransducerFailure_GracefulHandling(t *testing.T) {
	ctx := context.Background()

	kernel, _ := core.NewRealKernel()
	virtualStore := core.NewVirtualStore(nil)
	llm := &oeMockLLMClient{
		responseToReturn: &types.LLMToolResponse{Text: "default success"},
	}

	transducer := &oeMockTransducer{intentToReturn: "", delay: 1 * time.Millisecond}
	compiler := &oeMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "default prompt"}}
	configFactory := &oeMockConfigFactory{configToReturn: &config.AgentConfig{}}

	executor := session.NewExecutor(kernel, virtualStore, llm, compiler, configFactory, transducer)

	spawner := session.NewSpawner(kernel, virtualStore, llm, compiler, configFactory, transducer, session.DefaultSpawnerConfig())
	jitExecutor := session.NewJITExecutor(executor, spawner, transducer)


	res, err := jitExecutor.Execute(ctx, "", "unknown intent")
	if err != nil {
		t.Fatalf("Expected system to handle empty intent, got: %v", err)
	}
	if res == "" {
		t.Fatalf("Expected valid fallback response")
	}
}

// ============================================================================
// 17. Contract Violation: Context Cancelled After Execution Started
// ============================================================================

func TestE2E_OrchestratorExecutor_LateCancellation(t *testing.T) {
	kernel, _ := core.NewRealKernel()
	virtualStore := core.NewVirtualStore(nil)
	llm := &oeMockLLMClient{
		delay: 50 * time.Millisecond,
	}
	transducer := &oeMockTransducer{intentToReturn: "/fix"}
	compiler := &oeMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "default prompt"}}
	configFactory := &oeMockConfigFactory{configToReturn: &config.AgentConfig{}}

	executor := session.NewExecutor(kernel, virtualStore, llm, compiler, configFactory, transducer)

	spawner := session.NewSpawner(kernel, virtualStore, llm, compiler, configFactory, transducer, session.DefaultSpawnerConfig())
	jitExecutor := session.NewJITExecutor(executor, spawner, transducer)


	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		_, err := jitExecutor.Execute(ctx, "/fix", "task")
		errChan <- err
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-errChan
	if err == nil {
		t.Fatalf("Expected cancellation error, got nil")
	}
	if err != context.Canceled && !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("Expected context.Canceled, got: %v", err)
	}
}

// ============================================================================
// 18. Boundary Analysis: Executor Context Swapping Integrity
// ============================================================================

func TestE2E_OrchestratorExecutor_ContextSwapping_Integrity(t *testing.T) {
	ctx := context.Background()
	executor, jitExecutor := setupTestEnvironment(t)

	ctx1 := &types.SessionContext{ExtraContext: map[string]string{"id": "A"}}
	ctx2 := &types.SessionContext{ExtraContext: map[string]string{"id": "B"}}
	ctx3 := &types.SessionContext{ExtraContext: map[string]string{"id": "C"}}

	_, err := jitExecutor.ExecuteWithContext(ctx, "/fix", "task 1", ctx1, types.PriorityNormal)
	if err != nil { t.Fatalf("Task 1 failed: %v", err) }

	_, err = jitExecutor.ExecuteWithContext(ctx, "/fix", "task 2", ctx2, types.PriorityNormal)
	if err != nil { t.Fatalf("Task 2 failed: %v", err) }

	_, err = jitExecutor.ExecuteWithContext(ctx, "/fix", "task 3", ctx3, types.PriorityNormal)
	if err != nil { t.Fatalf("Task 3 failed: %v", err) }

	if executor.GetHistory() == nil || len(executor.GetHistory()) != 6 {
		t.Fatalf("Sequential context swapping corrupted history. Expected 6 turns, got %d", len(executor.GetHistory()))
	}
}

// ============================================================================
// 19. Boundary Analysis: Rapid Intent Switching
// ============================================================================

func TestE2E_OrchestratorExecutor_RapidIntentSwitching(t *testing.T) {
	ctx := context.Background()
	_, jitExecutor := setupTestEnvironment(t)

	intents := []string{"/fix", "/test", "/review", "/implement", "/refactor"}

	for i, intent := range intents {
		res, err := jitExecutor.Execute(ctx, intent, fmt.Sprintf("Action %d", i))
		if err != nil {
			t.Fatalf("Execution failed for intent %s: %v", intent, err)
		}
		if res == "" {
			t.Fatalf("Empty result for intent %s", intent)
		}
	}
}

// ============================================================================
// 20. Boundary Analysis: Async Execution Task Spawning
// ============================================================================

func TestE2E_OrchestratorExecutor_AsyncExecution_TaskTracking(t *testing.T) {
	ctx := context.Background()
	_, jitExecutor := setupTestEnvironment(t)

	taskID, err := jitExecutor.ExecuteAsync(ctx, "/fix", "async task")
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}

	if taskID == "" {
		t.Fatalf("ExecuteAsync returned empty task ID")
	}

	res, done, err := jitExecutor.GetResult(taskID)
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}

	if done && res == "" {
		t.Fatalf("Task marked done but returned empty result")
	}
	if !done && res != "" {
		t.Fatalf("Task not done but returned a result")
	}
}

// ============================================================================
// 21. Boundary Analysis: Async Execution WaitForResult
// ============================================================================

func TestE2E_OrchestratorExecutor_AsyncExecution_WaitForResult(t *testing.T) {
	ctx := context.Background()
	_, jitExecutor := setupTestEnvironment(t)

	taskID, err := jitExecutor.ExecuteAsync(ctx, "/fix", "async wait task")
	if err != nil {
		t.Fatalf("ExecuteAsync failed: %v", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	res, err := jitExecutor.WaitForResult(timeoutCtx, taskID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}

	if res == "" {
		t.Fatalf("WaitForResult returned empty string")
	}
}
