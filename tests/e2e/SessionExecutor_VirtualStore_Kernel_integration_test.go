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

	"codenerd/internal/articulation"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// ---------------------------------------------------------------------------
// Mocks & Helpers
// ---------------------------------------------------------------------------

// mockKernel implements a minimal types.Kernel interface.
type mockKernel struct {
	mu           sync.RWMutex
	facts        []types.Fact
	queryResults map[string][]types.Fact
	shouldPanic  bool
}

func newMockKernel() *mockKernel {
	return &mockKernel{
		facts:        make([]types.Fact, 0),
		queryResults: make(map[string][]types.Fact),
	}
}

func (m *mockKernel) Assert(fact types.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shouldPanic {
		panic("mock kernel panic")
	}
	m.facts = append(m.facts, fact)
	return nil
}

func (m *mockKernel) Retract(predicate string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	newFacts := []types.Fact{}
	for _, f := range m.facts {
		if f.Predicate != predicate {
			newFacts = append(newFacts, f)
		}
	}
	m.facts = newFacts
	return nil
}

func (m *mockKernel) Query(query string) ([]types.Fact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	baseQuery := strings.Split(query, "(")[0]
	if res, ok := m.queryResults[baseQuery]; ok {
		return res, nil
	}
	if res, ok := m.queryResults[query]; ok {
		return res, nil
	}
	return []types.Fact{}, nil
}

func (m *mockKernel) LoadFacts(facts []types.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.facts = append(m.facts, facts...)
	return nil
}

// mockVirtualStore implements types.VirtualStore
type mockVirtualStore struct {
	executeFunc func(ctx context.Context, call types.ToolCall) (string, error)
}

func (m *mockVirtualStore) ExecuteTool(ctx context.Context, call types.ToolCall) (string, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, call)
	}
	return "success", nil
}

func (m *mockVirtualStore) ReadFile(path string) ([]string, error)        { return nil, nil }
func (m *mockVirtualStore) WriteFile(path string, content []string) error { return nil }
func (m *mockVirtualStore) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	return "", "", nil
}
func (m *mockVirtualStore) ReadRaw(path string) ([]byte, error) { return nil, nil }

// mockLLMClient implements types.LLMClient
type mockLLMClient struct {
	completeFunc func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error)
	piggyback    bool
}

func (m *mockLLMClient) CompleteWithTools(ctx context.Context, prompt string, input string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt, input)
	}
	return &types.LLMToolResponse{Text: "mock response"}, nil
}

func (m *mockLLMClient) ShouldUsePiggybackTools() bool {
	return m.piggyback
}

// mockJITCompiler
type mockJITCompiler struct{}

func (m *mockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{
		Prompt: "Mock Prompt",
	}, nil
}

// mockConfigFactory
type mockConfigFactory struct{}

func (m *mockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
	return &config.AgentConfig{
		Tools: config.ToolSet{AllowedTools: []string{"read_file", "write_file", "shell_exec", "mock_tool", "blocking_tool", "huge_tool"}},
	}, nil
}

// mockTransducer
type mockTransducer struct {
	intent perception.Intent
	err    error
}

func (m *mockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return m.intent, m.err
}

// ---------------------------------------------------------------------------
// Setup Helper
// ---------------------------------------------------------------------------

func setupTestExecutor(t *testing.T) (*session.Executor, *mockKernel, *mockLLMClient) {
	t.Helper()
	kernel := newMockKernel()
	vStore := &mockVirtualStore{}
	llm := &mockLLMClient{}
	jit := &mockJITCompiler{}
	cf := &mockConfigFactory{}
	trans := &mockTransducer{
		intent: perception.Intent{
			Verb:   "/fix",
			Target: "test.go",
		},
	}

	exec := session.NewExecutor(kernel, vStore, llm, jit, cf, trans)

	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = false
	cfg.ToolTimeout = 1 * time.Second
	exec.SetConfig(cfg)

	return exec, kernel, llm
}

