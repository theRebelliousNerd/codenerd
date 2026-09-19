package session

import (
	"strings"
	"testing"

	"codenerd/internal/evidence"
	"codenerd/internal/types"
)

// TestRecoveredToolError_IsNotAFailedTurn pins the producer traced from the
// live `nerd fix` run of 2026-09-18 00:05 (pid 054828): run_build failed
// mid-turn, the repair loop converged, the post-edit build and tests both
// passed, and the command still exited non-zero.
//
// The old rule asked whether the final response was empty. It was — the
// working policy closed the turn before the model's last sentence — so a
// fully recovered build failure decided a green turn's verdict.
func TestRecoveredToolError_IsNotAFailedTurn(t *testing.T) {
	t.Parallel()

	toolErrs := []string{"run_build: modular tool execution failed: exit status 1"}

	cases := []struct {
		name    string
		result  *ExecutionResult
		wantErr bool
	}{
		{
			// The live case. No closing prose, every closing gate green.
			name: "buildFailedMidTurnFinalGatesPassedNoClosingProse",
			result: &ExecutionResult{
				Response:   "",
				BuildCheck: BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed},
				TestCheck:  TestVerification{Ran: true, OK: true, Outcome: VerifyPassed},
			},
			wantErr: false,
		},
		{
			// The closing build is red: the tool error was not recovered, and
			// closing prose must not talk the turn into success.
			name: "finalBuildFailedDespiteClosingProse",
			result: &ExecutionResult{
				Response:   "All done! Everything builds cleanly now.",
				BuildCheck: BuildVerification{Ran: true, OK: false, Outcome: VerifyFailed},
			},
			wantErr: true,
		},
		{
			name: "finalTestsFailed",
			result: &ExecutionResult{
				Response:   "Fixed.",
				BuildCheck: BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed},
				TestCheck:  TestVerification{Ran: true, OK: false, Outcome: VerifyFailed},
			},
			wantErr: true,
		},
		{
			// No gate ran at all, and the turn ended with the error as its
			// last word. Nothing answers the failure, so it stands.
			name:    "noGateRanAndNoClosingProse",
			result:  &ExecutionResult{Response: ""},
			wantErr: true,
		},
		{
			// No gate ran, but the model finished and answered. That is the
			// only case where prose decides, because it is the only signal.
			name:    "noGateRanButModelAnswered",
			result:  &ExecutionResult{Response: "I could not run the build; here is what I found instead."},
			wantErr: false,
		},
		{
			// A gate that was skipped proves nothing, so it cannot rescue a
			// turn that also produced no answer.
			name: "skippedGateIsNotAPass",
			result: &ExecutionResult{
				Response:   "",
				BuildCheck: BuildVerification{Outcome: VerifySkipped},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			surfaceToolErrors(tc.result, toolErrs)
			gotErr := tc.result.Error != nil
			if gotErr != tc.wantErr {
				t.Fatalf("surfaceToolErrors: error=%v (%v), want error=%v",
					gotErr, tc.result.Error, tc.wantErr)
			}
		})
	}
}

// TestSurfaceToolErrors_NoToolErrorsNeverFailsATurn keeps the change honest in
// the other direction: a turn with no tool errors is untouched no matter how
// bare its evidence is.
func TestSurfaceToolErrors_NoToolErrorsNeverFailsATurn(t *testing.T) {
	t.Parallel()
	res := &ExecutionResult{}
	surfaceToolErrors(res, nil)
	if res.Error != nil {
		t.Fatalf("a turn with no tool errors must not be failed, got %v", res.Error)
	}
}

// TestObservedReturn_CarriesKernelVerdict pins the boundary that used to drop
// the verdict: ExecutionResult holds TurnOutcome, ChangeStage, Acceptance and
// UntestedPaths on every turn, and observedReturn carried none of them, so
// every consumer downstream re-derived a status by reading the prose.
func TestObservedReturn_CarriesKernelVerdict(t *testing.T) {
	t.Parallel()

	report := evidence.Report{
		Status:     "unverified",
		ContractID: "contract-42",
	}
	res := &ExecutionResult{
		Response:        "Wrote the file.",
		TurnOutcome:     types.MangleAtom("/unverified"),
		MissingEvidence: []string{"/tests_not_written"},
		ChangeStage:     "checks_passed",
		Acceptance:      &report,
		WrittenPaths:    []string{"internal/session/executor.go"},
		UntestedPaths:   []string{"internal/session/executor.go"},
	}

	got := observedReturn("coder", "fix the thing", res)

	if got.Outcome != "/unverified" {
		t.Errorf("Outcome = %q, want %q — the kernel's verdict must cross the boundary", got.Outcome, "/unverified")
	}
	if got.Stage != "checks_passed" {
		t.Errorf("Stage = %q, want %q", got.Stage, "checks_passed")
	}
	if got.Acceptance == nil {
		t.Fatal("Acceptance must cross the boundary when the executor recorded one")
	}
	if got.Acceptance.Status != "unverified" {
		t.Errorf("Acceptance.Status = %q, want %q", got.Acceptance.Status, "unverified")
	}
	if got.Acceptance.Contract != "contract-42" {
		t.Errorf("Acceptance.Contract = %q, want %q", got.Acceptance.Contract, "contract-42")
	}
	if strings.TrimSpace(got.Acceptance.Summary) == "" {
		t.Error("Acceptance.Summary must carry the report's own summary")
	}
	if len(got.Untested) != 1 || got.Untested[0] != "internal/session/executor.go" {
		t.Errorf("Untested = %v, want the executor's UntestedPaths", got.Untested)
	}
	// What an /unverified turn left missing crosses too: a campaign fails the
	// task by it and the retry reads it (external audit F2).
	if len(got.Missing) != 1 || got.Missing[0] != "/tests_not_written" {
		t.Errorf("Missing = %v, want the kernel's turn_missing_evidence", got.Missing)
	}
	if got.Done() {
		t.Error("Done() = true for an /unverified turn")
	}
}

