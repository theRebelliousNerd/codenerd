package core

import (
	"context"
	"strings"
	"testing"

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

func (m *MockKernel) Query(predicate string) ([]Fact, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(predicate)
	}
	// Default: if querying permitted, return everything permitted
	if predicate == "permitted" {
		return []Fact{
			{Predicate: "permitted", Args: []interface{}{"/run_tests", "_"}},
			{Predicate: "permitted", Args: []interface{}{"/read_file", "_"}},
			{Predicate: "permitted", Args: []interface{}{"/write_file", "_"}},
			{Predicate: "permitted", Args: []interface{}{"/exec_cmd", "_"}},
			{Predicate: "permitted", Args: []interface{}{"/escalate", "_"}},
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
