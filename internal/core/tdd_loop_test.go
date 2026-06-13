package core

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// =============================================================================
// MOCKS
// =============================================================================

// MockExecutor implements tactile.Executor for testing.
type MockExecutor struct {
	ExecuteFunc func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error)
	History     []tactile.Command
}

func (m *MockExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	m.History = append(m.History, cmd)
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, cmd)
	}
	// Default success
	return &tactile.ExecutionResult{
		ExitCode: 0,
		Stdout:   "MOCK SUCCESS",
	}, nil
}

func (m *MockExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{}
}

func (m *MockExecutor) Validate(cmd tactile.Command) error {
	return nil
}

// MockKernel implements Kernel for testing.
type MockKernel struct {
	Facts      []Fact
	QueryFunc  func(predicate string) ([]Fact, error)
	AssertFunc func(fact Fact) error
}

func (m *MockKernel) Assert(fact Fact) error {
	m.Facts = append(m.Facts, fact)
	if m.AssertFunc != nil {
		return m.AssertFunc(fact)
	}
	return nil
}

func (m *MockKernel) AssertBatch(facts []Fact) error {
	m.Facts = append(m.Facts, facts...)
	return nil
}

func (m *MockKernel) Query(predicate string) ([]Fact, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(predicate)
	}
	// Default: if querying permitted, return everything permitted
	if predicate == "permitted" {
		return []Fact{
			{Predicate: "permitted", Args: []any{"/run_tests", "_"}},
			{Predicate: "permitted", Args: []any{"/read_file", "_"}},
			{Predicate: "permitted", Args: []any{"/write_file", "_"}},
			{Predicate: "permitted", Args: []any{"/exec_cmd", "_"}},
			{Predicate: "permitted", Args: []any{"/escalate", "_"}},
		}, nil
	}
	// Return collected facts matching predicate
	var results []Fact
	for _, f := range m.Facts {
		if f.Predicate == predicate {
			results = append(results, f)
		}
	}
	return results, nil
}

func (m *MockKernel) LoadFacts(facts []Fact) error                                   { return nil }
func (m *MockKernel) Retract(predicate string) error                                 { return nil }
func (m *MockKernel) RetractFact(fact Fact) error                                    { return nil }
func (m *MockKernel) QueryAll() (map[string][]Fact, error)                           { return nil, nil }
func (m *MockKernel) FactCount() int                                                 { return len(m.Facts) }
func (m *MockKernel) IsInitialized() bool                                            { return true }
func (m *MockKernel) LoadPolicyFile(file string) error                               { return nil }
func (m *MockKernel) GetSchemas() string                                             { return "" }
func (m *MockKernel) Clear()                                                         {}
func (m *MockKernel) Reset()                                                         {}
func (m *MockKernel) AppendPolicy(policy string)                                     {}
func (m *MockKernel) RetractExactFactsBatch(facts []Fact) error                      { return nil }
func (m *MockKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error { return nil }
func (m *MockKernel) UpdateSystemFacts() error                                       { return nil }
func (m *MockKernel) String() string                                                 { return "MockKernel" }
func (m *MockKernel) Transaction() types.KernelTransaction                           { return &MockKernelTx{k: m} }
func (m *MockKernel) GetProgramInfo() *analysis.ProgramInfo                          { return nil }

// MockKernelTx implements types.KernelTransaction for testing.
type MockKernelTx struct {
	k *MockKernel
}

func (tx *MockKernelTx) Retract(predicate string)                           {}
func (tx *MockKernelTx) RetractFact(fact Fact)                              {}
func (tx *MockKernelTx) RetractExactFact(fact Fact)                         {}
func (tx *MockKernelTx) RetractPredicateSet(predicates map[string]struct{}) {}
func (tx *MockKernelTx) Assert(fact Fact) {
	_ = tx.k.Assert(fact)
}
func (tx *MockKernelTx) Commit() error { return nil }

// MockLLM implements LLMClient for testing.
type MockLLM struct {
	CompleteFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *MockLLM) Complete(ctx context.Context, prompt string) (string, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, prompt)
	}
	return "MOCKED LLM RESPONSE", nil
}

func (m *MockLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.Complete(ctx, userPrompt)
}

func (m *MockLLM) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	txt, err := m.Complete(ctx, userPrompt)
	if err != nil {
		return nil, err
	}
	return &types.LLMToolResponse{Text: txt}, nil
}

