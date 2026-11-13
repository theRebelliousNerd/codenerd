package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/types"
)

type fakePinJudge struct {
	percent int64
	asked   []string
}

func (j *fakePinJudge) ReadPinEffect(_ context.Context, label, with, without string) (int64, error) {
	j.asked = append(j.asked, label+"\n"+with+"\n--\n"+without)
	return j.percent, nil
}

// A loop rewritten for speed: the turn's test exercises Sum, and Sum put back
// as it was passes it and the package's whole suite. With Jev reading no
// effect a test can force, the kernel classifies it not pinnable and the gate
// passes, naming it; with Jev reading an observable effect, it stays unpinned
// and the gate still asks for a test.
func TestPinGate_APerformanceOnlyChangeIsClassifiedNotPinnable(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	const (
		before = "package pinprobe\n\nfunc Sum(xs []int) int {\n\ts := 0\n\tfor i := 0; i < len(xs); i++ {\n\t\ts += xs[i]\n\t}\n\treturn s\n}\n"
		after  = "package pinprobe\n\nfunc Sum(xs []int) int {\n\ts := 0\n\tfor _, x := range xs {\n\t\ts += x\n\t}\n\treturn s\n}\n"
		test   = "package pinprobe\n\nimport \"testing\"\n\nfunc TestSumRuns(t *testing.T) { _ = Sum([]int{1, 2}) }\n"
	)
	for _, tc := range []struct {
		name        string
		percent     int64
		notPinnable bool
	}{
		{"jev reads no effect", 4, true},
		{"jev reads an effect", 88, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := writeBaselineModule(t, map[string]string{"go.mod": pinGoMod, "sum.go": after, "sum_test.go": test})
			result := mutationResult()
			result.Intent.Verb = "/fix"
			result.WrittenPaths = []string{"sum.go", "sum_test.go"}
			result.PreWriteContents = map[string]PreImage{"sum.go": existed(before), "sum_test.go": {}}

			e := NewExecutor(realKernel(t), nil, nil, nil, nil, nil)
			e.assertTurnFact(types.Fact{Predicate: "config_param", Args: []any{types.MangleAtom("/jev_no_percent"), int64(30)}})
			judge := &fakePinJudge{percent: tc.percent}
			e.SetPinEffectJudge(judge)

			result.PinCheck = verifyPinning(context.Background(), ws, result, false)
			if result.PinCheck.Verdict() != VerifyFailed {
				t.Fatalf("setup: verdict %s (%s), want the loop rewrite unpinned", result.PinCheck.Verdict(), result.PinCheck.Reason)
			}
			e.classifyUnpinnable(context.Background(), ws, result)
			if len(judge.asked) != 1 || !strings.Contains(judge.asked[0], "range xs") || !strings.Contains(judge.asked[0], "len(xs)") {
				t.Fatalf("the judge was not shown both sides of the change: %q", judge.asked)
			}
			if tc.notPinnable {
				if result.PinCheck.Verdict() != VerifyPassed || len(result.PinNotPinnable) != 1 || !strings.Contains(result.PinCheck.Reason, "not pinnable") {
					t.Fatalf("verdict %s, not pinnable %v, reason %q: want passed with the loop named not pinnable",
						result.PinCheck.Verdict(), result.PinNotPinnable, result.PinCheck.Reason)
				}
				return
			}
			if result.PinCheck.Verdict() != VerifyFailed || len(result.PinNotPinnable) != 0 {
				t.Fatalf("verdict %s, not pinnable %v: an observable change must stay owed", result.PinCheck.Verdict(), result.PinNotPinnable)
			}
		})
	}
}
