//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// =============================================================================
// MOCKS — tool safety fallback
// =============================================================================

// tsfMockLLMClient returns a tool call response so we can test tool allowlist.
type tsfMockLLMClient struct {
	toolCallName string
	callCount    int64
}

func (m *tsfMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "ok", nil
}
func (m *tsfMockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userInput string) (string, error) {
	atomic.AddInt64(&m.callCount, 1)
	return "ok", nil
}
func (m *tsfMockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userInput string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	atomic.AddInt64(&m.callCount, 1)

	if m.toolCallName != "" {
		return &types.LLMToolResponse{
			ToolCalls: []types.ToolCall{
				{
					Name: m.toolCallName,
				Input: map[string]interface{}{"target": "test"},
				},
			},
		}, nil
	}
	return &types.LLMToolResponse{Text: "ok"}, nil
}
func (m *tsfMockLLMClient) ShouldUsePiggybackTools() bool { return false }

type tsfMockVirtualStore struct {
	executedTools []string
}

func (m *tsfMockVirtualStore) ExecuteTool(ctx context.Context, call types.ToolCall) (string, error) {
	m.executedTools = append(m.executedTools, call.Name)
	return "ok", nil
}
func (m *tsfMockVirtualStore) ReadFile(path string) ([]string, error)        { return nil, nil }
func (m *tsfMockVirtualStore) WriteFile(path string, content []string) error { return nil }
func (m *tsfMockVirtualStore) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	return "", "", nil
}
func (m *tsfMockVirtualStore) ReadRaw(path string) ([]byte, error) { return nil, nil }

type tsfMockConfigFactory struct {
	cfg *config.AgentConfig
	err error
}

func (m *tsfMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.cfg != nil {
		return m.cfg, nil
	}
	return &config.AgentConfig{}, nil
}
func (m *tsfMockConfigFactory) RegisterSpecialist(name string, config *config.AgentConfig) error {
	return nil
}

type tsfMockJITCompiler struct{}

func (m *tsfMockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{Prompt: "mock"}, nil
}

type tsfMockTransducer struct{}

