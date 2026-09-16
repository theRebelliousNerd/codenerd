//go:build integration

package e2e_test

import (
	"context"
	"errors"
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

// The intent verb belongs in Verb: Category is the "/mutation" | "/query"
// taxonomy slot, and stuffing a verb there makes every turn look verbless to
// the routing, hollow-success, and tool-gating paths.
func (m *sclMockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return perception.Intent{Verb: m.intentToReturn}, nil
}

func (m *sclMockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return perception.Intent{}, ctx.Err()
		}
	}
	return perception.Intent{Verb: m.intentToReturn}, nil
}

func (m *sclMockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return perception.Intent{Verb: m.intentToReturn}, nil, nil
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
	// captured prompts prove which prompt the model actually saw — baseline
	// or JIT — so the fallback tests pin the fallback instead of the echo.
	systemPrompts []string
	userPrompts   []string
}

// Invocations reports the call count under the mock's lock; the field must
// never be read raw once generations run concurrently.
func (m *sclMockLLMClient) Invocations() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.invocations
}

// SystemPrompts returns a copy of every captured system prompt.
func (m *sclMockLLMClient) SystemPrompts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.systemPrompts...)
}

func (m *sclMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", fmt.Errorf("not implemented")
}
func (m *sclMockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	m.mu.Lock()
	m.invocations++
	m.systemPrompts = append(m.systemPrompts, systemPrompt)
	m.userPrompts = append(m.userPrompts, userPrompt)
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
	callSeq := m.invocations
	m.systemPrompts = append(m.systemPrompts, systemPrompt)
	m.userPrompts = append(m.userPrompts, userPrompt)
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
				{ID: fmt.Sprintf("call_%d", callSeq), Name: "dummy_tool", Input: map[string]interface{}{}},
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

// registerSCLToolsOnce guards the global dummy_tool/hanging_tool
// registration. tools.Global().Register is first-wins and errors on
// duplicates, so registering on every setup call is registration spam that
// hides real failures behind ignored errors.
var registerSCLToolsOnce sync.Once

func registerSCLTools(t *testing.T) {
	t.Helper()
	registerSCLToolsOnce.Do(func() {
		if err := tools.Global().Register(&tools.Tool{
			Name:   "dummy_tool",
			Effect: tools.EffectRead,
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				return "dummy result", nil
			},
		}); err != nil {
			panic(fmt.Sprintf("register dummy_tool: %v", err))
		}
		if err := tools.Global().Register(&tools.Tool{
			Name:   "hanging_tool",
			Effect: tools.EffectRead,
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				<-ctx.Done()
				return "", ctx.Err()
			},
		}); err != nil {
			panic(fmt.Sprintf("register hanging_tool: %v", err))
		}
	})
}

func setupExecutor(t *testing.T, tr *sclMockTransducer, jc *sclMockJITCompiler, cf *sclMockConfigFactory, lc *sclMockLLMClient) *session.Executor {
	t.Helper()
	registerSCLTools(t)

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
	if got := lc.Invocations(); got != 1 {
		t.Errorf("Expected 1 LLM invocation, got %d", got)
	}
	history := exec.GetHistory()
	if len(history) != 2 {
		t.Fatalf("Expected exactly one user turn and one assistant turn, got %d", len(history))
	}
	if history[0].Role != "user" || history[0].Content != "Hello" {
		t.Errorf("user turn not recorded intact: %+v", history[0])
	}
	if history[1].Role != "assistant" || history[1].Content != "Smoke test response" {
		t.Errorf("assistant turn not recorded intact: %+v", history[1])
	}
}

// --- Contract Violation Tests ---

