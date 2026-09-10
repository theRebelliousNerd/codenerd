// Package testoutput reads a test runner's plain output into pass and fail
// counts.
//
// It is its own package because two subsystems need the same answer and had
// diverged on it. The campaign checkpoint runner counts passes and failures to
// decide whether a phase advanced; the chat model summarizes a tester shard's
// result for the next turn's context. The second had its own parser that stored
// every value as a string while its consumer asserted .(int), so a tester's
// result reached the model as "0 pass, 0 fail" -- and, because the metrics map
// was never nil, that summary REPLACED the real output rather than falling
// back to it.
//
// One parser, so a fix to how a runner is read reaches both.
package testoutput

import "strings"

// Counts is what a runner's output says happened.
type Counts struct {
	Passed int
	Failed int
	// Parsed reports whether anything in the output was recognizable as a test
	// result. A caller that needs to fall back to showing the raw output has
	// to be able to tell "nothing ran" from "this is not a test runner's
	// output at all", and a zero/zero pair cannot say which.
	Parsed bool

	// FailedNames are the failures the runner named, in the order it reported
	// them. Go names every one; a runner that only reports "1 failed" names
	// none, so this can be shorter than Failed and callers must not use its
	// length as the failure count.
	//
	// It exists because naming the failing tests is what makes the count
	// actionable: a prompt that says three tests are failing and cannot say
	// which has told the model to go looking.
	FailedNames []string
}

// Parse counts passed and failed tests in a runner's plain output.
//
// Two defects lived in the original and both skewed every checkpoint verdict
// toward failure:
//
//   - A Go result line fell through into the generic patterns below, so any
//     test whose name contained "failed" was counted twice. "--- FAIL:
//     TestFailedLogin" matched "--- fail" and then matched "failed" again.
//   - The generic passing branch was empty while the generic failing branch
//     incremented, so a non-Go runner reporting "12 passed, 1 failed" scored
//     zero passes and one failure.
//
// Go's own markers are unambiguous, so they win and stop the line. The generic
// heuristics are symmetric, and match "ok" and "error" as whole words: a plain
// Contains matched "ok" inside "token" and "error" inside "errorless".
func Parse(output string) Counts {
	var c Counts

	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if lower == "" {
			continue
		}

		switch {
		case strings.Contains(lower, "--- pass"):
			c.Passed++
			c.Parsed = true
			continue
		case strings.Contains(lower, "--- fail"):
			c.Failed++
			c.Parsed = true
			if name := failedTestName(trimmed, strings.Index(lower, "--- fail")); name != "" {
				c.FailedNames = append(c.FailedNames, name)
			}
			continue
		case strings.Contains(lower, "--- skip"):
			// A skipped test is neither a pass nor a failure. Counting it
			// either way misreports the suite. It is still evidence that this
			// is test output.
			c.Parsed = true
			continue
		}

		// Failure wins a tie: a line reading "1 failed, 3 passed" is a failing
		// summary, and treating it as a pass is the dangerous direction.
		switch {
		case genericFailureLine(lower):
			c.Failed++
			c.Parsed = true
		case genericPassLine(lower):
			c.Passed++
			c.Parsed = true
		}
	}

	return c
}

// failedTestName pulls the test name out of a Go failure marker.
//
// It works on the original line rather than the lowercased copy the matching
// uses, because the name is the payload and "testfailedlogin" is not a name
// anyone can run. The offset of the marker is passed in so the two views of
// the line cannot drift apart.
//
// Returns "" for anything it cannot read confidently. A wrong name is worse
// than no name: it sends the model to a test that is not failing.
func failedTestName(line string, markerAt int) string {
	if markerAt < 0 || markerAt+len("--- fail") > len(line) {
		return ""
	}
	rest := strings.TrimSpace(line[markerAt+len("--- fail"):])
	rest = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
	// Go appends a duration, "TestName (0.00s)". Subtest names keep their
	// slashes: TestParse/empty_input is the name you rerun with -run.
	if idx := strings.Index(rest, " ("); idx >= 0 {
		rest = rest[:idx]
	}
	return strings.TrimSpace(rest)
}

// ParseOptimistic is Parse with the checkpoint runner's convention that
// unreadable output counts as a single pass.
//
// Preserved deliberately and kept separate: a runner this package cannot read
// should not manufacture a failure that fails a campaign phase. But a caller
// rendering a summary wants to know it read nothing, so it can show the real
// output instead of an invented "1 pass" -- which is why Parse reports Parsed
// and this wrapper is opt-in rather than the default.
func ParseOptimistic(output string) (passed, failed int) {
	c := Parse(output)
	if !c.Parsed || (c.Passed == 0 && c.Failed == 0) {
		return 1, 0
	}
	return c.Passed, c.Failed
}

// genericPassLine reports whether a non-Go runner's line announces success.
func genericPassLine(lower string) bool {
	return strings.Contains(lower, "passed") ||
		strings.Contains(lower, "passing") ||
		containsWord(lower, "ok")
}

// genericFailureLine reports whether a non-Go runner's line announces failure.
func genericFailureLine(lower string) bool {
	return strings.Contains(lower, "failed") ||
		strings.Contains(lower, "failing") ||
		containsWord(lower, "error") ||
		containsWord(lower, "errors")
}

// containsWord reports whether word appears in s bounded by non-letters.
//
// The bound is the point: strings.Contains(s, "ok") is true of "token" and
// "broken", and every `go test` summary line names a package path.
func containsWord(s, word string) bool {
	for i := 0; ; {
		idx := strings.Index(s[i:], word)
		if idx < 0 {
			return false
		}
		start := i + idx
		end := start + len(word)
		beforeOK := start == 0 || !isASCIILetter(s[start-1])
		afterOK := end == len(s) || !isASCIILetter(s[end])
		if beforeOK && afterOK {
			return true
		}
		i = start + 1
		if i >= len(s) {
			return false
		}
	}
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
