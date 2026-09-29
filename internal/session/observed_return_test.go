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

// TestObservedReturn_OwnPassOverRedImporters_IsNotTestsOK pins the suite a
// parent is told about. The turn's own run and the importer run are two
// measurements of one /test gate (importer_gate_facts.go); an own pass over
// an importer failure used to reach the parent as "tests OK" once the merged
// struct was gone, because only TestCheck was published.
func TestObservedReturn_OwnPassOverRedImporters_IsNotTestsOK(t *testing.T) {
	t.Parallel()

	res := &ExecutionResult{
		TestCheck:     TestVerification{Ran: true, OK: true, Output: "ok  example.com/own\n"},
		ImporterCheck: TestVerification{Ran: true, OK: false, Output: "--- FAIL: TestCaller\ncaller detail\n"},
	}
	got := observedReturn("coder", "task", res)
	if got.Tests == nil {
		t.Fatal("an importer run was made; the observation must report tests")
	}
	if got.Tests.OK {
		t.Fatal("own pass over a failing importer run was reported as tests OK")
	}
	if got.Tests.Outcome != string(VerifyFailed) {
		t.Fatalf("Outcome = %q, want %q", got.Tests.Outcome, VerifyFailed)
	}
	if got.Tests.Detail != "--- FAIL: TestCaller" {
		t.Fatalf("Detail = %q, want the importer run's first line", got.Tests.Detail)
	}

	res.ImporterCheck = TestVerification{Ran: true, OK: true}
	got = observedReturn("coder", "task", res)
	if got.Tests == nil || !got.Tests.OK || got.Tests.Outcome != string(VerifyPassed) {
		t.Fatalf("own pass and importer pass must be tests OK, got %+v", got.Tests)
	}
}