// ---------------------------------------------------------------------------
// Contract Violation Tests
// ---------------------------------------------------------------------------

func TestE2E_ContractViolation_TransducerUnknownIntent(t *testing.T) {
	t.Parallel()
	exec, kernel, llm := setupTestExecutor(t)

	trans := &mockTransducer{
		intent: perception.Intent{
			Verb: "/unknown_action",
		},
	}
	exec = session.NewExecutor(kernel, &mockVirtualStore{}, llm, &mockJITCompiler{}, &mockConfigFactory{}, trans)
	exec.SetConfig(session.DefaultExecutorConfig())

	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{Text: "fallback handled it"}, nil
	}

	res, err := exec.Process(context.Background(), "do something weird")

	if err != nil {
		t.Fatalf("Expected graceful fallback, got error: %v", err)
	}
	if res == nil || res.Response != "fallback handled it" {
		t.Errorf("Expected fallback response, got: %v", res)
	}
}

func TestE2E_ContractViolation_LLMReturnsMalformedPiggyback(t *testing.T) {
	t.Parallel()
	exec, _, llm := setupTestExecutor(t)

	llm.piggyback = true
	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{
			Text: `{"surface_response": "I did it", "control_packet": { "mangle_updates": [invalid json] }`,
		}, nil
	}

	res, err := exec.Process(context.Background(), "fix bug")

	if err == nil {
		t.Errorf("Expected error from malformed JSON, got success: %v", res)
	}
}

func TestE2E_ContractViolation_ToolSpamming(t *testing.T) {
	t.Parallel()
	exec, _, llm := setupTestExecutor(t)

	callCount := 0
	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		callCount++
		return &types.LLMToolResponse{
			Text: "I need more info",
			ToolCalls: []types.ToolCall{
				{
					ID:    "call_1",
					Name:  "mock_tool",
					Input: map[string]interface{}{},
				},
			},
		}, nil
	}

	tools.Global().Register(&tools.Tool{Name: "mock_tool", Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
		return "data", nil
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := exec.Process(ctx, "start infinite loop")

	if err == nil {
		t.Errorf("Expected max turns error, got success")
	}
	if callCount > 10 {
		t.Errorf("Executor didn't break out of tool loop in time, call count: %d", callCount)
	}
}

// ---------------------------------------------------------------------------
// Temporal Failure Tests
// ---------------------------------------------------------------------------

func TestE2E_TemporalFailure_ToolBlocksIndefinitely(t *testing.T) {
	t.Parallel()
	exec, _, llm := setupTestExecutor(t)

	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{
			Text: "executing tool",
			ToolCalls: []types.ToolCall{
				{
					ID:    "call_1",
					Name:  "blocking_tool",
					Input: map[string]interface{}{},
				},
			},
		}, nil
	}

	tools.Global().Register(&tools.Tool{Name: "blocking_tool", Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			return "too late", nil
		}
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := exec.Process(ctx, "run blocking tool")
	duration := time.Since(start)

	if err == nil {
		t.Errorf("Expected context error, got success")
	}
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
	if duration > 1*time.Second {
		t.Errorf("Process blocked too long: %v", duration)
	}
}

// ---------------------------------------------------------------------------
// State Corruption / Concurrent Tests
// ---------------------------------------------------------------------------

func TestE2E_StateCorruption_ConcurrentSessionInteraction(t *testing.T) {
	t.Parallel()
	exec, _, llm := setupTestExecutor(t)

	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		time.Sleep(10 * time.Millisecond)
		return &types.LLMToolResponse{Text: "done"}, nil
	}

	var wg sync.WaitGroup
	numConcurrent := 10
	errs := make([]error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := exec.Process(context.Background(), fmt.Sprintf("req %d", idx))
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	history := exec.GetHistory()
	if len(history) != numConcurrent {
		t.Errorf("Expected %d history items, got %d", numConcurrent, len(history))
	}
}

// ---------------------------------------------------------------------------
// Resource Exhaustion Tests
// ---------------------------------------------------------------------------

func TestE2E_ResourceExhaustion_GiganticToolResult(t *testing.T) {
	t.Parallel()
	exec, _, llm := setupTestExecutor(t)

	turnCount := 0
	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		turnCount++
		if turnCount > 1 {
			return &types.LLMToolResponse{Text: "done processing huge data"}, nil
		}
		return &types.LLMToolResponse{
			Text: "fetching data",
			ToolCalls: []types.ToolCall{
				{
					ID:    "call_1",
					Name:  "huge_tool",
					Input: map[string]interface{}{},
				},
			},
		}, nil
	}

	hugeData := strings.Repeat("A", 50*1024*1024)
	tools.Global().Register(&tools.Tool{Name: "huge_tool", Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
		return hugeData, nil
	}})

	cfg := session.DefaultExecutorConfig()
	cfg.MaxToolCalls = 2
	exec.SetConfig(cfg)

	_, err := exec.Process(context.Background(), "get huge data")

	if err != nil {
		t.Logf("Got error on huge data (expected if MaxTurns trips): %v", err)
	}
	history := exec.GetHistory()
	if len(history) == 0 {
		t.Errorf("History is empty")
	}
}

