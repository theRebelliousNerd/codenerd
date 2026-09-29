package testfacts

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

const mixSrc = `package mx

import "testing"

func TestOK(t *testing.T) {}

func TestErr(t *testing.T) { t.Errorf("kaboom") }
`

// Facts from a real mixed run: one test_case per test with a /name
// status, one test_failure_at per failure line, and one failing_test with
// the first failure's message. Every fact must survive ToAtom, which is
// what kernel insertion calls.
func TestFactsMixedRun(t *testing.T) {
	dir := writeModule(t, map[string]string{"mx_test.go": mixSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	facts := res.Facts()
	if len(facts) != 4 {
		t.Fatalf("facts = %v, want 4", facts)
	}
	assertFact(t, facts[0], PredTestCase, []any{
		types.MangleString("example.com/mod"),
		types.MangleString("TestErr"),
		types.MangleAtom("/fail"),
	})
	assertFact(t, facts[1], PredTestCase, []any{
		types.MangleString("example.com/mod"),
		types.MangleString("TestOK"),
		types.MangleAtom("/pass"),
	})
	for _, f := range facts[:2] {
		ms, ok := f.Args[3].(int64)
		if !ok || ms < 0 {
			t.Errorf("ElapsedMs = %v (%T), want non-negative int64", f.Args[3], f.Args[3])
		}
	}
	assertFact(t, facts[2], PredTestFailureAt, []any{
		types.MangleString("example.com/mod"),
		types.MangleString("TestErr"),
		types.MangleString("mx_test.go"),
		int64(7),
		types.MangleString("kaboom"),
		int64(1),
	})
	assertFact(t, facts[3], PredFailingTest, []any{
		types.MangleString("TestErr"),
		types.MangleString("kaboom"),
	})
	for _, f := range facts {
		if _, err := f.ToAtom(); err != nil {
			t.Errorf("ToAtom(%v): %v", f, err)
		}
	}
}

// assertFact checks predicate and leading args exactly.
func assertFact(t *testing.T, f types.Fact, pred string, want []any) {
	t.Helper()
	if f.Predicate != pred {
		t.Fatalf("predicate = %q, want %q (%v)", f.Predicate, pred, f)
	}
	if len(f.Args) < len(want) {
		t.Fatalf("args = %v, want at least %v", f.Args, want)
	}
	for i, w := range want {
		if f.Args[i] != w {
			t.Fatalf("args = %v, want prefix %v", f.Args, want)
		}
	}
}

// A real build failure yields the compiler diagnostic and a package-level
// failing_test. The repair rule only tests failing_test(_, _) for existence;
// the file stays on test_build_failure, which carries the package. Test is
// empty because no test ran, so the name cannot be mistaken for one that did.
func TestFactsBuildFailure(t *testing.T) {
	dir := writeModule(t, map[string]string{"bf_test.go": buildFailSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	facts := res.Facts()
	if len(facts) != 2 {
		t.Fatalf("facts = %v, want 2", facts)
	}
	assertFact(t, facts[0], PredTestBuildFailure, []any{
		types.MangleString("example.com/mod"),
		types.MangleString("bf_test.go"),
		int64(5),
		types.MangleString("undefined: undefinedSymbol"),
	})
	assertFact(t, facts[1], PredFailingTest, []any{
		types.MangleString(""),
		types.MangleString("undefined: undefinedSymbol"),
	})
	for _, f := range facts {
		if _, err := f.ToAtom(); err != nil {
			t.Errorf("ToAtom: %v", err)
		}
	}
}

// A real flood yields one test_output_repeat fact with the full count.
func TestFactsRepeat(t *testing.T) {
	dir := writeModule(t, map[string]string{"fl_test.go": floodSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	facts := res.Facts()
	found := false
	for _, f := range facts {
		if f.Predicate != PredTestOutputRepeat {
			continue
		}
		found = true
		assertFact(t, f, PredTestOutputRepeat, []any{
			types.MangleString("flood line standing by"),
			int64(10000),
		})
		if _, err := f.ToAtom(); err != nil {
			t.Errorf("ToAtom: %v", err)
		}
	}
	if !found {
		t.Fatalf("no test_output_repeat in %v", facts)
	}
}

// Exact summary of a real single-failure run: the FAIL line with
// file:line and message, then the tally. No repeats section: every line
// of this stream is unique.
func TestSummaryFailExact(t *testing.T) {
	dir := writeModule(t, map[string]string{"bad_test.go": errSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	want := "FAIL example.com/mod TestErr bad_test.go:6: wrong value: got 1 want 2\n" +
		"packages: 1 (fail 1); tests: 1 (fail 1)\n"
	if got := res.Summary(); got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

// Exact summary of a real build failure: diagnostics first, then the
// tally with its build-failed package.
func TestSummaryBuildExact(t *testing.T) {
	dir := writeModule(t, map[string]string{"bf_test.go": buildFailSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	want := "build-failed example.com/mod bf_test.go:5:33: undefined: undefinedSymbol\n" +
		"packages: 1 (build-failed 1); tests: 0 (none)\n"
	if got := res.Summary(); got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

// Output recalls one test's full output, the package's output with an
// empty test name, and "" for unknown names.
func TestOutputRecall(t *testing.T) {
	dir := writeModule(t, map[string]string{"bad_test.go": errSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	got := res.Output("example.com/mod", "TestErr")
	for _, want := range []string{"=== RUN   TestErr", "wrong value: got 1 want 2", "--- FAIL: TestErr"} {
		if !strings.Contains(got, want) {
			t.Errorf("Output missing %q:\n%s", want, got)
		}
	}
	if got := res.Output("example.com/mod", ""); !strings.Contains(got, "FAIL") {
		t.Errorf("package Output = %q, want FAIL lines", got)
	}
	if got := res.Output("example.com/mod", "Nope"); got != "" {
		t.Errorf("unknown test Output = %q, want empty", got)
	}
	if got := res.Output("example.com/other", "TestErr"); got != "" {
		t.Errorf("unknown package Output = %q, want empty", got)
	}
}

// Nil results degrade instead of panicking.
func TestNilResult(t *testing.T) {
	var res *Result
	if got := res.Summary(); got != "empty result: no events parsed\n" {
		t.Errorf("Summary = %q", got)
	}
	if facts := res.Facts(); facts != nil {
		t.Errorf("Facts = %v, want nil", facts)
	}
	if out := res.Output("p", "T"); out != "" {
		t.Errorf("Output = %q, want empty", out)
	}
}
