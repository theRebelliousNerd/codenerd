package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// When the tool loop runs out of iterations it makes one more call with the
// exploration tools removed. Which tools survive that cut decides whether a
// large task is truncated or made impossible.
//
// Stripping every tool is right for a query verb and wrong for a write verb.
// Live: `nerd create <architecture doc>` spent 35 tool calls researching, hit
// the ceiling, and the tool-free final call could only describe the file it had
// been asked to write. The hollow-success guard then correctly failed the turn,
// so the work was lost rather than merely incomplete.
func TestWriteOnlyToolDefinitions_KeepsWritesDropsExploration(t *testing.T) {
	defs := []types.ToolDefinition{
		{Name: "read_file"}, {Name: "write_file"}, {Name: "grep"},
		{Name: "edit_file"}, {Name: "glob"}, {Name: "list_files"},
		{Name: "delete_file"}, {Name: "search_code"}, {Name: "run_tests"},
	}

	got := writeOnlyToolDefinitions(defs)

	kept := map[string]bool{}
	for _, d := range got {
		kept[d.Name] = true
	}

	for _, want := range []string{"write_file", "edit_file", "delete_file"} {
		if !kept[want] {
			t.Errorf("%s was dropped; a write verb could then never land its deliverable", want)
		}
	}
	// The budget was exhausted precisely because reading is the cheap, endless
	// option. Handing read_file back guarantees the model spends the last call
	// on more exploration.
	for _, unwanted := range []string{"read_file", "grep", "glob", "list_files", "search_code", "run_tests"} {
		if kept[unwanted] {
			t.Errorf("%s survived the cut; the model will use it and the final call is wasted", unwanted)
		}
	}
}

func TestWriteOnlyToolDefinitions_EmptyInputIsEmptyOutput(t *testing.T) {
	if got := writeOnlyToolDefinitions(nil); len(got) != 0 {
		t.Errorf("writeOnlyToolDefinitions(nil) = %v, want empty", got)
	}
	readOnly := []types.ToolDefinition{{Name: "read_file"}, {Name: "grep"}}
	if got := writeOnlyToolDefinitions(readOnly); len(got) != 0 {
		t.Errorf("a read-only tool set must yield no write tools, got %v", got)
	}
}

// The two nudges must differ in the one way that matters: the write nudge has to
// forbid the specific failure it exists to prevent, which is answering with a
// description of the artifact instead of writing it.
func TestBudgetExhaustedNudges_TellTheModelDifferentThings(t *testing.T) {
	if readOnlyBudgetExhaustedNudge == writeBudgetExhaustedNudge {
		t.Fatal("a write turn and a query turn need different final instructions")
	}
	if !containsAll(writeBudgetExhaustedNudge, "write tool", "NOW") {
		t.Error("the write nudge must demand the artifact now, not a plan to produce it")
	}
	if !containsAll(readOnlyBudgetExhaustedNudge, "could not verify") {
		t.Error("the read-only nudge must ask for an honest account of gaps rather than more tools")
	}
}

func TestExecuteToolBatchPiggyback_ReportsEverySkippedCall(t *testing.T) {
	executor := &Executor{config: ExecutorConfig{MaxToolCalls: 1}}
	result := &ExecutionResult{ToolCallsExecuted: 1}
	calls := []types.ToolCall{{Name: "first"}, {Name: "second"}}

	errs := executor.executeToolBatchPiggyback(context.Background(), calls, nil, result)
	if len(errs) != len(calls) {
		t.Fatalf("errors = %v, want one budget error for each skipped call", errs)
	}
	for i, name := range []string{"first", "second"} {
		if !strings.Contains(errs[i], name) || !strings.Contains(errs[i], "budget exceeded") {
			t.Errorf("error %d = %q, want %s budget exceeded", i, errs[i], name)
		}
	}
}

