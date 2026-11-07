package testfacts

import (
	"strings"
	"testing"
)

// Synthetic stream cases for edges real runs cannot produce on demand:
// mid-line chunk splits, testify-style continuations, benchmarks,
// unknown actions, repeated verdicts, homeless output, and missing
// verdicts. Each stream is small; inputs are built, never pasted large.
func TestSyntheticStreams(t *testing.T) {
	run := func(pkg, test string) string {
		return `{"Action":"run","Package":"` + pkg + `","Test":"` + test + `"}` + "\n"
	}
	out := func(pkg, test, text string) string {
		return `{"Action":"output","Package":"` + pkg + `","Test":"` + test +
			`","Output":` + quoteForStream(text) + "}\n"
	}
	verdict := func(action, pkg, test string) string {
		return `{"Action":"` + action + `","Package":"` + pkg + `","Test":"` + test + `"}` + "\n"
	}
	cases := []struct {
		name   string
		stream string
		check  func(t *testing.T, res *Result)
	}{
		{
			name: "chunk split joins mid-line",
			stream: run("p", "T") +
				out("p", "T", "hel") + out("p", "T", "lo\nwor") + out("p", "T", "ld\n") +
				verdict("pass", "p", "T") + verdict("pass", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "T")
				if len(ct.Output) != 2 || ct.Output[0] != "hello" || ct.Output[1] != "world" {
					t.Fatalf("Output = %q", ct.Output)
				}
			},
		},
		{
			name: "indented lines continue the message",
			stream: run("p", "T") +
				out("p", "T", "    x_test.go:10: \n") +
				out("p", "T", "\tError: boom\n") +
				out("p", "T", "\tTest: T\n") +
				verdict("fail", "p", "T") + verdict("fail", "p", ""),
			check: func(t *testing.T, res *Result) {
				if len(res.Failures) != 1 {
					t.Fatalf("Failures = %+v", res.Failures)
				}
				if want := "\n\tError: boom\n\tTest: T"; res.Failures[0].Message != want {
					t.Errorf("Message = %q, want %q", res.Failures[0].Message, want)
				}
			},
		},
		{
			name: "blank line ends the message",
			stream: run("p", "T") +
				out("p", "T", "    x_test.go:10: note\n") +
				out("p", "T", "\n") +
				out("p", "T", "    stray indented\n") +
				verdict("fail", "p", "T") + verdict("fail", "p", ""),
			check: func(t *testing.T, res *Result) {
				if len(res.Failures) != 1 || res.Failures[0].Message != "note" {
					t.Fatalf("Failures = %+v", res.Failures)
				}
			},
		},
		{
			name: "bench is pass",
			stream: run("p", "BenchmarkX") +
				out("p", "BenchmarkX", "BenchmarkX 1 1 ns/op\n") +
				verdict("bench", "p", "BenchmarkX") + verdict("pass", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "BenchmarkX")
				if ct.Status != StatusPass {
					t.Fatalf("status = %q", ct.Status)
				}
			},
		},
		{
			name: "unknown action output is kept",
			stream: run("p", "T") +
				`{"Action":"frob","Package":"p","Test":"T","Output":"kept\n"}` + "\n" +
				verdict("pass", "p", "T") + verdict("pass", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "T")
				if len(ct.Output) != 1 || ct.Output[0] != "kept" {
					t.Fatalf("Output = %q", ct.Output)
				}
			},
		},
		{
			name: "last verdict wins",
			stream: run("p", "T") + verdict("fail", "p", "T") +
				run("p", "T") + verdict("pass", "p", "T") + verdict("pass", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "T")
				if ct.Status != StatusPass {
					t.Fatalf("status = %q, want pass", ct.Status)
				}
			},
		},
		{
			name:   "homeless output goes raw",
			stream: `{"Action":"output","Output":"orphan\n"}` + "\n",
			check: func(t *testing.T, res *Result) {
				if len(res.Raw) != 1 || res.Raw[0] != "orphan" {
					t.Fatalf("Raw = %q", res.Raw)
				}
			},
		},
		{
			name:   "run without verdict is unknown and factless",
			stream: run("p", "T") + verdict("fail", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "T")
				if ct.Status != StatusUnknown {
					t.Fatalf("status = %q", ct.Status)
				}
				for _, f := range res.Facts() {
					if f.Predicate == PredTestCase {
						t.Fatalf("unexpected test_case: %v", f)
					}
				}
			},
		},
		{
			name:   "skip without marker stays skip",
			stream: `{"Action":"skip","Package":"p"}` + "\n",
			check: func(t *testing.T, res *Result) {
				if res.Packages[0].Status != StatusSkip {
					t.Fatalf("status = %q", res.Packages[0].Status)
				}
			},
		},
		{
			name:   "crlf stream parses",
			stream: "{\"Action\":\"pass\",\"Package\":\"p\"}\r\n",
			check: func(t *testing.T, res *Result) {
				if len(res.Raw) != 0 || res.Packages[0].Status != StatusPass {
					t.Fatalf("Raw = %q status = %q", res.Raw, res.Packages[0].Status)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, parseString(t, "", tc.stream))
		})
	}
}

