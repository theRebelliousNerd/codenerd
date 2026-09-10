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