// ---------------------------------------------------------------------------
// Cascading Failure Tests
// ---------------------------------------------------------------------------

func TestE2E_CascadingFailure_KernelPanics(t *testing.T) {
	t.Parallel()
	exec, kernel, _ := setupTestExecutor(t)

	kernel.shouldPanic = true

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Panic caught in test as expected: %v", r)
		}
	}()

	exec.Process(context.Background(), "do stuff")
}

// ---------------------------------------------------------------------------
// Recovery Tests
// ---------------------------------------------------------------------------

func TestE2E_Recovery_InvalidPiggybackThenSuccess(t *testing.T) {
	t.Parallel()
	exec, _, llm := setupTestExecutor(t)

	llm.piggyback = true
	turn := 0
	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		turn++
		if turn == 1 {
			return &types.LLMToolResponse{Text: `{"surface_response": "I did it", "control_packet": { "mangle_updates": [invalid json] }`}, nil
		}
		return &types.LLMToolResponse{Text: `{"surface_response": "Fixed now", "control_packet": { "mangle_updates": [] }}`}, nil
	}

	_, err1 := exec.Process(context.Background(), "turn 1")
	if err1 == nil {
		t.Errorf("Expected error on turn 1")
	}

	res, err2 := exec.Process(context.Background(), "turn 2")
	if err2 != nil {
		t.Errorf("Expected recovery on turn 2, got error: %v", err2)
	}
	if res.Response != "Fixed now" {
		t.Errorf("Unexpected response: %v", res.Response)
	}
}
func (m *mockKernel) RetractFact(fact types.Fact) error                              { return m.Retract(fact.Predicate) }
func (m *mockKernel) AssertBatch(facts []types.Fact) error                           { return nil }
func (m *mockKernel) QueryAll() (map[string][]types.Fact, error)                     { return nil, nil }
func (m *mockKernel) UpdateSystemFacts() error                                       { return nil }
func (m *mockKernel) Reset()                                                         {}
func (m *mockKernel) AppendPolicy(policy string)                                     {}
func (m *mockKernel) RetractExactFactsBatch(facts []types.Fact) error                { return nil }
func (m *mockKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error { return nil }
func (m *mockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "mock", nil
}
func (m *mockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "mock", nil
}
func (m *mockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	return m.intent, m.err
}
func (m *mockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return m.intent, nil, m.err
}
func (m *mockTransducer) ResolveFocus(ctx context.Context, input string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}
func (m *mockTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {}
func (m *mockTransducer) SetStrategicContext(context string)                  {}
