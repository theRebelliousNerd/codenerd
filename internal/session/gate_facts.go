package session

import (
	"path/filepath"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The /test, /vet, /check and /test_run gates are derived (coder_safety.mg).
// These helpers assert the measurements and read the derived turn_gate back.
// They do not choose the verdict. /build and /pinned stay asserted in
// recordBuildState: one source per gate.

// retractTurnPredicates removes this turn's rows of each predicate and drops
// them from the cleanup list. RetractFact matches the predicate and the first
// argument, which is the turn for every fact these gates assert.
func (e *Executor) retractTurnPredicates(turn types.MangleAtom, predicates ...string) {
	if e == nil || e.kernel == nil || turn == "" || len(predicates) == 0 {
		return
	}
	drop := make(map[string]struct{}, len(predicates))
	for _, predicate := range predicates {
		drop[predicate] = struct{}{}
		if err := e.kernel.RetractFact(types.Fact{Predicate: predicate, Args: []any{turn}}); err != nil {
			logging.Get(logging.CategorySession).Warn("gate facts: retract %s(%s): %v", predicate, turn, err)
		}
	}
	e.mu.Lock()
	kept := make([]types.Fact, 0, len(e.turnFacts))
	for _, fact := range e.turnFacts {
		if _, ok := drop[fact.Predicate]; ok && len(fact.Args) > 0 && types.ExtractString(fact.Args[0]) == string(turn) {
			continue
		}
		kept = append(kept, fact)
	}
	e.turnFacts = kept
	e.mu.Unlock()
}

// syncTestGateFacts replaces this turn's /test measurements with the head
// run as it stands now. A repair re-runs the suite; the previous rows would
// otherwise stay, and RetractFact would not tell two messages for one test
// apart. test_state is not touched: a suite that fails only on tests that
// already failed is still a failing suite, and the turn is not charged for it.
func (e *Executor) syncTestGateFacts(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || turn == "" {
		return
	}
	e.retractTurnPredicates(turn,
		"turn_test_measured", "turn_test_failed_before",
		"turn_test_case", "turn_test_failure_at", "turn_test_build_failure",
		"turn_test_output_repeat", "turn_failing_test")
	switch result.TestCheck.Verdict() {
	case VerifyPassed:
		e.assertTurnFact(types.Fact{
			Predicate: "turn_test_measured",
			Args:      []any{turn, types.MangleAtom("/passing")},
		})
	case VerifyFailed:
		e.assertTurnFact(types.Fact{
			Predicate: "turn_test_measured",
			Args:      []any{turn, types.MangleAtom("/failing")},
		})
	}
	if result.TestCheck.BaselineRan {
		for _, name := range result.TestCheck.BaselineFailures {
			if name == "" {
				continue
			}
			e.assertTurnFact(types.Fact{
				Predicate: "turn_test_failed_before",
				Args:      []any{turn, types.MangleString(name)},
			})
		}
	}
	if result.TestCheck.Ran && result.TestCheck.Result != nil {
		for _, fact := range turnScopedTestFacts(turn, result.TestCheck.Result) {
			e.assertTurnFact(fact)
		}
	}
}

// syncVetGateFacts replaces this turn's /vet measurements. A conclusive run
// asserts turn_vet_ran and the counts verifyVet stored. A hand-built failure
// whose output is a positioned diagnostic is the same measurement, with no
// baseline, so every finding it names is charged. A failure that names
// nothing asserts nothing.
func (e *Executor) syncVetGateFacts(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || turn == "" {
		return
	}
	e.retractTurnPredicates(turn, "turn_vet_ran", "turn_vet_finding", "turn_vet_before")
	v := result.VetCheck
	now, before := v.VetNow, v.VetBefore
	beforeKnown := v.VetBeforeKnown
	if !v.VetMeasured {
		if v.Verdict() != VerifyFailed {
			return
		}
		now = vetDiagnostics(v.Output, func(p string) string { return filepath.ToSlash(p) })
		if len(now) == 0 {
			return
		}
		before, beforeKnown = nil, false
	}
	e.assertTurnFact(types.Fact{Predicate: "turn_vet_ran", Args: []any{turn}})
	e.assertVetCounts(turn, "turn_vet_finding", now)
	if beforeKnown {
		e.assertVetCounts(turn, "turn_vet_before", before)
	}
}

func (e *Executor) assertVetCounts(turn types.MangleAtom, predicate string, findings []vetDiagnostic) {
	type key struct{ file, message string }
	counts := make(map[key]int)
	var order []key
	for _, finding := range findings {
		k := key{finding.file, finding.message}
		if counts[k] == 0 {
			order = append(order, k)
		}
		counts[k]++
	}
	for _, k := range order {
		e.assertTurnFact(types.Fact{
			Predicate: predicate,
			Args:      []any{turn, types.MangleString(k.file), types.MangleString(k.message), int64(counts[k])},
		})
	}
}

// derivedGate reads the turn_gate atoms policy derived (or the executor
// asserted, for /build and /pinned) for this turn and gate.
func (e *Executor) derivedGate(turn types.MangleAtom, gate string) (pass, fail bool) {
	if e == nil || e.kernel == nil || turn == "" {
		return false, false
	}
	facts, err := e.kernel.Query("turn_gate")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("gate facts: query turn_gate: %v", err)
		return false, false
	}
	for _, fact := range facts {
		// Walk the args. An integer above 1 in a condition is counted as an
		// executive knob (defaults/executive_literals_test.go); this is an
		// arity check, so it peels one argument at a time.
		rest := fact.Args
		if len(rest) == 0 || types.ExtractString(rest[0]) != string(turn) {
			continue
		}
		rest = rest[1:]
		if len(rest) == 0 || types.ExtractString(rest[0]) != gate {
			continue
		}
		rest = rest[1:]
		if len(rest) == 0 {
			continue
		}
		switch types.ExtractString(rest[0]) {
		case "/passing":
			pass = true
		case "/failing":
			fail = true
		}
	}
	return pass, fail
}

