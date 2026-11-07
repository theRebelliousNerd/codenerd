package testoutput

import "testing"

func TestContainsWord_ShouldRespectLetterBoundaries(t *testing.T) {
	cases := []struct {
		s, word string
		want    bool
	}{
		{"ok  github.com/x", "ok", true},
		{"token", "ok", false},
		{"broken", "ok", false},
		{"an error occurred", "error", true},
		{"errorless", "error", false},
		{"terror", "error", false},
		{"ok", "ok", true},
		{"", "ok", false},
	}
	for _, tc := range cases {
		if got := containsWord(tc.s, tc.word); got != tc.want {
			t.Errorf("containsWord(%q, %q) = %v, want %v", tc.s, tc.word, got, tc.want)
		}
	}
}

func TestParseReportsWhetherItReadAnything(t *testing.T) {
	// The distinction Counts.Parsed exists for. A caller rendering a summary
	// has to tell "nothing ran" from "this is not test output", and a zero/zero
	// pair cannot say which. Without it, a tester shard's real output was
	// replaced by "0 pass, 0 fail".
	if got := Parse(""); got.Parsed {
		t.Error("empty output was reported as parsed")
	}
	if got := Parse("I looked at the code and it seems fine to me."); got.Parsed {
		t.Errorf("prose was reported as parsed: %+v", got)
	}
	if got := Parse("--- PASS: TestThing (0.01s)"); !got.Parsed || got.Passed != 1 {
		t.Errorf("Go pass line = %+v, want one parsed pass", got)
	}
}

func TestParseCountsGoMarkers(t *testing.T) {
	out := `--- PASS: TestAlpha (0.01s)
--- FAIL: TestFailedLogin (0.00s)
--- SKIP: TestGamma (0.00s)
ok  	codenerd/internal/thing	0.02s`

	got := Parse(out)
	// "--- FAIL: TestFailedLogin" must count once, not twice: it used to match
	// the Go marker and then match "failed" again in the generic pass below.
	if got.Failed != 1 {
		t.Errorf("failed = %d, want 1 — a test named ...Failed... was double counted", got.Failed)
	}
	// A skip is neither, and the trailing "ok" summary line is a pass.
	if got.Passed != 2 {
		t.Errorf("passed = %d, want 2 (one --- PASS and the ok summary)", got.Passed)
	}
}

func TestParseIsSymmetricForNonGoRunners(t *testing.T) {
	// The generic passing branch used to be empty while the failing branch
	// incremented, so "12 passed, 1 failed" scored zero passes.
	got := Parse("12 passed, 1 failed")
	// Failure wins a tie on a single mixed line: a summary naming a failure is
	// a failing summary, and treating it as a pass is the dangerous direction.
	if got.Failed != 1 || got.Passed != 0 {
		t.Errorf("mixed summary = %+v, want the failure to win the line", got)
	}

	if got := Parse("12 passed"); got.Passed != 1 || got.Failed != 0 {
		t.Errorf("passing summary = %+v, want one pass", got)
	}
}

func TestParseDoesNotMatchSubstrings(t *testing.T) {
	// containsWord's whole reason: every `go test` summary names a package
	// path, and a plain Contains matched "ok" inside "token".
	if got := Parse("processing token stream"); got.Parsed {
		t.Errorf("a line containing 'token' was read as a pass: %+v", got)
	}
	if got := Parse("the errorless path"); got.Parsed {
		t.Errorf("a line containing 'errorless' was read as a failure: %+v", got)
	}
}

func TestParseOptimisticKeepsTheCheckpointConvention(t *testing.T) {
	// A runner this package cannot read must not manufacture a failure that
	// fails a campaign phase.
	p, f := ParseOptimistic("some unrecognizable output")
	if p != 1 || f != 0 {
		t.Errorf("unreadable output = %d pass / %d fail, want the optimistic 1/0", p, f)
	}
	// But real counts pass through untouched.
	p, f = ParseOptimistic("--- PASS: A\n--- FAIL: B")
	if p != 1 || f != 1 {
		t.Errorf("readable output = %d pass / %d fail, want 1/1", p, f)
	}
}

