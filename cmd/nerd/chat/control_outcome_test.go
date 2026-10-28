package chat

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/articulation"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// "I saved your preference", and there is no long-term store on this model:
// the user is told, and history does not record the preference as asked-for
// and kept.
func TestApplyArticulationControlPacket_FailedMemoryWriteIsNotRecordedAsRemembered(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	m := Model{kernel: kernel} // no localDB
	note := articulation.MemoryOperation{Op: "note", Key: "focus", Value: "auth"}
	pref := articulation.MemoryOperation{Op: "promote_to_long_term", Key: "indent", Value: "tabs"}

	applied := m.applyArticulationControlPacket(&ArticulationOutput{
		Surface:          "I saved your preference.",
		MemoryOperations: []articulation.MemoryOperation{note, pref},
	})

	if len(applied.RecordedMemory) != 1 || applied.RecordedMemory[0] != note {
		t.Fatalf("recorded memory = %+v, want only the note that landed", applied.RecordedMemory)
	}
	if len(applied.NotApplied) != 1 || applied.NotApplied[0].Index != 1 || applied.NotApplied[0].Stage != articulation.StageFailed {
		t.Fatalf("not applied = %+v, want operation #2 failed", applied.NotApplied)
	}
	joined := strings.Join(applied.Warnings, "\n")
	if !strings.Contains(joined, "promote_to_long_term indent") || !strings.Contains(joined, "failed") {
		t.Fatalf("the user is not told the write failed:\n%s", joined)
	}
	if rows, _ := kernel.Query("session_note"); len(rows) != 1 {
		t.Fatalf("the note that could be applied was not: %v", rows)
	}
}

// The parser's removals ride to history beside the memory failures.
func TestApplyArticulationControlPacket_CarriesParserRemovals(t *testing.T) {
	removed := articulation.ControlDisposition{Kind: articulation.ControlMangleUpdate, Index: 3, Stage: articulation.StageRejectedParse, Reason: "no trailing period"}
	applied := Model{}.applyArticulationControlPacket(&ArticulationOutput{Dispositions: []articulation.ControlDisposition{removed}})
	if len(applied.NotApplied) != 1 || applied.NotApplied[0] != removed {
		t.Fatalf("not applied = %+v, want the parser's removal", applied.NotApplied)
	}
}

// End to end through the compressor handoff the chat turn uses: what the host
// could not apply is visible in the history the next turn is built from.
func TestChatHandoff_FailedControlReachesCompressedHistory(t *testing.T) {
	m := Model{}
	applied := m.applyArticulationControlPacket(&ArticulationOutput{
		MemoryOperations: []articulation.MemoryOperation{{Op: "promote_to_long_term", Key: "indent", Value: "tabs"}},
	})
	comp := ctxcompress.NewCompressor(nil, nil, nil)
	if _, err := comp.ProcessTurnWithAppliedControls(t.Context(), ctxcompress.Turn{
		Number: 1, Role: "assistant", UserInput: "remember tabs", SurfaceResponse: "Saved.",
		ControlPacket:   &articulation.ControlPacket{MemoryOperations: applied.RecordedMemory, MangleUpdates: applied.RecordedUpdates},
		ControlOutcomes: applied.NotApplied,
		Timestamp:       time.Now(),
	}); err != nil {
		t.Fatalf("ProcessTurnWithAppliedControls: %v", err)
	}
	turn := comp.GetState().RecentTurns[0]
	if len(turn.MemoryOperations) != 0 {
		t.Fatalf("history records %+v as remembered; the store never took it", turn.MemoryOperations)
	}
	if len(turn.ControlOutcomes) != 1 || turn.ControlOutcomes[0].Stage != articulation.StageFailed {
		t.Fatalf("history outcomes = %+v, want the failed preference", turn.ControlOutcomes)
	}
}

// The user's constraint is part of the intent atom history keeps; the handoff
// used to rebuild the classification from category, verb and target only.
func TestCompressionControlPacket_CarriesTheConstraintAndOnlyAppliedControls(t *testing.T) {
	intent := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "retryLoop", Constraint: "do not touch the tests"}
	note := articulation.MemoryOperation{Op: "note", Key: "k", Value: "v"}
	packet := compressionControlPacket(intent, appliedControls{
		RecordedMemory:  []articulation.MemoryOperation{note},
		RecordedUpdates: []string{`observation("a", "b").`},
	})
	if packet.IntentClassification.Constraint != "do not touch the tests" {
		t.Fatalf("constraint = %q, want the user's", packet.IntentClassification.Constraint)
	}
	atoms, err := ctxcompress.ExtractAtomsFromControlPacket(packet)
	if err != nil {
		t.Fatal(err)
	}
	if len(atoms) == 0 || !strings.Contains(atoms[0].String(), "do not touch the tests") {
		t.Fatalf("the intent atom history keeps lost the constraint: %v", atoms)
	}
	if len(packet.MemoryOperations) != 1 || len(packet.MangleUpdates) != 1 {
		t.Fatalf("packet = %+v", packet)
	}
}
