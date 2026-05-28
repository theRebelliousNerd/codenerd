package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"context"
	"codeberg.org/TauCeti/mangle-go/analysis"
	"sync"
)

// --- MockKernel ---

type MockKernel struct {
	mu             sync.Mutex
	Facts          []core.Fact
	LoadFactsErr   error
	QueryErr       error
	AssertErr      error
	RetractErr     error
	RetractFactErr error
	AssertBatchErr error
}

func (m *MockKernel) LoadFacts(facts []core.Fact) error {
	if m.LoadFactsErr != nil {
		return m.LoadFactsErr
	}
	m.mu.Lock()
	m.Facts = append(m.Facts, facts...)
	m.mu.Unlock()
	return nil
}

func (m *MockKernel) Query(predicate string) ([]core.Fact, error) {
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	var results []core.Fact
	m.mu.Lock()
	for _, f := range m.Facts {
		if f.Predicate == predicate {
			results = append(results, f)
		}
	}
	m.mu.Unlock()
	return results, nil
}

func (m *MockKernel) QueryAll() (map[string][]core.Fact, error) {
	results := make(map[string][]core.Fact)
	m.mu.Lock()
	for _, f := range m.Facts {
		results[f.Predicate] = append(results[f.Predicate], f)
	}
	m.mu.Unlock()
	return results, nil
}

func (m *MockKernel) Assert(fact core.Fact) error {
	if m.AssertErr != nil {
		return m.AssertErr
	}
	m.mu.Lock()
	m.Facts = append(m.Facts, fact)
	m.mu.Unlock()
	return nil
}

func (m *MockKernel) AssertBatch(facts []core.Fact) error {
	if m.AssertBatchErr != nil {
		return m.AssertBatchErr
	}
	m.mu.Lock()
	m.Facts = append(m.Facts, facts...)
	m.mu.Unlock()
	return nil
}

func (m *MockKernel) Retract(predicate string) error {
	if m.RetractErr != nil {
		return m.RetractErr
	}
	return nil
}

func (m *MockKernel) RetractFact(fact core.Fact) error {
	if m.RetractFactErr != nil {
		return m.RetractFactErr
	}
	return nil
}

func (m *MockKernel) UpdateSystemFacts() error                       { return nil }
func (m *MockKernel) Reset()                                         {}
func (m *MockKernel) AppendPolicy(policy string)                     {}
func (m *MockKernel) RetractExactFactsBatch(facts []core.Fact) error { return nil }
func (m *MockKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	return nil
}
func (m *MockKernel) GetProgramInfo() *analysis.ProgramInfo { return nil }

type MockKernelTx struct {
	k *MockKernel
}
func (tx *MockKernelTx) Retract(predicate string) {}
func (tx *MockKernelTx) RetractFact(fact core.Fact) {}
func (tx *MockKernelTx) RetractExactFact(fact core.Fact) {}
func (tx *MockKernelTx) RetractPredicateSet(predicates map[string]struct{}) {}
func (tx *MockKernelTx) Assert(fact core.Fact) {}
func (tx *MockKernelTx) Commit() error {
	if tx.k.AssertErr != nil {
		return tx.k.AssertErr
	}
	if tx.k.LoadFactsErr != nil {
		return tx.k.LoadFactsErr
	}
	return tx.k.RetractErr
}

func (m *MockKernel) Transaction() types.KernelTransaction {
	return &MockKernelTx{k: m}
}

// --- MockTransducer ---

type MockTransducer struct {
	ParseIntentFunc func(ctx context.Context, input string) (perception.Intent, error)
}

func (m *MockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	if m.ParseIntentFunc != nil {
		return m.ParseIntentFunc(ctx, input)
	}
	return perception.Intent{}, nil
}

func (m *MockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	return m.ParseIntent(ctx, input)
}

func (m *MockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	intent, err := m.ParseIntent(ctx, input)
	return intent, nil, err
}

func (m *MockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}

func (m *MockTransducer) SetPromptAssembler(pa perception.PromptAssembler) {}
func (m *MockTransducer) SetStrategicContext(context string)                  {}

// --- MockLLMClient ---

type MockLLMClient struct {
	CompleteFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, prompt)
	}
	return "ok", nil
}

func (m *MockLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	return "ok", nil
}

func (m *MockLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "ok", StopReason: "end_turn"}, nil
}
