package main

import (
	"strings"
	"testing"
	"time"
)

// The defect these guard (F-DREAM-1, observed live): `nerd dream` consulted 22
// agents, every one failed with "context deadline exceeded", and the command
// printed "✅ Dream state consultation complete" and exited 0. Nothing in the
// output or the exit code distinguished a total failure from a clean run, so
// no script and no unattended operator could tell.
//
// Three faults produced it, all fixed together: a hardcoded 5-minute ceiling
// that ignored --timeout, a strictly sequential fan-out that shared that one
// budget across every agent, and this summary, which never consulted its own
// results.
//
// The all-failed branch is now hard to reach live — the concurrent fan-out
// completes several agents even on a 32-second budget — which is precisely how
// the original bug survived unnoticed.

func TestDreamSummary_AllFailedIsAnError(t *testing.T) {
	msg, err := dreamSummary(0, 16, 5*time.Minute, true)

	if err == nil {
		t.Fatal("16 of 16 agents failed and the command would still exit 0")
	}
	if !strings.Contains(msg, "FAILED") {
		t.Errorf("summary does not report failure: %q", msg)
	}
	if strings.Contains(msg, "✅") {
		t.Errorf("summary claims success after total failure: %q", msg)
	}
}

// When the budget is what killed the run, say so and name the flag — a bare
// "context deadline exceeded" per agent gives an unattended operator nothing
// to act on.
func TestDreamSummary_NamesTheBudgetWhenTheDeadlineExpired(t *testing.T) {
	msg, _ := dreamSummary(0, 16, 75*time.Second, true)

	if !strings.Contains(msg, "1m15s") {
		t.Errorf("summary does not report the budget that expired: %q", msg)
	}
	if !strings.Contains(msg, "--timeout") {
		t.Errorf("summary does not name the flag that fixes it: %q", msg)
	}
}

// A total failure with no expired deadline is a different problem, and
// blaming the budget would send the reader in the wrong direction.
func TestDreamSummary_DoesNotBlameTheBudgetWhenItDidNotExpire(t *testing.T) {
	msg, err := dreamSummary(0, 3, 25*time.Minute, false)

	if err == nil {
		t.Fatal("total failure must still be an error")
	}
	if strings.Contains(msg, "--timeout") {
		t.Errorf("summary blames the timeout for a non-timeout failure: %q", msg)
	}
}

// Partial success is not failure — some agents produced usable perspectives —
// but it must not read as a clean run either. Verified live at a 75s budget:
// 7 succeeded, 9 failed.
func TestDreamSummary_PartialIsReportedAndNotAnError(t *testing.T) {
	msg, err := dreamSummary(7, 9, 75*time.Second, true)

	if err != nil {
		t.Errorf("partial success became a hard error: %v", err)
	}
	if !strings.Contains(msg, "7") || !strings.Contains(msg, "9") {
		t.Errorf("summary hides the split: %q", msg)
	}
	if strings.Contains(msg, "✅") {
		t.Errorf("partial run claims a clean result: %q", msg)
	}
}

func TestDreamSummary_FullSuccess(t *testing.T) {
	msg, err := dreamSummary(16, 0, 25*time.Minute, false)

	if err != nil {
		t.Errorf("a clean run returned an error: %v", err)
	}
	if !strings.Contains(msg, "✅") || !strings.Contains(msg, "16") {
		t.Errorf("clean run summary is wrong: %q", msg)
	}
}

// Zero agents consulted is not success. Reporting it as one would recreate the
// original defect in a different shape.
func TestDreamSummary_NoAgentsIsNotSuccess(t *testing.T) {
	msg, err := dreamSummary(0, 0, 25*time.Minute, false)

	if err != nil {
		t.Errorf("no agents available should not be a hard error: %v", err)
	}
	if strings.Contains(msg, "✅") {
		t.Errorf("consulting nothing reported as success: %q", msg)
	}
}
