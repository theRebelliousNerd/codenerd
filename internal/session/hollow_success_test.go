package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

func TestIsWriteOrientedIntent(t *testing.T) {
	cases := map[string]bool{
		"/create":   true,
		"/fix":      true,
		"/refactor": true,
		"/write":    true,
		"/explain":  false,
		"/review":   false,
		"/research": false,
		"":          false,
	}
	for verb, want := range cases {
		if got := isWriteOrientedIntent(verb); got != want {
			t.Errorf("isWriteOrientedIntent(%q)=%v want %v", verb, got, want)
		}
	}
}

func TestIsWriteMutationTool(t *testing.T) {
	if !isWriteMutationTool("write_file") || !isWriteMutationTool("edit_file") {
		t.Fatal("expected write_file/edit_file to be mutation tools")
	}
	if isWriteMutationTool("read_file") || isWriteMutationTool("search_files") {
		t.Fatal("read/search must not count as write mutations")
	}
}

func TestCheckHollowSuccess_WriteOrientedNoTools(t *testing.T) {
	exec := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
	result := &ExecutionResult{
		Intent:            perception.Intent{Verb: "/create"},
		ToolCallsExecuted: 0,
	}
	err := exec.checkHollowSuccess(result)
	if err == nil {
		t.Fatal("expected hollow success error for /create with zero tool calls")
	}
	if !strings.Contains(err.Error(), "hollow success blocked") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckHollowSuccess_WriteOrientedOnlyReadTools(t *testing.T) {
	exec := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
	result := &ExecutionResult{
		Intent:               perception.Intent{Verb: "/fix"},
		ToolCallsExecuted:    2,
		SuccessfulWriteTools: 0,
	}
	err := exec.checkHollowSuccess(result)
	if err == nil {
		t.Fatal("expected hollow success when write-oriented intent has no write tools")
	}
	if !strings.Contains(err.Error(), "write_file") {
		t.Fatalf("expected write_file mention, got: %v", err)
	}
}

func TestCheckHollowSuccess_WriteOrientedWithWriteTool(t *testing.T) {
	exec := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
	result := &ExecutionResult{
		Intent:               perception.Intent{Verb: "/create"},
		ToolCallsExecuted:    1,
		SuccessfulWriteTools: 1,
	}
	if err := exec.checkHollowSuccess(result); err != nil {
		t.Fatalf("expected success when write tool landed: %v", err)
	}
}

func TestCheckHollowSuccess_DreamModeExempt(t *testing.T) {
	exec := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
	exec.SetSessionContext(&types.SessionContext{DreamMode: true})
	result := &ExecutionResult{
		Intent:            perception.Intent{Verb: "/create"},
		ToolCallsExecuted: 0,
	}
	if err := exec.checkHollowSuccess(result); err != nil {
		t.Fatalf("dream mode must be exempt from hollow check: %v", err)
	}
}

func TestCheckHollowSuccess_QueryVerbAllowedProse(t *testing.T) {
	exec := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
	result := &ExecutionResult{
		Intent:            perception.Intent{Verb: "/explain"},
		ToolCallsExecuted: 0,
	}
	if err := exec.checkHollowSuccess(result); err != nil {
		t.Fatalf("query/analysis verbs may return prose only: %v", err)
	}
}

func TestProcessWithIntent_HollowCreateFails(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			// Planning-only prose — the hollow success pattern from live matrix.
			return &types.LLMToolResponse{
				Text: "Created both files for a minimal Go HTTP server:\n\n**backend/main.go** ...",
			}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "Created both files...", nil
		},
	}
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/create", Category: "/mutation"}, nil
		},
	}
	mockJIT := &MockJITCompiler{
		CompileFunc: func(ctx context.Context, cctx *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			return &prompt.CompilationResult{Prompt: "system"}, nil
		},
	}
	mockCfg := &MockConfigFactory{
		GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
			return &config.EffectiveAgentRuntimeConfig{
				AllowedTools: []string{"write_file", "edit_file", "read_file"},
			}, nil
		},
	}

	executor := NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		mockJIT,
		mockCfg,
		mockTransducer,
	)

	preset := &perception.Intent{Verb: "/create", Category: "/mutation", Confidence: 1.0}
	result, err := executor.ProcessWithIntent(context.Background(), "create a retry helper", preset)
	if err == nil {
		t.Fatal("expected hollow success to fail ProcessWithIntent for /create with no tools")
	}
	if result == nil {
		t.Fatal("expected partial result even on hollow failure")
	}
	if result.SuccessfulWriteTools != 0 {
		t.Fatalf("expected 0 write tools, got %d", result.SuccessfulWriteTools)
	}
	if !strings.Contains(err.Error(), "hollow success blocked") {
		t.Fatalf("expected hollow success error, got: %v", err)
	}
}

func TestNextActionHelpers_WriteToolsTracked(t *testing.T) {
	// Ensure the write-tool classifier covers the modular tool names used by tools.Global().
	for _, name := range []string{"write_file", "edit_file", "delete_file"} {
		if !isWriteMutationTool(name) {
			t.Errorf("expected %s to be a write mutation tool", name)
		}
	}
}
