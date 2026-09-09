package system

import (
	"strings"
	"testing"
	"time"

	pe "codenerd/internal/autopoiesis/prompt_evolution"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

func turnRec(outcome string) session.TurnRecord {
	return session.TurnRecord{
		SessionID:  "sess-1",
		TurnNumber: 4,
		IntentVerb: "/fix",
		Task:       "fix the parser",
		Response:   "done",
		Outcome:    types.MangleAtom(outcome),
		AtomIDs:    []string{"core/identity", "go/errors"},
		Duration:   3 * time.Second,
		Provider:   "anthropic",
		Model:      "claude-opus-5",
	}
}

// TestExecutionRecordFor_VerifiedTurnSkipsTheJudge pins the cost control. The
// kernel derived turn_done by watching the evidence land, so paying an LLM to
// re-decide what the kernel already knows would make the learning loop cost
// more than the work it learns from.
func TestExecutionRecordFor_VerifiedTurnSkipsTheJudge(t *testing.T) {
	rec, ok := executionRecordFor(turnRec("/done"))
	if !ok {
		t.Fatal("a verified turn must be recorded")
	}
	if rec.Verdict == nil {
		t.Fatal("a verified turn must carry a pre-filled verdict, or the judge will be billed for it")
	}
	if !rec.Verdict.IsPass() {
		t.Errorf("verdict = %q, want PASS", rec.Verdict.Verdict)
	}
	if !rec.ExecutionResult.Success {
		t.Error("ExecutionResult.Success must be true for a verified turn")
	}
	if rec.Verdict.EvaluatedBy != "mangle-kernel" {
		t.Errorf("EvaluatedBy = %q; the verdict's author must be attributable", rec.Verdict.EvaluatedBy)
	}
	// The atoms must ride along on the verdict: applyExecutionOutcomeLocked
	// credits them from there, and that credit is what promotion is scored on.
	if len(rec.Verdict.AtomIDs) != 2 {
		t.Errorf("verdict carries %d atom IDs, want 2 — atom credit assignment is broken",
			len(rec.Verdict.AtomIDs))
	}
}

// TestExecutionRecordFor_HollowIsAFailureTheJudgeExplains is the correctness
// half. err == nil would have recorded this turn as a win.
func TestExecutionRecordFor_HollowIsAFailureTheJudgeExplains(t *testing.T) {
	rec, ok := executionRecordFor(turnRec("/hollow"))
	if !ok {
		t.Fatal("a hollow turn is the most valuable failure there is; it must be recorded")
	}
	if rec.ExecutionResult.Success {
		t.Fatal("a hollow success was recorded as a success — the learner is being trained on the agent's own say-so")
	}
	if rec.Verdict != nil {
		t.Fatal("a failure must reach the judge unjudged: an atom is generated from the judge's explanation")
	}
	joined := strings.Join(rec.ExecutionResult.BuildErrors, "\n")
	if !strings.Contains(strings.ToLower(joined), "hollow success") {
		t.Errorf("BuildErrors = %q, want the kernel's reason so the judge has something to explain", joined)
	}
}

func TestExecutionRecordFor_FailedTurnReachesTheJudge(t *testing.T) {
	in := turnRec("/failed")
	in.Err = errFake("compile failed: undefined x")

	rec, ok := executionRecordFor(in)
	if !ok {
		t.Fatal("a failed turn must be recorded")
	}
	if rec.Verdict != nil {
		t.Fatal("a failure must reach the judge unjudged")
	}
	if !strings.Contains(strings.Join(rec.ExecutionResult.BuildErrors, "\n"), "undefined x") {
		t.Errorf("the turn's own error was dropped: %v", rec.ExecutionResult.BuildErrors)
	}
}

// TestExecutionRecordFor_UnverifiedIsDropped keeps answered questions out of
// the failure buckets. A read-only turn produces no acceptance evidence, so
// recording it either pollutes the corpus or buys a judge call to learn that
// nothing happened.
func TestExecutionRecordFor_UnverifiedIsDropped(t *testing.T) {
	for _, outcome := range []string{"/unverified", ""} {
		if _, ok := executionRecordFor(turnRec(outcome)); ok {
			t.Errorf("outcome %q was recorded; only turns that did verifiable work belong in the corpus", outcome)
		}
	}
}

// TestExecutionRecordFor_TaskIDsAreUniqueForConcurrentClones: delegated tasks
// run on executor clones that inherit the parent's session id, so several
// legitimately share one (session, turn) pair. A colliding TaskID would have
// them overwrite each other in the feedback database.
func TestExecutionRecordFor_TaskIDsAreUniqueForConcurrentClones(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		rec, ok := executionRecordFor(turnRec("/done"))
		if !ok {
			t.Fatal("record dropped")
		}
		if seen[rec.TaskID] {
			t.Fatalf("duplicate TaskID %q: concurrent delegated tasks would overwrite each other", rec.TaskID)
		}
		seen[rec.TaskID] = true
	}
}

