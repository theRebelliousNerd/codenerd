package session

import (
	"context"
	"testing"
	"time"
)

// Ladder run R1-18 (2026-09-19) is what this prevents. The importer gate ran
// six packages' tests, the run hit the four-minute verification budget, and
// the turn's signal line read "build ok | tests ok | 0 uncovered | 0 findings"
// -- over a check that had produced no verdict at all:
//
//	22:04:09 test gate: 6 package(s) import what this turn wrote; running their tests too
//	22:08:10 test verification timed out after 4m0s; recovery not verified
//	22:09:17 turn signals: build ok | tests ok | 0 uncovered | 0 findings
//
// gateTests propagated only VerifyFailed, so anything else from the importer
// check fell through to the turn's own packages' pass. A timeout is not proof
// that the importers passed; it is proof that nobody knows.
func TestMergeImporterVerdict_NoVerdictIsNotAPass(t *testing.T) {
	own := TestVerification{Ran: true, OK: true, Outcome: VerifyPassed}

	for _, tc := range []struct {
		name string
		imp  TestVerification
		want VerifyOutcome
	}{
		{
			name: "importersPassed",
			imp:  TestVerification{Ran: true, OK: true, Outcome: VerifyPassed},
			want: VerifyPassed,
		},
		{
			name: "nothingImportsTheChange",
			imp:  TestVerification{Outcome: VerifySkipped, Reason: "nothing imports what this turn wrote"},
			want: VerifyPassed,
		},
		{
			name: "importersFailed",
			imp:  TestVerification{Ran: true, Outcome: VerifyFailed, Output: "--- FAIL: TestA"},
			want: VerifyFailed,
		},
		{
			name: "importersRanOutOfBudget",
			imp:  TestVerification{Ran: true, Outcome: VerifyIndeterminate, Reason: "verification exceeded its budget"},
			want: VerifyIndeterminate,
		},
		{
			name: "importersWereCanceled",
			imp:  TestVerification{Ran: true, Outcome: VerifyCanceled},
			want: VerifyCanceled,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeImporterVerdict(own, tc.imp).Verdict()
			if got != tc.want {
				t.Fatalf("mergeImporterVerdict(passed, %s) = %s, want %s; a gate that reports a pass it did not measure is how a turn claims \"tests ok\" over a check that never finished",
					tc.imp.Verdict(), got, tc.want)
			}
		})
	}
}

// A verification budget of zero means the command is not bounded by the
// harness. Steve, 2026-09-19: "there should not be timeouts like that... some
// agentic runs are like hours long." The four-minute ceiling was shorter than
// the suite it gated -- internal/session's own tests take 259 s -- so any turn
// touching a package it imports could never have its importers verified.
//
// context.WithTimeout(ctx, 0) is already expired, so an unbounded budget needs
// its own path: without one, "no budget" would mean "no time at all" and every
// verification would come back indeterminate before the command started.
func TestRunVerificationCommand_ZeroBudgetIsUnbounded(t *testing.T) {
	ran := false
	runner := func(ctx context.Context, _ string, _ []string, _ string, _ []string) ([]byte, error) {
		// Long enough that any accidental sub-second budget would cut it,
		// short enough to keep the test quick.
		select {
		case <-time.After(150 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		ran = true
		return []byte("ok"), nil
	}
	out, outcome, reason := runVerificationCommand(context.Background(), t.TempDir(), nil, 0, "go", []string{"test"}, runner)
	if !ran {
		t.Fatal("an unbounded verification never ran its command")
	}
	if outcome != VerifyPassed {
		t.Fatalf("outcome = %s (%s), want passed; out=%q", outcome, reason, out)
	}
}
