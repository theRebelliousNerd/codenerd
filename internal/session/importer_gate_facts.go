package session

import (
	"codenerd/internal/testfacts"
	"codenerd/internal/types"
)

// The importer run is a second measurement of the same /test gate, not a
// second gate and not a Source column on turn_failing_test. That predicate
// is the turn's own `go test -json` run (turn_test_facts.go). A Source
// argument would change its arity and mix two runs into every rule that
// joins it. The importer rows are turn_importer_measured,
// turn_importer_failing_test and turn_importer_failed_before
// (coder_safety.mg). A failure is the turn's when the importer head names
// it and the importer baseline did not. /unfinished withholds a pass and
// names nothing: a timeout is not a test to repair (R1-18, 2026-09-19).

// suiteExit is the raw suite exit recorded as test_state, and the exit a
// nil kernel has to use because it cannot ask the /test rule. It is not
// the charge. The charge is turn_gate(/test).
//
// The turn's own run wins when it did not finish. A canceled or
// indeterminate own run must not be reported as an importer failure left
// from an earlier measurement. Otherwise the importer run replaces the own
// exit when it failed or did not finish, which is what the gate used to do
// by substituting the importer struct for the own one. A passing or
// skipped importer run leaves the own exit standing. The zero
// ImporterCheck is skipped, which is a turn that did not run that check.
func suiteExit(result *ExecutionResult) VerifyOutcome {
	if result == nil {
		return ""
	}
	switch result.TestCheck.Verdict() {
	case VerifyCanceled, VerifyIndeterminate:
		return result.TestCheck.Verdict()
	}
	switch result.ImporterCheck.Verdict() {
	case VerifyPassed, VerifySkipped:
		return result.TestCheck.Verdict()
	default:
		return result.ImporterCheck.Verdict()
	}
}

// suiteFailureText is the output of the run suiteExit would report when
// that run failed. An importer failure is named from the importer run; the
// own run's text is a different suite.
func suiteFailureText(result *ExecutionResult) string {
	if result == nil {
		return ""
	}
	if result.ImporterCheck.Verdict() == VerifyFailed {
		return result.ImporterCheck.Output
	}
	return result.TestCheck.Output
}

// suiteFailureResult is the parsed stream behind suiteFailureText, when
// seed is that text. A build recheck's seed is compiler output and has no
// test Result; the previous run's tests would name the wrong files.
func suiteFailureResult(result *ExecutionResult, seed string) *testfacts.Result {
	if result == nil {
		return nil
	}
	if result.ImporterCheck.Verdict() == VerifyFailed && result.ImporterCheck.Output == seed {
		return result.ImporterCheck.Result
	}
	if result.TestCheck.Output == seed {
		return result.TestCheck.Result
	}
	return nil
}

// syncImporterGateFacts replaces this turn's importer measurements.
// RetractFact matches the predicate and the first argument, so a repair
// that re-runs the importers drops the previous rows instead of charging
// both runs. A skipped run, and a turn that did not run importers, assert
// nothing: the own rules then stand. /unfinished is the measurement for a
// timeout or a cancel, and it carries no failing_test rows, so a partial
// stream cannot be repaired as if the run had finished.
func (e *Executor) syncImporterGateFacts(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || turn == "" {
		return
	}
	e.retractTurnPredicates(turn,
		"turn_importer_measured", "turn_importer_failed_before", "turn_importer_failing_test")
	var outcome types.MangleAtom
	switch result.ImporterCheck.Verdict() {
	case VerifyPassed:
		outcome = "/passing"
	case VerifyFailed:
		outcome = "/failing"
	case VerifyIndeterminate, VerifyCanceled:
		outcome = "/unfinished"
	default:
		return
	}
	e.assertTurnFact(types.Fact{
		Predicate: "turn_importer_measured",
		Args:      []any{turn, outcome},
	})
	if outcome != "/failing" {
		return
	}
	if result.ImporterCheck.BaselineRan {
		for _, name := range result.ImporterCheck.BaselineFailures {
			if name == "" {
				continue
			}
			e.assertTurnFact(types.Fact{
				Predicate: "turn_importer_failed_before",
				Args:      []any{turn, types.MangleString(name)},
			})
		}
	}
	if result.ImporterCheck.Result == nil {
		return
	}
	for _, fact := range turnScopedTestFacts(turn, result.ImporterCheck.Result) {
		if fact.Predicate != "turn_failing_test" {
			continue
		}
		fact.Predicate = "turn_importer_failing_test"
		e.assertTurnFact(fact)
	}
}
