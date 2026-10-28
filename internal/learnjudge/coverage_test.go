package learnjudge

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// The fragments the first dogfood rung's splitter produced from one
// instruction: an example and a place to look, beside the real asks.
var rungFragments = []string{
	"claude-opus-5-5) still matches later dated snapshots of that model because matching compares family tokens",
	"internal/autopoiesis/prompt_evolution responsible, explain precisely why it happens with file:line references",
	"describe the smallest correct fix",
}

func TestCheckCoverage_AsksBothJudgmentsPerFragmentOverOneState(t *testing.T) {
	a := &autopoiesisAsker{answers: map[string]float64{
		"asked_0": 0.05, "delivered_0": 0.9,
		"asked_1": 0.95, "delivered_1": 0.92,
		"asked_2": 0.97, "delivered_2": 0.1,
	}}
	verdicts, cut, err := CheckCoverage(context.Background(), a, "explain why, and describe the smallest fix", "the cause is ...", rungFragments)
	if err != nil || cut {
		t.Fatalf("err=%v cut=%v", err, cut)
	}
	if len(a.questions) != 2*len(rungFragments) {
		t.Fatalf("asked %d questions, want two per fragment", len(a.questions))
	}
	state := a.state.(map[string]any)
	if state["request"] == nil || state["result"] == nil {
		t.Fatalf("state = %v", state)
	}
	if len(verdicts) != 3 || verdicts[2].Fragment != rungFragments[2] || verdicts[2].Asked != 0.97 || verdicts[2].Delivered != 0.1 {
		t.Fatalf("verdicts = %+v", verdicts)
	}
}

func TestCheckCoverage_AFailedCallIsAnErrorNotAVerdict(t *testing.T) {
	if _, _, err := CheckCoverage(context.Background(), &autopoiesisAsker{err: errors.New("503")}, "r", "o", rungFragments); err == nil {
		t.Fatal("a failed call must be an error")
	}
	if _, _, err := CheckCoverage(context.Background(), nil, "r", "o", rungFragments); err == nil {
		t.Fatal("no client must be an error")
	}
}

func TestCheckCoverage_ALongResultIsBoundedAndSaysSo(t *testing.T) {
	a := &autopoiesisAsker{answers: map[string]float64{}}
	long := strings.Repeat("x", coverageResultLimit) + "THE END"
	if _, cut, err := CheckCoverage(context.Background(), a, "r", long, rungFragments[:1]); err != nil || !cut {
		t.Fatalf("cut=%v err=%v", cut, err)
	}
	if n, _ := a.state.(map[string]any)["result_omitted_bytes"].(int); n <= 0 {
		t.Fatal("the state does not say how much of the result was left out")
	}
}

// The truth table: not asked is ignored, delivered passes, undelivered is a
// gap unless the result was cut, and the middle band is unsettled.
func TestCoverageGaps_TruthTable(t *testing.T) {
	verdicts := []CoverageVerdict{
		{Fragment: "an example", Asked: 0.05, Delivered: 0.0},
		{Fragment: "explain why", Asked: 0.95, Delivered: 0.9},
		{Fragment: "describe the fix", Asked: 0.97, Delivered: 0.1},
		{Fragment: "name the test", Asked: 0.9, Delivered: 0.5},
		{Fragment: "maybe asked", Asked: 0.5, Delivered: 0.05},
	}
	missing, unsettled := CoverageGaps(verdicts, false, 0.7, 0.3)
	if !slices.Equal(missing, []string{"describe the fix", "maybe asked"}) {
		t.Fatalf("missing = %v", missing)
	}
	if !slices.Equal(unsettled, []string{"name the test"}) {
		t.Fatalf("unsettled = %v", unsettled)
	}
	missing, unsettled = CoverageGaps(verdicts, true, 0.7, 0.3)
	if len(missing) != 0 || !slices.Equal(unsettled, []string{"describe the fix", "name the test", "maybe asked"}) {
		t.Fatalf("a cut result must not yield gaps: missing=%v unsettled=%v", missing, unsettled)
	}
}
