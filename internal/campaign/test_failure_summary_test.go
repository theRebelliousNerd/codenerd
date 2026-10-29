package campaign

import (
	"strings"
	"testing"
)

const sampleGoFailure = `=== RUN   TestOk
--- PASS: TestOk (0.00s)
=== RUN   TestLoadSchemaStringParseFailClosed
    engine_failclosed_test.go:30: Evaluate after parse failure failed: no schemas loaded
--- FAIL: TestLoadSchemaStringParseFailClosed (0.00s)
=== RUN   TestStep2ParseUnitNilReaderFailsClosed
    engine_step2_regression_test.go:23: ParseUnit(nil) panicked: nil pointer dereference
--- FAIL: TestStep2ParseUnitNilReaderFailsClosed (0.00s)
FAIL
FAIL	codenerd/internal/mangle	0.010s
`

// The summary must lead with counts and names: downstream only the head of
// the error survives (the repro task carries 220 chars of "last error"), so
// anything past the first line is context the agent may never see.
func TestFailureSummaryLeadsWithCountsAndNames(t *testing.T) {
	s := testFailureSummary("./...", "exit status 1", sampleGoFailure)
	head := s
	if i := strings.Index(s, "\n"); i >= 0 {
		head = s[:i]
	}
	if !strings.Contains(head, "2 failed") || !strings.Contains(head, "1 passed") {
		t.Fatalf("head lacks counts: %q", head)
	}
	if !strings.Contains(head, "TestLoadSchemaStringParseFailClosed") ||
		!strings.Contains(head, "TestStep2ParseUnitNilReaderFailsClosed") {
		t.Fatalf("head lacks failing names: %q", head)
	}
	if !strings.Contains(head, "engine_failclosed_test.go:30") {
		t.Fatalf("head lacks first assertion detail: %q", head)
	}
	if len(head) > 400 {
		t.Fatalf("head too long to survive truncation (%d chars): %q", len(head), head)
	}
}

// Unparseable output must still identify the run error, never an empty summary.
func TestFailureSummaryUnparseableFallsBack(t *testing.T) {
	s := testFailureSummary("./...", "signal: killed", "not test output at all {{{")
	if !strings.Contains(s, "signal: killed") {
		t.Fatalf("fallback lost the run error: %q", s)
	}
}

// Long name lists are capped; the cap is announced, not silent.
func TestFailureSummaryCapsNameList(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 12; i++ {
		sb.WriteString("--- FAIL: TestNumbered" + string(rune('A'+i)) + " (0.00s)\n")
	}
	s := testFailureSummary("./...", "exit status 1", sb.String())
	if !strings.Contains(s, "(+4 more)") {
		t.Fatalf("cap not announced: %q", s[:200])
	}
}

// The raw tail is capped so a full-suite log cannot flood task state.
func TestFailureSummaryCapsRawOutput(t *testing.T) {
	big := "--- FAIL: TestBig (0.00s)\n" + strings.Repeat("x", 20000)
	s := testFailureSummary("./...", "exit status 1", big)
	if len(s) > 6000 {
		t.Fatalf("summary too long: %d chars", len(s))
	}
	if !strings.Contains(s, "truncated") {
		t.Fatal("truncation not announced")
	}
}
