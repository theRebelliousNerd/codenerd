package campaign

import "testing"

func TestParseTestOutput(t *testing.T) {
	cr := &CheckpointRunner{}
	p, f := cr.parseTestOutput("--- PASS: TestA (0.01s)\n--- FAIL: TestB (0.02s)\n")
	if p != 1 || f != 1 {
		t.Errorf("parseTestOutput=(%d,%d), want (1,1)", p, f)
	}
	// Empty/unparseable output optimistically assumes a single pass.
	if p, f := cr.parseTestOutput(""); p != 1 || f != 0 {
		t.Errorf("parseTestOutput(empty)=(%d,%d), want (1,0)", p, f)
	}
}

func TestParseGoTestJSON(t *testing.T) {
	cr := &CheckpointRunner{}
	out := `{"Action":"pass","Test":"TestA","Elapsed":0.5}
{"Action":"fail","Test":"TestB","Elapsed":0.1}
{"Action":"pass","Test":"TestC","Elapsed":0.2}`
	p, f, dur := cr.parseGoTestJSON(out)
	if p != 2 || f != 1 {
		t.Errorf("parseGoTestJSON counts=(%d,%d), want (2,1)", p, f)
	}
	if dur <= 0 {
		t.Errorf("expected positive accumulated duration, got %v", dur)
	}
}

func TestParseJestJSON(t *testing.T) {
	cr := &CheckpointRunner{}
	if p, f := cr.parseJestJSON([]byte(`{"numPassedTests":5,"numFailedTests":2}`)); p != 5 || f != 2 {
		t.Errorf("parseJestJSON=(%d,%d), want (5,2)", p, f)
	}
	// Malformed input yields zeros rather than an error/panic.
	if p, f := cr.parseJestJSON([]byte("not json")); p != 0 || f != 0 {
		t.Errorf("parseJestJSON(bad)=(%d,%d), want (0,0)", p, f)
	}
}

// The cases below pin the two defects that used to skew every checkpoint
// verdict toward failure.

func TestParseTestOutput_ShouldNotDoubleCountATestNamedFailed(t *testing.T) {
	cr := &CheckpointRunner{}
	// "--- FAIL: TestFailedLogin" matched the Go marker and then matched the
	// generic "failed" pattern on the same line, counting one failure twice.
	p, f := cr.parseTestOutput("--- FAIL: TestFailedLogin (0.01s)\n")
	if f != 1 {
		t.Errorf("failed=%d, want 1: a test whose name contains \"failed\" must count once", f)
	}
	if p != 0 {
		t.Errorf("passed=%d, want 0", p)
	}
}

func TestParseTestOutput_ShouldCountGenericPassesNotJustFailures(t *testing.T) {
	cr := &CheckpointRunner{}
	// The generic passing branch was empty while the failing branch
	// incremented, so this suite scored (0 passed, 1 failed).
	p, f := cr.parseTestOutput("12 passed\n1 failed\n")
	if p != 1 || f != 1 {
		t.Errorf("(passed,failed)=(%d,%d), want (1,1): the two generic branches must be symmetric", p, f)
	}
}

func TestParseTestOutput_ShouldNotSeeOKInsideAPackagePath(t *testing.T) {
	cr := &CheckpointRunner{}
	// strings.Contains(lower, "ok") is true of "token". Every go test summary
	// line carries a package path.
	p, f := cr.parseTestOutput("--- FAIL: TestX (0.01s)\nFAIL\tgithub.com/example/tokenizer\t0.5s\n")
	if p != 0 {
		t.Errorf("passed=%d, want 0: \"tokenizer\" must not read as a passing \"ok\"", p)
	}
	if f == 0 {
		t.Error("the failing run was not counted at all")
	}
}

func TestParseTestOutput_ShouldCountAPlainOKLine(t *testing.T) {
	cr := &CheckpointRunner{}
	p, f := cr.parseTestOutput("ok  \tgithub.com/example/project\t0.5s\n")
	if p != 1 || f != 0 {
		t.Errorf("(passed,failed)=(%d,%d), want (1,0)", p, f)
	}
}

func TestParseTestOutput_ShouldNotCountSkipsAsEitherOutcome(t *testing.T) {
	cr := &CheckpointRunner{}
	p, f := cr.parseTestOutput("--- SKIP: TestZ (0.00s)\n--- PASS: TestY (0.01s)\n")
	if p != 1 || f != 0 {
		t.Errorf("(passed,failed)=(%d,%d), want (1,0): a skip is neither outcome", p, f)
	}
}

func TestParseTestOutput_WhenSummaryMixesBoth_ShouldPreferFailure(t *testing.T) {
	cr := &CheckpointRunner{}
	// "1 failed, 3 passed" is a failing summary. Reading it as a pass is the
	// dangerous direction for a checkpoint gate.
	_, f := cr.parseTestOutput("1 failed, 3 passed\n")
	if f != 1 {
		t.Errorf("failed=%d, want 1 on a mixed summary line", f)
	}
}

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
