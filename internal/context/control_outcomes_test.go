package context

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/types"
)

// failingStore is a memory store whose writes the backend refuses.
type failingStore struct{ err error }

func (s failingStore) StoreFact(string, []any, string, int) error { return s.err }
func (s failingStore) StoreVector(string, map[string]any) error   { return s.err }

// scriptedKernel is a MemoryKernel whose two writes can be made to fail
// independently, and which records what landed.
type scriptedKernel struct {
	retractErr, assertErr error
	notes                 map[string]string
}

func (k *scriptedKernel) Assert(f types.Fact) error {
	if k.assertErr != nil {
		return k.assertErr
	}
	if k.notes == nil {
		k.notes = map[string]string{}
	}
	k.notes[f.Args[0].(string)] = f.Args[1].(string)
	return nil
}

func (k *scriptedKernel) RetractFact(f types.Fact) error {
	if k.retractErr != nil {
		return k.retractErr
	}
	delete(k.notes, f.Args[0].(string))
	return nil
}

// The model says "I saved your preference". The store refused. The operation's
// outcome has to say so, in words a person can read, without the preference's
// value in them.
func TestApplyMemoryOperation_ReportsAStoreFailureInsteadOfDroppingIt(t *testing.T) {
	op := articulation.MemoryOperation{Op: "promote_to_long_term", Key: "indent", Value: "tabs, my secret preference"}
	d := ApplyMemoryOperation(nil, failingStore{err: errors.New("disk is full")}, op)
	if d.Stage != articulation.StageFailed || d.Took() {
		t.Fatalf("stage = %q, want failed", d.Stage)
	}
	notice := d.Notice()
	if !strings.Contains(notice, "disk is full") || !strings.Contains(notice, "promote_to_long_term indent") {
		t.Fatalf("notice %q should name the operation and the store's reason", notice)
	}
	if strings.Contains(notice, "secret") {
		t.Fatalf("notice %q leaked the stored value", notice)
	}

	vec := ApplyMemoryOperation(nil, failingStore{err: errors.New("embedder down")}, articulation.MemoryOperation{Op: "store_vector", Key: "k", Value: "v"})
	if vec.Stage != articulation.StageFailed {
		t.Fatalf("store_vector stage = %q, want failed", vec.Stage)
	}
	if got := ApplyMemoryOperation(nil, nil, op); got.Stage != articulation.StageFailed {
		t.Fatalf("no store at all: stage = %q, want failed", got.Stage)
	}
}

func TestApplyMemoryOperation_AppliedAndUnsupported(t *testing.T) {
	k := &scriptedKernel{}
	if d := ApplyMemoryOperation(k, nil, articulation.MemoryOperation{Op: "note", Key: "focus", Value: "auth"}); !d.Took() {
		t.Fatalf("a note that landed reported %q", d.Stage)
	}
	if k.notes["focus"] != "auth" {
		t.Fatalf("the note did not land: %v", k.notes)
	}
	if d := ApplyMemoryOperation(k, nil, articulation.MemoryOperation{Op: "teleport", Key: "x"}); d.Stage != articulation.StageUnsupported {
		t.Fatalf("an operation this path cannot apply reported %q, want unsupported", d.Stage)
	}
	if d := ApplyMemoryOperation(k, nil, articulation.MemoryOperation{Op: "note", Key: " ", Value: "v"}); d.Stage != articulation.StageRejectedParse {
		t.Fatalf("a keyless note reported %q, want rejected_parse", d.Stage)
	}
}

// A note is a retraction then an assertion. Each half can fail on its own, and
// the outcome has to say which state it left.
func TestApplyMemoryOperation_NoteReplacementReportsPartialState(t *testing.T) {
	op := articulation.MemoryOperation{Op: "note", Key: "focus", Value: "new"}

	// Retraction succeeded, assertion failed: the earlier note is gone.
	k := &scriptedKernel{assertErr: errors.New("kernel full"), notes: map[string]string{"focus": "old"}}
	d := ApplyMemoryOperation(k, nil, op)
	if d.Stage != articulation.StagePartiallyApplied {
		t.Fatalf("retract ok, assert failed: stage = %q, want partially_applied", d.Stage)
	}
	if _, still := k.notes["focus"]; still {
		t.Fatal("the fixture should have lost the earlier note")
	}

	// Retraction failed, assertion succeeded: the old note sits beside the new.
	k = &scriptedKernel{retractErr: errors.New("busy"), notes: map[string]string{"focus": "old"}}
	d = ApplyMemoryOperation(k, nil, op)
	if d.Stage != articulation.StagePartiallyApplied || !strings.Contains(d.Reason, "earlier note") {
		t.Fatalf("retract failed, assert ok: %+v, want partially_applied naming the earlier note", d)
	}

	// Both failed: nothing changed.
	k = &scriptedKernel{retractErr: errors.New("busy"), assertErr: errors.New("full"), notes: map[string]string{"focus": "old"}}
	if d = ApplyMemoryOperation(k, nil, op); d.Stage != articulation.StageFailed {
		t.Fatalf("both writes failed: stage = %q, want failed", d.Stage)
	}
}

