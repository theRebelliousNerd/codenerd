package autopoiesis

import (
	"context"
	"testing"

	"codeberg.org/TauCeti/mangle-go/analysis"

	"codenerd/internal/types"
	"codenerd/internal/types/typestest"
)

// --- MockKernelInterface ---

type MockKernelInterface struct {
	// typestest.MockKernel supplies Transaction() (and the rest of types.Kernel) so this mock cannot panic when code under test starts batching updates.
	// Methods declared directly on MockKernelInterface still take precedence over the promoted ones for the behaviour the tests drive.
	typestest.MockKernel

	AssertFactFunc      func(fact types.Fact) error
	AssertFactBatchFunc func(facts []types.Fact) error
	QueryPredicateFunc  func(predicate string) ([]types.Fact, error)
	QueryBoolFunc       func(predicate string) bool
	RetractFactFunc     func(fact types.Fact) error

	QueryFunc       func(predicate string) ([]types.Fact, error)
	AssertFunc      func(fact types.Fact) error
	AssertBatchFunc func(facts []types.Fact) error
	RetractFunc     func(predicate string) error

	// State for verification
	AssertedFacts  []types.Fact
	RetractedFacts []types.Fact
}

var _ types.Kernel = (*MockKernelInterface)(nil)
var _ types.KernelTransactor = (*MockKernelInterface)(nil)

func (m *MockKernelInterface) AssertFact(fact types.Fact) error {
	m.AssertedFacts = append(m.AssertedFacts, fact)
	if m.AssertFactFunc != nil {
		return m.AssertFactFunc(fact)
	}
	return nil
}

func (m *MockKernelInterface) AssertFactBatch(facts []types.Fact) error {
	m.AssertedFacts = append(m.AssertedFacts, facts...)
	if m.AssertFactBatchFunc != nil {
		return m.AssertFactBatchFunc(facts)
	}
	return nil
}

func (m *MockKernelInterface) QueryPredicate(predicate string) ([]types.Fact, error) {
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

func (m *MockKernelInterface) RetractFact(fact types.Fact) error {
	m.RetractedFacts = append(m.RetractedFacts, fact)
	if m.RetractFactFunc != nil {
		return m.RetractFactFunc(fact)
	}
	return nil
}

func (m *MockKernelInterface) Assert(fact types.Fact) error {
	m.AssertedFacts = append(m.AssertedFacts, fact)
	if m.AssertFunc != nil {
		return m.AssertFunc(fact)
	}
	if m.AssertFactFunc != nil {
		return m.AssertFactFunc(fact)
	}
	return nil
}

func (m *MockKernelInterface) AssertBatch(facts []types.Fact) error {
	m.AssertedFacts = append(m.AssertedFacts, facts...)
	if m.AssertBatchFunc != nil {
		return m.AssertBatchFunc(facts)
	}
	if m.AssertFactBatchFunc != nil {
		return m.AssertFactBatchFunc(facts)
	}
	return nil
}

func (m *MockKernelInterface) Query(predicate string) ([]types.Fact, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(predicate)
	}
	if m.QueryPredicateFunc != nil {
		return m.QueryPredicateFunc(predicate)
	}
	if m.QueryBoolFunc != nil {
		if m.QueryBoolFunc(predicate) {
			return []types.Fact{{Predicate: predicate}}, nil
		}
		return nil, nil
	}
	return nil, nil
}

func (m *MockKernelInterface) Retract(predicate string) error {
	if m.RetractFunc != nil {
		return m.RetractFunc(predicate)
	}
	return nil
}

// LoadFacts exists to satisfy types.Kernel and is not exercised.
func (m *MockKernelInterface) LoadFacts(facts []types.Fact) error {
	return nil
}

// QueryAll exists to satisfy types.Kernel and is not exercised.
func (m *MockKernelInterface) QueryAll() (map[string][]types.Fact, error) {
	return nil, nil
}

// UpdateSystemFacts exists to satisfy types.Kernel and is not exercised.
func (m *MockKernelInterface) UpdateSystemFacts() error {
	return nil
}

// GetProgramInfo exists to satisfy types.Kernel and is not exercised.
func (m *MockKernelInterface) GetProgramInfo() *analysis.ProgramInfo {
	return nil
}

// Reset exists to satisfy types.Kernel and is not exercised.
func (m *MockKernelInterface) Reset() {
}

// AppendPolicy exists to satisfy types.Kernel and is not exercised.
func (m *MockKernelInterface) AppendPolicy(policy string) {
}

// RetractExactFactsBatch exists to satisfy types.Kernel and is not exercised.
func (m *MockKernelInterface) RetractExactFactsBatch(facts []types.Fact) error {
	return nil
}

// RemoveFactsByPredicateSet exists to satisfy types.Kernel and is not exercised.
func (m *MockKernelInterface) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	return nil
}
// Transaction exists to satisfy types.KernelTransactor via the embedded typestest.MockKernel.
// Declared explicitly so the KernelTransactor guard's AST scan sees the method; the body
// delegates to the promoted implementation so behaviour is identical to embedding alone.
func (m *MockKernelInterface) Transaction() types.KernelTransaction { return m.MockKernel.Transaction() }


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
