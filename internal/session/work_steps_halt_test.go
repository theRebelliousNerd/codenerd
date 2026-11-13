package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// failingStepProvider fails the first model call of one step's file the way a
// provider outage does, then plays the script.
type failingStepProvider struct {
	*stepScriptProvider
	failFile string
	failed   bool
}

func (p *failingStepProvider) CompleteWithToolResults(ctx context.Context, system string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if !p.failed && currentStepFile(history) == p.failFile {
		p.failed = true
		return nil, errors.New(`provider request failed with status 503`)
	}
	return p.stepScriptProvider.CompleteWithToolResults(ctx, system, history, defs)
}

// A step that ended in an execution error and wrote nothing after its retry
// stops the plan: the step after it, planned on that change existing, is not
// run and stays owed. Observed live 2026-11-12: step 1 of 2 failed on a
// provider 503 and made no edit; step 2 then wrote tests for code that did not
// exist, for 23 minutes.
func TestRunPlannedSteps_AnErroredStepThatWroteNothingHaltsThePlan(t *testing.T) {
	_, writeTool := registerStepTools(t)
	script := newStepScriptProvider("", map[string][]types.ToolCall{
		"b_test.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b_test.txt", "content": "tests for a change that was never made"}}},
	})
	client := &failingStepProvider{stepScriptProvider: script, failFile: "a.txt"}
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, _, err := e.runPlannedSteps(context.Background(), "system", "change a, then test it",
		[]workStep{{File: "a.txt", Change: "make the change"}, {File: "b_test.txt", Change: "test the change"}},
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)

	if !errors.Is(err, ErrStepsIncomplete) {
		t.Fatalf("runPlannedSteps: %v, want the policy's ErrStepsIncomplete; report:\n%s", err, result.StepReport)
	}
	if result.WriteToolEffects != 0 {
		t.Fatalf("writes = %d; the step after the errored one must not run", result.WriteToolEffects)
	}
	if _, statErr := os.Stat(filepath.Join(e.config.WorkspaceRoot, "b_test.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("b_test.txt exists (%v): step 2 ran after step 1 failed", statErr)
	}
	if script.played["b_test.txt"] != 0 {
		t.Fatalf("step 2's script was played %d time(s)", script.played["b_test.txt"])
	}
	turn := result.turnAtom()
	remaining, queryErr := e.kernel.Query("step_plan_remaining")
	if queryErr != nil {
		t.Fatal(queryErr)
	}
	owed := false
	for _, f := range remaining {
		if factTurn(f) == string(turn) && len(f.Args) == 3 && strings.Contains(types.ExtractString(f.Args[2]), "b_test.txt") {
			owed = true
		}
	}
	if !owed {
		t.Fatalf("step 2 is not recorded as owed: %+v", remaining)
	}
}

// A step that errors but then makes its edit on the retry does not stop the
// plan.
func TestRunPlannedSteps_AnErroredStepThatThenEditsGoesOn(t *testing.T) {
	_, writeTool := registerStepTools(t)
	script := newStepScriptProvider("", map[string][]types.ToolCall{
		"a.txt":      {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "the change"}}},
		"b_test.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b_test.txt", "content": "its test"}}},
	})
	client := &failingStepProvider{stepScriptProvider: script, failFile: "a.txt"}
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, _, _ = e.runPlannedSteps(context.Background(), "system", "change a, then test it",
		[]workStep{{File: "a.txt", Change: "make the change"}, {File: "b_test.txt", Change: "test the change"}},
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)

	for _, f := range []string{"a.txt", "b_test.txt"} {
		if _, statErr := os.Stat(filepath.Join(e.config.WorkspaceRoot, f)); statErr != nil {
			t.Fatalf("%s was not written (%v): a recovered step must not halt the plan; report:\n%s", f, statErr, result.StepReport)
		}
	}
}
