package session

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
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
		"/optimize": true,
		"/commit":   false,
		"/format":   false,
		"/migrate":  false,
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

func TestWriteOrientedIntentFallbackMatchesPolicy(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	facts, err := kernel.Query("write_oriented_intent")
	if err != nil {
		t.Fatalf("query write_oriented_intent: %v", err)
	}
	fromPolicy := make(map[string]struct{}, len(facts))
	for _, fact := range facts {
		if len(fact.Args) != 1 {
			t.Fatalf("malformed write_oriented_intent fact: %#v", fact)
		}
		fromPolicy[types.ExtractString(fact.Args[0])] = struct{}{}
	}
	if len(fromPolicy) != len(writeOrientedIntentFallback) {
		t.Fatalf("policy verbs=%v fallback=%v", fromPolicy, writeOrientedIntentFallback)
	}
	for verb := range writeOrientedIntentFallback {
		if _, ok := fromPolicy[verb]; !ok {
			t.Errorf("fallback verb %s is absent from write_oriented_intent policy", verb)
		}
	}

	executor := &Executor{kernel: kernel}
	for verb := range fromPolicy {
		if !executor.writeOrientedIntent(verb) {
			t.Errorf("kernel-backed writeOrientedIntent(%s) = false", verb)
		}
	}
	if executor.writeOrientedIntent("/commit") {
		t.Error("/commit requires a command, not a file mutation")
	}
}

func TestCheckHollowSuccess_CommandOrientedIntentDoesNotRequireWriteTool(t *testing.T) {
	exec := NewExecutor(
		&requiresToolKernel{MockKernel: &MockKernel{}},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
	for _, verb := range []string{"/commit", "/format", "/migrate"} {
		t.Run(verb, func(t *testing.T) {
			result := &ExecutionResult{
				Intent:              perception.Intent{Verb: verb},
				ToolCallsExecuted:   1,
				SuccessfulToolCalls: 1,
			}
			if err := exec.checkHollowSuccess(result); err != nil {
				t.Fatalf("a command-backed %s must not require write_file/edit_file: %v", verb, err)
			}
		})
	}
}

type requiresToolKernel struct{ *MockKernel }

func (k *requiresToolKernel) Query(query string) ([]types.Fact, error) {
	if strings.HasPrefix(query, "intent_requires_tool_call(") {
		return []types.Fact{{Predicate: "intent_requires_tool_call"}}, nil
	}
	return k.MockKernel.Query(query)
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
		Intent:              perception.Intent{Verb: "/fix"},
		ToolCallsExecuted:   2,
		SuccessfulToolCalls: 2,
	}
	err := exec.checkHollowSuccess(result)
	if err == nil {
		t.Fatal("expected hollow success when write-oriented intent has no write tools")
	}
	if !strings.Contains(err.Error(), "write-mutation tool") {
		t.Fatalf("expected recognized mutation-tool explanation, got: %v", err)
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
		SuccessfulToolCalls:  1,
		SuccessfulWriteTools: 1,
	}
	if err := exec.checkHollowSuccess(result); err != nil {
		t.Fatalf("expected success when write tool landed: %v", err)
	}
}

func TestCheckHollowSuccess_FailedToolDoesNotSatisfyCommandIntent(t *testing.T) {
	exec := NewExecutor(
		&requiresToolKernel{MockKernel: &MockKernel{}},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
	result := &ExecutionResult{
		Intent:              perception.Intent{Verb: "/commit"},
		ToolCallsExecuted:   1,
		SuccessfulToolCalls: 0,
	}
	if err := exec.checkHollowSuccess(result); err == nil {
		t.Fatal("a failed command attempt must not satisfy /commit")
	}
}

func TestIsHollowSuccessErrorSurvivesWrapping(t *testing.T) {
	original := newHollowSuccessError("test")
	if !isHollowSuccessError(fmt.Errorf("outer: %w", original)) {
		t.Fatal("typed hollow-success identity was lost through wrapping")
	}
	if isHollowSuccessError(fmt.Errorf("%s lookalike", hollowSuccessPrefix)) {
		t.Fatal("plain text containing the prefix must not impersonate a hollow-success error")
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
