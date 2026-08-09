package session

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// piggyEnvelope builds a minimal Piggyback envelope JSON string containing the
// supplied tool_requests. Every request must carry an id.
func piggyEnvelope(surface string, reqs []map[string]any) string {
	env := map[string]any{
		"control_packet": map[string]any{
			"tool_requests": reqs,
		},
		"surface_response": surface,
	}
	b, _ := json.Marshal(env)
	return string(b)
}

func piggySingleRequest(surface, id, toolName string, toolArgs map[string]any) string {
	req := map[string]any{
		"id":        id,
		"tool_name": toolName,
		"tool_args": toolArgs,
		"purpose":   "test probe",
		"required":  true,
	}
	return piggyEnvelope(surface, []map[string]any{req})
}

// scriptedProvider is a small LLM double embedding *MockLLMClient semantics
// but allowing explicit CompleteWithTools and CompleteWithToolResults handlers.
type scriptedProvider struct {
	*MockLLMClient
	completeWithTools       func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error)
	completeWithToolResults func(context.Context, string, []types.Message, []types.ToolDefinition) (*types.LLMToolResponse, error)
}

func (s *scriptedProvider) CompleteWithTools(ctx context.Context, sys, user string, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if s.completeWithTools != nil {
		return s.completeWithTools(ctx, sys, user, defs)
	}
	if s.MockLLMClient != nil {
		return s.MockLLMClient.CompleteWithTools(ctx, sys, user, defs)
	}
	return &types.LLMToolResponse{Text: "default"}, nil
}

func (s *scriptedProvider) CompleteWithToolResults(ctx context.Context, sys string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if s.completeWithToolResults != nil {
		return s.completeWithToolResults(ctx, sys, history, defs)
	}
	if s.MockLLMClient != nil && s.MockLLMClient.CompleteWithToolsFunc != nil {
		// fallback not used
	}
	return &types.LLMToolResponse{Text: "default"}, nil
}

func ensureGeneralProbe(t *testing.T, name string, execFn tools.ExecuteFunc) {
	t.Helper()
	if tools.Global().Has(name) {
		return
	}
	if err := tools.Global().Register(&tools.Tool{
		Name:     name,
		Category: tools.CategoryGeneral,
		Execute:  execFn,
	}); err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
}

func TestRunToolLoop_PiggybackInitialEnvelopeIsPromoted(t *testing.T) {
	const probe = "piggyback_initial_probe"
	executions := 0
	ensureGeneralProbe(t, probe, func(context.Context, map[string]any) (string, error) {
		executions++
		return "probe ok", nil
	})

	envelope := piggySingleRequest("surface after promotion", "req-initial-1", probe, map[string]any{})
	followUpCalls := 0
	provider := &scriptedProvider{
		completeWithTools: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				Text:      envelope,
				ToolCalls: nil,
			}, nil
		},
		completeWithToolResults: func(_ context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			followUpCalls++
			return &types.LLMToolResponse{Text: "terminal done"}, nil
		},
	}

	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		llmClient:    provider,
		config:       DefaultExecutorConfig(),
	}
	executor.config.EnableSafetyGate = false
	executor.config.VerifyBuildAfterEdits = false
	executor.config.VerifyTestsAfterEdits = false
	executor.config.CriticReviewAfterEdits = false
	executor.config.WorkspaceRoot = t.TempDir()

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review"}}
	resp, toolErrs, err := executor.runToolLoop(
		context.Background(), "system", "do probe",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{probe}}, nil, result,
	)
	if err != nil {
		t.Fatalf("runToolLoop error=%v toolErrs=%v", err, toolErrs)
	}
	if executions != 1 {
		t.Fatalf("executions=%d, want 1", executions)
	}
	if followUpCalls != 1 {
		t.Fatalf("CompleteWithToolResults calls=%d, want 1", followUpCalls)
	}
	if result.ToolCallsExecuted != 1 {
		t.Fatalf("ToolCallsExecuted=%d, want 1", result.ToolCallsExecuted)
	}
	if resp == nil || strings.TrimSpace(resp.Text) != "terminal done" {
		t.Fatalf("final response=%#v, want terminal done", resp)
	}
	if len(resp.ToolCalls) != 0 {
		t.Fatalf("final ToolCalls=%v, want empty", resp.ToolCalls)
	}
}

