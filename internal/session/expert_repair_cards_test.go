package session

import (
	"strings"
	"testing"

	"codenerd/internal/experts"
)

// The repairs after a planned task ran on the turn's own prompt, so a
// consultation a step bought for the file it edited was absent when a repair
// edited the same file; and dealing by the turn's whole write set handed a
// repair of one package the advice about another. Each repair now carries the
// step-born cards policy deals it from its own subject: a lead's always, an
// adviser's only when the repair's failure names its file -- never the cards
// the turn's prompt already holds, nor an unanswered one.
func TestRepair_IsDealtTheStepConsultationsItsFailureNames(t *testing.T) {
	e := newObligationExec(t)
	result := &ExecutionResult{WrittenPaths: []string{"internal/b/b.go", "internal/c/c.go"}}
	e.recordCards(result, []expertCard{{Agent: "turnlead", Mode: experts.SpawnLead, Disposition: cardAnswered, Body: "TURN_PROMPT_CARD"}})
	result.promptCards = len(result.expertCards)
	e.recordCards(result, []expertCard{
		{Agent: "steplead", Mode: experts.SpawnLead, Disposition: cardAnswered, Body: "STEP_LEAD_CARD"},
		{Agent: "badviser", Mode: experts.SpawnAdvise, Disposition: cardAnswered, Body: "STEP_B_CARD", Files: []string{"internal/b/b.go"}},
		{Agent: "cadviser", Mode: experts.SpawnAdvise, Disposition: cardAnswered, Body: "STEP_C_CARD", Files: []string{"internal/c/c.go"}},
		{Agent: "failedlead", Mode: experts.SpawnLead, Disposition: cardFailed, Body: "STEP_FAILED_CARD"},
	})
	has := func(got string, want ...string) bool {
		for _, w := range want {
			if !strings.Contains(got, w) {
				return false
			}
		}
		return true
	}

	build := e.repairCards(result, "# example/internal/b\ninternal/b/b.go:12:3: undefined: decodeRow")
	if !has(build, "STEP_LEAD_CARD", "STEP_B_CARD") || has(build, "STEP_C_CARD") || has(build, "TURN_PROMPT_CARD") || has(build, "STEP_FAILED_CARD") {
		t.Fatalf("the build repair on b.go was not dealt b's cards alone:\n%s", build)
	}
	if !has(build, "cadviser advises internal/c/c.go, which this repair's subject does not name") {
		t.Fatalf("the repair does not name the adviser it was not dealt:\n%s", build)
	}

	test := e.repairCards(result, "--- FAIL: TestDecode (0.00s)\n    c_test.go:40: got 1\nFAIL\texample/internal/c")
	if !has(test, "STEP_C_CARD") || has(test, "STEP_B_CARD") {
		t.Fatalf("the test repair on package c was not dealt c's card alone:\n%s", test)
	}

	foreign := e.repairCards(result, "vendor/x/y.go:3: syntax error")
	if !has(foreign, "STEP_LEAD_CARD") || has(foreign, "STEP_B_CARD") || has(foreign, "STEP_C_CARD") {
		t.Fatalf("a repair naming none of the turn's files was dealt an adviser's card:\n%s", foreign)
	}

	if none := e.repairCards(&ExecutionResult{}, "anything"); none != "" {
		t.Fatalf("a turn whose steps bought nothing adds a section: %q", none)
	}
}