// Naming the failures is what makes the count actionable. A prompt that says
// three tests are failing and cannot say which has told the model to go
// looking, which costs a turn and several file reads to recover information the
// runner already printed.
func TestParseNamesGoFailures(t *testing.T) {
	out := `=== RUN   TestAlpha
--- PASS: TestAlpha (0.00s)
=== RUN   TestBeta
    beta_test.go:12: want 5, got 0
--- FAIL: TestBeta (0.01s)
=== RUN   TestGamma/empty_input
--- FAIL: TestGamma/empty_input (0.00s)
--- SKIP: TestDelta (0.00s)
FAIL`

	c := Parse(out)
	if !c.Parsed {
		t.Fatal("Parse did not recognise Go test output")
	}
	if c.Failed != 2 {
		t.Errorf("Failed = %d, want 2", c.Failed)
	}
	want := []string{"TestBeta", "TestGamma/empty_input"}
	if len(c.FailedNames) != len(want) {
		t.Fatalf("FailedNames = %v, want %v", c.FailedNames, want)
	}
	for i := range want {
		if c.FailedNames[i] != want[i] {
			// The subtest case is the one worth stating: the slash is part of
			// the name you rerun with -run, so trimming it would hand back a
			// name that runs the wrong thing.
			t.Errorf("FailedNames[%d] = %q, want %q", i, c.FailedNames[i], want[i])
		}
	}
}

// A runner that reports a count without naming anything must not produce
// invented names. Callers are told FailedNames can be shorter than Failed; a
// wrong name is worse than no name, because it sends the model to a test that
// is not failing.
func TestParseInventsNoNamesForUnnamedFailures(t *testing.T) {
	c := Parse("12 passed, 3 failed")
	if !c.Parsed {
		t.Fatal("Parse did not recognise a generic summary line")
	}
	if len(c.FailedNames) != 0 {
		t.Errorf("FailedNames = %v, want none: this runner named nothing", c.FailedNames)
	}
}

func TestContainsWord_ShouldTreatDigitsAndUnderscoreAsWordChars(t *testing.T) {
	cases := []struct {
		s, word string
		want    bool
	}{
		{"ok2", "ok", false},
		{"2ok", "ok", false},
		{"ok_test", "ok", false},
		{"_ok", "ok", false},
		{"ok!", "ok", true},
		{"(ok)", "ok", true},
		{"ok, finished", "ok", true},
		{"1 passed", "passed", true},
		{"1passing", "passing", false},
		{"error2", "error", false},
		{"2 errors", "errors", true},
		{"ok", "", false},
	}
	for _, tc := range cases {
		if got := containsWord(tc.s, tc.word); got != tc.want {
			t.Errorf("containsWord(%q, %q) = %v, want %v", tc.s, tc.word, got, tc.want)
		}
	}
}

func TestFailedTestName_ShouldRejectNonBoundaryMarkers(t *testing.T) {
	if got := failedTestName("--- FAILED: TestX", 0); got != "" {
		t.Errorf("failedTestName(%q) = %q, want empty: --- FAILED is not a --- FAIL marker", "--- FAILED: TestX", got)
	}
	if got := failedTestName("--- FAIL", 0); got != "" {
		t.Errorf("failedTestName(%q) = %q, want empty: marker names nothing", "--- FAIL", got)
	}
	if got := failedTestName("--- FAIL:   ", 0); got != "" {
		t.Errorf("failedTestName(%q) = %q, want empty: colon with no name names nothing", "--- FAIL:   ", got)
	}
	if got := failedTestName("anything", -1); got != "" {
		t.Errorf("failedTestName with markerAt -1 = %q, want empty", got)
	}
	if got := failedTestName("--- FAIL: TestX", -1); got != "" {
		t.Errorf("failedTestName with markerAt -1 = %q, want empty", got)
	}
	if got := failedTestName("short", 100); got != "" {
		t.Errorf("failedTestName with out-of-range markerAt = %q, want empty", got)
	}
}

func TestFailedTestName_ShouldTrimDurationAndKeepSubtests(t *testing.T) {
	cases := []struct {
		line     string
		markerAt int
		want     string
	}{
		{"--- FAIL: TestBeta (0.01s)", 0, "TestBeta"},
		{"--- FAIL: TestGamma/empty_input (0.00s)", 0, "TestGamma/empty_input"},
		{"--- FAIL TestSpace (0.00s)", 0, "TestSpace"},
		{"--- FAIL:\tTestTab (0.00s)", 0, "TestTab"},
		{"--- FAIL: TestFailedLogin (0.00s)", 0, "TestFailedLogin"},
		{"--- FAIL: TestMixedCase (0.00s)", 0, "TestMixedCase"},
		{"   --- FAIL: TestIndented (0.00s)", 3, "TestIndented"},
	}
	for _, tc := range cases {
		if got := failedTestName(tc.line, tc.markerAt); got != tc.want {
			t.Errorf("failedTestName(%q, %d) = %q, want %q", tc.line, tc.markerAt, got, tc.want)
		}
	}
}

