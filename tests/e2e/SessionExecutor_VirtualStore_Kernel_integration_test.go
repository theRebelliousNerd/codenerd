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

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/analysis"
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

func (m *mockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	return &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{
			"read_file", "write_file", "shell_exec", "mock_tool", "blocking_tool", "huge_tool",
			"restricted_tool", "race_tool", "heavy_tool", "leaky_tool", "timing_tool",
		},
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
	// NOTE: t.Parallel() removed — each test creates a full kernel (~5s),
	// and PAUSE blocks these tests from running when sequential tests exceed timeout.
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
	// NOTE: t.Parallel() removed — each test creates a full kernel (~5s),
	// and PAUSE blocks these tests from running when sequential tests exceed timeout.
	exec, _, llm := setupTestExecutor(t)

	llm.piggyback = true
	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{
			Text: `{"surface_response": "I did it", "control_packet": { "mangle_updates": [invalid json] }`,
		}, nil
	}

	res, err := exec.Process(context.Background(), "fix bug")

	if err != nil {
		t.Errorf("Expected graceful fallback on malformed JSON, got error: %v", err)
	}
	if res.Response == "" {
		t.Errorf("Expected non-empty fallback response")
	}
}

func TestE2E_ContractViolation_ToolSpamming(t *testing.T) {
	// NOTE: t.Parallel() removed — each test creates a full kernel (~5s),
	// and PAUSE blocks these tests from running when sequential tests exceed timeout.
	exec, _, llm := setupTestExecutor(t)

	cfg := session.DefaultExecutorConfig()
	cfg.MaxToolCalls = 2
	cfg.EnableSafetyGate = false
	exec.SetConfig(cfg)

	callCount := 0
	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		callCount++
		toolsList := make([]types.ToolCall, 10)
		for i := 0; i < 10; i++ {
			toolsList[i] = types.ToolCall{
				ID:    fmt.Sprintf("call_%d", i),
				Name:  "mock_tool",
				Input: map[string]interface{}{},
			}
		}
		return &types.LLMToolResponse{
			Text:      "I need more info",
			ToolCalls: toolsList,
		}, nil
	}

	tools.Global().Register(&tools.Tool{Name: "mock_tool", Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
		return "data", nil
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := exec.Process(ctx, "start infinite loop")

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if res.ToolCallsExecuted != 2 {
		t.Errorf("Expected exactly 2 tools executed, got: %d", res.ToolCallsExecuted)
	}
}

// ---------------------------------------------------------------------------
// Temporal Failure Tests
// ---------------------------------------------------------------------------

func TestE2E_TemporalFailure_ToolBlocksIndefinitely(t *testing.T) {
	// NOTE: t.Parallel() removed — each test creates a full kernel (~5s),
	// and PAUSE blocks these tests from running when sequential tests exceed timeout.
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
	exec.Process(ctx, "run blocking tool")
	duration := time.Since(start)

	// Executor swallows tool errors and returns success, but it should abort quickly
	if duration > 1*time.Second {
		t.Errorf("Process blocked too long: %v", duration)
	}
}

// ---------------------------------------------------------------------------
// State Corruption / Concurrent Tests
// ---------------------------------------------------------------------------

func TestE2E_StateCorruption_ConcurrentSessionInteraction(t *testing.T) {
	// NOTE: t.Parallel() removed — each test creates a full kernel (~5s),
	// and PAUSE blocks these tests from running when sequential tests exceed timeout.
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
	if len(history) != numConcurrent*2 { // 1 user + 1 assistant turn per interaction
		t.Errorf("Expected %d history items, got %d", numConcurrent*2, len(history))
	}
}

// ---------------------------------------------------------------------------
// Resource Exhaustion Tests
// ---------------------------------------------------------------------------

func TestE2E_ResourceExhaustion_GiganticToolResult(t *testing.T) {
	// NOTE: t.Parallel() removed — each test creates a full kernel (~5s),
	// and PAUSE blocks these tests from running when sequential tests exceed timeout.
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
	cfg.EnableSafetyGate = false
	exec.SetConfig(cfg)

	_, err := exec.Process(context.Background(), "get huge data")

	if err != nil {
		t.Fatalf("Got unexpected fatal error on huge data instead of boundary cap: %v", err)
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
	// NOTE: t.Parallel() removed — each test creates a full kernel (~5s),
	// and PAUSE blocks these tests from running when sequential tests exceed timeout.
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
	// NOTE: t.Parallel() removed — each test creates a full kernel (~5s),
	// and PAUSE blocks these tests from running when sequential tests exceed timeout.
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
	if err1 != nil {
		t.Errorf("Expected fallback on turn 1, got error: %v", err1)
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
	return m.CompleteWithSystem(ctx, "", prompt)
}
func (m *mockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.completeFunc != nil {
		res, err := m.completeFunc(ctx, systemPrompt, userPrompt)
		if err != nil {
			return "", err
		}
		if res != nil {
			return res.Text, nil
		}
	}
	return "mock", nil
}
func (m *mockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	ch <- "mock streaming response"
	close(ch)
	close(errCh)
	return ch, errCh
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
func (m *mockTransducer) SetPromptAssembler(pa perception.PromptAssembler) {}
func (m *mockTransducer) SetStrategicContext(context string)               {}
func (m *mockKernel) GetProgramInfo() *analysis.ProgramInfo                { return nil }

func TestE2E_ContractViolation_TypeSafetyMangleStringVsAtom(t *testing.T) {
	exec, kernel, llm := setupTestExecutor(t)

	llm.piggyback = true
	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{Text: `{"surface_response": "done", "control_packet": {"mangle_updates": ["action_allowed(\"/malicious_atom\")"]}}`}, nil
	}

	_, err := exec.Process(context.Background(), "do a bad atom")
	if err != nil {
		t.Fatalf("Expected nil error but type mismatch blocked evaluation entirely: %v", err)
	}

	kernel.mu.RLock()
	defer kernel.mu.RUnlock()
	for _, f := range kernel.facts {
		if f.Predicate == "action_allowed" {
			if len(f.Args) > 0 {
				argStr := fmt.Sprintf("%v", f.Args[0])
				if argStr == "\"/malicious_atom\"" {
					t.Fatalf("Type safety violated: kernel accepted a Go string as an Atom!")
				}
			}
		}
	}
}

func TestE2E_StateCorruption_VirtualStoreFFIRace(t *testing.T) {
	// 50 goroutines call Execute on VirtualStore for the same kernel simultaneously.
	// We run this to trigger the race detector (-race).
	exec, kernel, llm := setupTestExecutor(t)

	var execCount int32
	tools.Global().Register(&tools.Tool{Name: "race_tool", Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
		time.Sleep(10 * time.Millisecond) // Ensure overlap
		atomic.AddInt32(&execCount, 1)
		return "done", nil
	}})

	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = false
	exec.SetConfig(cfg)

	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{
			Text: "racing",
			ToolCalls: []types.ToolCall{
				{
					ID:    "call_race",
					Name:  "race_tool",
					Input: map[string]interface{}{},
				},
			},
		}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = exec.Process(context.Background(), fmt.Sprintf("race %d", idx))
		}(i)
	}

	wg.Wait()

	if execCount != 50 {
		t.Fatalf("Race corruption: Expected 50 executions, got %d", execCount)
	}
	// Check kernel is alive
	kernel.mu.RLock()
	defer kernel.mu.RUnlock()
}

