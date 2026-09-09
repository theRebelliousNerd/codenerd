package session

import (
	"sync"
	"testing"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// recordingSink captures every TurnRecord it is handed.
type recordingSink struct {
	mu      sync.Mutex
	records []TurnRecord
}

func (s *recordingSink) RecordTurn(rec TurnRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, rec)
}

func (s *recordingSink) all() []TurnRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TurnRecord, len(s.records))
	copy(out, s.records)
	return out
}

type panickingSink struct{}

func (panickingSink) RecordTurn(TurnRecord) { panic("recorder blew up") }

// TestTurnRecord_HollowIsNotASuccess is the invariant this whole seam exists
// for. A learner fed `err == nil` credits the prompt atoms of a turn that
// announced it had done the work and produced no evidence — which trains the
// model toward exactly the behaviour the hollow-success gate catches.
func TestTurnRecord_HollowIsNotASuccess(t *testing.T) {
	hollow := TurnRecord{Outcome: types.MangleAtom("/hollow")}

	if hollow.Verified() {
		t.Fatal("a hollow turn must never count as verified work")
	}
	if !hollow.Failed() {
		t.Fatal("a hollow turn is evidence of failure and must be recorded as one")
	}
}

func TestTurnRecord_OutcomeClassification(t *testing.T) {
	for _, tc := range []struct {
		outcome  string
		verified bool
		failed   bool
	}{
		{"/done", true, false},
		{"/hollow", false, true},
		{"/failed", false, true},
		// Neither: a read-only turn produces no acceptance evidence, and
		// counting questions as failures buries the real ones.
		{"/unverified", false, false},
		{"", false, false},
	} {
		rec := TurnRecord{Outcome: types.MangleAtom(tc.outcome)}
		if got := rec.Verified(); got != tc.verified {
			t.Errorf("%s: Verified() = %v, want %v", tc.outcome, got, tc.verified)
		}
		if got := rec.Failed(); got != tc.failed {
			t.Errorf("%s: Failed() = %v, want %v", tc.outcome, got, tc.failed)
		}
	}
}

func TestExecutor_RecordTurn_NoRecorderIsANoOp(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	// Must not panic and must not require any wiring: an executor with no
	// learner behaves exactly as it did before the seam existed.
	e.recordTurn(TurnRecord{SessionID: "s", Outcome: types.MangleAtom("/done")})
}

func TestExecutor_RecordTurn_DeliversToTheSink(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	sink := &recordingSink{}
	e.SetTurnRecorder(sink)

	e.recordTurn(TurnRecord{SessionID: "s1", TurnNumber: 3, Outcome: types.MangleAtom("/done")})

	got := sink.all()
	if len(got) != 1 {
		t.Fatalf("recorder received %d records, want 1", len(got))
	}
	if got[0].SessionID != "s1" || got[0].TurnNumber != 3 {
		t.Errorf("record = %+v, want session s1 turn 3", got[0])
	}
}

// TestExecutor_RecordTurn_SurvivesAPanickingRecorder pins the guard. The
// recorder is a plugin seam the executor cannot audit, and a learning sink is
// never worth a crashed session.
func TestExecutor_RecordTurn_SurvivesAPanickingRecorder(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	e.SetTurnRecorder(panickingSink{})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a panicking recorder escaped recordTurn: %v", r)
		}
	}()
	e.recordTurn(TurnRecord{SessionID: "s", Outcome: types.MangleAtom("/failed")})
}

// TestCloneForTask_InheritsTurnRecorder is the difference between learning
// from everything and learning only from chat. Delegated tasks run on clones:
// a campaign is thousands of them, and a clone that dropped the recorder would
// leave every autonomous run teaching the system nothing.
func TestCloneForTask_InheritsTurnRecorder(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	sink := &recordingSink{}
	e.SetTurnRecorder(sink)

	clone := e.CloneForTask()
	clone.recordTurn(TurnRecord{SessionID: "task", Outcome: types.MangleAtom("/done")})

	if got := sink.all(); len(got) != 1 {
		t.Fatalf("clone recorded %d turns, want 1 — delegated work is not being learned from", len(got))
	}
}

