package session

import (
	"strings"
	"testing"

	"codenerd/internal/evidence"
	"codenerd/internal/types"
)

// acceptanceReportFixture is a host-issued acceptance witness in the shape
// assertTurnEvidence requires: status "verified", with a contract id and an
// after-snapshot to key the turn_acceptance fact on.
var acceptanceReportFixture = evidence.Report{
	ContractID: "contract-sha",
	After:      "after-snapshot",
	Status:     "verified",
}

// These tests are S4: turn_done must be REACHABLE from mechanical evidence on
// an ordinary turn, and the outcome the rest of the system reads must be that
// derivation and nothing else.
//
// Before this seam turn_done needed turn_acceptance, which only
// `nerd fix --acceptance` ever produced (cmd/nerd/cmd_direct_actions.go:313,
// the single production caller of evidence.WithContract). On every chat turn,
// campaign task and observer run the completion signal was unreachable by
// construction, and captureTurnOutcome did not even issue the query — a Go
// guard restating the acceptance conjunct sat in front of the kernel read.
//
// Every test here drives the real kernel with the shipped policy corpus
// (newObligationExec → core.NewRealKernel), through the production entry
// points assertTurnEvidence and captureTurnOutcome. Nothing is stubbed.

// writeTurnResult is the ExecutionResult shape a completed write-oriented turn
// produces, with both mechanical gates green. Callers dial the gates back to
// express the case they are testing.
func writeTurnResult() *ExecutionResult {
	res := mutationResult()
	res.WrittenPaths = []string{"pkg/foo.go"}
	res.BuildCheck = BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	res.TestCheck = TestVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	return res
}

// readOnlyTurnResult is an /explain turn: tools ran, nothing was written.
func readOnlyTurnResult() *ExecutionResult {
	res := &ExecutionResult{ToolCallsExecuted: 1, SuccessfulToolCalls: 1}
	res.Intent.Verb = "/explain"
	res.Intent.Category = "/query"
	return res
}

