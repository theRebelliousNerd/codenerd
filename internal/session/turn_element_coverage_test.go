package session

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// The shapes below are the ones `go test -coverprofile` actually emits
// (probed 2026-09-28, covermode set): a called function is one block with
// count 1, an uncalled one is count 0, a branched function is several blocks
// of which the taken branch counts, and an empty body is a block of zero
// statements. The element fact follows that, not the file-level debt.

func TestTurnElementUncovered_StatementRule(t *testing.T) {
	if elementUncovered(LineRange{Start: 9, End: 14}, []UncoveredBlock{
		{StartLine: 9, EndLine: 10, NumStmts: 1, Count: 1},
		{StartLine: 13, EndLine: 13, NumStmts: 1, Count: 0},
	}) {
		t.Fatal("a branch that ran means the element ran, even though another block did not")
	}
	if !elementUncovered(LineRange{Start: 5, End: 5}, []UncoveredBlock{
		{StartLine: 5, EndLine: 5, NumStmts: 1, Count: 0},
	}) {
		t.Fatal("the only statement block was never executed")
	}
	if elementUncovered(LineRange{Start: 16, End: 16}, []UncoveredBlock{
		{StartLine: 16, EndLine: 16, NumStmts: 0, Count: 0},
	}) {
		t.Fatal("an empty body has no statement to miss")
	}
	if elementUncovered(LineRange{Start: 3, End: 3}, nil) {
		t.Fatal("no block is not a missed block")
	}
}

const (
	elemCoverBefore = `package elemcover

func Used() int { return 0 }

func Unused() int { return 0 }

func Old() int { return 3 }

func Half(x int) int {
	if x > 0 {
		return 0
	}
	return 1
}

func Empty() int { return 9 }

func (b *Box) Get() int { return 0 }

type Box struct{}
`
	elemCoverAfter = `package elemcover

func Used() int { return 1 }

func Unused() int { return 2 }

func Old() int { return 3 }

func Half(x int) int {
	if x > 0 {
		return x
	}
	return 0
}

func Empty() {}

func (b *Box) Get() int { return 1 }

type Box struct{}
`
	elemCoverTest = `package elemcover

import "testing"

func TestUsed(t *testing.T) {
	if Used() != 1 {
		t.Fatal("used")
	}
	if Half(1) != 1 {
		t.Fatal("half")
	}
}
`
)

