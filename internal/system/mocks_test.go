package system

import (
	"context"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// --- MockSystemKernel ---

type MockSystemKernel struct {
	facts   []core.Fact
	asserts []core.Fact
}

func (m *MockSystemKernel) LoadFacts(facts []core.Fact) error {
	m.facts = append(m.facts, facts...)
	return nil
}

func (m *MockSystemKernel) Query(predicate string) ([]core.Fact, error) {
	var results []core.Fact
	for _, f := range m.facts {
		if f.Predicate == predicate {
			results = append(results, f)
		}
	}
	return results, nil
}

func (m *MockSystemKernel) QueryAll() (map[string][]core.Fact, error) {
	return nil, nil
}

func (m *MockSystemKernel) Assert(fact core.Fact) error {
	m.facts = append(m.facts, fact)
	m.asserts = append(m.asserts, fact)
	return nil
}

func (m *MockSystemKernel) AssertBatch(facts []core.Fact) error {
	m.facts = append(m.facts, facts...)
	return nil
}

func (m *MockSystemKernel) Retract(predicate string) error {
	return nil
}

func (m *MockSystemKernel) RetractFact(fact core.Fact) error {
	return nil
}

func (m *MockSystemKernel) UpdateSystemFacts() error                       { return nil }
func (m *MockSystemKernel) Reset()                                         {}
func (m *MockSystemKernel) AppendPolicy(policy string)                     {}
func (m *MockSystemKernel) RetractExactFactsBatch(facts []core.Fact) error { return nil }
func (m *MockSystemKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	return nil
}

// SystemKernel specific methods
func (m *MockSystemKernel) Evaluate() error                         { return nil }
func (m *MockSystemKernel) LoadFactsFromFile(path string) error     { return nil }
func (m *MockSystemKernel) ConsumeBootPrompts() []core.HybridPrompt { return nil }

// --- MockLLMClient ---

type MockLLMClient struct {
	CompleteFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, prompt)
	}
	return "", nil
}

func (m *MockLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	return "", nil
}

func (m *MockLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, nil
}
