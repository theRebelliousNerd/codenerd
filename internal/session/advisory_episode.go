package session

import (
	"context"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// adviseEpisode is one advisory episode: the expert critique's repair, the
// critic's uplift and the survivors round each hand the model fallible advice
// and let it act on it or reject it with a reason.
//
// Until 2026-10-29 each was one model response: its tool calls ran and their
// results reached no further response, so a model that read the cited code
// before judging a finding never got to judge it -- the round ended on the
// read. The episode is now a working attempt (workingAttempt): every round's
// results go back to the model, which ends it by editing, by answering in
// words (a reasoned rejection is a valid end), or when the working policy
// stops it. It runs under the caller's phase and the turn's own write rules;
// no repair write guard is added, as before.
//
// Only an episode that changed the turn's files is rechecked
// (recheckUplift): a read-only episode on a turn that already wrote Go buys
// no second build and test run. "Changed" is a write tool's effect or a turn
// file whose bytes differ from the snapshot; the final closure still measures
// whatever the workspace became.
//
// The episode needs the working policy to end it. A turn without one (no
// declared workspace) runs no advisory episode: the advice is a missing
// opinion, logged, never a failed turn.
func (e *Executor) adviseEpisode(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
	prompt, what string,
) ([]string, error) {
	if activeWorkingLoop(ctx) == nil {
		logging.Get(logging.CategorySession).Warn("%s: this turn has no working policy to end an advisory episode; the advice is not acted on", what)
		return nil, nil
	}
	workspace := e.workspaceForVerification()
	snap := snapshotTurnFiles(workspace, result)
	writesBefore := result.WriteToolEffects
	_, calls, _, errs, _, _, err := e.workingAttempt(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, prompt, false)
	if err != nil {
		// Advisory: an episode that could not finish is a missing opinion.
		logging.Get(logging.CategorySession).Warn("%s ended after %d model call(s) (%v); turn continues", what, calls, err)
	}
	if result.WriteToolEffects == writesBefore && !snap.changedOnDisk(workspace) {
		return errs, nil
	}
	return errs, recheckUplift(ctx, workspace, result, snap)
}

// changedOnDisk reports whether any file in the snapshot now differs from the
// bytes it held when the snapshot was taken.
func (snap turnFiles) changedOnDisk(workspace string) bool {
	for _, p := range snap.written {
		if readPreImage(turnFilePath(workspace, p)) != snap.pre[p] {
			return true
		}
	}
	return false
}
