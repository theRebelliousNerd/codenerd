package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

func TestToolBudgetController_GrantsOnlyBoundedProgressExtensions(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cfg.MaxToolCalls = 20
	cfg.MaxToolIterations = 2
	cfg.ToolIterationExtensionSize = 3
	cfg.MaxToolIterationExtensions = 2
	controller := newToolBudgetController(cfg)

	controller.observe(
		[]types.ToolCall{{ID: "read-1", Name: "read_file", Input: map[string]any{"path": "a.go"}}},
		[]types.ToolResult{{ToolUseID: "read-1", Content: "package a"}},
	)
	first := controller.maybeExtend()
	if !first.Granted || first.AddedRounds != 3 || first.NewLimit != 5 {
		t.Fatalf("first extension = %#v, want +3 to 5", first)
	}

	controller.observe(
		[]types.ToolCall{{ID: "read-2", Name: "read_file", Input: map[string]any{"path": "b.go"}}},
		[]types.ToolResult{{ToolUseID: "read-2", Content: "package b"}},
	)
	second := controller.maybeExtend()
	if !second.Granted || second.NewLimit != 8 {
		t.Fatalf("second extension = %#v, want limit 8", second)
	}
	third := controller.maybeExtend()
	if third.Granted || third.NewLimit != 8 || !strings.Contains(third.Reason, "hard limit") {
		t.Fatalf("third extension = %#v, want bounded refusal", third)
	}
}

func TestToolBudgetController_RefusesRepeatedTraceCycle(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cfg.MaxToolIterations = 2
	cfg.ToolLoopRepeatThreshold = 2
	controller := newToolBudgetController(cfg)
	call := types.ToolCall{ID: "first", Name: "read_file", Input: map[string]any{"path": "same.go"}}
	result := types.ToolResult{ToolUseID: "first", Content: "unchanged"}
	controller.observe([]types.ToolCall{call}, []types.ToolResult{result})
	call.ID, result.ToolUseID = "second", "second"
	controller.observe([]types.ToolCall{call}, []types.ToolResult{result})

	decision := controller.maybeExtend()
	if decision.Granted || !decision.LoopDetected || !strings.Contains(decision.Reason, "repeated") {
		t.Fatalf("decision = %#v, want deterministic loop refusal", decision)
	}
}

func TestToolBudgetController_ReReadWithChangedEvidenceIsProgress(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cfg.MaxToolIterations = 1
	controller := newToolBudgetController(cfg)
	call := types.ToolCall{ID: "before", Name: "read_file", Input: map[string]any{"path": "same.go"}}
	controller.observe([]types.ToolCall{call}, []types.ToolResult{{ToolUseID: "before", Content: "before"}})
	call.ID = "after"
	controller.observe([]types.ToolCall{call}, []types.ToolResult{{ToolUseID: "after", Content: "after"}})

	decision := controller.maybeExtend()
	if !decision.Granted || decision.LoopDetected {
		t.Fatalf("changed evidence must count as progress, got %#v", decision)
	}
}

func TestToolBudgetController_RefusesErrorsWithoutProgress(t *testing.T) {
	controller := newToolBudgetController(DefaultExecutorConfig())
	controller.observe(
		[]types.ToolCall{{ID: "bad", Name: "read_file", Input: map[string]any{"path": "guessed/missing.go"}}},
		[]types.ToolResult{{ToolUseID: "bad", Content: "file not found", IsError: true}},
	)
	decision := controller.maybeExtend()
	if decision.Granted || !strings.Contains(decision.Reason, "no novel successful") {
		t.Fatalf("decision = %#v, want no-progress refusal", decision)
	}
}

func TestToolBudgetNudge_IsProgressiveAndAdvertisesBatchCodeDOM(t *testing.T) {
	cfg := DefaultExecutorConfig()
	cfg.MaxToolCalls = 20
	cfg.MaxToolIterations = 8
	controller := newToolBudgetController(cfg)

	early := controller.nudge(1, 2, true, true)
	if !containsAll(early, "18 tool calls", "7 rounds", "Batch independent") {
		t.Fatalf("early nudge = %q", early)
	}
	critical := controller.nudge(7, 17, true, true)
	if !containsAll(critical, "3 tool calls", "1 rounds", "multi-file CodeDOM", "do not explore") {
		t.Fatalf("critical nudge = %q", critical)
	}
	if len(critical) > 240 {
		t.Fatalf("critical nudge is not token-efficient: %d chars", len(critical))
	}
}

type adaptiveBudgetProvider struct {
	*MockLLMClient
	toolName string
	follows  int
	history  [][]types.Message
}

func (p *adaptiveBudgetProvider) CompleteWithToolResults(
	_ context.Context,
	_ string,
	history []types.Message,
	available []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	p.history = append(p.history, append([]types.Message(nil), history...))
	p.follows++
	if p.follows == 1 {
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{
			ID: "second", Name: p.toolName, Input: map[string]any{"path": "second.go"},
		}}}, nil
	}
	return &types.LLMToolResponse{Text: "finished after extension"}, nil
}

func TestRunToolLoop_ExtendsProgressingTurnAndFeedsRemainingBudget(t *testing.T) {
	const toolName = "adaptive_budget_probe"
	executions := 0
	tools.Global().Register(&tools.Tool{
		Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) {
			executions++
			return "novel result", nil
		},
	})
	base := &MockLLMClient{CompleteWithToolsFunc: func(
		context.Context, string, string, []types.ToolDefinition,
	) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{
			ID: "first", Name: toolName, Input: map[string]any{"path": "first.go"},
		}}}, nil
	}}
	client := &adaptiveBudgetProvider{MockLLMClient: base, toolName: toolName}
	executor := &Executor{
		kernel: &MockKernel{}, virtualStore: &MockVirtualStore{}, llmClient: client,
		config: DefaultExecutorConfig(),
	}
	executor.config.EnableSafetyGate = false
	executor.config.MaxToolIterations = 1
	executor.config.ToolIterationExtensionSize = 1
	executor.config.MaxToolIterationExtensions = 1
	executor.config.VerifyBuildAfterEdits = false
	executor.config.VerifyTestsAfterEdits = false
	executor.config.CriticReviewAfterEdits = false
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review"}}

	response, _, err := executor.runToolLoop(
		context.Background(), "system", "review",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}}, nil, result,
	)
	if err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	if response.Text != "finished after extension" || executions != 2 || result.ToolCallsExecuted != 2 {
		t.Fatalf("response=%q executions=%d accounted=%d", response.Text, executions, result.ToolCallsExecuted)
	}
	foundBudget := false
	for _, turn := range client.history {
		for _, message := range turn {
			for _, toolResult := range message.ToolResults {
				if strings.Contains(toolResult.Content, "[orchestrator] Orchestrator budget:") {
					foundBudget = true
				}
			}
		}
	}
	if !foundBudget {
		t.Fatal("tool-result history did not receive remaining-budget guidance")
	}
}