func TestParseShouldNotNameMalformedFailMarkers(t *testing.T) {
	c := Parse("--- FAILED: TestX")
	if !c.Parsed {
		t.Fatal("Parse did not recognise a --- FAILED line as test output")
	}
	if c.Failed != 1 {
		t.Errorf("Failed = %d, want 1", c.Failed)
	}
	if len(c.FailedNames) != 0 {
		t.Errorf("FailedNames = %v, want none: --- FAILED must not yield a trimmed name", c.FailedNames)
	}

	c = Parse("--- FAIL")
	if !c.Parsed {
		t.Fatal("Parse did not recognise a bare --- FAIL line as test output")
	}
	if c.Failed != 1 {
		t.Errorf("Failed = %d, want 1 for a bare --- FAIL line", c.Failed)
	}
	if len(c.FailedNames) != 0 {
		t.Errorf("FailedNames = %v, want none: a bare marker names nothing", c.FailedNames)
	}

	c = Parse("--- FAIL: TestFailedLogin (0.00s)")
	if c.Failed != 1 {
		t.Errorf("Failed = %d, want 1: a name containing Failed must count once", c.Failed)
	}
	if len(c.FailedNames) != 1 || c.FailedNames[0] != "TestFailedLogin" {
		t.Errorf("FailedNames = %v, want [TestFailedLogin]", c.FailedNames)
	}
	if c.Passed != 0 {
		t.Errorf("Passed = %d, want 0: a failure line must not also count as a pass", c.Passed)
	}
}

func TestParseOptimisticShouldStayOptimisticForSkipOnlyOutput(t *testing.T) {
	c := Parse("--- SKIP: TestDelta (0.00s)")
	if !c.Parsed {
		t.Fatal("Parse did not recognise a skip-only line as test output")
	}
	if c.Passed != 0 || c.Failed != 0 {
		t.Errorf("skip-only counts = %+v, want zero passes and zero failures", c)
	}
	if len(c.FailedNames) != 0 {
		t.Errorf("FailedNames = %v, want none for skip-only output", c.FailedNames)
	}
	if p, f := ParseOptimistic("--- SKIP: TestDelta (0.00s)"); p != 1 || f != 0 {
		t.Errorf("ParseOptimistic(skip-only) = %d pass / %d fail, want 1/0", p, f)
	}
	if p, f := ParseOptimistic(""); p != 1 || f != 0 {
		t.Errorf("ParseOptimistic(empty) = %d pass / %d fail, want 1/0", p, f)
	}
}

func TestGenericLines_ShouldRecognizePassFailWords(t *testing.T) {
	if !genericPassLine("12 passing") {
		t.Error("genericPassLine did not recognise 'passing'")
	}
	if !genericPassLine("ok  pkg 0.02s") {
		t.Error("genericPassLine did not recognise 'ok'")
	}
	if genericPassLine("processing token stream") {
		t.Error("genericPassLine matched 'ok' inside 'token'")
	}
	if genericPassLine("broken pipe") {
		t.Error("genericPassLine matched 'ok' inside 'broken'")
	}
	if !genericFailureLine("1 failing") {
		t.Error("genericFailureLine did not recognise 'failing'")
	}
	if !genericFailureLine("2 errors") {
		t.Error("genericFailureLine did not recognise 'errors'")
	}
	if !genericFailureLine("an error occurred") {
		t.Error("genericFailureLine did not recognise 'error'")
	}
	if genericFailureLine("errorless") {
		t.Error("genericFailureLine matched 'error' inside 'errorless'")
	}
	if genericFailureLine("terror") {
		t.Error("genericFailureLine matched 'error' inside 'terror'")
	}
	c := Parse("1 failed, 3 passed")
	if c.Failed != 1 || c.Passed != 0 {
		t.Errorf("reversed mixed summary = %+v, want the failure to win the line", c)
	}
}

