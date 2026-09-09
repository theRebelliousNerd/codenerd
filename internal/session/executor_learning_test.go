package session

import (
	"sync"
	"testing"
	"time"

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
