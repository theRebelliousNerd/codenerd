package session

import (
	"context"
	"strings"

	"codenerd/internal/broker"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// survivorsPrompt hands the model the decisions its change makes that no test
// distinguishes, on a turn whose pin gate passed.
//
// The pin gate does not charge them, and nor does this round: five of eight
// survivors on the change that introduced the gate were guards whose forcing
// changes nothing observable (pin_gate.go), and telling a guard from a real
// decision is a reading of the code. The model makes that reading, and writes
// the test that takes the other side of each real one.
func survivorsPrompt(survivors []string) string {
	return "The tests pass and pin every function this change touched, but these decisions in it are not " +
		"distinguished by any test: held at a constant, every test still passes.\n\n```\n" +
		strings.Join(survivors, "\n") + "\n```\n\n" +
		"For each one that is a real choice -- where the other branch would give a caller a different result -- " +
		"write a test that takes the other side. A test you write that shows the change is wrong at that branch " +
		"is the point of this round: fix the code in the same edits. A guard whose other branch changes nothing " +
		"observable needs no test; say so in one line. The build and tests are checked again after you finish, " +
		"and edits that break them are undone."
}

// adviseOnSurvivors is the /survivors round: one advisory round on the pin
// gate's surviving condition mutants, like the critic's uplift. It cannot fail
// the turn; edits that break the build or the tests are restored
// (recheckUplift), and the rounds after it re-measure what it wrote.
func (e *Executor) adviseOnSurvivors(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) ([]string, error) {
	if result == nil || trp == nil || len(result.PinAdvisory) == 0 || result.PinCheck.Verdict() != VerifyPassed {
		return nil, nil
	}
	history = append(history, types.Message{Role: "user", Text: survivorsPrompt(result.PinAdvisory)})
	resp, err := e.completeWithWorkingContext(broker.WithPhase(ctx, broker.PhaseSurvivors), trp, systemPrompt, history, toolDefs)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("survivors round failed (%v); turn continues", err)
		return nil, nil
	}
	if resp == nil || len(resp.ToolCalls) == 0 {
		return nil, nil
	}
	workspace := e.workspaceForVerification()
	snap := snapshotTurnFiles(workspace, result)
	_, errs := e.executeToolBatch(ctx, resp.ToolCalls, cfg, result)
	return errs, recheckUplift(ctx, workspace, result, snap)
}
