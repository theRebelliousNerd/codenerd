//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"testing"
)

type correctionCase struct {
	name                    string
	request, reply, message string
	want                    bool
}

const (
	replyEditedMin   = "Done. I changed Min in internal/stats/stats.go so it returns 0 for an empty slice, and added TestMinEmpty."
	replyExplainedJIT = "The JIT compiler scores each atom by its selectors, then the kernel ranks the survivors with jit_selection.mg and the budget trims from the bottom."
	replyRunningTests = "Running the full test suite in the background now; I'll report failures as they come in."
	replyReviewed     = "Review of internal/approval: 2 findings. broker.go:88 opens a second request for a duplicate call; request.go:40 ignores ctx cancellation."
)

var correctionCases = []correctionCase{
	// Corrections, with and without rebuke words.
	{"wrong-function", "make Max return 0 for an empty slice", replyEditedMin, "no, I said Max, not Min", true},
	{"wrong-file-plain", "make Max return 0 for an empty slice", replyEditedMin, "that's the other function", true},
	{"bare-wrong", "make Max return 0 for an empty slice", replyEditedMin, "wrong", true},
	{"misread", "explain how the kernel ranks atoms", replyExplainedJIT, "that's not what I asked, I wanted the kernel side only", true},
	{"wrong-answer-fact", "explain how the kernel ranks atoms", replyExplainedJIT, "the budget doesn't trim from the bottom, it drops whole sections", true},
	{"you-didnt", "make Max return 0 for an empty slice", replyEditedMin, "you didn't add the test I asked for, TestMinEmpty isn't there", true},
	{"undo-that", "make Max return 0 for an empty slice", replyEditedMin, "undo that, you broke Min", true},
	// Not corrections, many with the old trigger words.
	{"stop-running", "run all the tests", replyRunningTests, "stop", false},
	{"stop-doing", "run all the tests", replyRunningTests, "stop the tests, I need the CPU", false},
	{"own-bug-wrong", "make Max return 0 for an empty slice", replyEditedMin, "thanks. now Mean is wrong too - TestMean says 6 instead of 4, fix it", false},
	{"tests-wrong", "make Max return 0 for an empty slice", replyEditedMin, "the expected values in stats_test.go are wrong, update them to match the spec", false},
	{"question-wrong", "make Max return 0 for an empty slice", replyEditedMin, "what's wrong with returning an error instead?", false},
	{"accepts", "make Max return 0 for an empty slice", replyEditedMin, "perfect, thanks", false},
	{"new-work", "make Max return 0 for an empty slice", replyEditedMin, "now add the same for Sum", false},
	{"follow-up-finding", "review internal/approval", replyReviewed, "fix the first finding", false},
	{"mistake-in-code", "review internal/approval", replyReviewed, "there's a mistake in how broker.go handles timeouts too, look at that next", false},
	{"agree-finding", "review internal/approval", replyReviewed, "right, the second one is a real bug", false},
}

// TestLiveCorrection probes the correction question over corrections and
// over messages the old keyword trigger misread.
func TestLiveCorrection(t *testing.T) {
	c, _ := liveClient(t, "correction_judgments")
	tally := map[string][3]int{}
	var misses []string
	for _, tc := range correctionCases {
		ctx := probeLabel(context.Background(), tc.name)
		p, asked, err := JudgeCorrection(ctx, c, CorrectionInput{PreviousRequest: tc.request, LastReply: tc.reply, Message: tc.message})
		if err != nil || !asked {
			t.Fatalf("%s: asked=%v err=%v", tc.name, asked, err)
		}
		scoreLine(tally, &misses, tc.name, CorrectsLastTurn, tc.message, p, tc.want)
		t.Logf("%-20s p=%.2f want=%v %q", tc.name, p, tc.want, tc.message)
	}
	lines := append([]string{fmt.Sprintf("correction probe: %d cases, thresholds yes>=0.70 no<=0.30", len(correctionCases))}, tallyLines(tally)...)
	probeSummary(t, "correction_judgments", append(append(lines, "misses (x wrong, ~ unsettled):"), misses...))
}