func (m *tsfMockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return perception.Intent{Verb: "/fix"}, nil
}
func (m *tsfMockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	return perception.Intent{Verb: "/fix"}, nil
}
func (m *tsfMockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return perception.Intent{Verb: "/fix"}, nil, nil
}
func (m *tsfMockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}
func (m *tsfMockTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {}
func (m *tsfMockTransducer) SetStrategicContext(ctx string)                      {}

// =============================================================================
// 1. Empty Config = All Tools Allowed (Documents Current Behavior)
// =============================================================================

// TestE2E_ToolSafety_EmptyConfig_AllToolsAllowed documents the current
// behavior: when ConfigFactory returns an empty AgentConfig (no AllowedTools),
// isToolAllowed returns true for ALL tools. This is a fail-open design.
func TestE2E_ToolSafety_EmptyConfig_AllToolsAllowed(t *testing.T) {
	// Empty config = no AllowedTools list
	emptyCfg := &config.AgentConfig{}

	// isToolAllowed(name, cfg) with empty AllowedTools returns true
	// We verify this behavior through the Executor.
	llm := &tsfMockLLMClient{toolCallName: "dangerous_tool"}
	vstore := &tsfMockVirtualStore{}
	jit := &tsfMockJITCompiler{}
	cfgFactory := &tsfMockConfigFactory{cfg: emptyCfg}
	trans := &tsfMockTransducer{}

	exec := session.NewExecutor(nil, vstore, llm, jit, cfgFactory, trans)
	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = false // Safety gate off (no kernel)
	exec.SetConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := exec.Process(ctx, "test dangerous tool")
	t.Logf("Process result: %v err: %v", result, err)
	t.Logf("Tools executed: %v", vstore.executedTools)

	// Document the behavior
	t.Log("NOTE: With empty AgentConfig.Tools.AllowedTools AND SafetyGate=false, " +
		"ALL tool calls are allowed. This is the current fail-open design.")
}

// =============================================================================
// 2. Safety Gate Blocks Tool Without Kernel
// =============================================================================

// TestE2E_ToolSafety_SafetyGateEnabled_NilKernel_BlocksTool verifies that
// when EnableSafetyGate=true and kernel is nil, tool calls are BLOCKED.
// This is the fail-closed behavior.
func TestE2E_ToolSafety_SafetyGateEnabled_NilKernel_BlocksTool(t *testing.T) {
	llm := &tsfMockLLMClient{toolCallName: "any_tool"}
	vstore := &tsfMockVirtualStore{}
	jit := &tsfMockJITCompiler{}
	cfgFactory := &tsfMockConfigFactory{cfg: &config.AgentConfig{
		Tools: config.ToolSet{AllowedTools: []string{"any_tool"}},
	}}
	trans := &tsfMockTransducer{}

	// Create executor with nil kernel and safety gate enabled
	exec := session.NewExecutor(nil, vstore, llm, jit, cfgFactory, trans)
	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = true // Safety gate ON, kernel nil
	exec.SetConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := exec.Process(ctx, "test blocked tool")
	t.Logf("Process result: %v err: %v", result, err)
	t.Logf("Tools executed: %v", vstore.executedTools)

	// With SafetyGate=true and kernel=nil, checkSafety returns false.
	// The executor should skip the tool call.
	for _, tool := range vstore.executedTools {
		if tool == "any_tool" {
			t.Error("FAIL: Tool was executed despite SafetyGate=true and kernel=nil")
		}
	}
}

// =============================================================================
// 3. Restricted Config Blocks Non-Allowed Tools
// =============================================================================

// TestE2E_ToolSafety_RestrictedConfig_BlocksUnlisted verifies that when
// AgentConfig has a specific AllowedTools list, tools not in the list are blocked.
func TestE2E_ToolSafety_RestrictedConfig_BlocksUnlisted(t *testing.T) {
	llm := &tsfMockLLMClient{toolCallName: "forbidden_tool"}
	vstore := &tsfMockVirtualStore{}
	jit := &tsfMockJITCompiler{}
	cfgFactory := &tsfMockConfigFactory{cfg: &config.AgentConfig{
		Tools: config.ToolSet{AllowedTools: []string{"safe_tool_1", "safe_tool_2"}},
	}}
	trans := &tsfMockTransducer{}

	exec := session.NewExecutor(nil, vstore, llm, jit, cfgFactory, trans)
	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = false // Safety gate off
	exec.SetConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := exec.Process(ctx, "test restricted")
	t.Logf("Process result: %v err: %v", result, err)
	t.Logf("Tools executed: %v", vstore.executedTools)

	// forbidden_tool should NOT be in executedTools
	for _, tool := range vstore.executedTools {
		if tool == "forbidden_tool" {
			t.Error("FAIL: forbidden_tool was executed despite being absent from AllowedTools")
		}
	}
}

// =============================================================================
// 4. Config Factory Failure = Open Question
// =============================================================================

// TestE2E_ToolSafety_ConfigFactoryFails_BehaviorDocumented documents what
// happens when the ConfigFactory returns an error. The executor should either
// (a) block all tools (fail-closed) or (b) allow all tools (fail-open).
// This test documents the current behavior.
func TestE2E_ToolSafety_ConfigFactoryFails_BehaviorDocumented(t *testing.T) {
	llm := &tsfMockLLMClient{toolCallName: "test_tool"}
	vstore := &tsfMockVirtualStore{}
	jit := &tsfMockJITCompiler{}
	cfgFactory := &tsfMockConfigFactory{err: fmt.Errorf("config factory explosion")}
	trans := &tsfMockTransducer{}

	exec := session.NewExecutor(nil, vstore, llm, jit, cfgFactory, trans)
	cfg := session.DefaultExecutorConfig()
	cfg.EnableSafetyGate = false
	exec.SetConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := exec.Process(ctx, "test after config failure")
	t.Logf("Process result: %v err: %v", result, err)
	t.Logf("Tools executed after config factory failure: %v", vstore.executedTools)

	if len(vstore.executedTools) > 0 {
		t.Log("KNOWN ISSUE: ConfigFactory failure results in nil config, which means " +
			"isToolAllowed(name, nil) returns true — all tools are allowed. " +
			"This is fail-open behavior when the config factory crashes.")
	} else {
		t.Log("Tools were blocked after config factory failure — good (fail-closed)")
	}
}
