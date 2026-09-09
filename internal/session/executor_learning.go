package session

import (
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
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

// criticWindow is how many conversation turns (user + assistant entries, so
// half this many exchanges) the taxonomy critic is shown.
//
// The number matters because of what the critic is looking for. Its prompt
// hunts for corrections ("No, I meant..."), frustration ("Stop"), and explicit
// teaching ("When I say X, do Y") — every one of which is a RELATIONSHIP
// BETWEEN consecutive turns, never a property of a single one. Its own worked
// examples are two-turn exchanges.
//
// It used to be handed exactly one turn per call. LearnFromInteraction even
// contains a "take the last 5 turns" truncation that could never fire, because
// the slice it truncated always had length one. So the critic ran an LLM call
// after every single turn and was structurally incapable of finding anything
// it was built to find: a pure cost with a guaranteed empty result.
//
// Ten entries is five exchanges, which is what LearnFromInteraction truncates
// to anyway.
const criticWindow = 10

// minCriticExchanges is the shortest history worth spending a critic call on.
//
// A correction needs something to correct: one exchange cannot contain
// "the agent did X" and "no, I meant Y". Below this the answer is known in
// advance, so the call is skipped rather than paid for.
const minCriticExchanges = 2

// queueTaxonomyLearning hands the recent conversation to the taxonomy critic
// for background pattern extraction.
//
// Success is the kernel's verdict, not the executor's return value. The trace
// is rendered into the critic's transcript as "Success: %v", so a hollow
// success — the turn announced the work as done and the kernel found no
// evidence for it — used to be presented to the critic as work that went
// right. Teaching the classifier from turns the agent only claimed to have
// completed is how a misclassification gets reinforced instead of corrected.
func (e *Executor) queueTaxonomyLearning(result *ExecutionResult, toolErrs []string) {
	if e == nil || perception.SharedTaxonomy == nil || result == nil {
		return
	}

	history := e.GetHistory()
	if len(history) > criticWindow {
		history = history[len(history)-criticWindow:]
	}

	traces := tracesFromHistory(history)
	if len(traces) < minCriticExchanges {
		return
	}

	// The verdict describes the LAST exchange, which is the turn that just
	// finished; earlier ones are context for spotting the correction.
	last := &traces[len(traces)-1]
	last.Success = e.resolveTurnOutcome(result) == types.MangleAtom("/done") && len(toolErrs) == 0

	perception.SharedTaxonomy.QueueForLearning(traces)
}

// tracesFromHistory pairs each user entry with the assistant reply that
// followed it. A trailing user entry with no reply yet is dropped: the critic
// reads "what the user asked, what the agent did", and half an exchange
// carries no signal about either.
func tracesFromHistory(history []perception.ConversationTurn) []perception.ReasoningTrace {
	var traces []perception.ReasoningTrace
	for i := 0; i < len(history); i++ {
		if history[i].Role != "user" {
			continue
		}
		if i+1 >= len(history) || history[i+1].Role != "assistant" {
			continue
		}
		traces = append(traces, perception.ReasoningTrace{
			UserPrompt: history[i].Content,
			Response:   history[i+1].Content,
			// Earlier turns default to false rather than claiming a success
			// this executor cannot re-derive; the critic reads the transcript
			// for corrections, and a correction is visible in the NEXT user
			// message whatever this flag says.
		})
	}
	return traces
}

// ContextFeedbackRecord is one turn's rating of the context it was given.
//
// The ratings come from the model itself, through the Piggyback protocol's
// context_feedback block: which injected predicates earned their tokens, which
// were noise, and what was missing. It is the only signal in the system that
// answers "was the context we assembled any good?" from the one party that
// actually read it.
type ContextFeedbackRecord struct {
	SessionID  string
	TurnNumber int

	// IntentVerb scopes the rating. "file_topology was noise" is a different
	// claim for /review than for /fix, and the store keys on the verb.
	IntentVerb string

	// ManifestHash identifies the exact compiled prompt that was rated, so a
	// rating can be traced back to the context selection that produced it.
	ManifestHash string

	// OverallUsefulness is 0.0 (irrelevant) to 1.0 (exactly what was needed).
	OverallUsefulness float64

	// HelpfulPredicates and NoisePredicates are predicate names, not fact
	// instances: the activation engine tunes per predicate.
	HelpfulPredicates []string
	NoisePredicates   []string

	// MissingContext is the model's description of what it needed and did not
	// get. Free-form, and the only part of the feedback that names something
	// the retrieval side could have supplied and did not.
	MissingContext string

	// Verified carries the kernel's verdict for the turn, so a rating from a
	// turn that only claimed to succeed can be told apart from one that did.
	Verified bool
}

// ContextFeedbackRecorder receives each turn's context rating.
//
// Same contract as TurnRecorder: it is called on the turn's own goroutine and
// must not block.
type ContextFeedbackRecorder interface {
	RecordContextFeedback(ContextFeedbackRecord)
}

// SetContextFeedbackRecorder installs the sink for context ratings. Nil
// disables recording, which is the default.
func (e *Executor) SetContextFeedbackRecorder(r ContextFeedbackRecorder) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.contextFeedbackRecorder = r
}

// HasContextFeedbackRecorder reports whether a context-rating sink is wired,
// so "does this boot path keep the model's context ratings?" is answerable by
// a test rather than by an empty database weeks later.
func (e *Executor) HasContextFeedbackRecorder() bool {
	if e == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.contextFeedbackRecorder != nil
}

// stashContextFeedback holds the piggyback rating until persistTurn, which is
// where the turn number, session id and compiled manifest are all in hand.
//
// One executor processes one turn at a time — that is what CloneForTask exists
// to guarantee — so a single slot is enough, and it is overwritten rather than
// appended so a turn that somehow reports twice keeps its last word.
func (e *Executor) stashContextFeedback(fb *articulation.ContextFeedback) {
	if e == nil || fb == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pendingContextFeedback = fb
}

// takeContextFeedback removes and returns the stashed rating. Clearing on read
// is what keeps one turn's rating from being attributed to the next.
func (e *Executor) takeContextFeedback() *articulation.ContextFeedback {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	fb := e.pendingContextFeedback
	e.pendingContextFeedback = nil
	return fb
}

// recordContextFeedback forwards this turn's rating to the installed sink.
//
// It always clears the stash, even with no sink installed, so an executor that
// is later given one cannot inherit a rating from an unrelated earlier turn.
func (e *Executor) recordContextFeedback(rec ContextFeedbackRecord, telemetry turnTelemetry) {
	fb := e.takeContextFeedback()
	if fb == nil {
		return
	}

	e.mu.RLock()
	recorder := e.contextFeedbackRecorder
	e.mu.RUnlock()
	if recorder == nil {
		return
	}

	rec.OverallUsefulness = fb.OverallUsefulness
	rec.HelpfulPredicates = fb.HelpfulFacts
	rec.NoisePredicates = fb.NoiseFacts
	rec.MissingContext = fb.MissingContext
	rec.ManifestHash = manifestHash(telemetry)

	defer func() {
		if r := recover(); r != nil {
			logging.Get(logging.CategoryContext).Warn(
				"Context feedback recorder panicked for session %s turn %d: %v",
				rec.SessionID, rec.TurnNumber, r)
		}
	}()
	recorder.RecordContextFeedback(rec)
}

// manifestHash identifies the compiled prompt this turn ran on, or "" when
// compilation was skipped.
func manifestHash(telemetry turnTelemetry) string {
	if telemetry.compileResult == nil || telemetry.compileResult.Manifest == nil {
		return ""
	}
	return telemetry.compileResult.Manifest.ContextHash
}