// =============================================================================
// TESTS
// =============================================================================

func SetupTDDLoop(t *testing.T) (*TDDLoop, *MockExecutor, *MockKernel, *MockLLM) {
	mockExec := &MockExecutor{}
	mockKernel := &MockKernel{}
	mockLLM := &MockLLM{}

	vs := NewVirtualStore(mockExec)
	vs.SetKernel(mockKernel)
	vs.DisableBootGuard() // Allow actions

	tdd := NewTDDLoop(vs, mockKernel, mockLLM)
	return tdd, mockExec, mockKernel, mockLLM
}

func TestTDDLoop_NextAction_Idle(t *testing.T) {
	tdd, _, _, _ := SetupTDDLoop(t)
	if action := tdd.NextAction(); action != TDDActionRunTests {
		t.Errorf("Expected NextAction to be RunTests when Idle, got %s", action)
	}
}

func TestTDDLoop_RunTests_Success(t *testing.T) {
	tdd, mockExec, _, _ := SetupTDDLoop(t)
	mockExec.ExecuteFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return &tactile.ExecutionResult{
			ExitCode: 0,
			Stdout:   "ok  	pkg/example	0.001s",
		}, nil
	}

	if err := tdd.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if tdd.GetState() != TDDStatePassing {
		t.Errorf("Expected state Passing, got %s", tdd.GetState())
	}
}

func TestTDDLoop_RunTests_Failure(t *testing.T) {
	tdd, mockExec, _, _ := SetupTDDLoop(t)
	mockExec.ExecuteFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return &tactile.ExecutionResult{
			ExitCode: 1,
			Stdout:   "--- FAIL: TestExample (0.00s)\n    example_test.go:10: expected 1, got 2\nFAIL",
		}, nil
	}

	if err := tdd.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if tdd.GetState() != TDDStateFailing {
		t.Errorf("Expected state Failing, got %s", tdd.GetState())
	}
	if tdd.GetRetryCount() != 1 {
		t.Errorf("Expected retry count 1, got %d", tdd.GetRetryCount())
	}
	if len(tdd.GetDiagnostics()) == 0 {
		t.Error("Expected diagnostics to be parsed")
	}
}

func TestTDDLoop_FullRepairCycle(t *testing.T) {
	tdd, mockExec, _, mockLLM := SetupTDDLoop(t) // mockKernel unused in explicit calls but used by TDDLoop

	// 1. Run Tests -> Fail
	mockExec.ExecuteFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		if strings.Contains(cmd.Arguments[1], "go test") {
			return &tactile.ExecutionResult{
				ExitCode: 1,
				Stdout:   "--- FAIL: TestFoo (0.00s)\n    foo_test.go:42: failure message\nFAIL",
			}, nil
		}
		// Build succeeds
		return &tactile.ExecutionResult{ExitCode: 0}, nil
	}
	_ = tdd.Run(context.Background())
	if tdd.GetState() != TDDStateFailing {
		t.Fatalf("Step 1 failed: expected Failing, got %s", tdd.GetState())
	}

	// 2. Read Error Log (implied transition logic in NextAction -> Run)
	if err := tdd.Run(context.Background()); err != nil {
		t.Fatalf("Step 2 error = %v", err)
	}
	if tdd.GetState() != TDDStateAnalyzing {
		t.Fatalf("Step 2 failed: expected Analyzing, got %s", tdd.GetState())
	}

	// 3. Analyze Root Cause
	if err := tdd.Run(context.Background()); err != nil {
		t.Fatalf("Step 3 error = %v", err)
	}
	if tdd.GetState() != TDDStateGenerating {
		t.Fatalf("Step 3 failed: expected Generating, got %s", tdd.GetState())
	}

	// 4. Generate Patch
	mockLLM.CompleteFunc = func(ctx context.Context, prompt string) (string, error) {
		return "FILE: foo.go\nOLD:\nfoo\nNEW:\nbar\nRATIONALE: fix", nil
	}
	if err := tdd.Run(context.Background()); err != nil {
		t.Fatalf("Step 4 error = %v", err)
	}
	if tdd.GetState() != TDDStateApplying {
		t.Fatalf("Step 4 failed: expected Applying, got %s", tdd.GetState())
	}

	// 5. Apply Patch - Check transition only
	// NOTE: Next execution would try action "apply_patch" which routes to "edit_file".
	// Since we mocked Executor, generic "MOCK SUCCESS" will return 0 exit code if we don't override.
	// But "edit_file" uses FS, not shell exec, unless TDDLoop uses "exec_cmd" for "sed"?
	// Checking tdd_loop.go: logic for ActionGeneratePatch -> transitions to Applying.
	// NextAction is ApplyPatch.
	// tdd.applyPatch() calls virtualStore.RouteAction(..., "/edit_file", ...)
	// VirtualStore.handleEditFile does REAL IO.
	// So if we run tdd.Run(), it will fail because file "foo.go" from mock LLM response doesn't exist.
	// We stop the cycle test here as verifying state transitions up to Applying is sufficient for logic coverage.
}

