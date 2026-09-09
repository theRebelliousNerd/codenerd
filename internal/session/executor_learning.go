package session

import (
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// TurnRecord is one finished turn described in the terms a learning loop needs
// and nothing else. It deliberately carries no prompt-evolution types: the
// session package is the funnel every execution path runs through (chat turn,
// `nerd run`, campaign task, spawned subagent all reach
// Executor.ProcessWithIntent), so the seam here has to stay narrow enough that
// wiring a learner into it never drags the learner's dependencies into the
// executor.
type TurnRecord struct {
	// SessionID and TurnNumber identify the turn within its session.
	SessionID  string
	TurnNumber int

	// IntentVerb is the canonical Mangle verb this turn ran as ("/fix",
	// "/review"). It is the grouping key a learner buckets failures under.
	IntentVerb string

	// Task is the request the turn was given; Response is what it answered.
	Task     string
	Response string

	// Outcome is the KERNEL's verdict on the turn, not the executor's:
	// /done (verified), /hollow (claimed success with no evidence), /failed,
	// or /unverified. See Verified for why this distinction is the whole point.
	Outcome types.MangleAtom

	// Err is the turn's error, if any. A /hollow turn carries one; a
	// /unverified turn usually does not.
	Err error

	// AtomIDs are the prompt atoms the JIT compiler included for this turn.
	// They are the credit-assignment handle: an atom present on a failing turn
	// is evidence against that atom.
	AtomIDs []string

	// Duration, ToolCalls, PromptTokens and CompletionTokens are the turn's
	// cost, so a learner can weigh an expensive win against a cheap one.
	Duration         time.Duration
	ToolCalls        int
	PromptTokens     int64
	CompletionTokens int64

	// Provider and Model name the LLM that actually served the turn (raw
	// vendor spellings). A failure is evidence about the model that produced
	// it, so a learner that pins what it learns needs the provenance.
	Provider string
	Model    string
}

// Verified reports whether the kernel derived turn_done for this turn.
//
// This is the method that exists to stop a learner grading the agent on the
// agent's own say-so. The obvious success test is "the turn returned no
// error", and it is wrong here: codeNERD has a whole hollow-success gate
// (checkHollowSuccess) precisely because a turn can announce it edited a file,
// or that the tests pass, having done neither. Those turns end at /hollow.
// Training a prompt learner on err == nil rewards the model for producing
// confident prose instead of evidence — it teaches exactly the behaviour the
// hollow-success gate exists to catch.
//
// Only /done means the kernel saw the evidence. Everything else is not a win.
func (r TurnRecord) Verified() bool {
	return r.Outcome == types.MangleAtom("/done")
}

// Failed reports whether this turn is evidence of something going wrong —
// an error, or a success claimed without evidence.
//
// /unverified is deliberately neither Verified nor Failed. Read-only turns
// ("what does this package do?") legitimately produce no acceptance evidence,
// and counting them as failures would bury the real ones under questions.
func (r TurnRecord) Failed() bool {
	return r.Outcome == types.MangleAtom("/hollow") ||
		r.Outcome == types.MangleAtom("/failed")
}

// TurnRecorder receives one record per finished turn.
//
// Implementations MUST NOT block: recordTurn is called on the turn's own
// goroutine at the end of Process, so a recorder that talks to an LLM or waits
// on a lock adds that latency to every user-visible turn. Do the work
// asynchronously behind this call.
type TurnRecorder interface {
	RecordTurn(TurnRecord)
}

// SetTurnRecorder installs the learning sink for finished turns. Passing nil
// disables recording, which is the default: an executor with no recorder
// behaves exactly as it did before this seam existed.
func (e *Executor) SetTurnRecorder(r TurnRecorder) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.turnRecorder = r
}

// HasTurnRecorder reports whether a learning sink is installed.
//
// It exists so "does this boot path learn?" is a question a test and a
// diagnostic can answer, rather than something inferred from the absence of
// records days later. The regression it guards is specific: for most of this
// system's life the learning loop was assembled only in the chat TUI, so every
// headless run silently taught it nothing.
func (e *Executor) HasTurnRecorder() bool {
	if e == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.turnRecorder != nil
}

// recordTurn hands a finished turn to the installed recorder.
//
// Best-effort in the strongest sense: a recorder that panics must not take the
// process with it. The panic guard is here rather than left to implementations
// because this is a plugin seam — the executor cannot audit what gets wired
// into it, and a learning sink is never worth a crashed session.
func (e *Executor) recordTurn(rec TurnRecord) {
	if e == nil {
		return
	}
	e.mu.RLock()
	recorder := e.turnRecorder
	e.mu.RUnlock()
	if recorder == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logging.Get(logging.CategorySession).Warn(
				"Turn recorder panicked for session %s turn %d: %v",
				rec.SessionID, rec.TurnNumber, r)
		}
	}()
	recorder.RecordTurn(rec)
}

// turnAtomIDs lists the prompt atoms the JIT compiler included this turn, in
// compilation order. Returns nil when compilation was skipped.
func turnAtomIDs(telemetry turnTelemetry) []string {
	if telemetry.compileResult == nil {
		return nil
	}
	ids := make([]string, 0, len(telemetry.compileResult.IncludedAtoms))
	for _, atom := range telemetry.compileResult.IncludedAtoms {
		if atom == nil || atom.ID == "" {
			continue
		}
		ids = append(ids, atom.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}
