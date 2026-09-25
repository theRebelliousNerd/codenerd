package core

import (
	"context"
	"sync"

	"codenerd/internal/tactile"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/analysis"
)

// Shared test doubles for the core package.

// MockExecutor implements tactile.Executor for testing. Execute may be called
// from many goroutines, so History is guarded.
type MockExecutor struct {
	mu          sync.Mutex
	ExecuteFunc func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error)
	History     []tactile.Command
}

func (m *MockExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	m.mu.Lock()
	m.History = append(m.History, cmd)
	m.mu.Unlock()
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, cmd)
	}
	// Default success
	return &tactile.ExecutionResult{
		Success: true, ExitCode: 0,
		Stdout: "MOCK SUCCESS",
	}, nil
}

func (m *MockExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{}
}

func (m *MockExecutor) Validate(cmd tactile.Command) error {
	return nil
}

// MockKernel implements Kernel for testing. Like the real kernel it is safe
// for concurrent use, since callers drive it from many goroutines.
type MockKernel struct {
	mu         sync.Mutex
	Facts      []Fact
	QueryFunc  func(predicate string) ([]Fact, error)
	AssertFunc func(fact Fact) error
}

func (m *MockKernel) Assert(fact Fact) error {
	m.mu.Lock()
	m.Facts = append(m.Facts, fact)
	m.mu.Unlock()
	if m.AssertFunc != nil {
		return m.AssertFunc(fact)
	}
	return nil
}

func (m *MockKernel) AssertBatch(facts []Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Facts = append(m.Facts, facts...)
	return nil
}

func (m *MockKernel) Query(predicate string) ([]Fact, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(predicate)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// Default: mirror the constitution rule, which derives permitted/3 from
	// an asserted pending_action/5 with an exactly matching canonical
	// payload. A hardcoded payload here would silently desync from the
	// payload the loop actually routes (e.g. timeout_seconds) and turn the
	// suite into a permit-denial test.
	if predicate == "permitted" {
		var out []Fact
		for _, f := range m.Facts {
			if f.Predicate == "pending_action" && len(f.Args) >= 4 {
				out = append(out, Fact{Predicate: "permitted", Args: []any{f.Args[1], f.Args[2], f.Args[3]}})
			}
		}
		if len(out) == 0 {
			out = []Fact{
				{Predicate: "permitted", Args: []any{"/run_tests", "go test ./...", "{}"}},
			}
		}
		return out, nil
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

func (m *MockKernel) LoadFacts(facts []Fact) error         { return nil }
func (m *MockKernel) Retract(predicate string) error       { return nil }
func (m *MockKernel) RetractFact(fact Fact) error          { return nil }
func (m *MockKernel) QueryAll() (map[string][]Fact, error) { return nil, nil }
func (m *MockKernel) FactCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Facts)
}
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