func TestRunToolLoop_PiggybackSecondTurnPromoted(t *testing.T) {
	const firstProbe = "piggyback_first_probe"
	const secondProbe = "piggyback_second_probe"
	firstExecs := 0
	secondExecs := 0
	ensureGeneralProbe(t, firstProbe, func(context.Context, map[string]any) (string, error) {
		firstExecs++
		return "first ok", nil
	})
	ensureGeneralProbe(t, secondProbe, func(context.Context, map[string]any) (string, error) {
		secondExecs++
		return "second ok", nil
	})

	secondEnvelope := piggySingleRequest("surface second", "req-second-1", secondProbe, map[string]any{})
	callIdx := 0
	provider := &scriptedProvider{
		completeWithTools: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				ToolCalls: []types.ToolCall{{ID: "native-1", Name: firstProbe, Input: map[string]any{}}},
				Text:      "initial native",
			}, nil
		},
		completeWithToolResults: func(_ context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			callIdx++
			if callIdx == 1 {
				return &types.LLMToolResponse{
					Text:      secondEnvelope,
					ToolCalls: nil,
				}, nil
			}
			return &types.LLMToolResponse{Text: "terminal second done"}, nil
		},
	}

	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		llmClient:    provider,
		config:       DefaultExecutorConfig(),
	}
	executor.config.EnableSafetyGate = false
	executor.config.VerifyBuildAfterEdits = false
	executor.config.VerifyTestsAfterEdits = false
	executor.config.CriticReviewAfterEdits = false
	executor.config.WorkspaceRoot = t.TempDir()

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review"}}
	resp, toolErrs, err := executor.runToolLoop(
		context.Background(), "system", "do two probes",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{firstProbe, secondProbe}}, nil, result,
	)
	if err != nil {
		t.Fatalf("runToolLoop error=%v toolErrs=%v", err, toolErrs)
	}
	if firstExecs != 1 {
		t.Fatalf("first executions=%d, want 1", firstExecs)
	}
	if secondExecs != 1 {
		t.Fatalf("second executions=%d, want 1", secondExecs)
	}
	if callIdx != 2 {
		t.Fatalf("CompleteWithToolResults calls=%d, want 2", callIdx)
	}
	if result.ToolCallsExecuted != 2 {
		t.Fatalf("ToolCallsExecuted=%d, want 2", result.ToolCallsExecuted)
	}
	if resp == nil || strings.TrimSpace(resp.Text) != "terminal second done" {
		t.Fatalf("final response=%#v, want terminal second done", resp)
	}
}

func TestPromotePiggybackToolRequests_NativePrecedence(t *testing.T) {
	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		config:       DefaultExecutorConfig(),
	}
	envelope := piggySingleRequest("surface should not be used", "req-precedence-1", "piggyback_initial_probe", map[string]any{"path": "x"})
	originalText := envelope
	originalCall := types.ToolCall{ID: "native-keep-1", Name: "read_file", Input: map[string]any{"path": "keep.go"}}
	resp := &types.LLMToolResponse{
		Text:      originalText,
		ToolCalls: []types.ToolCall{originalCall},
	}
	ok := executor.promotePiggybackToolRequests(resp)
	if ok {
		t.Fatalf("promotePiggybackToolRequests returned true, want false when native ToolCalls present")
	}
	if resp.Text != originalText {
		t.Fatalf("Text changed: got %q want %q", resp.Text, originalText)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len=%d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != originalCall.ID || resp.ToolCalls[0].Name != originalCall.Name {
		t.Fatalf("ToolCalls mutated: got %#v want %#v", resp.ToolCalls[0], originalCall)
	}
}

