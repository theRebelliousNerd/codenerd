package session

import (
	"strings"
	"testing"

	"codenerd/internal/experts"
)

// The build and test repairs after a planned task ran on the turn's own
// prompt, so a consultation a step bought for the file it edited was absent
// when a repair edited the same file. The repair stage now carries the
// step-born cards policy deals it: a lead's, and an adviser's whose file the
// turn wrote -- not the cards the turn's prompt already holds, and not an
// adviser's about a file the turn did not write.
func TestFinalRepair_IsDealtTheConsultationsStepsBought(t *testing.T) {
	e := newObligationExec(t)
	result := &ExecutionResult{WrittenPaths: []string{"internal/b/b.go"}}
	e.recordCards(result, []expertCard{{Agent: "turnlead", Mode: experts.SpawnLead, Disposition: cardAnswered, Body: "TURN_PROMPT_CARD"}})
	result.promptCards = len(result.expertCards)
	e.recordCards(result, []expertCard{
		{Agent: "steplead", Mode: experts.SpawnLead, Disposition: cardAnswered, Body: "STEP_LEAD_CARD"},
		{Agent: "badviser", Mode: experts.SpawnAdvise, Disposition: cardAnswered, Body: "STEP_B_CARD", Files: []string{"internal/b/b.go"}},
		{Agent: "zadviser", Mode: experts.SpawnAdvise, Disposition: cardAnswered, Body: "STEP_Z_CARD", Files: []string{"internal/z/z.go"}},
		{Agent: "failedlead", Mode: experts.SpawnLead, Disposition: cardFailed, Body: "STEP_FAILED_CARD"},
	})

	got := e.repairCards(result)
	for _, want := range []string{"STEP_LEAD_CARD", "STEP_B_CARD"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the repair stage lacks %s:\n%s", want, got)
		}
	}
	for _, leak := range []string{"TURN_PROMPT_CARD", "STEP_Z_CARD", "STEP_FAILED_CARD"} {
		if strings.Contains(got, leak) {
			t.Fatalf("the repair stage carries %s:\n%s", leak, got)
		}
	}

	if none := e.repairCards(&ExecutionResult{}); none != "" {
		t.Fatalf("a turn whose steps bought nothing adds a section: %q", none)
	}
}
