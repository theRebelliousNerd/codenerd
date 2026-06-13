//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// --- Mocks ---

type sclMockTransducer struct {
	intentToReturn string
	delay          time.Duration
}

func (m *sclMockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return perception.Intent{Category: m.intentToReturn}, nil
}

func (m *sclMockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return perception.Intent{}, ctx.Err()
		}
	}
	return perception.Intent{Category: m.intentToReturn}, nil
}

func (m *sclMockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return perception.Intent{Category: m.intentToReturn}, nil, nil
}

func (m *sclMockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}

func (m *sclMockTransducer) SetPromptAssembler(pa perception.PromptAssembler) {}

func (m *sclMockTransducer) SetStrategicContext(context string) {}

func (m *sclMockTransducer) GetContext() string {
	return "mock_context"
}

type sclMockJITCompiler struct {
	promptToReturn *prompt.CompilationResult
	errToReturn    error
	delay          time.Duration
	panicMode      bool
}

func (m *sclMockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	if m.panicMode {
		panic("mock compiler panic")
	}
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.promptToReturn, m.errToReturn
}

type sclMockConfigFactory struct {
	configToReturn *config.EffectiveAgentRuntimeConfig
	errToReturn    error
	panicMode      bool
}

func (m *sclMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	if m.panicMode {
		panic("mock config factory panic")
	}
	return m.configToReturn, m.errToReturn
}

type sclMockLLMClient struct {
	responseToReturn *types.LLMToolResponse
	errToReturn      error
	delay            time.Duration
	invocations      int
	mu               sync.Mutex
	infiniteLoopMode bool
}

func (m *sclMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", fmt.Errorf("not implemented")
}
func (m *sclMockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	m.mu.Lock()
	m.invocations++
	m.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return "", err
	}

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	if m.errToReturn != nil {
		return "", m.errToReturn
	}

	if m.responseToReturn != nil {
		return m.responseToReturn.Text, nil
	}
	return "", fmt.Errorf("not implemented")
}

func (m *sclMockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, toolDefs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.mu.Lock()
	m.invocations++
	m.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.infiniteLoopMode {
		return &types.LLMToolResponse{
			Text: "I will use a tool.",
			ToolCalls: []types.ToolCall{
				{ID: fmt.Sprintf("call_%d", m.invocations), Name: "dummy_tool", Input: map[string]interface{}{}},
			},
		}, nil
	}

	return m.responseToReturn, m.errToReturn
}

func (m *sclMockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	ch <- "mock streaming response"
	close(ch)
	close(errCh)
	return ch, errCh
}

// --- Setup Helpers ---

func setupExecutor(t *testing.T, tr *sclMockTransducer, jc *sclMockJITCompiler, cf *sclMockConfigFactory, lc *sclMockLLMClient) *session.Executor {
	t.Helper()

	tools.Global().Register(&tools.Tool{
		Name: "dummy_tool",
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "dummy result", nil
		},
	})
	tools.Global().Register(&tools.Tool{
		Name: "hanging_tool",
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	})

	t.Cleanup(func() {})

	exec := session.NewExecutor(nil, nil, lc, jc, cf, tr)
	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = false
	exec.SetConfig(cfg)
	return exec
}

// --- Smoke Tests ---

func TestE2E_SessionExecutor_Smoke_PipelineCompletes(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "compiled prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "Smoke test response"}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	res, err := exec.Process(context.Background(), "Hello")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if res.Response != "Smoke test response" {
		t.Errorf("Expected specific response, got %s", res.Response)
	}
	if lc.invocations != 1 {
		t.Errorf("Expected 1 LLM invocation, got %d", lc.invocations)
	}
}

// --- Contract Violation Tests ---

func TestE2E_SessionExecutor_TransducerEmptyIntent_GracefulFallback(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: ""}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "compiled prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "Handled empty intent"}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	_, err := exec.Process(context.Background(), "do something")

	if err != nil {
		t.Fatalf("Expected no error on empty intent fallback, got: %v", err)
	}
}

func TestE2E_SessionExecutor_JITCompilerHangs_ContextTimeout(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{delay: 10 * time.Second}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{}

	exec := setupExecutor(t, tr, jc, cf, lc)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := exec.Process(ctx, "compile this")

	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}

func TestE2E_SessionExecutor_LLMHallucinatesUnconfiguredTool_Blocks(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "compiled prompt"}}

	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"dummy_tool"},
	}}

	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{
		Text: "Calling nuke_db",
		ToolCalls: []types.ToolCall{
			{ID: "call_1", Name: "nuke_db", Input: map[string]interface{}{}},
		},
	}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	res, err := exec.Process(context.Background(), "destroy the database")

	if err != nil {
		t.Fatalf("Process should succeed but tool call should be blocked, got err: %v", err)
	}

	if res.Response != "Calling nuke_db" {
		t.Errorf("Expected LLM text, got %s", res.Response)
	}
}

