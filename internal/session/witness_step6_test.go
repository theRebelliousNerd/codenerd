package session

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// W6: a write turn is verified only when a passing test run executed every
// element it changed. These tests run the real coverage gate (`go test
// -coverprofile`, the same run whose verdict is turn_gate(Turn, /test, _))
// over throwaway modules and read the verdict through the production entry
// points, as turn_element_coverage_test.go does.

const (
	witnessCalcBefore = "package calc\n\nfunc Add(a, b int) int { return 0 }\n\nfunc Sub(a, b int) int { return 0 }\n"
	witnessCalcAfter  = "package calc\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Sub(a, b int) int { return a - b }\n"
	witnessTestAdd    = "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"add\")\n\t}\n}\n"
	witnessTestBoth   = witnessTestAdd + "\nfunc TestSub(t *testing.T) {\n\tif Sub(5, 3) != 2 {\n\t\tt.Fatal(\"sub\")\n\t}\n}\n"
)

// witnessVerdict runs the turn's real coverage gate and reads its verdict:
// the blocks narrowed to the turn's changed lines, the gates as green as the
// run measured them, and the pinning gate a behaviour change owes. The
// caller asserts on the kernel and the outcome.
func witnessVerdict(t *testing.T, testFile string) (*Executor, *ExecutionResult) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles and tests a throwaway package")
	}
	written := filepath.Join("sub", "calc.go")
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":           "module witnessprobe\n\ngo 1.21\n",
		"sub/calc.go":      witnessCalcAfter,
		"sub/calc_test.go": testFile,
	})
	v, blocks := verifyTestsWithCoverage(context.Background(), ws, packagesForPaths([]string{written}), []string{written})
	if !v.Ran {
		t.Fatalf("coverage run did not run: %q", v.Output)
	}
	result := writeTurnResult()
	result.WrittenPaths = []string{written}
	result.PreWriteContents = map[string]PreImage{written: existed(witnessCalcBefore)}
	result.UncoveredBlocks = narrowToChangedLines(ws, result, blocks)
	result.TestCheck = v
	result.UntestedPaths = untestedWithoutCoverageOnDisk(ws, result.WrittenPaths)
	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws
	e.assertTurnEvidence(testTurn, "/fix", result)
	e.captureTurnOutcome(testTurn, result, nil)
	t.Cleanup(e.cleanupTurnFacts)
	return e, result
}

// Changed Add+Sub, the passing run executes Add only: the turn is unverified,
// the missing evidence is /change_unwitnessed alone, and the verdict names
// fn:calc.Sub and nothing else. The file-level debt stays silent: Sub's
// block sits inside a changed element, so /changed_code_unexecuted must not
// name it too.
func TestWitnessStep6_PartialExecutionWithholdsVerification(t *testing.T) {
	e, result := witnessVerdict(t, witnessTestAdd)

	if result.TurnOutcome != types.MangleAtom("/unverified") {
		t.Fatalf("TurnOutcome = %q, want /unverified: no passing test executed Sub", result.TurnOutcome)
	}
	if len(result.MissingEvidence) != 1 || result.MissingEvidence[0] != "/change_unwitnessed" {
		t.Fatalf("MissingEvidence = %v, want [/change_unwitnessed]", result.MissingEvidence)
	}
	if len(result.UnwitnessedElements) != 1 || result.UnwitnessedElements[0] != "fn:calc.Sub" {
		t.Fatalf("UnwitnessedElements = %v, want [fn:calc.Sub]", result.UnwitnessedElements)
	}
	if got := factStrings(t, e, "turn_unwitnessed", 1); len(got) != 1 || got[0] != "fn:calc.Sub" {
		t.Fatalf("turn_unwitnessed = %v, want [fn:calc.Sub]", got)
	}
	if got := factStrings(t, e, "witness_met", 1); len(got) != 1 || got[0] != "fn:calc.Add" {
		t.Fatalf("witness_met = %v, want [fn:calc.Add]", got)
	}
	if got := queryCount(t, e, "turn_uncovered"); got != 0 {
		t.Fatalf("turn_uncovered = %d, want 0: no dual path over Sub's block", got)
	}
	if sentence := verdictSentence(result); !strings.Contains(sentence, "no passing test executed fn:calc.Sub") {
		t.Fatalf("verdict sentence %q does not name the unwitnessed element", sentence)
	}
}

// Changed Add+Sub, the passing run executes both: the witness is met and the
// turn verifies.
func TestWitnessStep6_FullExecutionVerifies(t *testing.T) {
	_, result := witnessVerdict(t, witnessTestBoth)

	if result.TurnOutcome != types.MangleAtom("/done") {
		t.Fatalf("TurnOutcome = %q (missing %v), want /done: the passing run executed every changed element",
			result.TurnOutcome, result.MissingEvidence)
	}
	if len(result.UnwitnessedElements) != 0 {
		t.Fatalf("UnwitnessedElements = %v, want none", result.UnwitnessedElements)
	}
}