// A real coverage run of a turn that changed two functions and executed one
// names exactly the other. Half is the same fact at branch granularity: the
// test takes one branch, the other block is uncovered, and the element is
// not turn_element_uncovered. Old is uncovered and unchanged, so it is not
// named either. Empty changed into a body with no statements. The written
// path is recorded in the host's spelling (a backslash on Windows, where the
// tools record one); the fact and the match use slashes.
func TestTurnElementUncovered_RealRunNamesTheMissedElements(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and tests a throwaway package")
	}
	written := filepath.Join("sub", "calc.go")
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":           "module elemcover\n\ngo 1.21\n",
		"sub/calc.go":      elemCoverAfter,
		"sub/calc_test.go": elemCoverTest,
	})

	v, blocks := verifyTestsWithCoverage(context.Background(), ws, packagesForPaths([]string{written}), []string{written})
	if !v.Ran || !v.OK {
		t.Fatalf("coverage run Ran=%v OK=%v output=%q", v.Ran, v.OK, v.Output)
	}
	if len(blocks) == 0 {
		t.Fatal("coverage run produced no blocks")
	}

	result := writeTurnResult()
	result.WrittenPaths = []string{written}
	result.PreWriteContents = map[string]PreImage{written: existed(elemCoverBefore)}
	result.UncoveredBlocks = narrowToChangedLines(ws, result, blocks)
	for _, b := range result.UncoveredBlocks {
		if b.Count != 0 {
			t.Fatalf("file-level debt kept an executed block: %+v", b)
		}
		if strings.Contains(b.File, `\`) {
			t.Fatalf("profile path %q uses a backslash; profiles are slash-separated", b.File)
		}
	}
	spans := elementSpans(elemCoverAfter)
	for _, b := range result.UncoveredBlocks {
		if overlapsSpan(b, spans["fn:elemcover.Used"]) {
			t.Fatalf("Used ran, and its block is still file-level debt: %+v", b)
		}
	}

	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws
	e.assertTurnEvidence(testTurn, "/fix", result)

	got := factStrings(t, e, "turn_element_uncovered", 1)
	want := []string{"fn:elemcover.Box.Get", "fn:elemcover.Unused"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("turn_element_uncovered = %v, want %v", got, want)
	}
	changed := factStrings(t, e, "turn_changed_element", 1)
	for _, ref := range []string{"fn:elemcover.Used", "fn:elemcover.Half", "fn:elemcover.Empty"} {
		if !containsString(changed, ref) {
			t.Fatalf("turn_changed_element = %v, want it to include %s", changed, ref)
		}
		if containsString(got, ref) {
			t.Fatalf("turn_element_uncovered names %s, which the run executed or which has no statement block", ref)
		}
	}
	if containsString(changed, "fn:elemcover.Old") || containsString(got, "fn:elemcover.Old") {
		t.Fatalf("Old did not change: changed %v uncovered %v", changed, got)
	}
	for _, ref := range got {
		if strings.Contains(ref, `\`) || strings.HasPrefix(ref, "/") {
			t.Fatalf("ref %q is not a code_element ref", ref)
		}
	}

	worldRefs := worldCodeRefs(t, elemCoverAfter)
	for _, ref := range append(append([]string{}, got...), changed...) {
		if !worldRefs[ref] {
			t.Errorf("ref %q is not a code_element ref the world produces for the same file", ref)
		}
	}

	paths := factStrings(t, e, "turn_uncovered", 1)
	if len(paths) != 1 || paths[0] != "sub/calc.go" {
		t.Fatalf("turn_uncovered = %v, want [sub/calc.go]", paths)
	}

	e.cleanupTurnFacts()
	if n := queryCount(t, e, "turn_element_uncovered"); n != 0 {
		t.Errorf("turn_element_uncovered = %d after cleanup, want 0", n)
	}
	if n := queryCount(t, e, "turn_changed_element"); n != 0 {
		t.Errorf("turn_changed_element = %d after cleanup, want 0", n)
	}
}

func TestTurnElementUncovered_NotInventedWithoutAProfile(t *testing.T) {
	e := newObligationExec(t)
	result := writeTurnResult()
	result.UncoveredBlocks = []UncoveredBlock{{File: "pkg/foo.go", StartLine: 1, EndLine: 2, NumStmts: 1}}
	e.assertTurnEvidence(testTurn, "/fix", result)
	if got := queryCount(t, e, "turn_element_uncovered"); got != 0 {
		t.Fatalf("turn_element_uncovered = %d with no profile mapped onto elements, want 0", got)
	}
	if got := queryCount(t, e, "turn_uncovered"); got != 1 {
		t.Fatalf("turn_uncovered = %d, want 1: the file-level debt is unchanged", got)
	}
}

func TestTurnElementUncovered_ModelCannotAssert(t *testing.T) {
	update := `turn_element_uncovered(/turn_test, "fn:elemcover.Unused").`
	permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
	if kept, _ := core.FilterMangleUpdates(nil, []string{update}, permissive); len(kept) != 0 {
		t.Errorf("the model can assert %s; the verdict's evidence must be the harness's alone", update)
	}
	if kept, _ := core.FilterMangleUpdates(nil, []string{update}, core.ModelObservationPolicy()); len(kept) != 0 {
		t.Errorf("the model can assert %s through the observation policy", update)
	}
}

func factStrings(t *testing.T, e *Executor, predicate string, arg int) []string {
	t.Helper()
	facts, err := e.kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	var out []string
	for _, f := range facts {
		if len(f.Args) <= arg {
			t.Fatalf("%s%v: want an argument at %d", predicate, f.Args, arg)
		}
		if got := types.ExtractString(f.Args[0]); got != string(testTurn) {
			t.Errorf("%s turn = %v, want %v", predicate, got, testTurn)
		}
		out = append(out, types.ExtractString(f.Args[arg]))
	}
	sort.Strings(out)
	return out
}

func overlapsSpan(b UncoveredBlock, span LineRange) bool {
	return b.StartLine <= span.End && b.EndLine >= span.Start
}

func worldCodeRefs(t *testing.T, content string) map[string]bool {
	t.Helper()
	elems, err := world.NewGoCodeParser(t.TempDir()).Parse("calc.go", []byte(content))
	if err != nil {
		t.Fatalf("world parse: %v", err)
	}
	refs := make(map[string]bool)
	for _, elem := range elems {
		if elem.Type == world.ElementFunction || elem.Type == world.ElementMethod {
			refs[elem.Ref] = true
		}
	}
	return refs
}