func TestExecutionRecordFor_CarriesServingProvenance(t *testing.T) {
	// Without provider/model an atom learned from one vendor's failure modes
	// is served to every other vendor.
	rec, _ := executionRecordFor(turnRec("/hollow"))
	if rec.Provider != "anthropic" || rec.Model != "claude-opus-5" {
		t.Errorf("provenance lost: provider=%q model=%q", rec.Provider, rec.Model)
	}
}

func TestExecutionRecordFor_UnnamedVerbStillGroups(t *testing.T) {
	in := turnRec("/done")
	in.IntentVerb = "  "
	rec, ok := executionRecordFor(in)
	if !ok {
		t.Fatal("record dropped")
	}
	if rec.ShardType == "" {
		t.Fatal("an empty ShardType collapses every unnamed verb into the same group as a real one")
	}
}

// TestTurnEvolutionRecorder_NilEvolverIsSafe: boot may fail to create an
// evolver (read-only .nerd/, corrupt database) and must still run.
func TestTurnEvolutionRecorder_NilEvolverIsSafe(t *testing.T) {
	var r *turnEvolutionRecorder
	r.RecordTurn(turnRec("/done"))
	newTurnEvolutionRecorder(nil).RecordTurn(turnRec("/done"))
}

// TestRunEvolutionCycle_IsOptIn: the cycle spends the operator's API budget,
// so a Cortex with the flag off must not start one.
func TestRunEvolutionCycle_IsOptIn(t *testing.T) {
	t.Setenv("CODENERD_PROMPT_EVOLUTION", "0")
	// A nil evolver is the strongest form of the assertion available without
	// standing up a real one: if the guard order were wrong this would panic.
	c := &Cortex{}
	c.runEvolutionCycle(t.Context())
}

var _ session.TurnRecorder = (*turnEvolutionRecorder)(nil)
var _ pe.LLMClient = (perceptionClient)(nil)

type errFake string

func (e errFake) Error() string { return string(e) }

// TestBootWiresTheLearningLoop is the regression guard for the gap this file
// was written to close: for most of this system's life prompt evolution was
// assembled in cmd/nerd/chat only, so `nerd campaign`, `nerd instruction`,
// `nerd spawn` and every delegated shard task booted the same Cortex and
// learned nothing at all. If this test ever fails, the agent has gone back to
// improving only while a human is watching it.
func TestBootWiresTheLearningLoop(t *testing.T) {
	cortex := bootSessionWiringCortex(t, "learning-loop")

	if cortex.PromptEvolver == nil {
		t.Fatal("Cortex.PromptEvolver is nil: this boot path cannot learn from anything it does")
	}
	if cortex.SessionExecutor == nil {
		t.Fatal("Cortex.SessionExecutor is nil")
	}
	if !cortex.SessionExecutor.HasTurnRecorder() {
		t.Fatal("the session executor has no turn recorder: turns run on this path are never learned from")
	}
	if cortex.ContextFeedback == nil {
		t.Fatal("Cortex.ContextFeedback is nil: the model's rating of its own context is discarded on this path")
	}
	if !cortex.SessionExecutor.HasContextFeedbackRecorder() {
		t.Fatal("the session executor has no context feedback recorder")
	}
}

var _ session.ContextFeedbackRecorder = (*contextFeedbackRecorder)(nil)

func TestContextFeedbackRecorder_NilStoreIsSafe(t *testing.T) {
	// Boot may fail to open the store (read-only .nerd/, corrupt database) and
	// the session must still run.
	var r *contextFeedbackRecorder
	r.RecordContextFeedback(session.ContextFeedbackRecord{})
	(&contextFeedbackRecorder{}).RecordContextFeedback(session.ContextFeedbackRecord{})
}