func TestExecuteToolBatch_ZeroConfigUsesSafeDefaults(t *testing.T) {
	const toolName = "zero_config_probe"
	tools.Global().Register(&tools.Tool{
		Name:     toolName,
		Category: tools.CategoryGeneral,
		Execute: func(ctx context.Context, _ map[string]any) (string, error) {
			if err := ctx.Err(); err != nil {
				t.Fatalf("zero ToolTimeout produced an already-expired context: %v", err)
			}
			return "ran", nil
		},
	})

	executor := &Executor{config: ExecutorConfig{}, virtualStore: &MockVirtualStore{}}
	result := &ExecutionResult{}
	results, errs := executor.executeToolBatch(
		context.Background(),
		[]types.ToolCall{{ID: "zero-1", Name: toolName}},
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		result,
	)
	if len(errs) != 0 || len(results) != 1 || results[0].IsError {
		t.Fatalf("zero config should use defaults, results=%v errors=%v", results, errs)
	}
	if result.ToolCallsExecuted != 1 {
		t.Fatalf("executed = %d, want 1", result.ToolCallsExecuted)
	}
}

func TestExecuteToolBatch_CancelledContextPairsEveryCallWithoutExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := &Executor{config: DefaultExecutorConfig()}
	result := &ExecutionResult{}
	calls := []types.ToolCall{{ID: "cancel-1", Name: "one"}, {ID: "cancel-2", Name: "two"}}

	results, errs := executor.executeToolBatch(ctx, calls, nil, result)
	if len(results) != len(calls) || len(errs) != len(calls) {
		t.Fatalf("results=%v errors=%v, want one paired cancellation per call", results, errs)
	}
	if result.ToolCallsExecuted != 0 {
		t.Fatalf("executed=%d, want 0", result.ToolCallsExecuted)
	}
	for i, result := range results {
		if result.ToolUseID != calls[i].ID || !result.IsError {
			t.Errorf("result %d = %#v, want paired cancellation for %s", i, result, calls[i].ID)
		}
	}
}

type finalToolCallProvider struct {
	toolName string
}

type piggybackStaticClient struct{ *MockLLMClient }

func (*piggybackStaticClient) ShouldUsePiggybackTools() bool { return true }

type forcedFinalVerificationClient struct {
	*MockLLMClient
	toolName string
}

func (c *forcedFinalVerificationClient) CompleteWithToolResults(
	_ context.Context, _ string, _ []types.Message, available []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	if len(available) > 0 {
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{
			ID: "pending-write", Name: c.toolName, Input: map[string]any{"path": "broken.go"},
		}}}, nil
	}
	return &types.LLMToolResponse{Text: "forced final"}, nil
}

func TestRunToolLoop_PiggybackRunsPostEditBuildGate(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.com/piggyback\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "broken.go"), []byte("package broken\nfunc nope(\n"), 0o600); err != nil {
		t.Fatalf("write broken.go: %v", err)
	}

	const toolName = "multi_edit"
	tools.Global().Register(&tools.Tool{
		Name: toolName, Category: tools.CategoryCode,
		Execute: func(context.Context, map[string]any) (string, error) { return "written", nil },
	})

	client := &piggybackStaticClient{MockLLMClient: &MockLLMClient{
		CompleteWithSystemFunc: func(context.Context, string, string) (string, error) {
			return `{"control_packet":{"tool_requests":[{"id":"piggy-write","tool_name":"multi_edit","tool_args":{"path":"broken.go"},"required":true}]},"surface_response":"done"}`, nil
		},
	}}
	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		llmClient:    client,
		config:       DefaultExecutorConfig(),
	}
	executor.config.EnableSafetyGate = false
	executor.config.VerifyTestsAfterEdits = false
	executor.config.CriticReviewAfterEdits = false
	executor.config.WorkspaceRoot = workspace

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/create"}}
	_, _, err := executor.runToolLoop(
		context.Background(), "system", "create it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}}, nil, result,
	)
	if err == nil || !strings.Contains(err.Error(), "edits broke the build") {
		t.Fatalf("Piggyback write bypassed the build gate: %v", err)
	}
	if result.SuccessfulWriteTools != 1 {
		t.Fatalf("successful writes=%d, want 1", result.SuccessfulWriteTools)
	}
}

