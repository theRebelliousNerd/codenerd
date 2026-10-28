package session

import (
	"context"
	"errors"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// turnClosure is what the closing of an admitted turn needs besides the
// result: the request, the intent it ran under, and the measurements taken
// before the model was first called.
type turnClosure struct {
	input         string
	intent        perception.Intent
	compileResult *prompt.CompilationResult
	usageBefore   turnUsage
	start         time.Time
}

// turnSucceeded is the one answer to "did this turn succeed" that the audit
// record and the prompt co-use feedback share with the caller: what was
// returned, not a local a return statement may not have assigned.
func turnSucceeded(result *ExecutionResult, err error) bool {
	return err == nil && result != nil && result.Error == nil
}

// settleTurn is the one terminal boundary of an admitted turn. Every path
// that holds an ExecutionResult leaves through it exactly once: the ordinary
// return, a tool failure that survived, and a hard error from the working loop
// (a provider failure after tool work, a continuation-policy stop, a cancel, a
// post-edit gate that could not be satisfied).
//
// primary is that hard error, nil on the ordinary path. It is the turn's
// error: it becomes result.Error and is returned unchanged, so errors.Is still
// finds the provider's, the context's or ErrVerificationFailed. What the loop
// had measured before it failed -- the writes, their preimages, the receipts,
// the checks already run -- is on result and stays there. Before this the hard
// error returned (nil, err): the delegation owner got the failure and lost the
// evidence of every file the turn had already changed, and the audit,
// prompt-co-use, acceptance, turn-cost and learning consumers each settled the
// turn against a result whose Error was still nil.
//
// Closure is not more work. It makes no model call and runs no tool; it
// reports. The final response of a failed turn is never parsed as a
// completion claim and its control packet is never applied -- the loop did not
// finish, so what it held is an unfinished attempt, not an answer. The checks
// the turn did not reach stay unmeasured, and the kernel's verdict over them
// (captureTurnOutcome, turn_missing_evidence) says so.
//
// A canceled turn is closed on a context that keeps its values but not its
// cancellation: the acceptance snapshot and the cost record are in-memory and
// file reads, with no provider and no tool authority, and a context that was
// canceled to stop the work must not also stop the account of the work.
// Learning, which does call a model, is not queued for a turn the caller
// stopped.
func (e *Executor) settleTurn(
	ctx context.Context,
	c turnClosure,
	result *ExecutionResult,
	final *types.LLMToolResponse,
	toolErrs []string,
	primary error,
) (*ExecutionResult, error) {
	result.ToolErrors = append([]string(nil), toolErrs...)
	result.Duration = time.Since(c.start)

	stopped := primary != nil && ctx.Err() != nil
	if primary != nil {
		ctx = context.WithoutCancel(ctx)
		result.Error = primary
		// What the turn can truthfully say: the ledger of a planned task's
		// steps and the notices of controls the executive blocked. Neither is
		// the model's closing word.
		result.Response = strings.TrimSpace(withSafetyNotices(strings.TrimSpace(result.StepReport), result.safetyNotices))
	} else {
		// Whether a mid-turn tool error survived is decided by the closing
		// evidence, not by the shape of the final response (surfaceToolErrors).
		e.surfaceToolErrors(result, toolErrs)
	}

	// Close the turn on every path: checkHollowSuccess asserts the turn's
	// evidence, reads the kernel's verdict once and retracts the per-turn
	// facts. Until 2026-09-18 it sat behind `if result.Error == nil`, so a
	// turn whose tool error survived never had its verdict captured and never
	// had the facts its tool loop asserted retracted -- one errored turn could
	// raise "new source was created without a test file" against every later
	// turn (REVIEW-wave1 F5 / open item 7).
	//
	// Block hollow success: mutation intents that require real side effects
	// must not report completion when the model only returned planning prose
	// (or only non-mutating tools). Dream/shadow runs skip this gate. A real
	// error already on the result outranks the hollow reason.
	//
	// Hollow failures are hard errors (non-nil return) so CLI one-shots exit
	// non-zero. Other soft tool failures stay on result.Error with a nil
	// return for interactive chat compatibility; TaskExecutor still surfaces
	// result.Error for SpawnTask callers.
	e.closeAcceptanceEvidence(ctx, result)
	auditErr := e.auditFinalReport(ctx, c.input, result)
	hollowErr := e.checkHollowSuccess(result)
	// An audit that could not be asked (its role prompt did not compile) is
	// the turn's error too, beside a hollow verdict and never in its place.
	if result.Error == nil {
		if err := errors.Join(hollowErr, auditErr); err != nil {
			result.Error = err
		}
	}
	// The closing evidence sentence comes LAST, because it is a function of
	// the verdict checkHollowSuccess just captured. Written before it (where
	// it used to live) the only thing it could say was "unverified", and it
	// said so on every write turn regardless of what the gates measured.
	e.appendEvidenceSummary(result)

	// The conversation records a request and its reply together. A failed turn
	// that has nothing to say contributes neither: a user entry with no
	// assistant entry after it is a turn the next request would be shown as
	// unanswered.
	if primary == nil || strings.TrimSpace(result.Response) != "" {
		reply := perception.ConversationTurn{Role: "assistant", Content: result.Response}
		if final != nil {
			reply.ThoughtSummary = final.ThoughtSummary
			reply.ThoughtSignature = final.ThoughtSignature
		}
		e.appendToHistory(perception.ConversationTurn{Role: "user", Content: c.input})
		e.appendToHistory(reply)
	}

	// Dispatch asynchronous learning over the conversation WINDOW, not this
	// turn alone. See criticWindow for why one trace could never work. The
	// critic is a model call; a turn its caller stopped does not start one.
	if !stopped {
		e.queueTaxonomyLearning(result, toolErrs)
	}

	// Persist session turn for cross-session continuity
	e.persistTurn(ctx, c.input, c.intent, result, turnTelemetry{compileResult: c.compileResult, usageBefore: c.usageBefore})

	logging.Session("Execution complete: %d tool calls, %v duration", result.ToolCallsExecuted, result.Duration)

	if primary != nil {
		return result, primary
	}
	if result.Error != nil && isHollowSuccessError(result.Error) {
		return result, result.Error
	}
	return result, nil
}
