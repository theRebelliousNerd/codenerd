package session

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

func TestRepairFailureDigest_DurationsDoNotMakeAFailureNew(t *testing.T) {
	a := "--- FAIL: TestProbe (0.00s)\n    main_test.go:6: broken\nFAIL\nFAIL\trepairprobe\t0.412s\n"
	b := "--- FAIL: TestProbe (0.03s)\n    main_test.go:6: broken\nFAIL\nFAIL\trepairprobe\t1.9s\n"
	other := "--- FAIL: TestProbe (0.00s)\n    main_test.go:6: still broken\nFAIL\nFAIL\trepairprobe\t0.412s\n"
	if repairFailureDigest(a) != repairFailureDigest(b) {
		t.Fatal("the same failure with other durations digests differently")
	}
	if repairFailureDigest(a) == repairFailureDigest(other) {
		t.Fatal("a different failure digests the same")
	}
}

// An edit that leaves exactly the failure an earlier edit left has not moved
// the episode: the policy gives up there, not at the count (sweep finding
// F10, repair_not_converging). With a cap of five, a model that writes the
// same broken file every attempt used to spend all five.
func TestRepairLoop_TheSameFailureAfterTwoEditsGivesUpBeforeTheCap(t *testing.T) {
	h := newRepairHarness(t, func(e *Executor) { e.config.RepairMaxAttempts = 5 })
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), repairTestBroken),
		}}
	}
	h.onRepair = func(call int, history []types.Message) *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "attempt", ToolCalls: []types.ToolCall{
			h.writeCall(fmt.Sprintf("r%d", call), "write_file", filepath.Join(h.ws, "main_test.go"), repairTestBroken),
		}}
	}
	_, err := h.drive(t, "fix make the failing test pass")
	if err == nil {
		t.Fatal("expected the episode to give up")
	}
	if !strings.Contains(err.Error(), "after 2 attempts") || !strings.Contains(err.Error(), "the same failure survived two edits") {
		t.Fatalf("want a give-up after 2 attempts for a repeated failure, got: %v", err)
	}
}

// An attempt's rounds are the working policy's to end, not a count: an
// attempt that checks seven things without writing and then makes the fix
// converges in that one attempt. A six-call ceiling used to cut it and
// start another attempt.
func TestRepairLoop_AnAttemptIsNotCutAtACallCount(t *testing.T) {
	const probe = "repair_probe_check"
	allowed := []string{"write_file", "read_file", "edit_file", probe}
	h := newRepairHarness(t, func(e *Executor) {
		e.configFactory = &MockConfigFactory{
			GenerateFunc: func(context.Context, *prompt.CompilationResult, ...string) (*jitconfig.EffectiveAgentRuntimeConfig, error) {
				return &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: allowed}, nil
			},
		}
		e.EffectiveAgentRuntimeConfig = &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: allowed}
	})
	// An execute-effect tool: the commit regime closes reading, not this.
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectExecute, Name: probe, Category: tools.CategoryGeneral,
		Schema: tools.ToolSchema{Properties: map[string]tools.Property{"n": {Type: "integer"}}},
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			return fmt.Sprintf("check %v: inconclusive", args["n"]), nil
		},
	})
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), repairTestBroken),
		}}
	}
	repairs := 0
	h.onRepair = func(call int, history []types.Message) *types.LLMToolResponse {
		repairs++
		if repairs <= 7 {
			return &types.LLMToolResponse{Text: "checking", ToolCalls: []types.ToolCall{
				{ID: fmt.Sprintf("p%d", repairs), Name: probe, Input: map[string]any{"n": repairs}},
			}}
		}
		return &types.LLMToolResponse{Text: "fixed", ToolCalls: []types.ToolCall{
			h.writeCall(fmt.Sprintf("w%d", repairs), "write_file", filepath.Join(h.ws, "main_test.go"), repairTestFixed),
		}}
	}
	result, err := h.drive(t, "fix make the failing test pass")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	rec := result.TestCheck.Repair
	if rec == nil || !rec.Passed {
		t.Fatalf("repair record = %+v, want passed", rec)
	}
	if rec.Cost.Attempts != 1 || rec.Attempts[0].LLMCalls != 8 {
		t.Fatalf("attempts=%d llm_calls=%d, want one attempt of 8 calls (7 checks, then the fix)", rec.Cost.Attempts, rec.Attempts[0].LLMCalls)
	}
}