// testGateRed is the repair loop's question: does policy charge this turn
// with a test failure? With no kernel there is nothing to ask, and the raw
// exit is what the loop has. That path does not record a verdict.
func (e *Executor) testGateRed(turn types.MangleAtom, result *ExecutionResult) bool {
	if result == nil {
		return false
	}
	if e == nil || e.kernel == nil {
		return result.TestCheck.Verdict() == VerifyFailed
	}
	e.syncTestGateFacts(turn, result)
	_, fail := e.derivedGate(turn, "/test")
	return fail
}

// testGatePassed is the repair recheck's question, the other side of
// testGateRed. A failing suite whose every failure predates the turn is
// passed here and still a failing suite on TestCheck.
func (e *Executor) testGatePassed(turn types.MangleAtom, result *ExecutionResult) bool {
	if result == nil {
		return false
	}
	if e == nil || e.kernel == nil {
		return result.TestCheck.Verdict() == VerifyPassed
	}
	e.syncTestGateFacts(turn, result)
	pass, _ := e.derivedGate(turn, "/test")
	return pass
}

func (e *Executor) vetGateRed(turn types.MangleAtom, result *ExecutionResult) bool {
	if result == nil {
		return false
	}
	if e == nil || e.kernel == nil {
		return result.VetCheck.Verdict() == VerifyFailed
	}
	e.syncVetGateFacts(turn, result)
	_, fail := e.derivedGate(turn, "/vet")
	return fail
}

func (e *Executor) vetGatePassed(turn types.MangleAtom, result *ExecutionResult) bool {
	if result == nil {
		return false
	}
	if e == nil || e.kernel == nil {
		return result.VetCheck.Verdict() == VerifyPassed
	}
	e.syncVetGateFacts(turn, result)
	pass, _ := e.derivedGate(turn, "/vet")
	return pass
}

// testRunGatePassed is the /test_run repair's question. The receipts are
// already asserted; this only reads what policy derived.
func (e *Executor) testRunGatePassed(turn types.MangleAtom) bool {
	if e == nil || e.kernel == nil {
		return false
	}
	pass, _ := e.derivedGate(turn, "/test_run")
	return pass
}

// noteWrite records one successful write on the turn's seq clock.
func (e *Executor) noteWrite(result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	result.gateSeq++
	result.writeSequenced = true
	e.assertTurnFact(types.Fact{
		Predicate: "turn_write_seq",
		Args:      []any{result.turnAtom(), int64(result.gateSeq)},
	})
}

// noteTestRun records one test process on the same clock.
func (e *Executor) noteTestRun(result *ExecutionResult, exitCode int) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	result.gateSeq++
	result.testRunSequenced = true
	e.assertTurnFact(types.Fact{
		Predicate: "turn_test_run",
		Args:      []any{result.turnAtom(), int64(result.gateSeq), int64(exitCode)},
	})
}

// noteCheckRun records one run_check on the same clock.
func (e *Executor) noteCheckRun(result *ExecutionResult, exitCode int) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	result.gateSeq++
	e.assertTurnFact(types.Fact{
		Predicate: "turn_check_run",
		Args:      []any{result.turnAtom(), int64(result.gateSeq), int64(exitCode)},
	})
}

// ensureWriteSequenced asserts one write at the start of the clock when the
// turn already wrote and nothing has been sequenced yet. Forcing rounds and
// the closure see writes that were recorded on the result rather than through
// the tool loop; those writes are one moment, before any later run. A clock
// the tool loop already advanced is left alone: that write is already a fact,
// or a run was recorded first and a write invented after it would hide the run.
func (e *Executor) ensureWriteSequenced(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || result.writeSequenced || result.SuccessfulWriteTools == 0 {
		return
	}
	if result.gateSeq > 0 {
		result.writeSequenced = true
		return
	}
	result.gateSeq++
	result.writeSequenced = true
	e.assertTurnFact(types.Fact{
		Predicate: "turn_write_seq",
		Args:      []any{turn, int64(result.gateSeq)},
	})
}

// ensurePointerTestRun asserts the test run a caller recorded on the result
// when the tool loop did not. It is after the writes already sequenced.
func (e *Executor) ensurePointerTestRun(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || result.testRunSequenced || result.TestRunSinceLastWrite == nil {
		return
	}
	result.gateSeq++
	result.testRunSequenced = true
	e.assertTurnFact(types.Fact{
		Predicate: "turn_test_run",
		Args:      []any{turn, int64(result.gateSeq), int64(result.TestRunSinceLastWrite.ExitCode)},
	})
}