// TestTurnDone_DerivesFromEvidenceWithoutAcceptance is the seam in one test: a
// write turn whose build and tests were both measured green is done, with no
// acceptance contract anywhere near it.
func TestTurnDone_DerivesFromEvidenceWithoutAcceptance(t *testing.T) {
	e := newObligationExec(t)
	result := writeTurnResult()

	e.assertTurnEvidence("/create", result)

	if got := queryCount(t, e, "turn_executed"); got != 1 {
		t.Fatalf("a clean write turn with green gates must derive turn_executed, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_verified"); got != 1 {
		t.Fatalf("green build + green tests must derive turn_verified, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_done"); got != 1 {
		t.Fatalf("turn_done must derive from evidence alone, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_acceptance"); got != 0 {
		t.Fatalf("this turn must have no acceptance contract, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_unverified"); got != 0 {
		t.Fatalf("a verified turn must derive no turn_unverified, got %d facts", got)
	}

	e.captureTurnOutcome(result, nil)
	if result.TurnOutcome != types.MangleAtom("/done") {
		t.Fatalf("TurnOutcome = %q, want /done — the outcome must bind to the derivation", result.TurnOutcome)
	}
	if len(result.MissingEvidence) != 0 {
		t.Fatalf("a done turn must name no missing evidence, got %v", result.MissingEvidence)
	}
}

// A turn that changed nothing has no workspace claim to verify, so execution is
// the whole of what it can owe. Until this seam every /explain and /review turn
// was recorded /unverified forever (executor_tools.go:1948-1952 says so in the
// code that asserts evidence for them).
func TestTurnDone_ReadOnlyTurnIsDoneWithoutGates(t *testing.T) {
	e := newObligationExec(t)
	result := readOnlyTurnResult()

	e.assertTurnEvidence("/explain", result)

	if got := queryCount(t, e, "build_state"); got != 0 {
		t.Fatalf("a read-only turn runs no gate, so no build_state, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_done"); got != 1 {
		t.Fatalf("a read-only turn that ran must derive turn_done, got %d facts", got)
	}

	e.captureTurnOutcome(result, nil)
	if result.TurnOutcome != types.MangleAtom("/done") {
		t.Fatalf("TurnOutcome = %q, want /done for a completed read-only turn", result.TurnOutcome)
	}
}

// TestTurnDone_BuildFailingExcludes: a red build is not a turn that merely
// lacks evidence, it is a failed turn, and the outcome must say so.
func TestTurnDone_BuildFailingExcludes(t *testing.T) {
	e := newObligationExec(t)
	result := writeTurnResult()
	result.BuildCheck = BuildVerification{Ran: true, OK: false, Outcome: VerifyFailed}

	e.assertTurnEvidence("/create", result)

	if got := queryCount(t, e, "turn_executed"); got != 0 {
		t.Fatalf("turn_executed must not derive while the build is red, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_done"); got != 0 {
		t.Fatalf("turn_done must not derive while the build is red, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_build_failed"); got != 1 {
		t.Fatalf("a red build must derive turn_build_failed, got %d facts", got)
	}

	e.captureTurnOutcome(result, nil)
	if result.TurnOutcome != types.MangleAtom("/failed") {
		t.Fatalf("TurnOutcome = %q, want /failed for a red build", result.TurnOutcome)
	}
}

// TestTurnOutcome_UnverifiedNamesMissingEvidence: a write turn that built green
// but never ran its tests is not done, is not failed, and must say which
// evidence is missing rather than emitting a constant.
func TestTurnOutcome_UnverifiedNamesMissingEvidence(t *testing.T) {
	e := newObligationExec(t)
	result := writeTurnResult()
	result.TestCheck = TestVerification{Outcome: VerifySkipped}

	e.assertTurnEvidence("/create", result)

	if got := queryCount(t, e, "turn_executed"); got != 1 {
		t.Fatalf("a green build with no test run still executed, got %d turn_executed facts", got)
	}
	if got := queryCount(t, e, "turn_done"); got != 0 {
		t.Fatalf("a write turn whose tests never ran must not be done, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_unverified"); got != 1 {
		t.Fatalf("expected exactly one turn_unverified, got %d facts", got)
	}

	e.captureTurnOutcome(result, nil)
	if result.TurnOutcome != types.MangleAtom("/unverified") {
		t.Fatalf("TurnOutcome = %q, want /unverified", result.TurnOutcome)
	}

	if !containsString(result.MissingEvidence, "/tests_not_green") {
		t.Fatalf("missing evidence must name the tests, got %v", result.MissingEvidence)
	}
	if containsString(result.MissingEvidence, "/build_not_green") {
		t.Fatalf("the build was measured green; it must not be reported missing, got %v", result.MissingEvidence)
	}

	// The sentence the user reads is the same derivation, spelled out. The
	// constant it replaces was "Requested behavior remains unverified (no
	// acceptance contract)." on every write turn, whatever the gates said.
	result.Response = "did the work"
	e.appendEvidenceSummary(result)
	if !strings.Contains(result.Response, "Unverified:") {
		t.Fatalf("the evidence sentence must state the verdict, got %q", result.Response)
	}
	if !strings.Contains(result.Response, "tests") {
		t.Fatalf("the evidence sentence must name the missing tests, got %q", result.Response)
	}
	if strings.Contains(result.Response, "no acceptance contract") {
		t.Fatalf("the constant sentence must be gone, got %q", result.Response)
	}
}

// A write turn with no gate at all owes both, and both are named.
func TestTurnOutcome_UnverifiedNamesBothGatesWhenNeitherRan(t *testing.T) {
	e := newObligationExec(t)
	result := writeTurnResult()
	result.BuildCheck = BuildVerification{Outcome: VerifySkipped}
	result.TestCheck = TestVerification{Outcome: VerifySkipped}

	e.assertTurnEvidence("/create", result)
	e.captureTurnOutcome(result, nil)

	if result.TurnOutcome != types.MangleAtom("/unverified") {
		t.Fatalf("TurnOutcome = %q, want /unverified when no gate ran", result.TurnOutcome)
	}
	if !containsString(result.MissingEvidence, "/build_not_green") ||
		!containsString(result.MissingEvidence, "/tests_not_green") {
		t.Fatalf("both gates must be named missing, got %v", result.MissingEvidence)
	}
}

// TestTurnDone_AcceptancePathStillSufficient: the contract path is untouched.
// An acceptance witness verifies the turn even with no build and no test run,
// because it is a stronger claim than either — it is about requested behavior.
func TestTurnDone_AcceptancePathStillSufficient(t *testing.T) {
	e := newObligationExec(t)
	result := writeTurnResult()
	result.BuildCheck = BuildVerification{Outcome: VerifySkipped}
	result.TestCheck = TestVerification{Outcome: VerifySkipped}
	result.Acceptance = &acceptanceReportFixture

	e.assertTurnEvidence("/create", result)

	if got := queryCount(t, e, "turn_acceptance"); got != 1 {
		t.Fatalf("expected the acceptance witness to reach the kernel, got %d facts", got)
	}
	if got := queryCount(t, e, "build_state"); got != 0 {
		t.Fatalf("no gate ran, so no build_state, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_done"); got != 1 {
		t.Fatalf("acceptance alone must still derive turn_done, got %d facts", got)
	}

	e.captureTurnOutcome(result, nil)
	if result.TurnOutcome != types.MangleAtom("/done") {
		t.Fatalf("TurnOutcome = %q, want /done on the acceptance path", result.TurnOutcome)
	}
}

// TestLearningTrainsOnlyOnDerivedDone: TurnRecord.Verified() is what decides
// whether the prompt learner treats a turn as a win, and it must follow the
// kernel's verdict rather than any Go re-derivation. resolveTurnOutcome used to
// re-query the kernel behind the same acceptance guard; it now returns what
// captureTurnOutcome recorded.
func TestLearningTrainsOnlyOnDerivedDone(t *testing.T) {
	cases := []struct {
		name         string
		mutate       func(*ExecutionResult)
		wantOutcome  types.MangleAtom
		wantVerified bool
	}{
		{
			name:         "greenGatesTrainAsAWin",
			mutate:       func(*ExecutionResult) {},
			wantOutcome:  types.MangleAtom("/done"),
			wantVerified: true,
		},
		{
			name:         "noTestRunIsNotAWin",
			mutate:       func(r *ExecutionResult) { r.TestCheck = TestVerification{Outcome: VerifySkipped} },
			wantOutcome:  types.MangleAtom("/unverified"),
			wantVerified: false,
		},
		{
			name:         "redBuildIsNotAWin",
			mutate:       func(r *ExecutionResult) { r.BuildCheck = BuildVerification{Ran: true, OK: false, Outcome: VerifyFailed} },
			wantOutcome:  types.MangleAtom("/failed"),
			wantVerified: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newObligationExec(t)
			result := writeTurnResult()
			tc.mutate(result)

			e.assertTurnEvidence("/create", result)
			e.captureTurnOutcome(result, nil)

			// The per-turn facts are retracted before persistTurn runs, which
			// is where resolveTurnOutcome is called. If it re-derived, it would
			// be asking a kernel that has already forgotten the turn.
			e.cleanupPerTurnCoverageFacts()

			outcome := e.resolveTurnOutcome(result)
			if outcome != tc.wantOutcome {
				t.Fatalf("resolveTurnOutcome = %q, want %q after cleanup", outcome, tc.wantOutcome)
			}
			rec := TurnRecord{IntentVerb: "/create", Outcome: outcome}
			if rec.Verified() != tc.wantVerified {
				t.Fatalf("TurnRecord.Verified() = %t, want %t for outcome %q", rec.Verified(), tc.wantVerified, outcome)
			}
		})
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