func TestE2E_ResourceExhaustion_ConcurrentToolExecutions(t *testing.T) {
	exec, _, llm := setupTestExecutor(t)

	hugeData := strings.Repeat("B", 10*1024*1024)
	var completeCount int32
	tools.Global().Register(&tools.Tool{Name: "heavy_tool", Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&completeCount, 1)
		return hugeData, nil
	}})
	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = false
	exec.SetConfig(cfg)

	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{
			Text: "Heavy",
			ToolCalls: []types.ToolCall{
				{
					ID:    "call_heavy",
					Name:  "heavy_tool",
					Input: map[string]interface{}{},
				},
			},
		}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = exec.Process(context.Background(), "get heavy data")
		}()
	}
	wg.Wait()

	if completeCount != 10 {
		t.Fatalf("Expected 10 complete heavy tool executions, got %d. OOM limit bypassed or execution dropped.", completeCount)
	}
}

func TestE2E_TemporalFailure_GoroutineLeakPrevention(t *testing.T) {
	exec, _, llm := setupTestExecutor(t)

	started := make(chan struct{})
	done := make(chan struct{})

	tools.Global().Register(&tools.Tool{Name: "leaky_tool", Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
		close(started)
		select {
		case <-ctx.Done():
			close(done)
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
			return "done", nil
		}
	}})

	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = false
	exec.SetConfig(cfg)

	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{
			Text: "Running leaky tool",
			ToolCalls: []types.ToolCall{
				{
					ID:    "call_leak",
					Name:  "leaky_tool",
					Input: map[string]interface{}{},
				},
			},
		}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_, _ = exec.Process(ctx, "run leak test")
	}()

	<-started // wait for tool to start executing
	cancel()

	select {
	case <-done:
		// Success, the goroutine exited properly
	case <-time.After(2 * time.Second):
		t.Fatalf("Goroutine leaked: did not exit after context cancellation")
	}
}

