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
// +0x25"). Frames carry no colon after the line number, so they never
// collide with testFailureRe.
var frameRe = regexp.MustCompile(`^\s*(\S+\.go):(\d+)(?:\s|$)`)

// repanickedSuffix is testing's decoration, not the program's message:
// tRunner recovers a test panic and repanics with this marker appended.
const repanickedSuffix = " [recovered, repanicked]"

// slashPath spells a reported file path with forward slashes. The Windows
// compiler reports `.\bf_test.go` while panic frames on that same host
// arrive as `C:/...`, so without this one file gets two spellings and its
// facts never join in the kernel. filepath.ToSlash cannot do this job: it
// is a no-op on Linux, where a stream from a Windows toolchain still
// parses. Only separators change; a leading ./ or drive letter is kept.
func slashPath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

// extractFailures walks sorted packages and tests in order, so Failures
// need no further sort to be deterministic.
func extractFailures(pkgs []*Package) []Failure {
	var out []Failure
	for _, p := range pkgs {
		for _, t := range p.Tests {
			if t.Status != StatusFail {
				continue
			}
			out = append(out, testFailures(p.Name, t)...)
		}
	}
	return out
}

// testFailures scans one failing test's output for failure lines with
// their continuations, then appends a panic record when the output holds
// one. Panics come last: a panic ends the test, so nothing follows it.
func testFailures(pkg string, t *Test) []Failure {
	var out []Failure
	var cur *Failure
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, line := range t.Output {
		if m := testFailureRe.FindStringSubmatch(strings.TrimLeft(line, " \t")); m != nil {
			flush()
			n, _ := strconv.Atoi(m[2])
			cur = &Failure{
				Package: pkg,
				Test:    t.Name,
				File:    slashPath(m[1]),
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
	if f, ok := panicFailure(pkg, t); ok {
		out = append(out, f)
	}
	return out
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

// panicFailure builds the Failure for a test whose output holds "panic:".
// The message is the panic line's text; the frame is the first stack frame
// in the workspace, i.e. outside the module cache, the local GOROOT, and
// the testing/runtime internals a repanicked test trace always starts
// with (observed 2026-09-28: testing.go, panic.go, then the test file).
func panicFailure(pkg string, t *Test) (Failure, bool) {
	f := Failure{Package: pkg, Test: t.Name}
	panicked := false
	for _, line := range t.Output {
		if !panicked {
			if i := strings.Index(line, "panic:"); i >= 0 {
				f.Message = strings.TrimSuffix(strings.TrimSpace(line[i+len("panic:"):]), repanickedSuffix)
				panicked = true
			}
			continue
		}
		m := frameRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		frame := slashPath(m[1])
		if !inWorkspace(frame) {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		f.File, f.Line = frame, n
		break
	}
	if !panicked {
		return Failure{}, false
	}
	// No workspace frame still records the message with zero File/Line:
	// the absence of a frame is explicit, the message is not lost.
	return f, true
}

// inWorkspace reports whether a stack-frame path belongs to the code
// under test rather than the toolchain that ran it. Both sides are
// slash-spelled first: a Windows frame may carry backslashes while GOROOT
// itself does, and the slash-form checks below would miss either.
func inWorkspace(path string) bool {
	path = slashPath(path)
	if strings.Contains(path, "/pkg/mod/") {
		return false
	}
	if goroots := slashPath(runtime.GOROOT()); goroots != "" && strings.HasPrefix(path, goroots+"/") {
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
func parseBuildLines(pkg string, lines []string) []BuildFailure {
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
			File:    slashPath(m[1]),
			Line:    ln,
			Column:  col,
			Message: strings.TrimPrefix(m[4], " "),
		})
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