// quoteForStream renders Go text as a JSON string for synthetic events.
func quoteForStream(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Stabilize: pure-unit pins for edges the toolchain tests never hit.
// All synthetic, no go toolchain, deterministic.

// decodeLine splits JSON events from raw lines; splitLines never leaves a
// phantom trailing empty line but keeps interior blanks.
func TestStabilizeDecodeSplitLines(t *testing.T) {
	ev, ok := decodeLine(`{"Action":"pass","Package":"p"}`)
	if !ok || ev.Action != "pass" || ev.Package != "p" {
		t.Fatalf("decodeLine = %+v %v", ev, ok)
	}
	if _, ok := decodeLine("not json"); ok {
		t.Fatalf("decodeLine(non-JSON) = true")
	}
	if _, ok := decodeLine(""); ok {
		t.Fatalf("decodeLine(empty) = true")
	}
	if got := splitLines(""); got != nil {
		t.Fatalf("splitLines(empty) = %q", got)
	}
	if got := splitLines("a\n"); len(got) != 1 || got[0] != "a" {
		t.Fatalf("splitLines(a newline) = %q", got)
	}
	if got := splitLines("a\nb"); len(got) != 2 || got[1] != "b" {
		t.Fatalf("splitLines(no trailing) = %q", got)
	}
	if got := splitLines("a\n\n"); len(got) != 2 || got[0] != "a" || got[1] != "" {
		t.Fatalf("splitLines(blank kept) = %q", got)
	}
}

// failureCount floors hand-built zero counts to one; statusAtom has no name
// for unknown verdicts; elapsedMs rounds test2json seconds to whole ms.
func TestStabilizeFailureCountStatusAtomElapsed(t *testing.T) {
	if failureCount(0) != 1 || failureCount(-3) != 1 {
		t.Fatalf("failureCount floors = %d %d", failureCount(0), failureCount(-3))
	}
	if failureCount(1) != 1 || failureCount(7) != 7 {
		t.Fatalf("failureCount passthrough")
	}
	for _, s := range []Status{StatusPass, StatusFail, StatusSkip} {
		if _, ok := statusAtom(s); !ok {
			t.Fatalf("statusAtom(%q) = false", s)
		}
	}
	for _, s := range []Status{StatusUnknown, StatusBuildFailed, StatusNoTestFiles, "bogus"} {
		if _, ok := statusAtom(s); ok {
			t.Fatalf("statusAtom(%q) = true", s)
		}
	}
	if got := elapsedMs(0); got != 0 {
		t.Fatalf("elapsedMs(0) = %d", got)
	}
	if got := elapsedMs(1.2345); got != 1235 {
		t.Fatalf("elapsedMs(1.2345) = %d", got)
	}
	if got := elapsedMs(2); got != 2000 {
		t.Fatalf("elapsedMs(2) = %d", got)
	}
}

// firstFailures keeps the first row per package+test; failingTests adds one
// package row for a build with no test rows, defaults an empty diagnostic
// to "build failed", and never duplicates a package already represented.
func TestStabilizeFirstFailuresAndFailingTests(t *testing.T) {
	in := []Failure{
		{Package: "p", Test: "T", Message: "first"},
		{Package: "p", Test: "T", Message: "second"},
		{Package: "p", Test: "U", Message: "other"},
	}
	got := firstFailures(in)
	if len(got) != 2 || got[0].Message != "first" || got[1].Test != "U" {
		t.Fatalf("firstFailures = %+v", got)
	}
	empty := (&Result{BuildFailures: []BuildFailure{{Package: "p"}}}).failingTests()
	if len(empty) != 1 || empty[0].Message != "build failed" || empty[0].Test != "" {
		t.Fatalf("empty build message = %+v", empty)
	}
	kept := (&Result{BuildFailures: []BuildFailure{{Package: "p", Message: "boom"}}}).failingTests()
	if len(kept) != 1 || kept[0].Message != "boom" {
		t.Fatalf("build message = %+v", kept)
	}
	dup := (&Result{
		Failures:      []Failure{{Package: "p", Test: "T", Message: "m"}},
		BuildFailures: []BuildFailure{{Package: "p", Message: "b"}},
	}).failingTests()
	if len(dup) != 1 {
		t.Fatalf("duplicate package rows = %+v", dup)
	}
	pkgRow := (&Result{Packages: []*Package{{Name: "q", Status: StatusBuildFailed}}}).failingTests()
	if len(pkgRow) != 1 || pkgRow[0].Package != "q" || pkgRow[0].Message != "build failed" {
		t.Fatalf("build-failed package row = %+v", pkgRow)
	}
	both := (&Result{
		BuildFailures: []BuildFailure{{Package: "q", Message: "diag"}},
		Packages:      []*Package{{Name: "q", Status: StatusBuildFailed}},
	}).failingTests()
	if len(both) != 1 {
		t.Fatalf("build diag plus package double-counts = %+v", both)
	}
}

// failLine spells all four identity shapes; spliceAnnotation lands on the
// first physical line because messages print raw with newlines.
func TestStabilizeFailLineSplice(t *testing.T) {
	cases := []struct {
		in   Failure
		want string
	}{
		{Failure{Package: "p", Message: "m"}, "FAIL p: m\n"},
		{Failure{Package: "p", File: "f.go", Line: 3, Message: "m"}, "FAIL p f.go:3: m\n"},
		{Failure{Package: "p", Test: "T", Message: "m"}, "FAIL p T: m\n"},
		{Failure{Package: "p", Test: "T", File: "f.go", Line: 3, Message: "m"}, "FAIL p T f.go:3: m\n"},
	}
	for _, tc := range cases {
		if got := failLine(tc.in); got != tc.want {
			t.Errorf("failLine(%+v) = %q want %q", tc.in, got, tc.want)
		}
	}
	if got := spliceAnnotation("FAIL p T f:1: msg\nsecond\n", "ANN"); got != "FAIL p T f:1: msg ANN\nsecond\n" {
		t.Fatalf("splice with newline = %q", got)
	}
	if got := spliceAnnotation("abc", "ANN"); got != "abc ANN" {
		t.Fatalf("splice without newline = %q", got)
	}
}

// tally degrades to the empty sentence with no packages and no raw lines;
// joinCounts renders verdicts in tally order with none for empty;
// joinOutput rejoins with a trailing newline and empty for none.
func TestStabilizeTallyJoinCountsJoinOutput(t *testing.T) {
	if got := (&Result{}).tally(); got != "empty result: no events parsed" {
		t.Fatalf("empty tally = %q", got)
	}
	if got := (&Result{Raw: []string{"a"}}).tally(); got != "raw lines: 1" {
		t.Fatalf("raw-only tally = %q", got)
	}
	r := &Result{
		Packages: []*Package{{Status: StatusPass, Tests: []*Test{{Status: StatusPass}}}},
		Raw:      []string{"x"},
	}
	if got := r.tally(); got != "packages: 1 (pass 1); tests: 1 (pass 1); raw lines: 1" {
		t.Fatalf("tally = %q", got)
	}
	if got := joinCounts(map[Status]int{}); got != "none" {
		t.Fatalf("joinCounts(empty) = %q", got)
	}
	if got := joinCounts(map[Status]int{StatusFail: 2, StatusPass: 1}); got != "pass 1, fail 2" {
		t.Fatalf("joinCounts(order) = %q", got)
	}
	if got := joinOutput(nil); got != "" {
		t.Fatalf("joinOutput(nil) = %q", got)
	}
	if got := joinOutput([]string{"a", "b"}); got != "a\nb\n" {
		t.Fatalf("joinOutput = %q", got)
	}
}

// severity orders build-failed above fail above unknown above pass above
// skip above no-test-files; unknown strings sort as unknown.
func TestStabilizeSeverityOverallStatus(t *testing.T) {
	if !(severity(StatusBuildFailed) > severity(StatusFail) &&
		severity(StatusFail) > severity(StatusUnknown) &&
		severity(StatusUnknown) > severity(StatusPass) &&
		severity(StatusPass) > severity(StatusSkip) &&
		severity(StatusSkip) > severity(StatusNoTestFiles)) {
		t.Fatalf("severity order broken")
	}
	if severity(Status("bogus")) != severity(StatusUnknown) {
		t.Fatalf("unknown severity defaults")
	}
	if got := overallStatus(nil); got != StatusUnknown {
		t.Fatalf("overall empty = %q", got)
	}
	if got := overallStatus([]*Package{{Status: StatusPass}, {Status: StatusFail}}); got != StatusFail {
		t.Fatalf("overall pass+fail = %q", got)
	}
	if got := overallStatus([]*Package{{Status: StatusFail}, {Status: StatusBuildFailed}}); got != StatusBuildFailed {
		t.Fatalf("overall fail+build = %q", got)
	}
	if got := overallStatus([]*Package{{Status: StatusNoTestFiles}, {Status: StatusSkip}}); got != StatusSkip {
		t.Fatalf("overall skip wins = %q", got)
	}
}

// stripVariant drops the test-binary suffix; buildLinesFor merges both
// spellings deterministically; finalPkgStatus resolves FailedBuild,
// build-fail shadow, and the no-test-files marker.
func TestStabilizeStripVariantBuildLinesFinalPkg(t *testing.T) {
	if stripVariant("a [a.test]") != "a" || stripVariant("a") != "a" {
		t.Fatalf("stripVariant")
	}
	bl := map[string][]string{"b": {"l1"}, "a [a.test]": {"l2"}, "a": {"l0"}}
	if got := buildLinesFor("a", bl); len(got) != 2 || got[0] != "l0" || got[1] != "l2" {
		t.Fatalf("buildLinesFor = %q", got)
	}
	a := &accumulator{}
	if got := a.finalPkgStatus(&pkgAcc{name: "p", status: StatusFail, failedBld: true}, nil); got != StatusBuildFailed {
		t.Fatalf("failedBld = %q", got)
	}
	a2 := &accumulator{buildFailed: map[string]bool{"p": true}}
	if got := a2.finalPkgStatus(&pkgAcc{name: "p", status: StatusFail}, nil); got != StatusBuildFailed {
		t.Fatalf("buildFailed shadow = %q", got)
	}
	a3 := &accumulator{buildFailed: map[string]bool{}}
	if got := a3.finalPkgStatus(&pkgAcc{name: "p", status: StatusSkip}, []string{"x [no test files] y"}); got != StatusNoTestFiles {
		t.Fatalf("skip marker = %q", got)
	}
	if got := a3.finalPkgStatus(&pkgAcc{name: "p", status: StatusSkip}, []string{"hello"}); got != StatusSkip {
		t.Fatalf("skip stays = %q", got)
	}
}

// isContinuation needs indent and no marker; panicText only accepts a
// line-start panic marker with testing's suffix stripped; inWorkspace keeps
// workspace frames and drops stdlib and module-cache frames.
func TestStabilizeContinuationPanicWorkspace(t *testing.T) {
	if isContinuation("") || isContinuation("x") {
		t.Fatalf("empty/unindented continues")
	}
	if isContinuation("    --- FAIL: T") || isContinuation("    === RUN T") {
		t.Fatalf("marker continues")
	}
	if !isContinuation("    detail") || !isContinuation("\tindented") {
		t.Fatalf("indented detail does not continue")
	}
	if msg, ok := panicText("panic: boom"); !ok || msg != "boom" {
		t.Fatalf("panicText = %q %v", msg, ok)
	}
	if msg, ok := panicText("panic: boom [recovered, repanicked]"); !ok || msg != "boom" {
		t.Fatalf("repanicked suffix = %q %v", msg, ok)
	}
	if _, ok := panicText("    panic: boom"); ok {
		t.Fatalf("indented panic accepted")
	}
	if _, ok := panicText("a_test.go:5: saw panic: x"); ok {
		t.Fatalf("panic substring accepted")
	}
	if msg, ok := panicText("panic: boom\r"); !ok || msg != "boom" {
		t.Fatalf("panic CR = %q %v", msg, ok)
	}
	if !inWorkspace("sub/x_test.go") || inWorkspace("/pkg/mod/foo.go") {
		t.Fatalf("workspace/mod split")
	}
	if inWorkspace("a/src/testing/foo.go") || inWorkspace("a/src/runtime/foo.go") {
		t.Fatalf("stdlib frame kept")
	}
}

// dedupe collapses repeats with counts; countRepeats skips blanks and
// singletons, sorting by count then line; parseBuildLines keeps only
// compiler diagnostics.
func TestStabilizeDedupeRepeatsBuildParse(t *testing.T) {
	if got := dedupeFailures(nil); len(got) != 0 {
		t.Fatalf("dedupe empty = %+v", got)
	}
	d := dedupeFailures([]Failure{
		{Package: "p", Test: "T", File: "f", Line: 1, Message: "m"},
		{Package: "p", Test: "T", File: "f", Line: 1, Message: "m"},
		{Package: "p", Test: "U", File: "f", Line: 1, Message: "m"},
	})
	if len(d) != 2 || d[0].Count != 2 || d[1].Count != 1 {
		t.Fatalf("dedupe = %+v", d)
	}
	pkgs := []*Package{{
		Name:   "p",
		Output: []string{"", "  ", "x", "x", "b", "b", "a", "a"},
		Tests:  []*Test{{Name: "T", Output: []string{"x"}}},
	}}
	rep := countRepeats(pkgs, []string{"x"})
	if len(rep) != 3 {
		t.Fatalf("repeats = %+v", rep)
	}
	if rep[0].Line != "x" || rep[0].Count != 4 {
		t.Fatalf("top repeat = %+v", rep[0])
	}
	if rep[1].Line != "a" || rep[2].Line != "b" {
		t.Fatalf("tie sort = %+v", rep)
	}
	if rep[0].FirstPackage != "p" || rep[0].FirstTest != "" {
		t.Fatalf("first holder = %+v", rep[0])
	}
	bf := parseBuildLines(locator{}, "p", []string{"# pkg [pkg.test]", "./a_test.go:5:33: undefined: x", "noise"})
	if len(bf) != 1 || bf[0].File != "a_test.go" || bf[0].Line != 5 || bf[0].Column != 33 || bf[0].Message != "undefined: x" {
		t.Fatalf("parseBuildLines = %+v", bf)
	}
}

// parseModuleLine covers quoted, parenthesised, commented, and missing
// module lines; stripGoComment cuts at //; absSlash matches POSIX and
// Windows drive forms.
func TestStabilizePathPure(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"module example.com/mod\n", "example.com/mod"},
		{"module \"example.com/quoted\"\n", "example.com/quoted"},
		{"module (\n example.com/paren\n)\n", "example.com/paren"},
		{"// lead\nmodule example.com/mod // trail\n", "example.com/mod"},
		{"package foo\n", ""},
		{"", ""},
		{"module\n", ""},
		{"module (\n)\n", ""},
	}
	for _, tc := range cases {
		if got := parseModuleLine([]byte(tc.in)); got != tc.want {
			t.Errorf("parseModuleLine(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
	if got := stripGoComment("a // b"); got != "a " {
		t.Fatalf("stripGoComment = %q", got)
	}
	if got := stripGoComment("no comment"); got != "no comment" {
		t.Fatalf("stripGoComment passthrough = %q", got)
	}
	for _, p := range []string{"/a", "C:/a", "C:/"} {
		if !absSlash(p) {
			t.Errorf("absSlash(%q) = false", p)
		}
	}
	for _, p := range []string{"", "rel/a", "C:a"} {
		if absSlash(p) {
			t.Errorf("absSlash(%q) = true", p)
		}
	}
	if _, _, ok := readModule(""); ok {
		t.Fatalf("readModule(empty) ok")
	}
	l := locator{}
	if got := l.file("p", "", true); got != "" {
		t.Fatalf("empty file = %q", got)
	}
	if got := l.file("p", "x_test.go", true); got != "x_test.go" {
		t.Fatalf("basename unanchored = %q", got)
	}
	if got := l.file("p", "./a_test.go", false); got != "a_test.go" {
		t.Fatalf("dot slash = %q", got)
	}
	if _, ok := l.packageDir(""); ok {
		t.Fatalf("packageDir(empty) ok")
	}
	full := locator{workspace: "/w", moduleRoot: "/w", modulePath: "example.com/mod"}
	if _, ok := full.packageDir("other.com/x"); ok {
		t.Fatalf("dependency has package dir")
	}
}

// Verdict, start, and unknown-action edges that never occur in passing
// runs: nameless verdicts drop, starts without a package drop, unknown
// actions without output drop, FailedBuild resolves to build-failed, and a
// lone build-fail still yields its dependency package.
func TestStabilizeSyntheticVerdictEdges(t *testing.T) {
	mustParse := func(stream string) *Result {
		t.Helper()
		res, err := Parse("", strings.NewReader(stream))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		return res
	}
	if res := mustParse("{\"Action\":\"pass\"}\n"); len(res.Packages) != 0 {
		t.Fatalf("nameless verdict packages = %+v", res.Packages)
	}
	if res := mustParse("{\"Action\":\"start\"}\n"); len(res.Packages) != 0 {
		t.Fatalf("packageless start packages = %+v", res.Packages)
	}
	if res := mustParse("{\"Action\":\"frob\",\"Package\":\"p\"}\n"); len(res.Packages) != 0 {
		t.Fatalf("outputless unknown packages = %+v", res.Packages)
	}
	if res := mustParse("{\"Action\":\"run\"}\n"); len(res.Raw) != 1 {
		t.Fatalf("homeless run Raw = %q", res.Raw)
	}
	fb := mustParse("{\"Action\":\"fail\",\"Package\":\"p\",\"FailedBuild\":\"example.com/mod\"}\n")
	if len(fb.Packages) != 1 || fb.Packages[0].Status != StatusBuildFailed {
		t.Fatalf("FailedBuild status = %+v", fb.Packages)
	}
	bf := mustParse("{\"Action\":\"build-fail\",\"ImportPath\":\"example.com/mod\"}\n")
	if len(bf.Packages) != 1 || bf.Packages[0].Name != "example.com/mod" || bf.Packages[0].Status != StatusBuildFailed {
		t.Fatalf("lone build-fail packages = %+v", bf.Packages)
	}
}

// A failing test with only runner markers has a failed verdict but no
// diagnostic row: test_case survives, failing_test stays dark by design.
func TestStabilizeStatusOnlyFailureHasNoDiagnostic(t *testing.T) {
	stream := `{"Action":"run","Package":"p","Test":"T"}` + "\n" +
		`{"Action":"output","Package":"p","Test":"T","Output":"hello\n"}` + "\n" +
		`{"Action":"fail","Package":"p","Test":"T"}` + "\n" +
		`{"Action":"fail","Package":"p"}` + "\n"
	res, err := Parse("", strings.NewReader(stream))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Failures) != 0 {
		t.Fatalf("Failures = %+v, want none", res.Failures)
	}
	hasCase, hasFailing := false, false
	for _, f := range res.Facts() {
		if f.Predicate == PredTestCase {
			hasCase = true
		}
		if f.Predicate == PredFailingTest {
			hasFailing = true
		}
	}
	if !hasCase || hasFailing {
		t.Fatalf("case=%v failing=%v, want true/false", hasCase, hasFailing)
	}
}

// A panic outside the workspace still records its message with an empty
// file; package output without a panic yields no rows.
func TestStabilizePanicWithoutFrameAndPackagePanicEmpty(t *testing.T) {
	loc := locator{}
	f, ok := panicFailure(loc, "p", &Test{Output: []string{"panic: boom", "/pkg/mod/foo.go:10 bar"}})
	if !ok || f.Message != "boom" || f.File != "" || f.Line != 0 {
		t.Fatalf("frameless panic = %+v %v", f, ok)
	}
	if got := packagePanic(loc, &Package{Output: []string{"hello"}}); len(got) != 0 {
		t.Fatalf("panicless package = %+v", got)
	}
}
