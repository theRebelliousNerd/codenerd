package session

import (
	"context"
	"encoding/json"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// MemoryHydrator is the optional capability a VirtualStore can expose to make
// persisted memory readable on the session-executor path. Without it the
// executor persists every turn but starts each turn with no learned facts and
// no prior-session context in the kernel — memory is write-only there.
//
// The executor type-asserts e.virtualStore against this interface; when the
// store does not implement it (e.g. a nil store or a stub adapter), hydration
// is simply skipped — a graceful fallback that leaves behavior identical to
// before.
//
// Implemented by *core.VirtualStore (see virtual_store_predicates.go).
type MemoryHydrator interface {
	// HydrateLearnings loads learned facts from knowledge.db and asserts them
	// into the kernel.
	HydrateLearnings(ctx context.Context) (int, error)
	// HydrateSessionContext loads short-term context (session turns, similar
	// content, traces) into the kernel for the current turn.
	HydrateSessionContext(ctx context.Context, sessionID, query string, shardTypes []string) (int, error)
}

// hydrateMemory reads memory back into the kernel before prompt compilation:
// learned facts once per executor lifetime (first Process call) and session
// context once per turn. Errors are logged at Warn and never fail the turn.
func (e *Executor) hydrateMemory(ctx context.Context, input string) {
	if e == nil || e.virtualStore == nil {
		return
	}
	hydrator, ok := e.virtualStore.(MemoryHydrator)
	if !ok || hydrator == nil {
		return
	}
	e.learningsOnce.Do(func() {
		if _, err := hydrator.HydrateLearnings(ctx); err != nil {
			logging.Get(logging.CategorySession).Warn("HydrateLearnings failed: %v", err)
		}
	})
	e.mu.RLock()
	sessionID := e.sessionID
	e.mu.RUnlock()
	if _, err := hydrator.HydrateSessionContext(ctx, sessionID, input, nil); err != nil {
		logging.Get(logging.CategorySession).Warn("HydrateSessionContext failed for session %q: %v", sessionID, err)
	}
}

// turnUsage snapshots one side of a per-turn token delta.
type turnUsage struct {
	prompt     int64
	completion int64
}

// snapshotTurnUsage reads the usage tracker's counts for this turn when the
// context carries a turn id (ProcessWithIntent tags one), else for this
// session. Project totals are merged across processes and session totals are
// shared by every concurrent executor in a campaign, so deltas over either
// count other work (observed live: 455K "prompt tokens" from three fix runs;
// 4.9 M from sibling shards). Per-turn counts are exact and local. It returns
// zeros when no tracker is present.
func snapshotTurnUsage(ctx context.Context, sessionID string) turnUsage {
	tracker := usage.FromContext(ctx)
	if tracker == nil {
		return turnUsage{}
	}
	var counts usage.TokenCounts
	if turnID := usage.TurnIDFromContext(ctx); turnID != "" {
		counts = tracker.TurnTokens(turnID)
	} else {
		counts = tracker.SessionTokens(sessionID)
	}
	return turnUsage{prompt: counts.Input, completion: counts.Output}
}

// delta returns the token growth between two snapshots, clamped at zero so a
// tracker reset between snapshots cannot produce a negative cost.
func (u turnUsage) delta(after turnUsage) (promptTokens, completionTokens int64) {
	promptTokens = after.prompt - u.prompt
	if promptTokens < 0 {
		promptTokens = 0
	}
	completionTokens = after.completion - u.completion
	if completionTokens < 0 {
		completionTokens = 0
	}
	return promptTokens, completionTokens
}

// turnTelemetry carries per-turn compilation and usage data into persistTurn
// as a single parameter object.
type turnTelemetry struct {
	compileResult *prompt.CompilationResult
	usageBefore   turnUsage
}

// compilationAtomsJSON renders the JIT compiler's selected atom IDs as a JSON
// array for session_turns.atoms_json. It returns "[]" when compilation was
// skipped or selected no atoms.
//
// It shares turnAtomIDs with the learning path deliberately: persistence and
// credit assignment must agree on which atoms were in a prompt, or an atom
// blamed for a failure is not the one the turn recorded.
func compilationAtomsJSON(compileResult *prompt.CompilationResult) string {
	ids := turnAtomIDs(turnTelemetry{compileResult: compileResult})
	if ids == nil {
		ids = []string{}
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// captureTurnOutcome records the kernel's verdict on the result BEFORE the
// per-turn facts are retracted: checkHollowSuccess calls this between the
// verdict and the deferred cleanup, while turn_done is still derivable. After
// cleanup the derivation is gone and turn_cost could never record /done.
//
// This is the ONLY place the outcome is decided. The switch below maps derived
// facts onto atoms; it does not re-check the build, re-read the response, or
// gate the kernel read behind a Go precondition. It used to do the last of
// those — the turn_done query sat inside `if result.Acceptance != nil &&
// result.Acceptance.Status == "verified"`, which is the acceptance conjunct of
// the rule restated in Go, in front of the rule. On a turn without a contract
// the kernel was never asked, so /done was unreachable whatever the corpus
// derived.
func (e *Executor) captureTurnOutcome(turn types.MangleAtom, result *ExecutionResult, hollowErr error) {
	if result == nil {
		return
	}
	verdict := e.consumeTurnDoneSignal(turn, strings.TrimSpace(result.Intent.Verb))
	result.MissingEvidence = verdict.Missing

	switch {
	case result.Error != nil:
		// An error that survived the turn (an unrecovered tool failure) is
		// the turn's outcome whatever else the kernel derived; the hollow
		// reason, if any, does not overwrite a real error. On the ordinary
		// path result.Error is still nil here -- the caller sets it from
		// hollowErr after this returns -- so the next arm decides.
		result.TurnOutcome = types.MangleAtom("/failed")
	case hollowErr != nil:
		// /hollow is a failure with a reason, and it stays its own atom:
		// TurnRecord.Failed() counts it, and the chat routes it back to the
		// same shard as /incomplete, which /failed does not do.
		result.TurnOutcome = types.MangleAtom("/hollow")
	case verdict.BuildFailed:
		result.TurnOutcome = types.MangleAtom("/failed")
	case verdict.Done:
		result.TurnOutcome = types.MangleAtom("/done")
	default:
		result.TurnOutcome = types.MangleAtom("/unverified")
	}
}

// resolveTurnOutcome returns the verdict captureTurnOutcome recorded.
//
// It does not re-derive. The kernel re-query that used to live here ran from
// persistTurn, long after cleanupPerTurnCoverageFacts had retracted this turn's
// evidence — it was asking a kernel that had already forgotten the turn — and a
// second derivation path for the same question is exactly the "two truths
// coexist" the no-shims rule forbids.
//
// The error classification below is not a second path: it is the answer for a
// turn that never reached the verdict at all, because something threw before
// checkHollowSuccess ran and TurnOutcome was never set.
func (e *Executor) resolveTurnOutcome(result *ExecutionResult) types.MangleAtom {
	if result != nil && result.TurnOutcome != "" {
		return result.TurnOutcome
	}
	if result != nil && result.Error != nil {
		if isHollowSuccessError(result.Error) {
			return types.MangleAtom("/hollow")
		}
		return types.MangleAtom("/failed")
	}
	return types.MangleAtom("/unverified")
}

// turnCost carries one turn's cost denominator into assertTurnCost as a single
// parameter object.
type turnCost struct {
	sessionID        string
	turnNumber       int
	promptTokens     int64
	completionTokens int64
	toolCalls        int
	outcome          types.MangleAtom
}

// assertTurnCost records one turn_cost fact per turn — the denominator for
// tokens-per-verified-work — and logs the per-turn cost line. Best-effort: an
// assert failure is logged but never fails the turn.
func (e *Executor) assertTurnCost(cost turnCost) {
	if e.kernel != nil {
		fact := types.Fact{Predicate: "turn_cost", Args: []any{
			cost.sessionID,
			int64(cost.turnNumber),
			cost.promptTokens,
			cost.completionTokens,
			int64(cost.toolCalls),
			cost.outcome,
		}}
		if err := e.kernel.Assert(fact); err != nil {
			logging.Get(logging.CategorySession).Warn("Failed to assert turn_cost for session %s turn %d: %v",
				cost.sessionID, cost.turnNumber, err)
		}
	}
	logging.Get(logging.CategorySession).Info("turn_cost session=%s turn=%d prompt=%d completion=%d tools=%d outcome=%s",
		cost.sessionID, cost.turnNumber, cost.promptTokens, cost.completionTokens, cost.toolCalls, cost.outcome)
}