func TestWordChar_ShouldClassifyWordCharacters(t *testing.T) {
	for _, b := range []byte{'a', 'z', 'A', 'Z'} {
		if !isASCIILetter(b) {
			t.Errorf("isASCIILetter(%q) = false, want true", b)
		}
		if !isWordChar(b) {
			t.Errorf("isWordChar(%q) = false, want true", b)
		}
	}
	for _, b := range []byte{'0', '_', ' ', '!', '.', '-'} {
		if isASCIILetter(b) {
			t.Errorf("isASCIILetter(%q) = true, want false", b)
		}
	}
	for _, b := range []byte{'0', '9', '_'} {
		if !isWordChar(b) {
			t.Errorf("isWordChar(%q) = false, want true: digits and underscore are word chars", b)
		}
	}
	for _, b := range []byte{' ', '!', '.', ',', ':', '('} {
		if isWordChar(b) {
			t.Errorf("isWordChar(%q) = true, want false", b)
		}
	}
}

func TestParse_GenericPassVariantsCountAsPass(t *testing.T) {
	cases := []string{
		"12 passed",
		"3 passing",
		"ok suite",
		"OK suite",
	}
	for _, input := range cases {
		c := Parse(input)
		if !c.Parsed {
			t.Errorf("Parse(%q).Parsed = false, want true", input)
		}
		if c.Passed != 1 {
			t.Errorf("Parse(%q).Passed = %d, want 1", input, c.Passed)
		}
		if c.Failed != 0 {
			t.Errorf("Parse(%q).Failed = %d, want 0", input, c.Failed)
		}
	}
}

func TestParse_GenericFailureVariantsCountAsFail(t *testing.T) {
	cases := []string{
		"1 failed",
		"2 failing",
		"error occurred",
		"3 errors",
		"ERROR boom",
	}
	for _, input := range cases {
		c := Parse(input)
		if !c.Parsed {
			t.Errorf("Parse(%q).Parsed = false, want true", input)
		}
		if c.Failed != 1 {
			t.Errorf("Parse(%q).Failed = %d, want 1", input, c.Failed)
		}
		if c.Passed != 0 {
			t.Errorf("Parse(%q).Passed = %d, want 0", input, c.Passed)
		}
	}
}

func TestParse_GenericSubstringsDoNotCountViaCaller(t *testing.T) {
	cases := []string{
		"token refreshed",
		"broken pipe",
		"errorless run",
		"unpassed check",
	}
	for _, input := range cases {
		c := Parse(input)
		if c.Parsed {
			t.Errorf("Parse(%q).Parsed = true, want false", input)
		}
		if c.Passed != 0 || c.Failed != 0 {
			t.Errorf("Parse(%q) = %+v, want zero counts", input, c)
		}
	}
}

func TestContainsWord_EmptyWordReturnsFalse(t *testing.T) {
	for _, s := range []string{"", " ", "anything", "ok"} {
		if containsWord(s, "") {
			t.Errorf("containsWord(%q, %q) = true, want false", s, "")
		}
	}
}

func TestGenericFailureLine_PinsFailureWordsDirectly(t *testing.T) {
	mustBeFailure := []string{
		"failed",
		"failing",
		"error",
		"errors",
		"1 failed",
		"2 failing",
		"error occurred",
		"3 errors",
	}
	for _, s := range mustBeFailure {
		if !genericFailureLine(s) {
			t.Errorf("genericFailureLine(%q) = false, want true", s)
		}
	}
	mustNotBeFailure := []string{
		"",
		"fail",
		"err",
		"passed",
		"passing",
		"ok",
		"token refreshed",
		"broken pipe",
		"errorless run",
		"failedx",
		"xfailed",
		"2failing",
		"failing2",
		"failing_test",
		"error2",
		"2error",
		"error_x",
		"errorsx",
		"xerrors",
	}
	for _, s := range mustNotBeFailure {
		if genericFailureLine(s) {
			t.Errorf("genericFailureLine(%q) = true, want false", s)
		}
	}
}

func TestParse_FailureWinsTieOverPass(t *testing.T) {
	c := Parse("1 failed, 3 passed")
	if c.Failed != 1 || c.Passed != 0 {
		t.Errorf("Parse tie = %+v, want 1 failure and 0 passes", c)
	}
	if !c.Parsed {
		t.Errorf("Parse tie Parsed = false, want true")
	}
}
