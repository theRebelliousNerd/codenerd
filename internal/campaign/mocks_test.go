package campaign

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"context"
)

// --- MockKernel ---

type MockKernel struct {
	Facts []core.Fact
}

func (m *MockKernel) LoadFacts(facts []core.Fact) error {
	m.Facts = append(m.Facts, facts...)
	return nil
}

func (m *MockKernel) Query(predicate string) ([]core.Fact, error) {
	var results []core.Fact
	for _, f := range m.Facts {
		if f.Predicate == predicate {
			results = append(results, f)
		}
	}
	return results, nil
}

func (m *MockKernel) QueryAll() (map[string][]core.Fact, error) {
	results := make(map[string][]core.Fact)
	for _, f := range m.Facts {
		results[f.Predicate] = append(results[f.Predicate], f)
	}
	return results, nil
}

func (m *MockKernel) Assert(fact core.Fact) error {
	m.Facts = append(m.Facts, fact)
	return nil
}

func (m *MockKernel) AssertBatch(facts []core.Fact) error {
	m.Facts = append(m.Facts, facts...)
	return nil
}

func (m *MockKernel) Retract(predicate string) error {
	return nil
}

func (m *MockKernel) RetractFact(fact core.Fact) error {
	return nil
}

func (m *MockKernel) UpdateSystemFacts() error                       { return nil }
func (m *MockKernel) Reset()                                         {}
func (m *MockKernel) AppendPolicy(policy string)                     {}
func (m *MockKernel) RetractExactFactsBatch(facts []core.Fact) error { return nil }
func (m *MockKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	return nil
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

func (m *MockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}

func (m *MockTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {}
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
