package context

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
)

func TestProcessTurnWithAppliedControlsRetainsMetadataWithoutReapplying(t *testing.T) {
	t.Parallel()
	comp := newKernelBackedCompressor(t)
	hostKernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("host kernel: %v", err)
	}

	observationText := "observation(\"applied_control_probe\", \"already stored\")."
	observation, err := core.ParseSingleFact(observationText)
	if err != nil {
		t.Fatalf("parse observation: %v", err)
	}
	if err := hostKernel.Assert(observation); err != nil {
		t.Fatalf("apply host observation: %v", err)
	}
	note := articulation.MemoryOperation{Op: "note", Key: "applied_control_probe", Value: "already stored"}
	ApplyMemoryOperation(hostKernel, nil, note)

	packet := &articulation.ControlPacket{
		IntentClassification: articulation.IntentClassification{
			Category: "/query",
			Verb:     "/explain",
			Target:   "applied controls",
		},
		MangleUpdates:    []string{observationText},
		MemoryOperations: []articulation.MemoryOperation{note},
	}
	result, err := comp.ProcessTurnWithAppliedControls(context.Background(), Turn{
		Number:          1,
		Role:            "assistant",
		UserInput:       "explain applied controls",
		SurfaceResponse: "The controls are recorded.",
		ControlPacket:   packet,
		Timestamp:       time.Now(),
	})
	if err != nil {
		t.Fatalf("ProcessTurnWithAppliedControls: %v", err)
	}
	if len(result.CommittedAtoms) < 2 || len(result.MemoryOps) != 1 {
		t.Fatalf("recorded result = %+v, want extracted atoms and one memory op", result)
	}

	state := comp.GetState()
	if len(state.RecentTurns) != 1 {
		t.Fatalf("compressed recent turns = %d, want one", len(state.RecentTurns))
	}
	turn := state.RecentTurns[0]
	if len(turn.MangleUpdates) != 1 || turn.MangleUpdates[0] != observationText {
		t.Errorf("compressed MangleUpdates = %v, want sanitized observation metadata", turn.MangleUpdates)
	}
	if len(turn.MemoryOperations) != 1 || turn.MemoryOperations[0] != note {
		t.Errorf("compressed MemoryOperations = %v, want one retained note operation", turn.MemoryOperations)
	}

	hostObservations, err := hostKernel.Query("observation")
	if err != nil {
		t.Fatalf("query host observation: %v", err)
	}
	if !containsFact(hostObservations, "observation", "already stored") {
		t.Fatalf("host observation effect was lost: %v", hostObservations)
	}
	hostNotes, err := hostKernel.Query(sessionNotePredicate)
	if err != nil {
		t.Fatalf("query host note: %v", err)
	}
	if len(hostNotes) != 1 || !containsFact(hostNotes, sessionNotePredicate, "already stored") {
		t.Fatalf("host memory effect = %v, want exactly one stored note", hostNotes)
	}

	compressorObservations, err := comp.kernel.Query("observation")
	if err != nil {
		t.Fatalf("query compressor observation: %v", err)
	}
	if len(compressorObservations) != 0 {
		t.Fatalf("compressor replayed an already-applied observation: %v", compressorObservations)
	}
	compressorNotes, err := comp.kernel.Query(sessionNotePredicate)
	if err != nil {
		t.Fatalf("query compressor note: %v", err)
	}
	if len(compressorNotes) != 0 {
		t.Fatalf("compressor replayed an already-applied memory operation: %v", compressorNotes)
	}
}
