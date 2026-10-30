package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/experts"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
)

// agentMarkingConsultant answers each agent with a marker naming it.
type agentMarkingConsultant struct{}

func (agentMarkingConsultant) SpawnConsultation(_ context.Context, agent, _ string) (string, error) {
	return "CARD_FROM_" + agent, nil
}

// A planned turn carried its whole consultation into every step, so a step
// on one file was handed advice about another, and nothing said which advice
// applied where. The consultations are now typed cards the kernel deals per
// step (turn_card_dealt): a lead's card to every step, an adviser's card to
// the steps on its domain's files, and the step names each card it was not
// dealt.
func TestPlannedStep_IsDealtTheCardsForItsFile(t *testing.T) {
	e := newObligationExec(t)
	e.jitCompiler = &MockJITCompiler{CompileFunc: func(_ context.Context, _ *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		return &prompt.CompilationResult{Prompt: "step prompt"}, nil
	}}
	e.SetExpertConsultant(agentMarkingConsultant{})
	result := &ExecutionResult{}
	sel := experts.Selection{Needs: []experts.Need{
		{Agent: "leadexpert", Spawn: experts.SpawnLead, Files: []string{"internal/a/a.go"}},
		{Agent: "viewexpert", Spawn: experts.SpawnAdvise, Files: []string{"internal/view/view.go"}},
	}}
	turnPrompt := e.withExpertConsultations(t.Context(), "turn prompt", perception.Intent{Verb: "/fix", Target: "internal/a/a.go"}, "fix it", sel, result)
	if !strings.Contains(turnPrompt, "CARD_FROM_leadexpert") || !strings.Contains(turnPrompt, "CARD_FROM_viewexpert") {
		t.Fatalf("fixture: the turn prompt lacks its consultations:\n%s", turnPrompt)
	}

	step := func(file string) string {
		stepCtx := prompt.NewCompilationContext()
		stepCtx.IntentVerb, stepCtx.IntentTarget = "/fix", file
		return e.stepSystemPrompt(t.Context(), turnPrompt, stepCtx, nil, "fix it", result)
	}

	other := step("internal/a/a.go")
	if !strings.Contains(other, "CARD_FROM_leadexpert") {
		t.Fatalf("the lead's card was not dealt to every step:\n%s", other)
	}
	if strings.Contains(other, "CARD_FROM_viewexpert") {
		t.Fatalf("the view adviser's card was dealt to a step on another file:\n%s", other)
	}
	if !strings.Contains(other, "viewexpert advises internal/view/view.go, not this step's file") {
		t.Fatalf("the step does not name the card it was not dealt:\n%s", other)
	}

	view := step("internal/view/view.go")
	if !strings.Contains(view, "CARD_FROM_viewexpert") || !strings.Contains(view, "CARD_FROM_leadexpert") {
		t.Fatalf("the step on the adviser's file was not dealt both cards:\n%s", view)
	}
}
