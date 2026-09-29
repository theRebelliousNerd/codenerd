package session

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/testfacts"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// nextTurn is the second turn on a kernel the first turn also wrote. The
// two stay distinct so a fact keyed by one cannot satisfy a join on the other.
const nextTurn types.MangleAtom = "/turn_next"

// The turn-scoped predicates are testfacts.Facts() with this turn prepended.
// A failing `go test -json` asserts that run's failure, file included, under
// this turn only. The unscoped test_failure_at row is a different measurement
// and is not written. A second turn on the same kernel does not carry the
// first turn's test, and cleanup of the first turn leaves the second.
func TestTurnTestFacts_FailingRunIsThisTurnOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and tests throwaway packages")
	}
	ws := writeBaselineModule(t, map[string]string{
		"go.mod": "module turnprobe\n\ngo 1.21\n",
		"a/a.go": "package a\n\nfunc A() int { return 0 }\n",
		"a/a_test.go": "package a\n\nimport \"testing\"\n\n" +
			"func TestA(t *testing.T) {\n\tt.Fatal(\"from-a\")\n}\n",
		"b/b.go": "package b\n\nfunc B() int { return 1 }\n",
		"b/b_test.go": "package b\n\nimport \"testing\"\n\n" +
			"func TestB(t *testing.T) {\n\tif B() != 1 {\n\t\tt.Fatal(\"from-b\")\n\t}\n}\n",
	})

	failed := verifyTests(context.Background(), ws, []string{"./a"})
	if !failed.Ran || failed.OK || failed.Result == nil {
		t.Fatalf("package a: Ran=%v OK=%v result=%v output=%q reason=%q", failed.Ran, failed.OK, failed.Result != nil, failed.Output, failed.Reason)
	}
	passed := verifyTests(context.Background(), ws, []string{"./b"})
	if !passed.Ran || !passed.OK || passed.Result == nil {
		t.Fatalf("package b: Ran=%v OK=%v result=%v output=%q reason=%q", passed.Ran, passed.OK, passed.Result != nil, passed.Output, passed.Reason)
	}

	e1 := newObligationExec(t)
	e2 := newObligationExec(t)
	e2.kernel = e1.kernel

	unscoped := []string{"test_case", "test_failure_at", "test_build_failure", "test_output_repeat", "failing_test"}
	before := map[string]int{}
	for _, p := range unscoped {
		before[p] = queryCount(t, e1, p)
	}

	failResult := writeTurnResult()
	failResult.TestCheck = failed
	e1.assertTurnEvidence(testTurn, "/fix", failResult)
	passResult := writeTurnResult()
	passResult.TestCheck = passed
	e2.assertTurnEvidence(nextTurn, "/fix", passResult)

	for _, p := range unscoped {
		if got := queryCount(t, e1, p); got != before[p] {
			t.Errorf("%s = %d after the turn asserts, was %d: the unscoped predicate is not this turn's fact", p, got, before[p])
		}
	}

	rows := factsForTurn(t, e1, "turn_test_failure_at", testTurn)
	hit := failureForTest(t, rows, "TestA")
	file := factArg(t, hit, 3)
	if file != "a/a_test.go" {
		t.Fatalf("failure file = %q, want a/a_test.go", file)
	}
	if strings.Contains(file, `\`) || strings.Contains(file, ":") || strings.HasPrefix(file, "/") {
		t.Fatalf("failure file %q is not a workspace-relative slash path", file)
	}
	if factArg(t, hit, 5) != "from-a" {
		t.Fatalf("failure message = %q, want from-a", factArg(t, hit, 5))
	}
	line, err := strconv.Atoi(factArg(t, hit, 4))
	if err != nil || line <= 0 {
		t.Fatalf("failure line = %q, want a positive line", factArg(t, hit, 4))
	}
	if got := caseStatus(t, factsForTurn(t, e1, "turn_test_case", testTurn), "TestA"); got != "/fail" {
		t.Fatalf("TestA status = %q, want /fail", got)
	}
	if !rowsMention(factsForTurn(t, e1, "turn_failing_test", testTurn), "TestA") {
		t.Fatal("turn_failing_test does not name TestA")
	}

	for _, pred := range []string{"turn_test_case", "turn_test_failure_at", "turn_test_build_failure", "turn_test_output_repeat", "turn_failing_test"} {
		second := factsForTurn(t, e1, pred, nextTurn)
		if rowsMention(second, "TestA") || rowsMention(second, "from-a") {
			t.Fatalf("%s for %s carries the first turn's failure: %s", pred, nextTurn, describeFacts(second))
		}
		for _, f := range factsForTurn(t, e1, pred, "") {
			key := factArg(t, f, 0)
			if key != string(testTurn) && key != string(nextTurn) {
				t.Fatalf("%s row is not keyed by either turn: %s", pred, describeFacts([]types.Fact{f}))
			}
		}
	}
	if got := caseStatus(t, factsForTurn(t, e1, "turn_test_case", nextTurn), "TestB"); got != "/pass" {
		t.Fatalf("TestB status = %q, want /pass", got)
	}
	if n := len(factsForTurn(t, e1, "turn_test_failure_at", nextTurn)); n != 0 {
		t.Fatalf("the passing turn has %d turn_test_failure_at rows, want 0", n)
	}

	e1.cleanupTurnFacts()
	for _, pred := range []string{"turn_test_case", "turn_test_failure_at", "turn_failing_test"} {
		if n := len(factsForTurn(t, e1, pred, testTurn)); n != 0 {
			t.Fatalf("%s still has %d rows for %s after its cleanup", pred, n, testTurn)
		}
	}
	if got := caseStatus(t, factsForTurn(t, e1, "turn_test_case", nextTurn), "TestB"); got != "/pass" {
		t.Fatalf("after the first turn's cleanup TestB status = %q, want /pass", got)
	}
	if rowsMention(factsForTurn(t, e1, "turn_test_failure_at", nextTurn), "from-a") ||
		rowsMention(factsForTurn(t, e1, "turn_test_case", nextTurn), "TestA") {
		t.Fatal("cleaning up the first turn removed or rewrote the second turn's facts")
	}

	e2.cleanupTurnFacts()
	for _, pred := range []string{"turn_test_case", "turn_test_failure_at", "turn_failing_test", "turn_test_build_failure", "turn_test_output_repeat"} {
		if n := queryCount(t, e1, pred); n != 0 {
			t.Fatalf("%s = %d after both turns cleaned up, want 0", pred, n)
		}
	}
}

// A gate that did not run, and a gate that ran but kept no Result, are not
// measurements. A Result sitting on a skipped verification is not asserted.
func TestTurnTestFacts_SkippedGateAssertsNothing(t *testing.T) {
	res := &testfacts.Result{
		Status: testfacts.StatusFail,
		Packages: []*testfacts.Package{{
			Name:   "p",
			Status: testfacts.StatusFail,
			Tests:  []*testfacts.Test{{Name: "TestA", Status: testfacts.StatusFail}},
		}},
		Failures: []testfacts.Failure{{
			Package: "p", Test: "TestA", File: "a_test.go", Line: 4, Message: "from-a", Count: 1,
		}},
	}
	if len(res.Facts()) == 0 {
		t.Fatal("the fixture Result has no facts to leak")
	}

	e := newObligationExec(t)
	skipped := writeTurnResult()
	skipped.TestCheck = TestVerification{Ran: false, Outcome: VerifySkipped, Result: res}
	e.assertTurnEvidence(testTurn, "/fix", skipped)
	for _, pred := range []string{"turn_test_case", "turn_test_failure_at", "turn_failing_test"} {
		if n := queryCount(t, e, pred); n != 0 {
			t.Fatalf("%s = %d for a gate that did not run, want 0", pred, n)
		}
	}

	e.cleanupTurnFacts()
	empty := writeTurnResult()
	empty.TestCheck = TestVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	e.assertTurnEvidence(testTurn, "/fix", empty)
	for _, pred := range []string{"turn_test_case", "turn_test_failure_at", "turn_failing_test"} {
		if n := queryCount(t, e, pred); n != 0 {
			t.Fatalf("%s = %d for a run with no Result, want 0", pred, n)
		}
	}
}

// Measured is the positive half of the statement rule: a statement block in
// the span counts whether or not it executed. An empty body and a span the
// profile does not mention count as neither measured nor missed.
func TestElementMeasured_StatementRule(t *testing.T) {
	span := LineRange{Start: 5, End: 8}
	unexecuted := []UncoveredBlock{{StartLine: 5, EndLine: 6, NumStmts: 1, Count: 0}}
	if !elementMeasured(span, unexecuted) || !elementUncovered(span, unexecuted) {
		t.Fatal("an unexecuted statement block is measured and uncovered")
	}
	executed := []UncoveredBlock{{StartLine: 5, EndLine: 6, NumStmts: 1, Count: 1}}
	if !elementMeasured(span, executed) || elementUncovered(span, executed) {
		t.Fatal("an executed statement block is measured and not uncovered")
	}
	branched := []UncoveredBlock{
		{StartLine: 5, EndLine: 6, NumStmts: 1, Count: 1},
		{StartLine: 8, EndLine: 8, NumStmts: 1, Count: 0},
	}
	if !elementMeasured(span, branched) || elementUncovered(span, branched) {
		t.Fatal("one executed branch measures the element and discharges uncovered")
	}
	empty := []UncoveredBlock{{StartLine: 5, EndLine: 5, NumStmts: 0, Count: 0}}
	if elementMeasured(span, empty) || elementUncovered(span, empty) {
		t.Fatal("a zero-statement block is neither measured nor uncovered")
	}
	if elementMeasured(span, nil) || elementUncovered(span, nil) {
		t.Fatal("no block is neither measured nor uncovered")
	}

	// The file-level walk drops executed blocks after this returns, so a
	// stale measured list must not survive an empty profile.
	result := &ExecutionResult{ElementMeasured: []string{"fn:stale.Kept"}, WrittenPaths: []string{"x.go"}}
	if got := elementUncoveredRefs(t.TempDir(), result, nil); got != nil || result.ElementMeasured != nil {
		t.Fatalf("empty profile: uncovered %v measured %v, want none", got, result.ElementMeasured)
	}
}

const (
	otherBefore = "package other\n\nfunc Gone() int { return 0 }\n"
	otherAfter  = "package other\n\nfunc Gone() int { return 1 }\n"
)

// A real coverprofile of ./sub measures the changed elements whose spans hold
// a statement block and leaves the rest off the list. Used and Half ran (Half
// only one branch) so they are measured and not uncovered. Unused and Box.Get
// are on both lists. Empty changed into a body with no statements. Gone
// changed in a package this run did not test, so the profile does not mention
// its file. Old did not change.
func TestTurnElementMeasured_ProfileSplitsMeasuredFromMissed(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and tests a throwaway package")
	}
	written := filepath.Join("sub", "calc.go")
	other := filepath.Join("other", "other.go")
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":           "module elemcover\n\ngo 1.21\n",
		"sub/calc.go":      elemCoverAfter,
		"sub/calc_test.go": elemCoverTest,
		"other/other.go":   otherAfter,
	})

	v, blocks := verifyTestsWithCoverage(context.Background(), ws, []string{"./sub"}, []string{written, other})
	if !v.Ran || !v.OK || v.Result == nil {
		t.Fatalf("coverage run Ran=%v OK=%v result=%v output=%q", v.Ran, v.OK, v.Result != nil, v.Output)
	}
	if len(blocks) == 0 {
		t.Fatal("coverage run produced no blocks for the tested file")
	}
	for _, b := range blocks {
		if strings.Contains(b.File, "other.go") {
			t.Fatalf("profile mentions %s; ./sub must not measure the other package", b.File)
		}
		if strings.Contains(b.File, `\`) {
			t.Fatalf("profile path %q uses a backslash", b.File)
		}
	}

	result := writeTurnResult()
	result.WrittenPaths = []string{written, other}
	result.PreWriteContents = map[string]PreImage{
		written: existed(elemCoverBefore),
		other:   existed(otherBefore),
	}
	result.UncoveredBlocks = narrowToChangedLines(ws, result, blocks)
	result.TestCheck = v

	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws
	e.assertTurnEvidence(testTurn, "/fix", result)

	measured := factStrings(t, e, "turn_element_measured", 1)
	uncovered := factStrings(t, e, "turn_element_uncovered", 1)
	wantMeasured := []string{
		"fn:elemcover.Box.Get",
		"fn:elemcover.Half",
		"fn:elemcover.Unused",
		"fn:elemcover.Used",
	}
	wantUncovered := []string{"fn:elemcover.Box.Get", "fn:elemcover.Unused"}
	if strings.Join(measured, ",") != strings.Join(wantMeasured, ",") {
		t.Fatalf("turn_element_measured = %v, want %v", measured, wantMeasured)
	}
	if strings.Join(uncovered, ",") != strings.Join(wantUncovered, ",") {
		t.Fatalf("turn_element_uncovered = %v, want %v", uncovered, wantUncovered)
	}
	changed := factStrings(t, e, "turn_changed_element", 1)
	for _, ref := range []string{"fn:elemcover.Empty", "fn:other.Gone"} {
		if !containsString(changed, ref) {
			t.Fatalf("turn_changed_element = %v, want it to include %s", changed, ref)
		}
		if containsString(measured, ref) || containsString(uncovered, ref) {
			t.Fatalf("%s is changed but the profile has no statement block for it: measured %v uncovered %v", ref, measured, uncovered)
		}
	}
	if containsString(changed, "fn:elemcover.Old") || containsString(measured, "fn:elemcover.Old") || containsString(uncovered, "fn:elemcover.Old") {
		t.Fatalf("Old did not change: changed %v measured %v uncovered %v", changed, measured, uncovered)
	}

	if got := caseStatus(t, factsForTurn(t, e, "turn_test_case", testTurn), "TestUsed"); got != "/pass" {
		t.Fatalf("TestUsed status = %q, want /pass", got)
	}
	if n := len(factsForTurn(t, e, "turn_test_failure_at", testTurn)); n != 0 {
		t.Fatalf("the passing coverage run has %d failure rows, want 0", n)
	}

	e.cleanupTurnFacts()
	for _, pred := range []string{"turn_element_measured", "turn_element_uncovered", "turn_test_case", "turn_test_failure_at"} {
		if n := queryCount(t, e, pred); n != 0 {
			t.Errorf("%s = %d after cleanup, want 0", pred, n)
		}
	}
}

// The failing test's full output stays out of the summary. With no working
// loop the summary is unchanged, because recall_context has nothing to read.
// Inside the loop the FAIL line names the handle, and recalling it returns
// the line the summary does not carry.
func TestFailingTestRecall_HandleNamesTheArchive(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and tests a throwaway package")
	}
	const marker = "only-in-the-full-output"
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":  "module recallprobe\n\ngo 1.21\n",
		"calc.go": "package recallprobe\n\nfunc Add(a, b int) int { return a + b }\n",
		"calc_test.go": "package recallprobe\n\nimport (\n\t\"fmt\"\n\t\"testing\"\n)\n\n" +
			"func TestAdd(t *testing.T) {\n\tfmt.Println(\"" + marker + "\")\n\tt.Fatal(\"intentional failure\")\n}\n",
	})

	plain := verifyTests(context.Background(), ws, []string{"."})
	if !plain.Ran || plain.OK || plain.Result == nil || len(plain.Result.Failures) == 0 {
		t.Fatalf("Ran=%v OK=%v failures=%d output=%q", plain.Ran, plain.OK, failureCount(plain.Result), plain.Output)
	}
	if strings.Contains(plain.Output, "recall_context") {
		t.Fatalf("a run with no working loop named a recall handle:\n%s", plain.Output)
	}
	if strings.Contains(plain.Output, marker) {
		t.Fatalf("the summary inlines the test body:\n%s", plain.Output)
	}
	f := plain.Result.Failures[0]
	if full := plain.Result.Output(f.Package, f.Test); !strings.Contains(full, marker) || !strings.Contains(full, "intentional failure") {
		t.Fatalf("Result.Output lost the test body: %q", full)
	}
	if f.File != "calc_test.go" || strings.Contains(f.File, `\`) {
		t.Fatalf("failure file = %q, want calc_test.go", f.File)
	}

	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws
	ctx := inTurnWorkingLoop(t, e)
	v := verifyTests(ctx, ws, []string{"."})
	if !v.Ran || v.OK {
		t.Fatalf("loop run Ran=%v OK=%v output=%q", v.Ran, v.OK, v.Output)
	}
	for _, want := range []string{"TestAdd", "calc_test.go", "intentional failure", "recall_context id="} {
		if !strings.Contains(v.Output, want) {
			t.Fatalf("summary missing %q:\n%s", want, v.Output)
		}
	}
	if strings.Contains(v.Output, `\`) {
		t.Fatalf("summary path uses a backslash:\n%s", v.Output)
	}
	if strings.Contains(v.Output, marker) {
		t.Fatalf("the summary inlines the archived body:\n%s", v.Output)
	}
	ids := recallIDs(t, v.Output)
	if len(ids) != 1 {
		t.Fatalf("recall ids = %v, want one", ids)
	}
	recalled, err := tools.ContextRecallFrom(ctx).Recall(ctx, ids[0], 0, 0)
	if err != nil {
		t.Fatalf("recall %s: %v", ids[0], err)
	}
	if !strings.Contains(recalled, marker) || !strings.Contains(recalled, "intentional failure") {
		t.Fatalf("recall of %s did not return the test output: %s", ids[0], recalled)
	}
}

// Two failing tests archive two bodies. The summary names each handle and
// does not carry either body. A second pass does not append the handle again.
// Without a loop the summary stays the Result's own rendering.
func TestFailingTestRecall_TwoBodiesStayDistinct(t *testing.T) {
	res := &testfacts.Result{
		Status: testfacts.StatusFail,
		Packages: []*testfacts.Package{{
			Name:   "p",
			Status: testfacts.StatusFail,
			Tests: []*testfacts.Test{
				{Name: "TestA", Status: testfacts.StatusFail, Output: []string{"a-body-line"}},
				{Name: "TestB", Status: testfacts.StatusFail, Output: []string{"b-body-line"}},
			},
		}},
		Failures: []testfacts.Failure{
			{Package: "p", Test: "TestA", File: "a_test.go", Line: 4, Message: "a broke", Count: 1},
			{Package: "p", Test: "TestB", File: "b_test.go", Line: 8, Message: "b broke", Count: 1},
		},
	}
	rendered := verificationOutput(res)
	bare := TestVerification{Ran: true, OK: false, Output: rendered, Outcome: VerifyFailed, Result: res}
	annotateFailingTestRecall(context.Background(), &bare)
	if bare.Output != rendered {
		t.Fatalf("no working loop changed the summary:\n%q\nvs\n%q", bare.Output, rendered)
	}

	e := newObligationExec(t)
	ctx := inTurnWorkingLoop(t, e)
	v := TestVerification{Ran: true, OK: false, Output: rendered, Outcome: VerifyFailed, Result: res}
	annotateFailingTestRecall(ctx, &v)
	if strings.Contains(v.Output, "a-body-line") || strings.Contains(v.Output, "b-body-line") {
		t.Fatalf("the summary inlines a body:\n%s", v.Output)
	}
	ids := recallIDs(t, v.Output)
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("recall ids = %v, want two distinct ids\n%s", ids, v.Output)
	}
	recall := tools.ContextRecallFrom(ctx)
	if recall == nil {
		t.Fatal("the working loop installed no recall_context reader")
	}
	var sawA, sawB bool
	for _, id := range ids {
		body, err := recall.Recall(ctx, id, 0, 0)
		if err != nil {
			t.Fatalf("recall %s: %v", id, err)
		}
		switch {
		case strings.Contains(body, "a-body-line") && !strings.Contains(body, "b-body-line"):
			sawA = true
		case strings.Contains(body, "b-body-line") && !strings.Contains(body, "a-body-line"):
			sawB = true
		default:
			t.Fatalf("recall %s mixed or dropped the bodies: %s", id, body)
		}
	}
	if !sawA || !sawB {
		t.Fatalf("archived bodies incomplete: a=%v b=%v", sawA, sawB)
	}
	once := v.Output
	annotateFailingTestRecall(ctx, &v)
	if v.Output != once {
		t.Fatalf("annotating twice changed the summary:\n%s", v.Output)
	}
}

// A panic message spans lines. The handle still has to land on the FAIL
// line the model reads, and recalling it has to return the stack the
// summary does not carry in full.
func TestFailingTestRecall_MultilineMessageNamesTheHandle(t *testing.T) {
	const stack = "goroutine 1 [running]:"
	res := &testfacts.Result{
		Status: testfacts.StatusFail,
		Packages: []*testfacts.Package{{
			Name:   "p",
			Status: testfacts.StatusFail,
			Tests: []*testfacts.Test{{
				Name:   "TestPanic",
				Status: testfacts.StatusFail,
				Output: []string{"panic: boom", stack, "created by testing.(*T).FailNow"},
			}},
		}},
		Failures: []testfacts.Failure{{
			Package: "p", Test: "TestPanic", File: "a_test.go", Line: 9,
			Message: "panic: boom\n" + stack, Count: 1,
		}},
	}
	rendered := verificationOutput(res)
	if !strings.Contains(rendered, "\n"+stack) {
		t.Fatalf("the fixture summary is not multi-line:\n%s", rendered)
	}
	e := newObligationExec(t)
	ctx := inTurnWorkingLoop(t, e)
	v := TestVerification{Ran: true, OK: false, Output: rendered, Outcome: VerifyFailed, Result: res}
	annotateFailingTestRecall(ctx, &v)
	ids := recallIDs(t, v.Output)
	if len(ids) != 1 {
		t.Fatalf("recall ids = %v, want one on the FAIL line\n%s", ids, v.Output)
	}
	first := strings.Split(v.Output, "\n")[0]
	if !strings.Contains(first, "TestPanic") || !strings.Contains(first, ids[0]) {
		t.Fatalf("the handle is not on the FAIL line %q", first)
	}
	body, err := tools.ContextRecallFrom(ctx).Recall(ctx, ids[0], 0, 0)
	if err != nil {
		t.Fatalf("recall %s: %v", ids[0], err)
	}
	if !strings.Contains(body, "created by testing.(*T).FailNow") {
		t.Fatalf("recall did not return the full output: %s", body)
	}
	once := v.Output
	annotateFailingTestRecall(ctx, &v)
	if v.Output != once {
		t.Fatalf("annotating twice changed the summary:\n%s", v.Output)
	}
}

func TestTurnTestFacts_ModelCannotAssert(t *testing.T) {
	updates := []string{
		`turn_test_failure_at(/turn_test, "p", "TestA", "a_test.go", 4, "from-a", 1).`,
		`turn_element_measured(/turn_test, "fn:elemcover.Used").`,
		`turn_test_case(/turn_test, "p", "TestA", /fail, 0).`,
		`turn_failing_test(/turn_test, "TestA", "from-a").`,
	}
	permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
	for _, update := range updates {
		if kept, _ := core.FilterMangleUpdates(nil, []string{update}, permissive); len(kept) != 0 {
			t.Errorf("the model can assert %s", update)
		}
		if kept, _ := core.FilterMangleUpdates(nil, []string{update}, core.ModelObservationPolicy()); len(kept) != 0 {
			t.Errorf("the model can assert %s through the observation policy", update)
		}
	}
}

func factsForTurn(t *testing.T, e *Executor, predicate string, turn types.MangleAtom) []types.Fact {
	t.Helper()
	facts, err := e.kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	if turn == "" {
		return facts
	}
	var out []types.Fact
	for _, f := range facts {
		if len(f.Args) == 0 {
			t.Fatalf("%s fact has no arguments", predicate)
		}
		if types.ExtractString(f.Args[0]) == string(turn) {
			out = append(out, f)
		}
	}
	return out
}

func factArg(t *testing.T, f types.Fact, i int) string {
	t.Helper()
	if len(f.Args) <= i {
		t.Fatalf("%s%v: want an argument at %d", f.Predicate, f.Args, i)
	}
	return types.ExtractString(f.Args[i])
}

func failureForTest(t *testing.T, rows []types.Fact, name string) types.Fact {
	t.Helper()
	var found []types.Fact
	for _, f := range rows {
		if factArg(t, f, 2) == name {
			found = append(found, f)
		}
	}
	if len(found) != 1 {
		t.Fatalf("turn_test_failure_at for %s = %d, want 1 (%s)", name, len(found), describeFacts(rows))
	}
	return found[0]
}

func caseStatus(t *testing.T, rows []types.Fact, name string) string {
	t.Helper()
	for _, f := range rows {
		if factArg(t, f, 2) == name {
			return factArg(t, f, 3)
		}
	}
	t.Fatalf("no turn_test_case for %s (%s)", name, describeFacts(rows))
	return ""
}

func rowsMention(rows []types.Fact, needle string) bool {
	for _, f := range rows {
		for _, a := range f.Args {
			if strings.Contains(types.ExtractString(a), needle) {
				return true
			}
		}
	}
	return false
}

func describeFacts(rows []types.Fact) string {
	parts := make([]string, 0, len(rows))
	for _, f := range rows {
		var args []string
		for _, a := range f.Args {
			args = append(args, types.ExtractString(a))
		}
		parts = append(parts, f.Predicate+"("+strings.Join(args, ", ")+")")
	}
	return strings.Join(parts, "; ")
}

func failureCount(res *testfacts.Result) int {
	if res == nil {
		return 0
	}
	return len(res.Failures)
}

var recallIDPattern = regexp.MustCompile(`recall_context id="([0-9]+)"`)

func recallIDs(t *testing.T, text string) []string {
	t.Helper()
	matches := recallIDPattern.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}