func TestRunToolLoop_ForcedFinalRunsPostEditBuildGate(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.com/forcedfinal\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "broken.go"), []byte("package broken\nfunc nope(\n"), 0o600); err != nil {
		t.Fatalf("write broken.go: %v", err)
	}

	const toolName = "multi_edit"
	tools.Global().Register(&tools.Tool{
		Name: toolName, Category: tools.CategoryCode,
		Execute: func(context.Context, map[string]any) (string, error) { return "written", nil },
	})
	base := &MockLLMClient{CompleteWithToolsFunc: func(
		context.Context, string, string, []types.ToolDefinition,
	) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{
			ID: "initial-write", Name: toolName, Input: map[string]any{"path": "broken.go"},
		}}}, nil
	}}
	client := &forcedFinalVerificationClient{MockLLMClient: base, toolName: toolName}
	executor := &Executor{
		kernel: &MockKernel{}, virtualStore: &MockVirtualStore{}, llmClient: client,
		config: DefaultExecutorConfig(),
	}
	executor.config.EnableSafetyGate = false
	executor.config.MaxToolIterations = 1
	executor.config.VerifyTestsAfterEdits = false
	executor.config.CriticReviewAfterEdits = false
	executor.config.WorkspaceRoot = workspace

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/create"}}
	_, _, err := executor.runToolLoop(
		context.Background(), "system", "create it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}}, nil, result,
	)
	if err == nil || !strings.Contains(err.Error(), "edits broke the build") {
		t.Fatalf("forced final bypassed the build gate: %v", err)
	}
}

func (p finalToolCallProvider) CompleteWithToolResults(
	context.Context, string, []types.Message, []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{
		Text: "finalized",
		ToolCalls: []types.ToolCall{{
			ID: p.toolName, Name: p.toolName,
			Input: map[string]any{"path": "finalized.txt"},
		}},
	}, nil
}

func TestForceFinalAnswer_RefusesUnofferedToolCalls(t *testing.T) {
	const toolName = "forced_final_probe"
	executions := 0
	tools.Global().Register(&tools.Tool{
		Name:     toolName,
		Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) {
			executions++
			return "done", nil
		},
	})

	executor := &Executor{
		config:       DefaultExecutorConfig(),
		virtualStore: &MockVirtualStore{},
	}
	executor.config.EnableSafetyGate = false
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review"}}
	final, errs, err := executor.forceFinalAnswer(
		context.Background(),
		finalToolCallProvider{toolName: toolName},
		"system",
		nil,
		&types.LLMToolResponse{},
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		result,
	)
	if err != nil {
		t.Fatalf("forceFinalAnswer error=%v", err)
	}
	if executions != 0 {
		t.Fatalf("unoffered tool executions = %d, want 0", executions)
	}
	if len(errs) != 1 || !strings.Contains(errs[0], "not offered") {
		t.Fatalf("toolErrors=%v, want an explicit unoffered-tool refusal", errs)
	}
	if len(final.ToolCalls) != 0 {
		t.Fatalf("returned response still advertises refused calls: %v", final.ToolCalls)
	}
}

func TestForceFinalAnswer_ExecutesOfferedWriteThenClearsIt(t *testing.T) {
	const toolName = "create_file"
	executions := 0
	tools.Global().Register(&tools.Tool{
		Name:     toolName,
		Category: tools.CategoryCode,
		Execute: func(context.Context, map[string]any) (string, error) {
			executions++
			return "done", nil
		},
	})

	executor := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig(), virtualStore: &MockVirtualStore{}}
	executor.config.EnableSafetyGate = false
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/create"}}
	final, errs, err := executor.forceFinalAnswer(
		context.Background(),
		finalToolCallProvider{toolName: toolName},
		"system",
		nil,
		&types.LLMToolResponse{},
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		result,
	)
	if err != nil || len(errs) != 0 {
		t.Fatalf("forceFinalAnswer error=%v toolErrors=%v", err, errs)
	}
	if executions != 1 || result.SuccessfulWriteTools != 1 {
		t.Fatalf("executions=%d successfulWrites=%d, want 1/1", executions, result.SuccessfulWriteTools)
	}
	if len(final.ToolCalls) != 0 {
		t.Fatalf("returned response still advertises executed calls: %v", final.ToolCalls)
	}
}

func TestTruncateToolResult_PreservesUTF8(t *testing.T) {
	prefix := strings.Repeat("a", 16*1024-1)
	got := truncateToolResult(prefix + "€tail")
	if !strings.Contains(got, "...[truncated]") {
		t.Fatal("expected truncation marker")
	}
	if !utf8.ValidString(got) {
		t.Fatal("truncated tool result is not valid UTF-8")
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
