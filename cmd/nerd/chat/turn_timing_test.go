package chat

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/observation"
	"codenerd/internal/turntime"
)

// The end-of-turn summary says where the turn's time went, in one line, and
// says nothing extra for a turn that kept no ledger.
func TestContinuationSummaryShowsTheTimeSplit(t *testing.T) {
	ret := observation.Return{
		Outcome: "/done",
		Timing: &turntime.Split{
			Total: 12*time.Minute + 3*time.Second, Model: 8 * time.Minute, Tool: time.Minute, Other: 11 * time.Second,
			ModelCalls: 41, ToolCalls: 96,
			Verify: []turntime.Kind{{Kind: turntime.KindBuild, Count: 2, Time: 20 * time.Second}, {Kind: turntime.KindTests, Count: 3, Time: 2*time.Minute + 32*time.Second}},
			Repair: []turntime.Kind{{Kind: "tests", Count: 2, Time: 90 * time.Second}},
		},
	}
	got := continuationSummary(1, ret, "")
	want := "Time 12m3s: model 8m0s (41 calls), tools 1m0s (96), verify 2m52s (build 20s, tests 2m32s), repair 2 rounds (tests 2; 1m30s), other 11s."
	if !strings.Contains(got, want) {
		t.Errorf("summary %q lacks the time line %q", got, want)
	}
	if strings.Count(got, "Time ") != 1 || strings.Contains(got[strings.Index(got, "Time "):], "\n") {
		t.Errorf("the time split is one line: %q", got)
	}

	ret.Timing = nil
	if got := continuationSummary(1, ret, ""); strings.Contains(got, "Time ") {
		t.Errorf("a turn with no ledger shows a time line: %q", got)
	}
}
