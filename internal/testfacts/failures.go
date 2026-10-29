package testfacts

import (
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// testFailureRe matches one `file.go:NN: message` line as printed by
// t.Log/t.Errorf/t.Fatalf (observed: "    all_test.go:7: wrong value").
// The file group is greedy up to ".go" so paths with spaces and colons
// (Windows drive letters) still match.
var testFailureRe = regexp.MustCompile(`^(.+\.go):(\d+):(.*)$`)

// markerRe matches Go's own result markers, which never continue a
// failure message even when indented (a parent test's output holds
// "    --- FAIL: TestSub/case_two" lines for its children).
var markerRe = regexp.MustCompile(`^\s*(--- (PASS|FAIL|SKIP):|=== (RUN|PAUSE|CONT) )`)

// buildDiagRe matches compiler diagnostics (`./x_test.go:5:33: message`).
// The column is required: that is what distinguishes a compiler line from
// a test log line, and go always prints it.
var buildDiagRe = regexp.MustCompile(`^(.+\.go):(\d+):(\d+):(.*)$`)

// frameRe matches one stack-frame file line ("\t/tmp/x/all_test.go:18
// +0x25", and the same line when the directory name contains a space).
// `.+` is greedy and backtracks to the ".go" before the line number, so a
// space in the path is part of the file. `\S+` stopped at that space and
// the failure was recorded with an empty file (observed 2026-09-28).
// Frames carry no colon after the line number, so they never collide with
// testFailureRe.
var frameRe = regexp.MustCompile(`^\s*(.+\.go):(\d+)(?:\s|$)`)

// repanickedSuffix is testing's decoration, not the program's message:
// tRunner recovers a test panic and repanics with this marker appended.
const repanickedSuffix = " [recovered, repanicked]"

// extractFailures walks sorted packages and tests in order, so Failures
// need no further sort to be deterministic. A package that failed without
// a failing test (an init panic: the output is on the package, Test is
// empty) is scanned after its tests. Repeated identical lines collapse
// here; the kernel fact carries the count instead of one row per print.
func extractFailures(loc locator, pkgs []*Package) []Failure {
	var out []Failure
	for _, p := range pkgs {
		var pkgFails []Failure
		for _, t := range p.Tests {
			if t.Status != StatusFail {
				continue
			}
			pkgFails = append(pkgFails, testFailures(loc, p.Name, t)...)
		}
		if len(pkgFails) == 0 && p.Status == StatusFail {
			pkgFails = append(pkgFails, packagePanic(loc, p)...)
		}
		out = append(out, pkgFails...)
	}
	return dedupeFailures(out)
}

// testFailures scans one failing test's output for failure lines with
// their continuations, then appends a panic record when the output holds
// one. Panics come last: a panic ends the test, so nothing follows it.
func testFailures(loc locator, pkg string, t *Test) []Failure {
	var out []Failure
	var cur *Failure
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, line := range t.Output {
		trimmed := strings.TrimLeft(line, " \t")
		// Compiler shape (file:line:col:) before the test-log shape. The
		// test-log pattern does not require a column, so `file.go:5:33: msg`
		// would otherwise store line 5 and message "33: msg". buildDiagRe
		// is also what parseBuildLines uses; a column-bearing line is
		// relative to the run directory, not a package basename.
		if m := buildDiagRe.FindStringSubmatch(trimmed); m != nil {
			flush()
			n, _ := strconv.Atoi(m[2])
			cur = &Failure{
				Package: pkg,
				Test:    t.Name,
				File:    loc.file(pkg, m[1], false),
				Line:    n,
				Message: strings.TrimPrefix(m[4], " "),
			}
			continue
		}
		if m := testFailureRe.FindStringSubmatch(trimmed); m != nil {
			flush()
			n, _ := strconv.Atoi(m[2])
			cur = &Failure{
				Package: pkg,
				Test:    t.Name,
				File:    loc.file(pkg, m[1], true),
				Line:    n,
				Message: strings.TrimPrefix(m[3], " "),
			}
			continue
		}
		if cur != nil && isContinuation(line) {
			cur.Message += "\n" + line
			continue
		}
		flush()
	}
	flush()
	if f, ok := panicFailure(loc, pkg, t); ok {
		out = append(out, f)
	}
	return out
}

// packagePanic records an init (or other package-level) panic. The runner
// puts that output on the package with an empty Test and never starts a
// test, so scanning only StatusFail tests drops it and failing_test stays
// dark. Test is empty: no test ran. The frame is still a file.
func packagePanic(loc locator, p *Package) []Failure {
	f, ok := panicFailure(loc, p.Name, &Test{Output: p.Output})
	if !ok {
		return nil
	}
	return []Failure{f}
}

// isContinuation reports whether a non-failure line continues the open
// message: indented (testify-style nested detail lines are), and not one
// of Go's own markers.
func isContinuation(line string) bool {
	if line == "" || (line[0] != ' ' && line[0] != '\t') {
		return false
	}
	return !markerRe.MatchString(line)
}

// panicFailure builds the Failure for output that holds a panic the
// runtime printed. The testing harness writes that as a line whose first
// bytes are "panic:" (observed: "panic: kaboom panic [recovered,
// repanicked]", and "panic: init boom" with no suffix on an init panic).
// A t.Errorf whose text contains the substring is a different line
// (`file_test.go:5: saw panic: in the text`) and must not append a second
// failure. The frame is the first stack frame in the workspace.
func panicFailure(loc locator, pkg string, t *Test) (Failure, bool) {
	f := Failure{Package: pkg, Test: t.Name}
	panicked := false
	for _, line := range t.Output {
		if !panicked {
			msg, ok := panicText(line)
			if !ok {
				continue
			}
			f.Message = msg
			panicked = true
			continue
		}
		m := frameRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		frame := strings.ReplaceAll(m[1], `\`, "/")
		if !inWorkspace(frame) {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		f.File, f.Line = loc.file(pkg, frame, false), n
		break
	}
	if !panicked {
		return Failure{}, false
	}
	// No workspace frame still records the message with zero File/Line:
	// the absence of a frame is explicit, the message is not lost.
	return f, true
}

// panicText reports the message of a line the runtime printed as a panic.
// The line starts with "panic:"; leading indentation is a logged line, not
// that marker. The "[recovered, repanicked]" suffix is testing's, not the
// program's.
func panicText(line string) (string, bool) {
	s := strings.TrimRight(line, "\r")
	if !strings.HasPrefix(s, "panic:") {
		return "", false
	}
	msg := strings.TrimSpace(s[len("panic:"):])
	msg = strings.TrimSuffix(msg, repanickedSuffix)
	return strings.TrimSpace(msg), true
}

// inWorkspace reports whether a stack-frame path belongs to the code
// under test rather than the toolchain that ran it. Both sides are
// slash-spelled first: a Windows frame may carry backslashes while GOROOT
// itself does, and the slash-form checks below would miss either.
func inWorkspace(path string) bool {
	path = strings.ReplaceAll(path, `\`, "/")
	if strings.Contains(path, "/pkg/mod/") {
		return false
	}
	if goroots := strings.ReplaceAll(runtime.GOROOT(), `\`, "/"); goroots != "" && strings.HasPrefix(path, goroots+"/") {
		return false
	}
	// A toolchain GOROOT outside both of the above (a versioned toolchain
	// resolved away from the system one) still shows its stdlib frames as
	// testing/runtime paths at the top of a repanicked trace.
	if strings.Contains(path, "/src/testing/") || strings.Contains(path, "/src/runtime/") {
		return false
	}
	return true
}

// parseBuildLines extracts compiler diagnostics from one target's
// build-output lines, in line order. Headers ("# pkg [pkg.test]") and
// anything else simply yield no entry; the lines themselves stay in the
// package's Output.
func parseBuildLines(loc locator, pkg string, lines []string) []BuildFailure {
	var out []BuildFailure
	for _, line := range lines {
		m := buildDiagRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ln, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		out = append(out, BuildFailure{
			Package: pkg,
			File:    loc.file(pkg, m[1], false),
			Line:    ln,
			Column:  col,
			Message: strings.TrimPrefix(m[4], " "),
		})
	}
	return out
}

// dedupeFailures collapses repeated file:line:message rows into one, in
// first-seen order, with Count set. The key includes the package and the
// test because those are part of the fact: two tests failing at the same
// line stay two rows. A loop of one t.Errorf is one row.
func dedupeFailures(in []Failure) []Failure {
	if len(in) == 0 {
		return nil
	}
	type key struct {
		pkg, test, file, msg string
		line                 int
	}
	idx := make(map[key]int, len(in))
	var out []Failure
	for _, f := range in {
		k := key{f.Package, f.Test, f.File, f.Message, f.Line}
		if i, seen := idx[k]; seen {
			out[i].Count++
			continue
		}
		f.Count = 1
		idx[k] = len(out)
		out = append(out, f)
	}
	return out
}

// countRepeats indexes every output line seen more than once across the
// run: package output (build lines included), test output, then raw
// lines, each in deterministic order. Blank lines are skipped: they
// repeat trivially (panic traces are full of them) and carry no signal,
// and skipping them from the index drops nothing from any Output.
func countRepeats(pkgs []*Package, raw []string) []Repeat {
	counts := make(map[string]int)
	firstPkg := make(map[string]string)
	firstTest := make(map[string]string)
	note := func(pkg, test, line string) {
		if strings.TrimSpace(line) == "" {
			return
		}
		if _, seen := counts[line]; !seen {
			firstPkg[line], firstTest[line] = pkg, test
		}
		counts[line]++
	}
	for _, p := range pkgs {
		for _, line := range p.Output {
			note(p.Name, "", line)
		}
		for _, t := range p.Tests {
			for _, line := range t.Output {
				note(p.Name, t.Name, line)
			}
		}
	}
	for _, line := range raw {
		note("", "", line)
	}
	var out []Repeat
	for line, n := range counts {
		if n < 2 {
			continue
		}
		out = append(out, Repeat{Line: line, Count: n, FirstPackage: firstPkg[line], FirstTest: firstTest[line]})
	}
	slices.SortFunc(out, func(a, b Repeat) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Line, b.Line)
	})
	return out
}
