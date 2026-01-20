package autopoiesis

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// --- MockKernelInterface ---

type MockKernelInterface struct {
	AssertFactFunc      func(fact types.KernelFact) error
	AssertFactBatchFunc func(facts []types.KernelFact) error
	QueryPredicateFunc  func(predicate string) ([]types.KernelFact, error)
	QueryBoolFunc       func(predicate string) bool
	RetractFactFunc     func(fact types.KernelFact) error

	// State for verification
	AssertedFacts  []types.KernelFact
	RetractedFacts []types.KernelFact
}

func (m *MockKernelInterface) AssertFact(fact types.KernelFact) error {
	m.AssertedFacts = append(m.AssertedFacts, fact)
	if m.AssertFactFunc != nil {
		return m.AssertFactFunc(fact)
	}
	return nil
}

func (m *MockKernelInterface) AssertFactBatch(facts []types.KernelFact) error {
	m.AssertedFacts = append(m.AssertedFacts, facts...)
	if m.AssertFactBatchFunc != nil {
		return m.AssertFactBatchFunc(facts)
	}
	return nil
}

func (m *MockKernelInterface) QueryPredicate(predicate string) ([]types.KernelFact, error) {
	if m.QueryPredicateFunc != nil {
		return m.QueryPredicateFunc(predicate)
	}
	return nil, nil
}

func (m *MockKernelInterface) QueryBool(predicate string) bool {
	if m.QueryBoolFunc != nil {
		return m.QueryBoolFunc(predicate)
	}
	return false
}

func (m *MockKernelInterface) RetractFact(fact types.KernelFact) error {
	m.RetractedFacts = append(m.RetractedFacts, fact)
	if m.RetractFactFunc != nil {
		return m.RetractFactFunc(fact)
	}
	return nil
}

// --- MockLLMClient ---

type MockLLMClient struct {
	CompleteFunc           func(ctx context.Context, prompt string) (string, error)
	CompleteWithSystemFunc func(ctx context.Context, sys, user string) (string, error)
	CompleteWithToolsFunc  func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error)
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, prompt)
	}
	return "", nil
}

func (m *MockLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	if m.CompleteWithSystemFunc != nil {
		return m.CompleteWithSystemFunc(ctx, sys, user)
	}
	return "", nil
}

func (m *MockLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if m.CompleteWithToolsFunc != nil {
		return m.CompleteWithToolsFunc(ctx, sys, user, tools)
	}
	return &types.LLMToolResponse{Text: "default"}, nil
}

// Helper to create a test orchestrator
func createTestOrchestrator(t *testing.T) (*Orchestrator, *MockKernelInterface, *MockLLMClient) {
	mockLLM := &MockLLMClient{}
	mockKernel := &MockKernelInterface{}

	cfg := Config{
		ToolsDir:         t.TempDir(),
		AgentsDir:        t.TempDir(),
		WorkspaceRoot:    t.TempDir(),
		MinConfidence:    0.6,
		MaxLearningFacts: 10,
	}

	orch := NewOrchestrator(mockLLM, cfg)
	orch.SetKernel(mockKernel)

	return orch, mockKernel, mockLLM
}