func TestE2E_SessionExecutor_TransducerEmptyIntent_GracefulFallback(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: ""}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "compiled prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "Handled empty intent"}}

	exec := setupExecutor(t, tr, jc, cf, lc)
	res, err := exec.Process(context.Background(), "do something")

	if err != nil {
		t.Fatalf("Expected no error on empty intent fallback, got: %v", err)
	}
	if res.Response != "Handled empty intent" {
		t.Errorf("Expected fallback response text, got %q", res.Response)
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

	if !errors.Is(err, context.DeadlineExceeded) {
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
	// The block proof: the attempt is recorded (denied, not silently
	// dropped) but no effect lands — zero successful tool calls.
	if res.ToolCallsExecuted != 1 {
		t.Errorf("Expected 1 recorded tool attempt, got %d", res.ToolCallsExecuted)
	}
	if res.SuccessfulToolCalls != 0 {
		t.Errorf("Hallucinated unconfigured tool executed %d time(s), want 0", res.SuccessfulToolCalls)
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
	// Empty config means the text-only path: no tool may execute.
	if res.ToolCallsExecuted != 0 {
		t.Errorf("Expected 0 tool executions under empty config, got %d", res.ToolCallsExecuted)
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

	const concurrent = 50
	errs := make([]error, concurrent)
	var wg sync.WaitGroup
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = exec.Process(context.Background(), fmt.Sprintf("Message %d", idx))
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("Concurrent execution %d failed: %v", i, err)
		}
	}

	// History is append-locked per turn (pairs may interleave) and capped at
	// 50 entries, so 50 concurrent turns land as the newest 50 with no
	// corruption: every entry well-formed, every content from the known set.
	history := exec.GetHistory()
	if len(history) != 50 {
		t.Fatalf("Expected the 50-entry bounded history after %d concurrent requests, got %d", concurrent, len(history))
	}
	wantInputs := make(map[string]struct{}, concurrent)
	for i := 0; i < concurrent; i++ {
		wantInputs[fmt.Sprintf("Message %d", i)] = struct{}{}
	}
	for i, turn := range history {
		switch turn.Role {
		case "user":
			if _, ok := wantInputs[turn.Content]; !ok {
				t.Errorf("history[%d] has unexpected user content %q", i, turn.Content)
			}
		case "assistant":
			if turn.Content != "concurrent response" {
				t.Errorf("history[%d] has unexpected assistant content %q", i, turn.Content)
			}
		default:
			t.Errorf("history[%d] has unexpected role %q", i, turn.Role)
		}
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
	if !errors.Is(err, context.Canceled) {
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
	// This client cannot consume tool results (no ToolResultsProvider), so
	// the loop degrades to one execution pass instead of looping forever:
	// exactly one generation, one batch, one successful dummy_tool call.
	if res.Response != "I will use a tool." {
		t.Errorf("Expected model text, got %q", res.Response)
	}
	if res.ToolCallsExecuted != 1 {
		t.Errorf("Expected exactly 1 executed tool call, got %d", res.ToolCallsExecuted)
	}
	if res.SuccessfulToolCalls != 1 {
		t.Errorf("Expected exactly 1 successful tool call, got %d", res.SuccessfulToolCalls)
	}
	if got := lc.Invocations(); got != 1 {
		t.Errorf("Expected exactly 1 LLM invocation, got %d", got)
	}
}

// A tool call that outlives the turn's own context aborts the turn with the
// context error: there is no time left to degrade into. The deadline must
// still abandon the hung call instead of pinning the turn forever.
func TestE2E_SessionExecutor_ToolExecutionHangs_DeadlineAbortsTurn(t *testing.T) {
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

	start := time.Now()
	_, err := exec.Process(ctx, "call hanging tool")
	duration := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected the turn deadline to abort the hung tool call, got: %v", err)
	}
	if duration > 5*time.Second {
		t.Errorf("Hung tool pinned the turn for %v; the deadline must abandon it", duration)
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
// CONTRACT: a panicking config factory degrades exactly like a failing one —
// empty config, text-only turn — instead of crashing the turn. (Any panic here
// fails the test outright: there is deliberately no recover.)
func TestE2E_SessionExecutor_PartialPipelineFailure(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{panicMode: true}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "handled panic"}}

	exec := setupExecutor(t, tr, jc, cf, lc)

	res, err := exec.Process(context.Background(), "test panic")
	if err != nil {
		t.Fatalf("Expected no error from factory-panic degradation, got: %v", err)
	}
	if res.Response != "handled panic" {
		t.Errorf("Expected model text to survive the factory panic, got %q", res.Response)
	}
	if res.ToolCallsExecuted != 0 {
		t.Errorf("Expected 0 tool executions under panic-degraded empty config, got %d", res.ToolCallsExecuted)
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
	res, err := exec.Process(context.Background(), "quick question")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if res.Response != "fast response" {
		t.Errorf("Expected model text, got %q", res.Response)
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
// CONTRACT: a 10MB user input neither crashes the turn nor lands whole in
// history: the recorded turn is clamped to head+tail with a marker naming
// what was removed, while the turn itself completes normally.
func TestE2E_SessionExecutor_LargePayload_Truncation(t *testing.T) {
	tr := &sclMockTransducer{intentToReturn: "/coder"}
	jc := &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "prompt"}}
	cf := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}
	lc := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "handled"}}

	exec := setupExecutor(t, tr, jc, cf, lc)

	largeInput := strings.Repeat("A", 10*1024*1024)
	res, err := exec.Process(context.Background(), largeInput)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if res.Response != "handled" {
		t.Errorf("Expected model text, got %q", res.Response)
	}
	history := exec.GetHistory()
	if len(history) != 2 {
		t.Fatalf("Expected 2 history turns, got %d", len(history))
	}
	recorded := history[0].Content
	if len(recorded) >= len(largeInput) {
		t.Fatalf("History stored the full 10MB input unclamped (%d chars)", len(recorded))
	}
	if !strings.Contains(recorded, "conversation turn") || !strings.Contains(recorded, "10485760") {
		t.Errorf("Clamped turn lost its truncation marker: %.120q...", recorded)
	}
	if history[1].Content != "handled" {
		t.Errorf("Assistant turn not recorded intact: %q", history[1].Content)
	}
}