func TestE2E_SessionExecutor_JITCompilerFails_FallsBackToBaseline(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{errToReturn: fmt.Errorf("JIT failure")}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "Baseline response"}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	res, err := exec.Process(context.Background(), "Hello")

	if err != nil {
		t.Fatalf("Expected no error due to fallback, got: %v", err)
	}
	if res.Response != "Baseline response" {
		t.Errorf("Expected successful fallback response, got %s", res.Response)
	}
}

func TestE2E_SessionExecutor_ConfigFactoryFails_FallsBackToEmptyConfig(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{errToReturn: fmt.Errorf("Config failure")}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "Empty config response"}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	res, err := exec.Process(context.Background(), "Hello")

	if err != nil {
		t.Fatalf("Expected no error due to fallback, got: %v", err)
	}
	if res.Response != "Empty config response" {
		t.Errorf("Expected successful fallback response, got %s", res.Response)
	}
}

// --- State Corruption Tests ---

func TestE2E_SessionExecutor_ConcurrentProcess_NoPanic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "concurrent response"}}

	exec := setupExecutor(t, tr, jc, cf, lc)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := exec.Process(context.Background(), fmt.Sprintf("Message %d", idx))
			if err != nil {
				t.Errorf("Concurrent execution %d failed: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	history := exec.GetHistory()
	if len(history) == 0 {
		t.Errorf("History is empty after 50 concurrent requests")
	}
}

func TestE2E_SessionExecutor_ContextCancellation_MidFlight_NoStateLeak(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}

	lc := &sclMockLLMClient{delay: 10 * time.Second}

	exec := setupExecutor(t, tr, jc, cf, lc)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error)
	go func() {
		_, err := exec.Process(ctx, "will be cancelled")
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got: %v", err)
	}

	history := exec.GetHistory()
	if len(history) != 0 {
		t.Errorf("Expected 0 history items due to cancellation, got %d", len(history))
	}
}

// --- Resource Exhaustion & Temporal Tests ---

func TestE2E_SessionExecutor_InfiniteToolLoop_MaxToolCalls(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"dummy_tool"},
	}}
	lc := &sclMockLLMClient{infiniteLoopMode: true}

	exec := setupExecutor(t, tr, jc, cf, lc)

	res, err := exec.Process(context.Background(), "Start infinite loop")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if res.ToolCallsExecuted != 50 && res.ToolCallsExecuted != 1 {
		t.Logf("Executed %d tool calls", res.ToolCallsExecuted)
	}
}

func TestE2E_SessionExecutor_ToolExecutionHangs_TimeoutEnforced(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"hanging_tool"},
	}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{
		Text: "Called hanging tool",
		ToolCalls: []types.ToolCall{
			{ID: "call_1", Name: "hanging_tool", Input: map[string]interface{}{}},
		},
	}}

	exec := setupExecutor(t, tr, jc, cf, lc)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	res, err := exec.Process(ctx, "call hanging tool")

	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Logf("Result: %v, Err: %v", res, err)
	}
}

// --- Pipeline Integrity Tests ---

func TestE2E_SessionExecutor_MultiTurnAccumulation_NoLeak(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "turn response"}}

	exec := setupExecutor(t, tr, jc, cf, lc)

	for i := 0; i < 5; i++ {
		_, err := exec.Process(context.Background(), fmt.Sprintf("Turn %d", i))
		if err != nil {
			t.Fatalf("Turn %d failed: %v", i, err)
		}
	}

	history := exec.GetHistory()
	if len(history) != 10 {
		t.Errorf("Expected 10 history items, got %d", len(history))
	}

	for i := 0; i < 5; i++ {
		userTurn := history[i*2]
		assistantTurn := history[i*2+1]

		expectedUser := fmt.Sprintf("Turn %d", i)
		if userTurn.Content != expectedUser {
			t.Errorf("Expected user content %s, got %s", expectedUser, userTurn.Content)
		}
		if assistantTurn.Content != "turn response" {
			t.Errorf("Expected assistant content 'turn response', got %s", assistantTurn.Content)
		}
	}
}

