package session

import (
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/observation"
)

// TestJITExecutor_ImplementsObservedTaskExecutor pins the assertion the
// delegate action makes at runtime. A type assertion that silently starts
// failing would send every delegation down the prose fallback, where the write
// set and the build verdict are simply absent — and absent reads as "nothing
// changed, nothing checked".
func TestJITExecutor_ImplementsObservedTaskExecutor(t *testing.T) {
	t.Parallel()

	var _ ObservedTaskExecutor = (*JITExecutor)(nil)
}

// TestObservedReturn_ShouldCarryWhatTheExecutorMeasured is the wire this branch
// exists to reconnect. ExecutionResult computes all of this on every turn and
// SubAgent.execute discarded it, so every consumer downstream had to re-derive
// from prose what the runtime had already measured exactly.
func TestObservedReturn_ShouldCarryWhatTheExecutorMeasured(t *testing.T) {
	t.Parallel()

	res := &ExecutionResult{
		Response:     "I fixed the lock ordering in internal/widget/widget.go.",
		Duration:     3 * time.Second,
		WrittenPaths: []string{"internal/widget/widget.go"},
		BuildCheck:   BuildVerification{Ran: true, OK: true},
		TestCheck:    TestVerification{Ran: true, OK: false, Output: "--- FAIL: TestFlush\nmore detail\n"},
		CriticFindings: []CriticFinding{
			{File: "internal/widget/widget.go", Line: 42, Severity: "high", Claim: "the guard is still missing"},
		},
		UntestedPaths: []string{"internal/widget/widget.go"},
	}

	ret := observedReturn("coder", "fix file:internal/widget/widget.go", res)

	if ret.Output != res.Response {
		t.Errorf("output = %q, want the whole response", ret.Output)
	}
	if len(ret.Changed) != 1 || ret.Changed[0] != "internal/widget/widget.go" {
		t.Errorf("changed = %v, want the executor's write set", ret.Changed)
	}
	if ret.Build == nil || !ret.Build.Ran || !ret.Build.OK || ret.Build.Source != observation.SourceObserved {
		t.Errorf("build = %+v, want an observed pass", ret.Build)
	}
	if ret.Tests == nil || ret.Tests.OK || ret.Tests.Source != observation.SourceObserved {
		t.Errorf("tests = %+v, want an observed failure", ret.Tests)
	}
	if ret.Tests != nil && ret.Tests.Detail != "--- FAIL: TestFlush" {
		t.Errorf("test detail = %q, want the first line only; the rest is behind the handle", ret.Tests.Detail)
	}
	if len(ret.Findings) != 1 || ret.Findings[0].Line != 42 {
		t.Errorf("findings = %+v, want the critic's finding with its citation", ret.Findings)
	}
	if !containsSubstring(ret.Notes, "no test file alongside") {
		t.Errorf("notes = %v, want the untested write named as uncertainty", ret.Notes)
	}
}

// TestObservedReturn_WhenTheTurnFailed_ShouldCarryTheFailureAndTheOutput. The
// output a turn produced before failing is usually where the reason is.
func TestObservedReturn_WhenTheTurnFailed_ShouldCarryTheFailureAndTheOutput(t *testing.T) {
	t.Parallel()

	ret := observedReturn("coder", "fix a.go", &ExecutionResult{
		Response: "I got as far as reading a.go.",
		Error:    errors.New("context deadline exceeded"),
	})

	if ret.Failure != "context deadline exceeded" {
		t.Errorf("failure = %q, want the turn's error", ret.Failure)
	}
	if ret.Output == "" {
		t.Error("a failed turn's partial output was dropped; that is usually where the reason is")
	}
}

// TestObservedReturn_WhenNothingRan_ShouldReportNoVerification rather than a
// zero-valued pass. A skipped verification is not a pass, and a Verification
// with Ran false and OK false carries that; a nil one carries nothing at all,
// which is the honest answer when no check was even attempted.
func TestObservedReturn_WhenNothingRan_ShouldReportNoVerification(t *testing.T) {
	t.Parallel()

	ret := observedReturn("researcher", "research x", &ExecutionResult{Response: "Here is what I found."})
	if ret.Build != nil {
		t.Errorf("build = %+v, want nothing: no build was attempted", ret.Build)
	}
	if ret.Tests != nil {
		t.Errorf("tests = %+v, want nothing: no test run was attempted", ret.Tests)
	}
}

func TestObservedReturn_WhenResultIsNil_ShouldStillNameTheAgentAndTask(t *testing.T) {
	t.Parallel()

	ret := observedReturn("coder", "fix a.go", nil)
	if ret.Agent != "coder" || ret.Task != "fix a.go" {
		t.Errorf("return = %+v, want the agent and task still named", ret)
	}
}

func TestFirstLine_ShouldTakeOnlyTheFirstLine(t *testing.T) {
	t.Parallel()

	if got := firstLine("  first \n second \n third "); got != "first" {
		t.Errorf("firstLine = %q, want %q", got, "first")
	}
	if got := firstLine("   "); got != "" {
		t.Errorf("firstLine of blank = %q, want empty", got)
	}
}

func containsSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