func TestTurnAtomIDs(t *testing.T) {
	if ids := turnAtomIDs(turnTelemetry{}); ids != nil {
		t.Errorf("no compilation should yield no atom IDs, got %v", ids)
	}

	telemetry := turnTelemetry{compileResult: &prompt.CompilationResult{
		IncludedAtoms: []*prompt.PromptAtom{
			{ID: "core/identity"},
			nil,      // a nil atom must not panic or produce an entry
			{ID: ""}, // nor must an unnamed one
			{ID: "go/errors"},
		},
	}}
	got := turnAtomIDs(telemetry)
	want := []string{"core/identity", "go/errors"}
	if len(got) != len(want) {
		t.Fatalf("turnAtomIDs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("turnAtomIDs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSetTurnRecorder_NilDisablesRecording(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	sink := &recordingSink{}
	e.SetTurnRecorder(sink)
	e.SetTurnRecorder(nil)

	e.recordTurn(TurnRecord{SessionID: "s", Outcome: types.MangleAtom("/done")})
	if got := sink.all(); len(got) != 0 {
		t.Fatalf("recorder still received %d records after being cleared", len(got))
	}
}

func TestTurnRecord_CarriesCostForWeighing(t *testing.T) {
	// Not behaviour so much as a contract check: a learner has to be able to
	// tell an expensive win from a cheap one, so the cost fields must survive
	// the trip through the seam.
	rec := TurnRecord{
		Duration:         2 * time.Second,
		ToolCalls:        7,
		PromptTokens:     1200,
		CompletionTokens: 300,
	}
	if rec.Duration == 0 || rec.ToolCalls == 0 || rec.PromptTokens == 0 || rec.CompletionTokens == 0 {
		t.Fatal("TurnRecord dropped its cost fields")
	}
}

// TestTracesFromHistory_PairsExchanges is the fix for a critic that could
// never work. Its prompt hunts for corrections, frustration and explicit
// teaching — all relationships BETWEEN turns — and it was handed one turn per
// call, so the pattern it was built to find was structurally invisible.
func TestTracesFromHistory_PairsExchanges(t *testing.T) {
	history := []perception.ConversationTurn{
		{Role: "user", Content: "nuke it"},
		{Role: "assistant", Content: "I am not sure what to remove."},
		{Role: "user", Content: "nuke it means drop the database"},
		{Role: "assistant", Content: "Dropped the database."},
	}

	traces := tracesFromHistory(history)
	if len(traces) != 2 {
		t.Fatalf("got %d traces, want 2 — the critic cannot see a correction without both turns", len(traces))
	}
	if traces[0].UserPrompt != "nuke it" || traces[0].Response != "I am not sure what to remove." {
		t.Errorf("first exchange mispaired: %+v", traces[0])
	}
	if traces[1].UserPrompt != "nuke it means drop the database" {
		t.Errorf("second exchange mispaired: %+v", traces[1])
	}
}

// TestTracesFromHistory_DropsUnpairedTurns: a trailing user message with no
// reply is half an exchange and carries no signal about what the agent did.
func TestTracesFromHistory_DropsUnpairedTurns(t *testing.T) {
	for name, history := range map[string][]perception.ConversationTurn{
		"trailing user": {
			{Role: "user", Content: "a"},
			{Role: "assistant", Content: "b"},
			{Role: "user", Content: "c"},
		},
		"assistant first": {
			{Role: "assistant", Content: "unsolicited"},
		},
		"two users in a row": {
			{Role: "user", Content: "a"},
			{Role: "user", Content: "b"},
			{Role: "assistant", Content: "c"},
		},
	} {
		traces := tracesFromHistory(history)
		for _, tr := range traces {
			if tr.UserPrompt == "" || tr.Response == "" {
				t.Errorf("%s: produced a half exchange: %+v", name, tr)
			}
		}
	}
	if got := tracesFromHistory(nil); got != nil {
		t.Errorf("empty history produced %d traces", len(got))
	}
}

// TestQueueTaxonomyLearning_SkipsTooShortAHistory: a correction needs
// something to correct. Below two exchanges the critic's answer is known in
// advance, so the LLM call is skipped rather than paid for.
func TestQueueTaxonomyLearning_SkipsTooShortAHistory(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	e.appendToHistory(perception.ConversationTurn{Role: "user", Content: "hello"})
	e.appendToHistory(perception.ConversationTurn{Role: "assistant", Content: "hi"})

	if got := tracesFromHistory(e.GetHistory()); len(got) >= minCriticExchanges {
		t.Fatalf("one exchange yielded %d traces; the skip threshold is wrong", len(got))
	}
	// Must not panic with no taxonomy configured.
	e.queueTaxonomyLearning(&ExecutionResult{}, nil)
}

// TestQueueTaxonomyLearning_WindowIsBounded keeps a long session from sending
// the whole transcript to the critic on every turn.
func TestQueueTaxonomyLearning_WindowIsBounded(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	for i := 0; i < 40; i++ {
		e.appendToHistory(perception.ConversationTurn{Role: "user", Content: "q"})
		e.appendToHistory(perception.ConversationTurn{Role: "assistant", Content: "a"})
	}

	history := e.GetHistory()
	if len(history) > criticWindow {
		history = history[len(history)-criticWindow:]
	}
	if got := len(tracesFromHistory(history)); got > criticWindow/2 {
		t.Errorf("window yielded %d traces, want at most %d", got, criticWindow/2)
	}
}

type contextSink struct {
	mu      sync.Mutex
	records []ContextFeedbackRecord
}

func (s *contextSink) RecordContextFeedback(rec ContextFeedbackRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, rec)
}

func (s *contextSink) all() []ContextFeedbackRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ContextFeedbackRecord, len(s.records))
	copy(out, s.records)
	return out
}