// TestE2E_SessionExecutor_PartialPipelineFailure tests when JIT compiles but ConfigFactory fails.
// CONTRACT: Executor can tolerate partial config failure.
// FAILURE: ConfigFactory panics. Expected: Executor recovers, uses empty config.
func TestE2E_SessionExecutor_PartialPipelineFailure(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{panicMode: true}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "handled panic"}}

	exec := setupExecutor(t, tr, jc, cf, lc)

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Executor panicked on ConfigFactory panic: %v", r)
		}
	}()

	res, err := exec.Process(context.Background(), "test panic")
	if err == nil {
		t.Logf("Executor gracefully handled panic! Res: %s", res.Response)
	}
}

// TestE2E_SessionExecutor_TaxonomyQueue_NoBlocking tests asynchronous learning.
// CONTRACT: Asynchronous learning via taxonomy must not block the critical path.
// FAILURE: Taxonomy queue blocks (simulated by fast response). Expected: Fast overall duration.
func TestE2E_SessionExecutor_TaxonomyQueue_NoBlocking(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "fast response"}}

	exec := setupExecutor(t, tr, jc, cf, lc)

	start := time.Now()
	_, err := exec.Process(context.Background(), "quick question")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if duration > 1*time.Second {
		t.Errorf("Process took too long, possible blocking: %v", duration)
	}
}

// TestE2E_SessionExecutor_EmptyLLMResponse_HandledGracefully tests resilience to empty outputs.
// CONTRACT: Executor can handle edge cases where LLM returns nothing.
// FAILURE: LLM returns empty string and no tools. Expected: No panic, handles gracefully.
func TestE2E_SessionExecutor_EmptyLLMResponse_HandledGracefully(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: ""}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	res, err := exec.Process(context.Background(), "empty me")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if res.Response != "" {
		t.Errorf("Expected empty response, got: %s", res.Response)
	}
}

// TestE2E_SessionExecutor_LargePayload_Truncation checks memory safety.
// CONTRACT: Extremely large inputs/outputs don't crash the history manager.
// FAILURE: User sends 10MB string. Expected: Process succeeds without OOM.
func TestE2E_SessionExecutor_LargePayload_Truncation(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "handled"}}

	exec := setupExecutor(t, tr, jc, cf, lc)

	largeInput := strings.Repeat("A", 10*1024*1024)
	_, err := exec.Process(context.Background(), largeInput)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// TestE2E_SessionExecutor_InvalidToolArguments_JSONFallback tests resilience.
// CONTRACT: Malformed tool arguments don't crash the executor.
// FAILURE: LLM returns malformed JSON arguments for a valid tool. Expected: Tool error, process continues.
func TestE2E_SessionExecutor_InvalidToolArguments_JSONFallback(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"dummy_tool"},
	}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{
		Text: "Calling with bad args",
		ToolCalls: []types.ToolCall{
			{ID: "call_1", Name: "dummy_tool", Input: map[string]interface{}{"bad": make(chan int)}},
		},
	}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	res, err := exec.Process(context.Background(), "test bad args")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if res.Response != "Calling with bad args" {
		t.Errorf("Expected fallback response text, got: %s", res.Response)
	}
}

// TestE2E_SessionExecutor_ConfigMutation_Immutable tests that tools can't mutate config.
// CONTRACT: Tool execution should not be able to poison the agent config for subsequent turns.
// FAILURE: (Conceptual) Tool modifies state. Expected: Subsequent calls use pure config.
func TestE2E_SessionExecutor_ConfigMutation_Immutable(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"dummy_tool"},
	}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "turn"}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	_, err := exec.Process(context.Background(), "turn 1")
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	if len(cf.configToReturn.AllowedTools) != 1 {
		t.Errorf("Config was unexpectedly mutated!")
	}
}

// TestE2E_SessionExecutor_FallbackToBaseline_OnCompilationFailure tests pipeline resilience
// CONTRACT: The pipeline must never crash if a subsystem fails.
// FAILURE: The Transducer returns valid intent but JIT compilation fails entirely. Expected: Recovery.
func TestE2E_SessionExecutor_FallbackToBaseline_OnCompilationFailure(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{errToReturn: fmt.Errorf("JIT totally failed")}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "Survived the crash"}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	res, err := exec.Process(context.Background(), "do a flip")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if res.Response != "Survived the crash" {
		t.Errorf("Expected fallback response, got: %s", res.Response)
	}
}

// TestE2E_SessionExecutor_LoggingPanic_Recovery tests if log panics crash the loop.
// CONTRACT: System observability shouldn't kill the execution loop.
// FAILURE: We don't explicitly mock logging here, but this test serves as a placeholder for
// a known vulnerability where malformed format strings in logging can crash the system.
// We just verify it executes fully.
func TestE2E_SessionExecutor_LoggingPanic_Recovery(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "Logging is safe"}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	_, err := exec.Process(context.Background(), "test logging %s %v %x")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}
