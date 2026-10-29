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
// and turn_owes_gate is asked. The measure is the seq clock (turn_test_run,
// turn_write_seq). The pointer is the shortfall text, not the verdict. A round
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
	if result == nil || result.WriteToolEffects == 0 || trp == nil || e.kernel == nil {
		return nil, nil, nil
	}
	if e.sessionContext != nil && e.sessionContext.DreamMode {
		return nil, nil, nil
	}
	// Writes already on the result are one moment, before a run this round
	// starts. A clock the tool loop advanced already has that write.
	e.ensureWriteSequenced(result.turnAtom(), result)
	// When the workspace says how it tests what was written, the executor
	// plans and runs that, whatever test process the model already started: a
	// passing process proves only the obligation it matches.
	planned, gateOutput := e.workspaceTestRun(ctx, result)
	// A run recorded only on the pointer (the closure's other path) is the
	// same measurement. The tool loop already sequenced its own.
	e.ensurePointerTestRun(result.turnAtom(), result)
	if e.testRunGatePassed(result.turnAtom()) {
		return nil, nil, nil
	}
	if planned && !planRepairable(result) {
		// What is owed cannot be met by running more tests: a declared check
		// that cannot start, or a run that was stopped. The verdict names it.
		logging.Get(logging.CategorySession).Warn("the workspace's test plan is unmet and not repairable by a test run: %s", testRunShortfall(result))
		return nil, nil, nil
	}
	seed := testRunShortfall(result)
	if planned {
		seed += "\n\n" + gateOutput
	}
	logging.Get(logging.CategorySession).Warn("this turn's writes owe a test run: %s; giving the model rounds to run one", seed)
	spec := repairSpec{
		kind:         "test_run",
		brokenPhrase: "no test run passed after this turn's last write",
		promptFor: func(shortfall string) string {
			return testRunPrompt(result.WrittenPaths, shortfall)
		},
		recheck: func(ctx context.Context) (bool, repairFailure, VerifyOutcome) {
			out := ""
			if planned {
				// The workspace's checks decide, not a run the model picked
				// after its repair: plan and run them again on what the repair
				// left. Every outcome is recorded, so a check that can no
				// longer run is unavailable, not silently the old receipt.
				_, out = e.workspaceTestRun(ctx, result)
			}
			if e.testRunGatePassed(result.turnAtom()) {
				return true, repairFailure{}, VerifyPassed
			}
			if planned {
				return false, repairFailure{Output: testRunShortfall(result) + "\n\n" + out}, VerifyFailed
			}
			// The shortfall is a missing run, not a red suite.
			return false, repairFailure{Output: testRunShortfall(result)}, VerifyFailed
		},
		followups: func() []string {
			return []string{"run_tests over the tests that exercise " + strings.Join(result.WrittenPaths, ", ")}
		},
	}
	repaired, repairErrs, _, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, seed, spec)
	return repaired, repairErrs, settleForcingRepair(err)
}

// testRunShortfall says what the /test_run gate saw since the turn's last
// write: no test process at all, or one that failed.
func testRunShortfall(result *ExecutionResult) string {
	var parts []string
	if plan := planShortfall(result); plan != "" {
		parts = append(parts, plan)
	}
	run := result.TestRunSinceLastWrite
	switch {
	case run != nil && run.ExitCode != 0:
		parts = append(parts, fmt.Sprintf("the last test process after this turn's last write failed: `%s` exited %d", strings.Join(run.Argv, " "), run.ExitCode))
	case run == nil && len(result.testPlan) == 0:
		return "no test process ran after this turn's last write"
	}
	if len(parts) == 0 {
		return "no test process passed after this turn's last write"
	}
	return strings.Join(parts, "; ")
}

// planRepairable reports whether a test run could still change the plan's
// state: some obligation is failed, collected no test, or has no declared
// check, or the model's own last process failed.
func planRepairable(result *ExecutionResult) bool {
	if run := result.TestRunSinceLastWrite; run != nil && run.ExitCode != 0 {
		return true
	}
	for _, o := range result.testPlan {
		if obligationOpen(o) {
			return true
		}
	}
	return false
}

func testRunPrompt(written []string, shortfall string) string {
	return "This turn wrote files the build and test gates do not check -- " + strings.Join(written, ", ") +
		" -- and " + shortfall + ".\n\n" +
		"Run the tests that exercise these files with run_tests: the package or suite that loads them, not a " +
		"test you write to read them back. When the run fails, fix the cause and run it again. A run that " +
		"passes after your last edit is what this round needs; the turn's verdict reads it."
}
