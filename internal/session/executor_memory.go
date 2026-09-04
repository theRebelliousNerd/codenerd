package session

import (
	"context"
	"encoding/json"

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
func compilationAtomsJSON(compileResult *prompt.CompilationResult) string {
	ids := []string{}
	if compileResult != nil {
		for _, atom := range compileResult.IncludedAtoms {
			if atom == nil || atom.ID == "" {
				continue
			}
			ids = append(ids, atom.ID)
		}
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
func (e *Executor) captureTurnOutcome(result *ExecutionResult, hollowErr error) {
	if result == nil {
		return
	}
	switch {
	case hollowErr != nil:
		result.TurnOutcome = types.MangleAtom("/hollow")
	case result.Error != nil:
		result.TurnOutcome = types.MangleAtom("/failed")
	default:
		outcome := types.MangleAtom("/unverified")
		if e.kernel != nil {
			doneFacts, err := e.kernel.Query("turn_done")
			if err == nil && len(doneFacts) > 0 {
				outcome = types.MangleAtom("/done")
			}
		}
		result.TurnOutcome = outcome
	}
}

// resolveTurnOutcome maps a finished turn to its turn_cost VerifiedOutcome:
// /done when the kernel derived turn_done, /hollow when hollow_success fired,
// /failed when the turn errored, /unverified otherwise.
func (e *Executor) resolveTurnOutcome(result *ExecutionResult) types.MangleAtom {
	if result != nil && result.TurnOutcome != "" {
		return result.TurnOutcome
	}
	if e.kernel != nil {
		doneFacts, err := e.kernel.Query("turn_done")
		if err == nil && len(doneFacts) > 0 {
			return types.MangleAtom("/done")
		}
		hollowFacts, herr := e.kernel.Query("hollow_success")
		if herr == nil && len(hollowFacts) > 0 {
			return types.MangleAtom("/hollow")
		}
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