func TestTDDLoop_Escalation(t *testing.T) {
	tdd, mockExec, _, _ := SetupTDDLoop(t)
	// Fail 3 times
	mockExec.ExecuteFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return &tactile.ExecutionResult{
			ExitCode: 1,
			Stdout:   "FAIL",
		}, nil
	}

	// Simulate max retries
	tdd.config.MaxRetries = 3
	tdd.retryCount = 3
	tdd.state = TDDStateFailing

	if action := tdd.NextAction(); action != TDDActionEscalate {
		t.Errorf("Expected Escalate after max retries, got %s", action)
	}
}

// =============================================================================
// NEGATIVE AND BOUNDARY TESTS
// =============================================================================

// 1. [Null/Empty] Diagnostics: analyzeRootCause handles empty diagnostics
func TestTDDLoop_AnalyzeRootCause_EmptyDiagnostics(t *testing.T) {
	tdd, _, _, _ := SetupTDDLoop(t)
	tdd.state = TDDStateAnalyzing
	tdd.diagnostics = []Diagnostic{}

	if err := tdd.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if tdd.GetState() != TDDStateGenerating {
		t.Errorf("Expected state Generating, got %s", tdd.GetState())
	}
	if tdd.hypothesis != "unknown error - no diagnostics available" {
		t.Errorf("Expected default hypothesis, got %q", tdd.hypothesis)
	}
}

// 2. [Null/Empty] Zero Patches: applyPatch skips applying safely
func TestTDDLoop_ApplyPatch_ZeroPatches(t *testing.T) {
	tdd, _, _, _ := SetupTDDLoop(t)
	tdd.state = TDDStateApplying
	tdd.patches = []Patch{}

	if err := tdd.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if tdd.GetState() != TDDStateCompiling {
		t.Errorf("Expected state Compiling, got %s", tdd.GetState())
	}
}

// 3. [Type Coercion] Mangle Kernel Types: ToFacts works smoothly
func TestTDDLoop_ToFacts_MangleTypes(t *testing.T) {
	tdd, _, _, _ := SetupTDDLoop(t)
	tdd.state = TDDStateFailing
	tdd.retryCount = 2
	tdd.maxRetries = 3

	facts := tdd.ToFacts()

	// Basic validation that facts are structured correctly
	if len(facts) < 3 {
		t.Fatalf("Expected at least 3 facts, got %d", len(facts))
	}

	hasTestState := false
	hasRetryCount := false
	for _, f := range facts {
		if f.Predicate == "test_state" && f.Args[0] == "/failing" {
			hasTestState = true
		}
		if f.Predicate == "retry_count" {
			if count, ok := f.Args[0].(int64); ok && count == 2 {
				hasRetryCount = true
			}
		}
	}

	if !hasTestState || !hasRetryCount {
		t.Errorf("Facts did not contain properly coerced state and retry_count")
	}
}

// 4. [Type Coercion] Test Output Formats: JSON safely handled
func TestTDDLoop_ParseTestOutput_JSON(t *testing.T) {
	tdd, _, _, _ := SetupTDDLoop(t)

	jsonOutput := `{"errors": [{"file": "main.go", "line": 10, "msg": "syntax error"}]}`
	diagnostics := tdd.parseTestOutput(jsonOutput)

	// Should gracefully return 0 diagnostics since it doesn't match standard regex, not panic
	if len(diagnostics) != 0 {
		t.Errorf("Expected 0 diagnostics for JSON, got %d", len(diagnostics))
	}
}

