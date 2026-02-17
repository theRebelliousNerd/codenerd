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

// --- MockToolSynthesizer ---

type MockToolSynthesizer struct {
	ExecuteFunc              func(ctx context.Context, need *ToolNeed) *LoopResult
	GenerateToolFromCodeFunc func(ctx context.Context, name, purpose, code string, confidence, priority float64, isDiagnostic bool) (success bool, toolName, binaryPath, errMsg string)
	SetOnToolRegisteredFunc  func(callback ToolRegisteredCallback)
	GetStatsFunc             func() OuroborosStats
	ListToolsFunc            func() []types.ToolInfo
	GetToolFunc              func(name string) (*types.ToolInfo, bool)
	ExecuteToolFunc          func(ctx context.Context, toolName string, input string) (string, error)
	GetRuntimeToolFunc       func(name string) (*RuntimeTool, bool)
	ListRuntimeToolsFunc     func() []*RuntimeTool
	CheckToolSafetyFunc      func(code string) *SafetyReport
	SetLearningsContextFunc  func(ctx string)
}

func (m *MockToolSynthesizer) Execute(ctx context.Context, need *ToolNeed) *LoopResult {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, need)
	}
	return &LoopResult{Success: false, Error: "mock execute not implemented"}
}

func (m *MockToolSynthesizer) GenerateToolFromCode(ctx context.Context, name, purpose, code string, confidence, priority float64, isDiagnostic bool) (success bool, toolName, binaryPath, errMsg string) {
	if m.GenerateToolFromCodeFunc != nil {
		return m.GenerateToolFromCodeFunc(ctx, name, purpose, code, confidence, priority, isDiagnostic)
	}
	return false, name, "", "mock generate not implemented"
}

func (m *MockToolSynthesizer) SetOnToolRegistered(callback ToolRegisteredCallback) {
	if m.SetOnToolRegisteredFunc != nil {
		m.SetOnToolRegisteredFunc(callback)
	}
}

func (m *MockToolSynthesizer) GetStats() OuroborosStats {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc()
	}
	return OuroborosStats{}
}

func (m *MockToolSynthesizer) ListTools() []types.ToolInfo {
	if m.ListToolsFunc != nil {
		return m.ListToolsFunc()
	}
	return nil
}

func (m *MockToolSynthesizer) GetTool(name string) (*types.ToolInfo, bool) {
	if m.GetToolFunc != nil {
		return m.GetToolFunc(name)
	}
	return nil, false
}

func (m *MockToolSynthesizer) ExecuteTool(ctx context.Context, toolName string, input string) (string, error) {
	if m.ExecuteToolFunc != nil {
		return m.ExecuteToolFunc(ctx, toolName, input)
	}
	return "", nil
}

func (m *MockToolSynthesizer) GetRuntimeTool(name string) (*RuntimeTool, bool) {
	if m.GetRuntimeToolFunc != nil {
		return m.GetRuntimeToolFunc(name)
	}
	return nil, false
}

func (m *MockToolSynthesizer) ListRuntimeTools() []*RuntimeTool {
	if m.ListRuntimeToolsFunc != nil {
		return m.ListRuntimeToolsFunc()
	}
	return nil
}

func (m *MockToolSynthesizer) CheckToolSafety(code string) *SafetyReport {
	if m.CheckToolSafetyFunc != nil {
		return m.CheckToolSafetyFunc(code)
	}
	return &SafetyReport{Safe: true}
}

func (m *MockToolSynthesizer) SetLearningsContext(ctx string) {
	if m.SetLearningsContextFunc != nil {
		m.SetLearningsContextFunc(ctx)
	}
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

// Helper to replace ouroboros with mock
func replaceOuroborosWithMock(orch *Orchestrator) *MockToolSynthesizer {
	mock := &MockToolSynthesizer{}
	orch.mu.Lock()
	orch.ouroboros = mock
	orch.mu.Unlock()
	return mock
}