// TestObservedReturn_AbsentVerdictStaysAbsent guards the other half of the
// contract: an empty Outcome must mean "the producer had no verdict", never a
// default that a consumer could read as success.
func TestObservedReturn_AbsentVerdictStaysAbsent(t *testing.T) {
	t.Parallel()
	got := observedReturn("researcher", "look it up", &ExecutionResult{Response: "here you go"})
	if got.Outcome != "" {
		t.Errorf("Outcome = %q, want empty for an executor that recorded none", got.Outcome)
	}
	if got.Acceptance != nil {
		t.Errorf("Acceptance = %+v, want nil when there was no contract", got.Acceptance)
	}
}

// TestBuildFailure_ExcludesTurnExecuted closes the wiring gap the comment on
// perTurnBuildStateFacts claimed was already closed: the field named a
// recordBuildState that did not exist anywhere in the tree, so the only
// build_state assertions in the repo were in a test, and the
// !build_state(/failing) conjunct of turn_executed
// (internal/core/defaults/policy/coder_safety.mg) excluded nothing in
// production. A turn whose build failed could still be turn_executed.
//
// This drives the production path — recordBuildState via assertTurnEvidence —
// against the real loaded corpus.
func TestBuildFailure_ExcludesTurnExecuted(t *testing.T) {
	e := newObligationExec(t)

	result := mutationResult()
	result.BuildCheck = BuildVerification{Ran: true, OK: false, Outcome: VerifyFailed}
	e.assertTurnEvidence(testTurn, "/create", result)

	if got := queryCount(t, e, "build_state"); got != 1 {
		t.Fatalf("expected exactly one build_state fact asserted from BuildCheck, got %d", got)
	}
	if got := queryCount(t, e, "turn_executed"); got != 0 {
		t.Fatalf("turn_executed must not derive while the build is red, got %d facts", got)
	}
}

// TestPassingBuildDerivesTurnExecuted proves the gate above blocks for the
// right reason rather than turn_executed never deriving at all, and that the
// passing verdict reaches the kernel too.
func TestPassingBuildDerivesTurnExecuted(t *testing.T) {
	e := newObligationExec(t)

	result := mutationResult()
	result.BuildCheck = BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	result.TestCheck = TestVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	e.assertTurnEvidence(testTurn, "/create", result)

	facts, err := e.kernel.Query("build_state")
	if err != nil {
		t.Fatalf("query build_state: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected one build_state fact, got %v", facts)
	}
	if got := types.ExtractString(facts[0].Args[0]); got != "/passing" {
		t.Errorf("build_state = %q, want /passing", got)
	}
	if got := queryCount(t, e, "test_state"); got != 1 {
		t.Errorf("expected test_state asserted from TestCheck, got %d facts", got)
	}
	if got := queryCount(t, e, "turn_executed"); got != 1 {
		t.Fatalf("a clean write turn with a green build must derive turn_executed, got %d facts", got)
	}
}

// TestSkippedGateAssertsNoState keeps "not proven" from becoming either
// verdict: a gate that never ran must not assert /passing (a guess) or
// /failing (which would fail every turn on a machine with no Go toolchain).
func TestSkippedGateAssertsNoState(t *testing.T) {
	e := newObligationExec(t)

	result := mutationResult()
	result.BuildCheck = BuildVerification{Outcome: VerifySkipped}
	result.TestCheck = TestVerification{Outcome: VerifySkipped}
	e.assertTurnEvidence(testTurn, "/create", result)

	if got := queryCount(t, e, "build_state"); got != 0 {
		t.Errorf("a skipped build must assert no build_state, got %d facts", got)
	}
	if got := queryCount(t, e, "test_state"); got != 0 {
		t.Errorf("a skipped test gate must assert no test_state, got %d facts", got)
	}
}

// TestPerTurnBuildStateIsRetracted proves the red build is scoped to its own
// turn. Left asserted it would exclude turn_executed for the rest of the
// session, so one failed compile would make the session unable to finish
// anything.
func TestPerTurnBuildStateIsRetracted(t *testing.T) {
	e := newObligationExec(t)

	result := mutationResult()
	result.BuildCheck = BuildVerification{Ran: true, OK: false, Outcome: VerifyFailed}
	e.assertTurnEvidence(testTurn, "/create", result)
	if got := queryCount(t, e, "build_state"); got != 1 {
		t.Fatalf("expected build_state asserted for this turn, got %d", got)
	}

	e.cleanupTurnFacts()

	if got := queryCount(t, e, "build_state"); got != 0 {
		t.Fatalf("build_state must not survive its own turn, got %d facts", got)
	}
}