// 5. [User Request Extremes] Large Log File: streaming parsing
func TestTDDLoop_ParseTestOutput_LargeFile(t *testing.T) {
	tdd, _, _, _ := SetupTDDLoop(t)

	// Construct a massive 10MB string
	var sb strings.Builder
	line := "some standard output log that is not an error\n"
	for range 200000 { // approx 10MB
		sb.WriteString(line)
	}
	sb.WriteString("--- FAIL: TestLarge (0.00s)\n")

	output := sb.String()

	// Should not OOM or take excessively long due to bufio.Scanner
	start := time.Now()
	diagnostics := tdd.parseTestOutput(output)
	elapsed := time.Since(start)

	if elapsed > 10*time.Second {
		t.Errorf("Parsing large output took too long: %v", elapsed)
	}

	if len(diagnostics) != 1 {
		t.Errorf("Expected 1 diagnostic, got %d", len(diagnostics))
	}
}

// 6. [User Request Extremes] Long Hypothesis: Truncation
func TestTDDLoop_GeneratePatch_LongHypothesis(t *testing.T) {
	tdd, _, _, mockLLM := SetupTDDLoop(t)
	tdd.state = TDDStateGenerating

	var receivedPrompt string
	mockLLM.CompleteFunc = func(ctx context.Context, prompt string) (string, error) {
		receivedPrompt = prompt
		return "FILE: a.go\nOLD:\n\nNEW:\n\nRATIONALE: r", nil
	}

	// Create a 50,000 char hypothesis
	tdd.hypothesis = strings.Repeat("A", 50000)

	if err := tdd.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// The prompt should contain truncated hypothesis
	if len(receivedPrompt) > 20000 {
		t.Errorf("Prompt is too large (%d bytes), hypothesis was not truncated", len(receivedPrompt))
	}
	if !strings.Contains(receivedPrompt, "... (truncated)") {
		t.Errorf("Prompt missing truncation marker")
	}
}

// 7. [State Conflicts] Concurrent Execution
func TestTDDLoop_Concurrent_Locks(t *testing.T) {
	tdd, mockExec, _, _ := SetupTDDLoop(t)

	mockExec.ExecuteFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return &tactile.ExecutionResult{ExitCode: 0, Stdout: "OK"}, nil
	}

	var wg sync.WaitGroup

	// Spam GetState, InjectPatch, Run concurrently
	for i := range 100 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tdd.GetState()
			if idx%2 == 0 {
				tdd.InjectPatch(Patch{FilePath: "a.go"})
			} else {
				_ = tdd.Run(context.Background())
			}
		}(i)
	}

	wg.Wait()
	// If it doesn't deadlock or panic, it passes.
}

// 8. [State Conflicts] External State Change during generation
func TestTDDLoop_ExternalStateChange_MidGeneration(t *testing.T) {
	tdd, _, _, mockLLM := SetupTDDLoop(t)
	tdd.state = TDDStateGenerating

	// Make LLM complete artificially slow to allow concurrent state change
	syncCh := make(chan struct{})
	mockLLM.CompleteFunc = func(ctx context.Context, prompt string) (string, error) {
		// Signal that we are inside LLM call
		close(syncCh)
		// Sleep a bit to let other goroutine mutate state
		time.Sleep(50 * time.Millisecond)
		return "FILE: a.go\nOLD:\n\nNEW:\n\nRATIONALE: r", nil
	}

	var runErr error
	var wg sync.WaitGroup
	wg.Go(func() {
		runErr = tdd.Run(context.Background())
	})

	// Wait until generatePatch drops the lock and calls LLM
	<-syncCh

	// Mutate state externally (e.g. Cancelled by Guardian)
	tdd.mu.Lock()
	tdd.state = TDDStateIdle
	tdd.mu.Unlock()

	wg.Wait()

	// tdd.Run should have noticed state change and returned an error
	if runErr == nil {
		t.Errorf("Expected error aborting patch application due to state change")
	} else if !strings.Contains(runErr.Error(), "state changed") {
		t.Errorf("Expected 'state changed' error, got: %v", runErr)
	}

	// Patches should not have been applied
	if len(tdd.patches) > 0 {
		t.Errorf("Expected no patches applied, got %d", len(tdd.patches))
	}
}

func (m *MockLLM) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, forceJSON bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	go func() {
		defer close(contentChan)
		defer close(errorChan)
		res, err := m.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()
	return contentChan, errorChan
}
