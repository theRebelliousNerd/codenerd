//go:build jevlive

package learnjudge

import (
	"context"
	"os"
	"testing"
)

// The first dogfood rung's instruction and the reviewer's real answer
// (testdata/coverage_rung1_result.txt): the answer covers every ask, and the
// splitter's example and place-to-look fragments are not asks.
const rung1Request = "Read-only, do not change any files: evolved prompt atoms are pinned to an exact model, but an exact pin with no date suffix (e.g. claude-opus-5-5) still matches later dated snapshots of that model because matching compares family tokens. Find the code in internal/prompt and internal/autopoiesis/prompt_evolution responsible, explain precisely why it happens with file:line references, and describe the smallest correct fix and the test that would prove it."

func TestLiveCoverage_Rung1(t *testing.T) {
	client, sec := liveClient(t, "instruction_coverage")
	result, err := os.ReadFile("testdata/coverage_rung1_result.txt")
	if err != nil {
		t.Fatal(err)
	}
	fragments := []string{
		"(e.g",
		"claude-opus-5-5) still matches later dated snapshots of that model because matching compares family tokens",
		"Find the code in internal/prompt",
		"internal/autopoiesis/prompt_evolution responsible, explain precisely why it happens with file:line references",
		"describe the smallest correct fix",
		"the test that would prove it",
	}
	verdicts, cut, err := CheckCoverage(context.Background(), client, rung1Request, string(result), fragments)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range verdicts {
		t.Logf("asked=%.2f delivered=%.2f  %q", v.Asked, v.Delivered, v.Fragment)
	}
	yes, no := sec.Thresholds()
	missing, unsettled := CoverageGaps(verdicts, cut, float64(yes)/100, float64(no)/100)
	t.Logf("cut=%v missing=%q unsettled=%q", cut, missing, unsettled)
	if len(missing) != 0 {
		t.Errorf("the answer covers every ask, but these were judged missing: %q", missing)
	}

	// Against an answer that only describes the code, the fix and the test
	// are missing.
	describeOnly := "Research only. internal/prompt/pinning.go normalizes model names into tokens, and ModelFamilyToken strips date suffixes. MatchesContext in atoms.go accepts an atom when any context token is in its Models list."
	verdicts, cut, err = CheckCoverage(context.Background(), client, rung1Request, describeOnly, fragments)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range verdicts {
		t.Logf("describe-only: asked=%.2f delivered=%.2f  %q", v.Asked, v.Delivered, v.Fragment)
	}
	missing, _ = CoverageGaps(verdicts, cut, float64(yes)/100, float64(no)/100)
	t.Logf("describe-only missing=%q", missing)
	if len(missing) == 0 {
		t.Error("a describe-only answer must leave the fix and the test missing")
	}
}