func TestForceFinalAnswer_PiggybackOfferedWriteExecutes(t *testing.T) {
	const toolName = "create_file"
	execCount := 0
	// Ensure create_file is registered; mutate existing if needed to capture execCount.
	var origExec tools.ExecuteFunc
	if existing := tools.Global().Get(toolName); existing != nil {
		origExec = existing.Execute
		existing.Execute = func(ctx context.Context, args map[string]any) (string, error) {
			execCount++
			return "written", nil
		}
		defer func() { existing.Execute = origExec }()
	} else {
		if err := tools.Global().Register(&tools.Tool{
			Name:     toolName,
			Category: tools.CategoryCode,
			Execute: func(context.Context, map[string]any) (string, error) {
				execCount++
				return "written", nil
			},
		}); err != nil {
			t.Fatalf("register create_file: %v", err)
		}
	}

	envelope := piggySingleRequest("final surface", "req-create-1", toolName, map[string]any{"path": "out.txt", "content": "hi"})

	provider := &scriptedProvider{
		completeWithToolResults: func(_ context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				Text:      envelope,
				ToolCalls: nil,
			}, nil
		},
	}

	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		config:       DefaultExecutorConfig(),
	}
	executor.config.EnableSafetyGate = false
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/create"}}
	history := []types.Message{}
	pending := &types.LLMToolResponse{}
	final, toolErrs, err := executor.forceFinalAnswer(
		context.Background(),
		provider,
		"system",
		&history,
		pending,
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		result,
	)
	if err != nil {
		t.Fatalf("forceFinalAnswer error=%v toolErrs=%v", err, toolErrs)
	}
	if len(toolErrs) != 0 {
		t.Fatalf("toolErrs=%v, want empty", toolErrs)
	}
	if execCount != 1 {
		t.Fatalf("execCount=%d, want 1", execCount)
	}
	if result.SuccessfulWriteTools != 1 {
		t.Fatalf("SuccessfulWriteTools=%d, want 1", result.SuccessfulWriteTools)
	}
	if len(final.ToolCalls) != 0 {
		t.Fatalf("final.ToolCalls=%v, want cleared", final.ToolCalls)
	}
	assertToolCallPaired(t, history, "req-create-1", false)
}

func TestForceFinalAnswer_PiggybackUnofferedIsRefused(t *testing.T) {
	const probe = "piggyback_unoffered_probe"
	execCount := 0
	ensureGeneralProbe(t, probe, func(context.Context, map[string]any) (string, error) {
		execCount++
		return "should not run", nil
	})

	envelope := piggySingleRequest("final surface unoffered", "req-unoffered-1", probe, map[string]any{})

	provider := &scriptedProvider{
		completeWithToolResults: func(_ context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{
				Text:      envelope,
				ToolCalls: nil,
			}, nil
		},
	}

	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		config:       DefaultExecutorConfig(),
	}
	executor.config.EnableSafetyGate = false
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review"}}
	history := []types.Message{}
	pending := &types.LLMToolResponse{}
	final, toolErrs, err := executor.forceFinalAnswer(
		context.Background(),
		provider,
		"system",
		&history,
		pending,
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{probe}},
		result,
	)
	if err != nil {
		t.Fatalf("forceFinalAnswer error=%v", err)
	}
	if execCount != 0 {
		t.Fatalf("execCount=%d, want 0", execCount)
	}
	if len(toolErrs) != 1 || !strings.Contains(toolErrs[0], "not offered") {
		t.Fatalf("toolErrs=%v, want one not-offered error", toolErrs)
	}
	if len(final.ToolCalls) != 0 {
		t.Fatalf("final.ToolCalls=%v, want cleared", final.ToolCalls)
	}
	assertToolCallPaired(t, history, "req-unoffered-1", true)
	// Verify synthetic result explicitly mentions not offered.
	found := false
	for _, m := range history {
		for _, r := range m.ToolResults {
			if r.ToolUseID == "req-unoffered-1" && r.IsError && strings.Contains(r.Content, "not offered") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("history missing synthetic IsError result for %q: %#v", "req-unoffered-1", history)
	}
}
