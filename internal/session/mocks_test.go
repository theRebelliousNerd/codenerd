package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// --- MockKernel ---

// MockKernel implements types.Kernel for testing.
type MockKernel struct {
	facts        []types.Fact
	asserts      []types.Fact // Track assertions for verification
	AssertError  error
	QueryError   error
	RetractError error
}

func (m *MockKernel) LoadFacts(facts []types.Fact) error {
	m.facts = append(m.facts, facts...)
	return nil
}

func (m *MockKernel) Query(predicate string) ([]types.Fact, error) {
	if m.QueryError != nil {
		return nil, m.QueryError
	}
	var results []types.Fact
	for _, f := range m.facts {
		if f.Predicate == predicate {
			results = append(results, f)
		}
	}
	return results, nil
}

func (m *MockKernel) QueryAll() (map[string][]types.Fact, error) {
	return nil, nil
}

func (m *MockKernel) Assert(fact types.Fact) error {
	if m.AssertError != nil {
		return m.AssertError
	}
	m.facts = append(m.facts, fact)
	m.asserts = append(m.asserts, fact)
	return nil
}

func (m *MockKernel) AssertBatch(facts []types.Fact) error {
	if m.AssertError != nil {
		return m.AssertError
	}
	m.facts = append(m.facts, facts...)
	return nil
}

func (m *MockKernel) Retract(predicate string) error {
	if m.RetractError != nil {
		return m.RetractError
	}
	var newFacts []types.Fact
	for _, f := range m.facts {
		if f.Predicate != predicate {
			newFacts = append(newFacts, f)
		}
	}
	m.facts = newFacts
	return nil
}

func (m *MockKernel) RetractFact(fact types.Fact) error {
	if m.RetractError != nil {
		return m.RetractError
	}
	var newFacts []types.Fact
	for _, f := range m.facts {
		if f.Predicate != fact.Predicate {
			newFacts = append(newFacts, f)
		}
	}
	m.facts = newFacts
	return nil
}

func (m *MockKernel) UpdateSystemFacts() error                                       { return nil }
func (m *MockKernel) Reset()                                                         {}
func (m *MockKernel) AppendPolicy(policy string)                                     {}
func (m *MockKernel) RetractExactFactsBatch(facts []types.Fact) error                { return nil }
func (m *MockKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error { return nil }

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
	return &types.LLMToolResponse{Text: "default response"}, nil
}

// --- MockTransducer ---

type MockTransducer struct {
	ParseIntentFunc            func(ctx context.Context, input string) (perception.Intent, error)
	ParseIntentWithContextFunc func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error)
}

func (m *MockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	if m.ParseIntentFunc != nil {
		return m.ParseIntentFunc(ctx, input)
	}
	return perception.Intent{Verb: "/general", Category: "/general"}, nil
}

func (m *MockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	if m.ParseIntentWithContextFunc != nil {
		return m.ParseIntentWithContextFunc(ctx, input, history)
	}
	return perception.Intent{Verb: "/general", Category: "/general"}, nil
}

func (m *MockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}

func (m *MockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	// Use the regular ParseIntentWithContext if available
	if m.ParseIntentWithContextFunc != nil {
		intent, err := m.ParseIntentWithContextFunc(ctx, input, history)
		return intent, nil, err
	}
	return perception.Intent{Verb: "/general", Category: "/general"}, nil, nil
}

func (m *MockTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {}
func (m *MockTransducer) SetStrategicContext(context string)                  {}

// --- MockJITCompiler ---

type MockJITCompiler struct {
	CompileFunc func(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error)
}

func (m *MockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	if m.CompileFunc != nil {
		return m.CompileFunc(ctx, cc)
	}
	return &prompt.CompilationResult{Prompt: "default prompt"}, nil
}

// --- MockConfigFactory ---

type MockConfigFactory struct {
	GenerateFunc func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error)
}

func (m *MockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, result, intents...)
	}
	return &config.AgentConfig{}, nil
}

// --- MockVirtualStore ---

type MockVirtualStore struct {
	ReadFileFunc func(path string) ([]string, error)
	ExecFunc     func(ctx context.Context, cmd string, env []string) (string, string, error)
	ReadRawFunc  func(path string) ([]byte, error)
}

func (m *MockVirtualStore) ReadFile(path string) ([]string, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(path)
	}
	return nil, nil
}

func (m *MockVirtualStore) WriteFile(path string, content []string) error { return nil }

func (m *MockVirtualStore) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, cmd, env)
	}
	return "", "", nil
}

func (m *MockVirtualStore) ReadRaw(path string) ([]byte, error) {
	if m.ReadRawFunc != nil {
		return m.ReadRawFunc(path)
	}
	return nil, nil
}

func (m *MockVirtualStore) ListFiles(dir string) ([]string, error) { return nil, nil }
func (m *MockVirtualStore) DeleteFile(path string) error           { return nil }
func (m *MockVirtualStore) Exists(path string) bool                { return true }
func (m *MockVirtualStore) Glob(pattern string) ([]string, error)  { return nil, nil }

// --- Helpers ---

// Helper to create a basic executor with defaults
func createTestExecutor(t *testing.T) *Executor {
	return NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
}

// MockCompressor for subagent tests
type MockCompressor struct {
	CompressFunc func(ctx context.Context, turns []perception.ConversationTurn) (string, error)
}

func (m *MockCompressor) Compress(ctx context.Context, turns []perception.ConversationTurn) (string, error) {
	if m.CompressFunc != nil {
		return m.CompressFunc(ctx, turns)
	}
	return "compressed memory", nil
}

// JoinLines helper for ReadFile mocks
func JoinLines(s string) []string {
	return strings.Split(s, "\n")
}