func TestApplyMemoryOperations_IndexesByPositionInTheModelsList(t *testing.T) {
	k := &scriptedKernel{}
	ops := []articulation.MemoryOperation{
		{Op: "note", Key: "a", Value: "1"},
		{Op: "teleport", Key: "b"},
		{Op: "note", Key: "a", Value: "1"}, // equal to #0, still its own control
	}
	got := ApplyMemoryOperations(k, nil, ops)
	if len(got) != 3 || got[0].Index != 0 || got[1].Index != 1 || got[2].Index != 2 {
		t.Fatalf("dispositions = %+v, want one per position", got)
	}
	if !got[0].Took() || got[1].Took() || !got[2].Took() {
		t.Fatalf("stages = %q %q %q", got[0].Stage, got[1].Stage, got[2].Stage)
	}
}

// The compressor that applies a packet itself keeps only the operations that
// landed as the turn's memory, and keeps the rest as outcomes, so compressed
// history never says "remembered" for a failed write.
func TestProcessTurn_FailedMemoryOperationIsRecordedAsFailedNotRemembered(t *testing.T) {
	t.Parallel()
	comp := newKernelBackedCompressor(t)
	comp.store = nil
	good := articulation.MemoryOperation{Op: "note", Key: "focus", Value: "auth"}
	bad := articulation.MemoryOperation{Op: "promote_to_long_term", Key: "indent", Value: "tabs"}
	result, err := comp.ProcessTurn(context.Background(), Turn{
		Number: 1, Role: "assistant", UserInput: "remember", SurfaceResponse: "I saved your preference.",
		ControlPacket: &articulation.ControlPacket{MemoryOperations: []articulation.MemoryOperation{good, bad}},
		Timestamp:     time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.MemoryOps) != 1 || result.MemoryOps[0].Key != "focus" {
		t.Fatalf("result.MemoryOps = %+v, want only the note that landed", result.MemoryOps)
	}
	turn := comp.GetState().RecentTurns[0]
	if len(turn.MemoryOperations) != 1 || len(turn.ControlOutcomes) != 1 {
		t.Fatalf("compressed turn memory = %+v outcomes = %+v", turn.MemoryOperations, turn.ControlOutcomes)
	}
	if o := turn.ControlOutcomes[0]; o.Index != 1 || o.Stage != articulation.StageFailed {
		t.Fatalf("outcome = %+v, want operation #2 failed", o)
	}
}

// A host that applied the controls itself hands its own outcomes over; the
// compressor retains them without reapplying, and they reach both renderings
// of the history the model reads.
func TestProcessTurnWithAppliedControls_RetainsOutcomesInBothRenderings(t *testing.T) {
	t.Parallel()
	comp := newKernelBackedCompressor(t)
	outcome := articulation.ControlDisposition{
		Kind: articulation.ControlMemoryOperation, Index: 0, Stage: articulation.StageFailed,
		Detail: "promote_to_long_term indent", Reason: "the preference was not persisted: disk is full",
	}
	if _, err := comp.ProcessTurnWithAppliedControls(context.Background(), Turn{
		Number: 1, Role: "assistant", UserInput: "remember", SurfaceResponse: "ok",
		ControlPacket:   &articulation.ControlPacket{},
		ControlOutcomes: []articulation.ControlDisposition{outcome},
		Timestamp:       time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	turn := comp.GetState().RecentTurns[0]
	if len(turn.ControlOutcomes) != 1 || turn.ControlOutcomes[0] != outcome {
		t.Fatalf("outcomes = %+v, want the host's record", turn.ControlOutcomes)
	}
	recent := NewFactSerializer().SerializeCompressedTurn(turn)
	summary, _ := comp.generateObservationMaskedSummary([]CompressedTurn{turn}, map[string]bool{turnMaskID(1): true})
	for name, text := range map[string]string{"recent window": recent, "masked summary": summary} {
		if !strings.Contains(text, "disk is full") || !strings.Contains(text, "failed") {
			t.Errorf("%s does not carry the failed control:\n%s", name, text)
		}
	}
	// And the snapshot is detached from the compressor's own turn.
	turn.ControlOutcomes[0].Stage = articulation.StageApplied
	if comp.GetState().RecentTurns[0].ControlOutcomes[0].Stage != articulation.StageFailed {
		t.Fatal("GetState shares the outcomes slice with the compressor")
	}
}
