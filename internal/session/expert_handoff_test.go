package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/experts"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

func TestParseHandoffs(t *testing.T) {
	got := parseHandoffs("/ws", "Plan: do it.\nHANDOFF internal/a.go -> /ws/internal/b.go: b must accept the new field\nHANDOFF a.go -> a.go: same file\nHANDOFF a.go b.go: no arrow\nHANDOFF a.go -> c.go:")
	if len(got) != 1 || got[0] != (cardHandoff{From: "internal/a.go", To: "internal/b.go", Contract: "b must accept the new field"}) {
		t.Fatalf("parseHandoffs = %+v", got)
	}
}

// A lead's advice that a change in one file obliges another reached no step
// as such, and a consumer the plan never named was never edited. A hand-off
// is now an edge the kernel checks against the plan: an existing consumer the
// plan misses gains a step, one that is not on disk is reported unresolved.
func TestPlannedSteps_AHandoffToAnUnplannedConsumerGainsAStep(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: add the field\nSTEP b.txt :: read the field\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "a"}}},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "b"}}},
		"c.txt": {{ID: "w-c", Name: writeTool, Input: map[string]any{"path": "c.txt", "content": "c"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, "c.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	e.recordCards(result, []expertCard{{
		Agent: "leadexpert", Mode: experts.SpawnLead, Disposition: cardAnswered, Body: "plan",
		Handoffs: []cardHandoff{
			{From: "a.txt", To: "c.txt", Contract: "c must decode the new field"},
			{From: "a.txt", To: "gone.txt", Contract: "gone must learn the field"},
		},
	}})

	_, toolErrs, err := e.runToolLoop(context.Background(), "system", "change a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q); report:\n%s", err, toolErrs, result.StepReport)
	}
	if !strings.Contains(result.StepReport, "[3] c.txt :: carry the hand-off from a.txt (leadexpert): c must decode the new field — edited") {
		t.Fatalf("the unplanned consumer gained no step:\n%s", result.StepReport)
	}
	if !strings.Contains(result.StepReport, "Unresolved hand-off: leadexpert") || !strings.Contains(result.StepReport, "gone.txt") {
		t.Fatalf("the hand-off to a file not on disk was not reported:\n%s", result.StepReport)
	}
}

// The hand-off goes to the step on either of its ends.
func TestPlannedStep_IsDealtTheHandoffsItIsAnEndOf(t *testing.T) {
	e := newObligationExec(t)
	e.jitCompiler = &MockJITCompiler{CompileFunc: func(_ context.Context, _ *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		return &prompt.CompilationResult{Prompt: "step prompt"}, nil
	}}
	result := &ExecutionResult{}
	e.recordCards(result, []expertCard{{
		Agent: "schemaexpert", Mode: experts.SpawnAdvise, Disposition: cardAnswered, Body: "advice",
		Files:    []string{"internal/a/a.go"},
		Handoffs: []cardHandoff{{From: "internal/a/a.go", To: "internal/b/b.go", Contract: "b keeps reading the old column"}},
	}})
	step := func(file string) string {
		stepCtx := prompt.NewCompilationContext()
		stepCtx.IntentVerb, stepCtx.IntentTarget = "/fix", file
		return e.stepSystemPrompt(t.Context(), "turn prompt", stepCtx, nil, "fix it", result)
	}
	const line = "internal/a/a.go -> internal/b/b.go (schemaexpert): b keeps reading the old column"
	if got := step("internal/b/b.go"); !strings.Contains(got, line) {
		t.Fatalf("the consumer's step was not dealt the hand-off:\n%s", got)
	}
	if got := step("internal/a/a.go"); !strings.Contains(got, line) {
		t.Fatalf("the producer's step was not dealt the hand-off:\n%s", got)
	}
	if got := step("internal/z/z.go"); strings.Contains(got, line) {
		t.Fatalf("a step on neither end was dealt the hand-off:\n%s", got)
	}
}
