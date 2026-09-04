package session

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// TestSubAgent_Execute_SurfacesToolExecutionError is a regression test for the
// "done claimed without evidence" hole: ProcessWithIntent deliberately returns
// (result, nil) with result.Error set for non-hollow tool failures ("tool
// execution failed: ..." when the final response is empty and tools errored).
// The sub-agent path must mirror the inline JITExecutor path
// (task_executor.go: result.Error -> return response + error) instead of
// dropping result.Error and reporting success with an empty string.
func TestSubAgent_Execute_SurfacesToolExecutionError(t *testing.T) {
	const toolName = "subagent_tool_error_probe"

	// Register a modular tool that always fails. Registry is process-global,
	// so skip registration when a prior run already added it.
	if !tools.Global().Has(toolName) {
		if err := tools.Global().Register(&tools.Tool{
			Name:        toolName,
			Description: "Probe tool that always fails for sub-agent error propagation test",
			Category:    tools.CategoryGeneral,
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				return "", errors.New("boom exploded")
			},
		}); err != nil {
			t.Fatalf("register probe tool: %v", err)
		}
	}

	// LLM adapter: request the failing tool once with an empty final text, so
	// the executor records toolErrs and an empty response. MockLLMClient does
	// not implement ToolResultsProvider, so the loop takes the single-batch
	// path and returns the initial response verbatim for verification.
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				Text: "",
				ToolCalls: []types.ToolCall{
					{
						ID:    "call_1",
						Name:  toolName,
						Input: map[string]any{},
					},
				},
			}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "", nil
		},
	}

	mockConfig := &MockConfigFactory{
		GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
			return &config.EffectiveAgentRuntimeConfig{
				AllowedTools: []string{toolName},
			}, nil
		},
	}

	// Default transducer returns /general (a non-mutating query verb), so the
	// hollow-success gate stays out of the way and the soft tool-failure path
	// (result.Error with nil return) is exercised.
	agent := NewSubAgent(
		DefaultSubAgentConfig("tool-error-probe"),
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		mockConfig,
		&MockTransducer{},
	)

	// Disable gates that depend on workspace/policy state so the only failure
	// signal is the tool error itself. No writes occur, so build/test/critic
	// verification would be no-ops, but turning them off keeps the test
	// hermetic and fast.
	agent.executor.config.EnableSafetyGate = false
	agent.executor.config.VerifyBuildAfterEdits = false
	agent.executor.config.VerifyTestsAfterEdits = false
	agent.executor.config.CriticReviewAfterEdits = false

	// Sanity-check the executor contract first: soft tool failures stay on
	// result.Error with a nil return.
	preset := &perception.Intent{Verb: "/general", Category: "/query"}
	directResult, directErr := agent.executor.ProcessWithIntent(context.Background(), "do failing work", preset)
	if directErr != nil {
		t.Fatalf("ProcessWithIntent returned hard error %v, want soft result.Error contract", directErr)
	}
	if directResult == nil || directResult.Error == nil {
		t.Fatalf("expected soft tool failure on result.Error, got result=%+v", directResult)
	}
	if !strings.Contains(directResult.Error.Error(), "tool execution failed") {
		t.Fatalf("expected result.Error to contain %q, got %q", "tool execution failed", directResult.Error.Error())
	}

	// The actual regression assertion: the sub-agent path must surface that
	// same error instead of reporting success with an empty string.
	resp, err := agent.execute(context.Background(), "do failing work")
	if err == nil {
		t.Fatalf("execute returned nil error with response %q, want error containing %q", resp, "tool execution failed")
	}
	if !strings.Contains(err.Error(), "tool execution failed") {
		t.Errorf("expected error to contain %q, got %q", "tool execution failed", err.Error())
	}
	if strings.TrimSpace(resp) != "" {
		t.Errorf("expected empty response on tool failure, got %q", resp)
	}
}