type panickingContextSink struct{}

func (panickingContextSink) RecordContextFeedback(ContextFeedbackRecord) { panic("boom") }

// TestRecordContextFeedback_ForwardsTheModelsRating covers the loop that was
// logged and dropped everywhere but the chat TUI: the model's own verdict on
// which context earned its tokens.
func TestRecordContextFeedback_ForwardsTheModelsRating(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	sink := &contextSink{}
	e.SetContextFeedbackRecorder(sink)

	e.stashContextFeedback(&articulation.ContextFeedback{
		OverallUsefulness: 0.8,
		HelpfulFacts:      []string{"file_topology"},
		NoiseFacts:        []string{"dom_node"},
		MissingContext:    "needed the dependency graph",
	})
	e.recordContextFeedback(ContextFeedbackRecord{
		SessionID: "s1", TurnNumber: 2, IntentVerb: "/fix", Verified: true,
	}, turnTelemetry{compileResult: &prompt.CompilationResult{
		Manifest: &prompt.PromptManifest{ContextHash: "abc123"},
	}})

	got := sink.all()
	if len(got) != 1 {
		t.Fatalf("sink received %d ratings, want 1", len(got))
	}
	r := got[0]
	if r.OverallUsefulness != 0.8 || len(r.HelpfulPredicates) != 1 || len(r.NoisePredicates) != 1 {
		t.Errorf("rating lost detail in transit: %+v", r)
	}
	if r.MissingContext != "needed the dependency graph" {
		t.Errorf("MissingContext = %q; the only signal naming what retrieval failed to supply", r.MissingContext)
	}
	if r.ManifestHash != "abc123" {
		t.Errorf("ManifestHash = %q, want the compiled prompt the rating applies to", r.ManifestHash)
	}
	if !r.Verified {
		t.Error("the kernel verdict did not survive; a claimed success would weigh the same as a real one")
	}
}

// TestRecordContextFeedback_ClearsTheStash: one turn's rating must never be
// attributed to the next.
func TestRecordContextFeedback_ClearsTheStash(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	sink := &contextSink{}
	e.SetContextFeedbackRecorder(sink)

	e.stashContextFeedback(&articulation.ContextFeedback{OverallUsefulness: 0.9})
	e.recordContextFeedback(ContextFeedbackRecord{TurnNumber: 1}, turnTelemetry{})
	e.recordContextFeedback(ContextFeedbackRecord{TurnNumber: 2}, turnTelemetry{})

	if got := sink.all(); len(got) != 1 {
		t.Fatalf("a rating was recorded %d times; turn 2 inherited turn 1's verdict", len(got))
	}
}

// TestRecordContextFeedback_ClearsTheStashWithNoSink: an executor wired up
// later must not inherit a rating from an unrelated earlier turn.
func TestRecordContextFeedback_ClearsTheStashWithNoSink(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	e.stashContextFeedback(&articulation.ContextFeedback{OverallUsefulness: 0.9})
	e.recordContextFeedback(ContextFeedbackRecord{TurnNumber: 1}, turnTelemetry{})

	sink := &contextSink{}
	e.SetContextFeedbackRecorder(sink)
	e.recordContextFeedback(ContextFeedbackRecord{TurnNumber: 2}, turnTelemetry{})

	if got := sink.all(); len(got) != 0 {
		t.Fatalf("a stale rating survived into a later turn: %+v", got)
	}
}

func TestRecordContextFeedback_SurvivesAPanickingSink(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	e.SetContextFeedbackRecorder(panickingContextSink{})
	e.stashContextFeedback(&articulation.ContextFeedback{OverallUsefulness: 0.1})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a panicking context sink escaped the turn: %v", r)
		}
	}()
	e.recordContextFeedback(ContextFeedbackRecord{}, turnTelemetry{})
}

func TestCloneForTask_InheritsContextFeedbackRecorder(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	sink := &contextSink{}
	e.SetContextFeedbackRecorder(sink)

	clone := e.CloneForTask()
	if !clone.HasContextFeedbackRecorder() {
		t.Fatal("a delegated task compiles its own prompt; its rating of that prompt must be kept too")
	}
	clone.stashContextFeedback(&articulation.ContextFeedback{OverallUsefulness: 0.5})
	clone.recordContextFeedback(ContextFeedbackRecord{}, turnTelemetry{})
	if len(sink.all()) != 1 {
		t.Fatal("the clone's rating was dropped")
	}
}

func TestManifestHash_MissingCompilationIsEmpty(t *testing.T) {
	if got := manifestHash(turnTelemetry{}); got != "" {
		t.Errorf("manifestHash = %q, want empty", got)
	}
	if got := manifestHash(turnTelemetry{compileResult: &prompt.CompilationResult{}}); got != "" {
		t.Errorf("manifestHash with no manifest = %q, want empty", got)
	}
}
