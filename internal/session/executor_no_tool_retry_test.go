package session

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

type proseRetryClient struct {
	calls atomic.Int32
}

type nilResponseClient struct {
	toolName string
	followup bool
}

func (*nilResponseClient) Complete(context.Context, string) (string, error) { return "", nil }
func (*nilResponseClient) CompleteWithSystem(context.Context, string, string) (string, error) {
	return "", nil
}
func (*nilResponseClient) CompleteWithStreaming(
	context.Context, string, string, bool,
) (<-chan string, <-chan error) {
	out := make(chan string)
	errs := make(chan error)
	close(out)
	close(errs)
	return out, errs
}
func (c *nilResponseClient) CompleteWithTools(
	context.Context, string, string, []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	if c.followup {
		return &types.LLMToolResponse{
			ToolCalls: []types.ToolCall{{ID: "nil-followup-1", Name: c.toolName}},
		}, nil
	}
	return nil, nil
}
func (*nilResponseClient) CompleteWithToolResults(
	context.Context, string, []types.Message, []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	return nil, nil
}

func (c *proseRetryClient) Complete(context.Context, string) (string, error) { return "", nil }

func (c *proseRetryClient) CompleteWithSystem(context.Context, string, string) (string, error) {
	return "", nil
}

func (c *proseRetryClient) CompleteWithStreaming(
	context.Context, string, string, bool,
) (<-chan string, <-chan error) {
	out := make(chan string)
	errs := make(chan error)
	close(out)
	close(errs)
	return out, errs
}

func (c *proseRetryClient) CompleteWithTools(
	context.Context, string, string, []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	if c.calls.Add(1) == 1 {
		return &types.LLMToolResponse{Text: "original planning-only response"}, nil
	}
	return &types.LLMToolResponse{Text: "retry-aware final response"}, nil
}

func TestRunToolLoop_PreservesProseOnlyRetryResponse(t *testing.T) {
	const toolName = "no_tool_retry_probe"
	tools.Global().Register(&tools.Tool{
		Name:     toolName,
		Category: tools.CategoryGeneral,
		Execute:  func(context.Context, map[string]any) (string, error) { return "unused", nil },
	})

	client := &proseRetryClient{}
	executor := NewExecutor(
		&requiresToolKernel{MockKernel: &MockKernel{}},
		&MockVirtualStore{},
		client,
		&MockJITCompiler{CompileFunc: func(context.Context, *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			return &prompt.CompilationResult{Prompt: "retry prompt"}, nil
		}},
		&MockConfigFactory{},
		&MockTransducer{},
	)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/create"}}
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}}

	response, _, err := executor.runToolLoop(
		context.Background(), "initial prompt", "create this", cfg, &prompt.CompilationContext{}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	if response == nil || response.Text != "retry-aware final response" {
		t.Fatalf("response = %#v, want the response from the retry prompt", response)
	}
	if calls := client.calls.Load(); calls != 2 {
		t.Fatalf("LLM calls = %d, want initial + one retry", calls)
	}
}

func TestRunToolLoop_RejectsNilInitialResponse(t *testing.T) {
	const toolName = "nil_initial_probe"
	tools.Global().Register(&tools.Tool{
		Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "unused", nil },
	})
	executor := NewExecutor(
		nil, &MockVirtualStore{}, &nilResponseClient{toolName: toolName},
		&MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	_, _, err := executor.runToolLoop(
		context.Background(), "system", "review", &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		nil, &ExecutionResult{Intent: perception.Intent{Verb: "/review"}})
	if err == nil || !strings.Contains(err.Error(), "nil response") {
		t.Fatalf("error = %v, want explicit nil-response rejection", err)
	}
}

func TestRunToolLoop_RejectsNilFollowupResponse(t *testing.T) {
	const toolName = "nil_followup_probe"
	tools.Global().Register(&tools.Tool{
		Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "evidence", nil },
	})
	client := &nilResponseClient{toolName: toolName, followup: true}
	executor := NewExecutor(
		nil, &MockVirtualStore{}, client,
		&MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.config.EnableSafetyGate = false
	_, _, err := executor.runToolLoop(
		context.Background(), "system", "review", &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		nil, &ExecutionResult{Intent: perception.Intent{Verb: "/review"}})
	if err == nil || !strings.Contains(err.Error(), "nil response") {
		t.Fatalf("error = %v, want explicit nil follow-up rejection", err)
	}
}
