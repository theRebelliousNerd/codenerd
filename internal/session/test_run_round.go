package session

import (
	"context"
	"fmt"
	"strings"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// verifyAndRepairTestRun is the forcing round for the /test_run gate
// (external audit N01, its follow-on). The policy decides from each written
// path's extension what the write owes (coder_safety.mg turn_owes_gate): Go
// owes the build and test gates the executor runs itself; anything but Go and
// documentation -- a policy file, a script, a config -- owes a test process
// the model started after its last write, and that run's exit decides. Until
// this round a turn that owed one and never ran it ended /unverified with the
// run named, and nothing sent it back to run it.
//
// The obligation is the kernel's: the turn's writes are asserted under its key
// and turn_owes_gate is asked. The measure is Go's: the last test process the
// tool layer recorded since the last write (TestRunSinceLastWrite). A round
// that does not converge leaves /test_run_not_green to the verdict; it does
// not fail the turn.
func (e *Executor) verifyAndRepairTestRun(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || result.SuccessfulWriteTools == 0 || trp == nil || e.kernel == nil {
		return nil, nil, nil
	}
	if e.sessionContext != nil && e.sessionContext.DreamMode {
		return nil, nil, nil
	}
	if !e.turnOwesGate(result, "/test_run") || result.testRunVerdict() == VerifyPassed {
		return nil, nil, nil
	}
	seed := testRunShortfall(result)
	logging.Get(logging.CategorySession).Warn("this turn's writes owe a test run: %s; giving the model rounds to run one", seed)
	spec := repairSpec{
		kind:         "test_run",
		brokenPhrase: "no test run passed after this turn's last write",
		promptFor: func(shortfall string) string {
			return testRunPrompt(result.WrittenPaths, shortfall)
		},
		recheck: func(context.Context) (bool, string, VerifyOutcome) {
			if result.testRunVerdict() == VerifyPassed {
				return true, "", VerifyPassed
			}
			return false, testRunShortfall(result), VerifyFailed
		},
		followups: func() []string {
			return []string{"run_tests over the tests that exercise " + strings.Join(result.WrittenPaths, ", ")}
		},
	}
	repaired, repairErrs, _, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, seed, spec)
	return repaired, repairErrs, settleForcingRepair(err)
}

// turnOwesGate asks the policy whether this turn's writes owe gate. The
// writes are asserted under the turn's own key -- the closure asserts its
// verdict under the same one -- so the answer is the corpus's, not a Go
// reading of the extensions.
func (e *Executor) turnOwesGate(result *ExecutionResult, gate string) bool {
	turn := result.turnAtom()
	e.assertTurnVerb(turn, result.Intent.Verb, result)
	e.assertTurnWrites(turn, result)
	rows, err := e.turnRows("turn_owes_gate", turn)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("turn_owes_gate query failed; no %s round: %v", gate, err)
		return false
	}
	for _, row := range rows {
		if len(row.Args) > 1 && types.ExtractString(row.Args[1]) == gate {
			return true
		}
	}
	return false
}

// testRunShortfall says what the /test_run gate saw since the turn's last
// write: no test process at all, or one that failed.
func testRunShortfall(result *ExecutionResult) string {
	run := result.TestRunSinceLastWrite
	if run == nil {
		return "no test process ran after this turn's last write"
	}
	return fmt.Sprintf("the last test process after this turn's last write failed: `%s` exited %d", strings.Join(run.Argv, " "), run.ExitCode)
}

func testRunPrompt(written []string, shortfall string) string {
	return "This turn wrote files the build and test gates do not check -- " + strings.Join(written, ", ") +
		" -- and " + shortfall + ".\n\n" +
		"Run the tests that exercise these files with run_tests: the package or suite that loads them, not a " +
		"test you write to read them back. When the run fails, fix the cause and run it again. A run that " +
		"passes after your last edit is what this round needs; the turn's verdict reads it."
}