func TestE2E_Recovery_ContextTimeoutThenSuccess(t *testing.T) {
	exec, _, llm := setupTestExecutor(t)

	var secondTurnSuccess bool
	tools.Global().Register(&tools.Tool{Name: "timing_tool", Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
		if val, ok := args["sleep"]; ok && val == "yes" {
			select {
			case <-time.After(1 * time.Second):
				return "done", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		secondTurnSuccess = true
		return "fast success", nil
	}})

	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = false
	exec.SetConfig(cfg)

	callCount := 0
	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		callCount++
		if callCount == 1 {
			return &types.LLMToolResponse{
				Text: "Timeout call",
				ToolCalls: []types.ToolCall{
					{
						ID:    "call_time1",
						Name:  "timing_tool",
						Input: map[string]interface{}{"sleep": "yes"},
					},
				},
			}, nil
		}
		return &types.LLMToolResponse{
			Text: "Fast call",
			ToolCalls: []types.ToolCall{
				{
					ID:    "call_time2",
					Name:  "timing_tool",
					Input: map[string]interface{}{"sleep": "no"},
				},
			},
		}, nil
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel1()

	_, err1 := exec.Process(ctx1, "do slow")
	if err1 == nil {
		t.Fatalf("Expected timeout error on first turn")
	}

	res, err2 := exec.Process(context.Background(), "do fast")
	if err2 != nil {
		t.Fatalf("Expected success on second turn after previous timeout, got error: %v", err2)
	}

	if res.Response == "" {
		t.Fatalf("Expected valid response")
	}

	if !secondTurnSuccess {
		t.Fatalf("Expected second turn tool to successfully execute")
	}
}

func TestE2E_Smoke_HappyPathPipeline(t *testing.T) {
	// Verifies the end-to-end integration works without failures.
	exec, _, llm := setupTestExecutor(t)

	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{Text: "Success Pipeline"}, nil
	}

	res, err := exec.Process(context.Background(), "run happy path")
	if err != nil {
		t.Fatalf("Expected nil error on happy path, got: %v", err)
	}
	if res.Response != "Success Pipeline" {
		t.Fatalf("Expected 'Success Pipeline', got: %v", res.Response)
	}
}

func TestE2E_ContractViolation_ZeroResultQueryHandling(t *testing.T) {
	exec, kernel, llm := setupTestExecutor(t)

	kernel.queryResults["permitted"] = []types.Fact{} // Empty result set

	tools.Global().Register(&tools.Tool{Name: "restricted_tool", Execute: func(ctx context.Context, args map[string]interface{}) (string, error) {
		return "success", nil
	}})

	cfg := session.DefaultExecutorConfig()
	exec.SetConfig(cfg)

	llm.completeFunc = func(ctx context.Context, prompt string, input string) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{
			Text: "Running restricted",
			ToolCalls: []types.ToolCall{
				{Name: "restricted_tool"},
			},
		}, nil
	}

	_, err := exec.Process(context.Background(), "run zero result")
	if err == nil {
		t.Fatalf("Expected denial error due to zero results, got nil")
	}
}
