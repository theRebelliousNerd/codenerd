package session

import (
	"testing"

	"codenerd/internal/testfacts"
)

// The importer run is its own measurement of /test. These cases are the
// rule in coder_safety.mg, fed by syncTestGateFacts the way a real turn
// is: a failure the importer baseline already had is not red, a new one
// is, and a run that did not finish is neither. R1-18 (2026-09-19) is the
// last of those. The importer gate timed out and the turn printed
// "tests ok", because the unfinished run was discarded and the own pass
// stood. /unfinished withholds that pass and does not name a failure, so
// a partial stream is not repaired as if the run had finished.

func failedImporterResult(name, msg string) *testfacts.Result {
	return &testfacts.Result{Failures: []testfacts.Failure{{
		Package: "p", Test: name, Message: msg, File: "p_test.go", Line: 1,
	}}}
}

func ownPass() TestVerification {
	return TestVerification{Ran: true, OK: true, Outcome: VerifyPassed}
}

func chargeTestGate(t *testing.T, own, imp TestVerification) VerifyOutcome {
	t.Helper()
	e := newObligationExec(t)
	e.syncTestGateFacts(testTurn, &ExecutionResult{TestCheck: own, ImporterCheck: imp})
	return derivedVerify(t, e, testTurn, "/test")
}

func TestImporterGate_AttributionAndUnfinished(t *testing.T) {
	oldFail := TestVerification{
		Ran: true, Outcome: VerifyFailed, Output: "FAIL TestOld",
		BaselineRan: true, BaselineFailures: []string{"TestOld"},
		Result: failedImporterResult("TestOld", "old"),
	}
	newFail := TestVerification{
		Ran: true, Outcome: VerifyFailed, Output: "FAIL TestNew",
		Result: failedImporterResult("TestNew", "new"),
	}
	// A timeout that already parsed a failure is still unfinished. Charging
	// that row would start a repair of a run that did not finish.
	unfinished := TestVerification{
		Ran: true, Outcome: VerifyIndeterminate, Reason: "verification exceeded its budget",
		Result: failedImporterResult("TestPartial", "cut off"),
	}
	canceled := TestVerification{Ran: true, Outcome: VerifyCanceled, Reason: "operator canceled verification"}
	skipped := TestVerification{Outcome: VerifySkipped, Reason: "nothing imports what this turn wrote"}

	for _, tc := range []struct {
		name     string
		own, imp TestVerification
		want     VerifyOutcome
	}{
		{name: "own pass, importer not run", own: ownPass(), want: VerifyPassed},
		{name: "own pass, importer passed", own: ownPass(), imp: ownPass(), want: VerifyPassed},
		{name: "own pass, nothing imports", own: ownPass(), imp: skipped, want: VerifyPassed},
		{name: "own pass, importer failure is new", own: ownPass(), imp: newFail, want: VerifyFailed},
		{name: "own pass, importer failure predates the turn", own: ownPass(), imp: oldFail, want: VerifyPassed},
		{name: "own pass, importer failed and named no test", own: ownPass(), imp: TestVerification{Ran: true, Outcome: VerifyFailed, Output: "build failed"}, want: VerifyFailed},
		{name: "own pass, importer timed out", own: ownPass(), imp: unfinished, want: VerifySkipped},
		{name: "own pass, importer canceled", own: ownPass(), imp: canceled, want: VerifySkipped},
		{name: "own failures predate, importer failure is new", own: oldFail, imp: newFail, want: VerifyFailed},
		{name: "own and importer failures both predate", own: oldFail, imp: oldFail, want: VerifyPassed},
		{name: "own failures predate, importer timed out", own: oldFail, imp: unfinished, want: VerifySkipped},
		{name: "own failure is new, importer not run", own: newFail, want: VerifyFailed},
		{name: "own run timed out, importer not run", own: unfinished, want: VerifySkipped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := chargeTestGate(t, tc.own, tc.imp); got != tc.want {
				t.Fatalf("test gate = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestImporterGate_UnfinishedIsAssertedAndNotAFailure(t *testing.T) {
	e := newObligationExec(t)
	result := &ExecutionResult{
		TestCheck: ownPass(),
		ImporterCheck: TestVerification{
			Ran: true, Outcome: VerifyCanceled, Reason: "operator canceled verification",
			Result: failedImporterResult("TestPartial", "cut off"),
		},
	}
	e.syncTestGateFacts(testTurn, result)
	if !gateFact(t, e, "turn_importer_measured", testTurn, "/unfinished") {
		t.Fatal("a canceled importer run was not recorded as /unfinished")
	}
	facts, err := e.kernel.Query("turn_importer_failing_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 0 {
		t.Fatalf("an unfinished importer run named %d failure(s); a partial stream is not a charge", len(facts))
	}
	if got := derivedVerify(t, e, testTurn, "/test"); got != VerifySkipped {
		t.Fatalf("test gate = %s, want neither passing nor failing", got)
	}
}

func TestImporterGate_ALaterPassDropsThePreviousFailure(t *testing.T) {
	e := newObligationExec(t)
	result := &ExecutionResult{
		TestCheck:     ownPass(),
		ImporterCheck: TestVerification{Ran: true, Outcome: VerifyFailed, Output: "FAIL TestNew", Result: failedImporterResult("TestNew", "new")},
	}
	e.syncTestGateFacts(testTurn, result)
	if got := derivedVerify(t, e, testTurn, "/test"); got != VerifyFailed {
		t.Fatalf("first gate = %s, want failing", got)
	}
	result.ImporterCheck = ownPass()
	e.syncTestGateFacts(testTurn, result)
	if got := derivedVerify(t, e, testTurn, "/test"); got != VerifyPassed {
		t.Fatalf("test gate = %s, want passing: the previous importer failure is not this run", got)
	}
}

func TestSuiteExit_OwnUnfinishedWinsAndImporterFailureReplacesAPass(t *testing.T) {
	result := &ExecutionResult{
		TestCheck:     TestVerification{Ran: true, Outcome: VerifyIndeterminate},
		ImporterCheck: TestVerification{Ran: true, Outcome: VerifyFailed, Output: "old importer"},
	}
	if got := suiteExit(result); got != VerifyIndeterminate {
		t.Fatalf("suiteExit = %s, want the own run that did not finish", got)
	}
	result.TestCheck = ownPass()
	if got := suiteExit(result); got != VerifyFailed {
		t.Fatalf("suiteExit = %s, want the importer failure once the own run finished", got)
	}
	result.ImporterCheck = TestVerification{Outcome: VerifySkipped}
	if got := suiteExit(result); got != VerifyPassed {
		t.Fatalf("suiteExit = %s, want the own pass when the importer run was skipped", got)
	}
	bare := &Executor{}
	result.ImporterCheck = TestVerification{Ran: true, Outcome: VerifyFailed, Output: "importer"}
	if !bare.testGateRed(testTurn, result) || bare.testGatePassed(testTurn, result) {
		t.Fatal("a nil kernel reads the raw suite exit and does not attribute the importer failure")
	}
}

func TestFailedChecksSummary_NamesAnImporterFailure(t *testing.T) {
	result := &ExecutionResult{
		TestCheck:     ownPass(),
		ImporterCheck: TestVerification{Outcome: VerifyFailed, Reason: "the tests of the packages that import what this turn changed"},
	}
	got := failedChecksSummary(result)
	if got != "importer tests fail: the tests of the packages that import what this turn changed" {
		t.Fatalf("failedChecksSummary = %q", got)
	}
}
