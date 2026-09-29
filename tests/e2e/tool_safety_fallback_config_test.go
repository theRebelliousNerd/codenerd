//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

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
					Name:  m.toolCallName,
					Input: map[string]interface{}{"target": "test"},
				},
			},
		}, nil
	}
	return &types.LLMToolResponse{Text: "ok"}, nil
}
func (m *tsfMockLLMClient) ShouldUsePiggybackTools() bool { return false }

func (m *tsfMockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	ch <- "mock streaming response"
	close(ch)
	close(errCh)
	return ch, errCh
}

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
	cfg *config.EffectiveAgentRuntimeConfig
	err error
}

func (m *tsfMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.cfg != nil {
		return m.cfg, nil
	}
	return &config.EffectiveAgentRuntimeConfig{}, nil
}
func (m *tsfMockConfigFactory) RegisterSpecialist(name string, config *config.EffectiveAgentRuntimeConfig) error {
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
func (m *tsfMockTransducer) SetPromptAssembler(pa perception.PromptAssembler) {}
func (m *tsfMockTransducer) SetStrategicContext(ctx string)                   {}

// =============================================================================
// 1. Empty Config = No Tools Allowed
// =============================================================================

// TestE2E_ToolSafety_EmptyConfig_BlocksAllTools proves that a missing capability
// envelope cannot grant ambient access, even when the constitutional safety gate
// is disabled for this isolated capability test.
func TestE2E_ToolSafety_EmptyConfig_BlocksAllTools(t *testing.T) {
	// Empty config = no AllowedTools list
	emptyCfg := &config.EffectiveAgentRuntimeConfig{}

	// Verify the effective capability boundary through the Executor.
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

	if len(vstore.executedTools) != 0 {
		t.Fatalf("empty capability config executed tools: %v", vstore.executedTools)
	}
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
	cfgFactory := &tsfMockConfigFactory{cfg: &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"any_tool"},
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
	cfgFactory := &tsfMockConfigFactory{cfg: &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"safe_tool_1", "safe_tool_2"},
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
// 4. Config Factory Failure = No Ambient Capabilities
// =============================================================================

// TestE2E_ToolSafety_ConfigFactoryFails_BlocksTools proves that compilation
// fallback cannot turn a missing capability envelope into unrestricted access.
func TestE2E_ToolSafety_ConfigFactoryFails_BlocksTools(t *testing.T) {
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

	if len(vstore.executedTools) != 0 {
		t.Fatalf("config factory failure executed tools: %v", vstore.executedTools)
	}
}