// TestE2E_SessionExecutor_InvalidToolArguments_OpaquePassthrough tests resilience.
// CONTRACT: unmarshallable tool arguments (a chan can never survive JSON)
// don't disturb modular Go dispatch: args travel as map[string]any straight
// to the handler, so the call succeeds and the turn completes.
func TestE2E_SessionExecutor_InvalidToolArguments_OpaquePassthrough(t *testing.T) {
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
	if res.ToolCallsExecuted != 1 || res.SuccessfulToolCalls != 1 {
		t.Errorf("Expected the opaque-args call to dispatch and succeed, got executed=%d successful=%d",
			res.ToolCallsExecuted, res.SuccessfulToolCalls)
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
	res, err := exec.Process(context.Background(), "turn 1")
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if res.Response != "turn" {
		t.Errorf("Expected model text, got %q", res.Response)
	}

	if got := cf.configToReturn.AllowedTools; len(got) != 1 || got[0] != "dummy_tool" {
		t.Errorf("Config was unexpectedly mutated: %q", got)
	}
}

// TestE2E_SessionExecutor_FallbackToBaseline_OnCompilationFailure tests pipeline resilience
// CONTRACT: when JIT compilation fails entirely, the model is served the
// hardcoded baseline prompt — not an empty system prompt, not a crash.
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
	prompts := lc.SystemPrompts()
	if len(prompts) != 1 {
		t.Fatalf("Expected exactly 1 captured system prompt, got %d", len(prompts))
	}
	if !strings.Contains(prompts[0], "You are an AI assistant helping with software development.") {
		t.Errorf("Model did not see the baseline prompt after JIT failure: %q", prompts[0])
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
	res, err := exec.Process(context.Background(), "test logging %s %v %x")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if res.Response != "Logging is safe" {
		t.Errorf("Expected model text, got %q", res.Response)
	}
}

// ResolveAllowedTools projects the same fixture envelope before JIT selection.
func (m *sclMockConfigFactory) ResolveAllowedTools(ctx context.Context, intents ...string) ([]string, error) {
	resolved, err := m.configToReturn, m.errToReturn
	if err != nil || resolved == nil {
		return nil, err
	}
	return append([]string(nil), resolved.AllowedTools...), nil
}
