package context

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/articulation"
)

// The "note" memory operation lands (TODO-CTX-07A). It was admitted by the
// protocol schema, described to the model as a short-term session observation,
// and dropped with a warning. Now it is session_note(Key, Value) in the kernel,
// and the kernel's relevance policy puts it in the window.
func TestNoteMemoryOp_ReachesTheNextContextThroughTheKernel(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	turn := func(n int, ops ...articulation.MemoryOperation) {
		t.Helper()
		if _, err := comp.ProcessTurn(context.Background(), Turn{
			Number:          n,
			Role:            "assistant",
			UserInput:       "keep going",
			SurfaceResponse: "working on it",
			ControlPacket:   &articulation.ControlPacket{MemoryOperations: ops},
			Timestamp:       time.Now(),
		}); err != nil {
			t.Fatalf("ProcessTurn %d: %v", n, err)
		}
	}

	turn(1, articulation.MemoryOperation{Op: "note", Key: "current_focus", Value: "refactoring auth module"})
	built, err := comp.BuildContext(context.Background())
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if !strings.Contains(built.ContextAtoms, "refactoring auth module") {
		t.Fatalf("the note did not reach the next context's active block:\n%s", built.ContextAtoms)
	}
	if built.Selection != SelectionKernel {
		t.Errorf("the note entered by %s (%s), want the kernel's should_include_context", built.Selection, built.SelectionReason)
	}

	// One note per key: a later note replaces the earlier one.
	turn(2, articulation.MemoryOperation{Op: "note", Key: "current_focus", Value: "writing the session tests"})
	notes, err := comp.kernel.Query(sessionNotePredicate)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || !containsFact(notes, sessionNotePredicate, "writing the session tests") {
		t.Fatalf("a second note under the same key did not replace the first: %v", notes)
	}

	// forget drops it.
	turn(3, articulation.MemoryOperation{Op: "forget", Key: "current_focus"})
	if notes, _ = comp.kernel.Query(sessionNotePredicate); len(notes) != 0 {
		t.Fatalf("forget left the note in the kernel: %v", notes)
	}
}

func TestNoteMemoryOp_WithoutAKeyIsDroppedNotStored(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	comp.processMemoryOperation(articulation.MemoryOperation{Op: "note", Key: "  ", Value: "orphan"})
	if notes, _ := comp.kernel.Query(sessionNotePredicate); len(notes) != 0 {
		t.Fatalf("a keyless note was stored: %v", notes)
	}
}
