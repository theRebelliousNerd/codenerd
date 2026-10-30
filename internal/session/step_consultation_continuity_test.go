package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/prompt"
)

// stepCountingConsultant counts consultations and answers with a marker.
type stepCountingConsultant struct{ calls int }

func (c *stepCountingConsultant) SpawnConsultation(_ context.Context, agent, _ string) (string, error) {
	c.calls++
	return "STEP_ADVICE_MARKER from " + agent, nil
}

// A planned turn ran each step on a freshly compiled prompt, so the expert
// advice the turn paid for before its edits reached only the final
// verification. The turn's consultation now rides into every step's prompt,
// the step does not consult the same expert again, and an expert a step
// newly needs is consulted for it and owes its critique.
func TestPlannedStep_CarriesTheTurnsConsultation(t *testing.T) {
	k := spawnRosterKernel(t)
	jit := &MockJITCompiler{CompileFunc: func(_ context.Context, _ *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		return &prompt.CompilationResult{Prompt: "step prompt"}, nil
	}}
	e := NewExecutor(k, nil, nil, jit, nil, nil)
	consultant := &stepCountingConsultant{}
	e.SetExpertConsultant(consultant)
	target := "cmd/nerd/chat/view.go"
	sel := e.selectExperts("/fix", target)
	if len(sel.Agents()) == 0 {
		t.Fatal("fixture: the roster selects no expert for the view")
	}
	result := &ExecutionResult{}
	turnPrompt := e.withExpertConsultations(t.Context(), "turn prompt", perception.Intent{Verb: "/fix", Target: target}, "fix the resize", sel, result)
	if consultant.calls == 0 || !strings.Contains(turnPrompt, "STEP_ADVICE_MARKER") {
		t.Fatalf("fixture: the turn did not consult (asked %d): %q", consultant.calls, turnPrompt)
	}
	asked := consultant.calls

	stepCtx := prompt.NewCompilationContext()
	stepCtx.IntentVerb, stepCtx.IntentTarget = "/fix", target
	stepPrompt := e.stepSystemPrompt(t.Context(), turnPrompt, stepCtx, nil, "fix the resize", result)
	if !strings.Contains(stepPrompt, "step prompt") || !strings.Contains(stepPrompt, "STEP_ADVICE_MARKER") {
		t.Fatalf("the step's prompt does not carry the turn's consultation:\n%s", stepPrompt)
	}
	if consultant.calls != asked {
		t.Fatalf("the step consulted the same expert again (%d consultations, want %d)", consultant.calls, asked)
	}

	// A step whose expert the turn never consulted consults it.
	fresh := &ExecutionResult{}
	stepPrompt = e.stepSystemPrompt(t.Context(), "turn prompt", stepCtx, nil, "fix the resize", fresh)
	if consultant.calls != asked+1 || !strings.Contains(stepPrompt, "STEP_ADVICE_MARKER") {
		t.Fatalf("a step that newly needs an expert did not consult it (asked %d): %s", consultant.calls, stepPrompt)
	}
}
