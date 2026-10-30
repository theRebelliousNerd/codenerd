package chat

import (
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// A derived test-authoring step writes: its obligation reads "Write tests
// for:", so Propose mode holds it before the tester launches. It used to be
// classified by the shard's name, tester read-only, and launched without the
// pause. The effect is the kernel's (pending_subtask_effect); a read-only
// step still runs on, and a step the kernel declares nothing for is held.
func TestContinuation_ATestAuthoringStepIsHeldInProposeMode(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	if err := m.kernel.Assert(types.Fact{Predicate: "pending_test", Args: []any{"task-tests", "Write tests for: internal/widget/widget.go"}}); err != nil {
		t.Fatal(err)
	}
	next := m.derivedNextStep()
	if next == nil || next.shardType != "tester" {
		t.Fatalf("derived step = %+v, want the tester's", next)
	}
	if !next.isMutation || !m.pausesBefore(next.isMutation) {
		t.Fatalf("a test-authoring step was not held in Propose mode (isMutation=%v)", next.isMutation)
	}

	if err := m.kernel.Retract("pending_test"); err != nil {
		t.Fatal(err)
	}
	if err := m.kernel.Assert(types.Fact{Predicate: "shard_result", Args: []any{"task-review", types.MangleAtom("/incomplete"), types.MangleAtom("/reviewer"), "review the change", "half done"}}); err != nil {
		t.Fatal(err)
	}
	next = m.derivedNextStep()
	if next == nil || next.shardType != "reviewer" || next.isMutation {
		t.Fatalf("a retried read-only review step = %+v, want it to run on", next)
	}
}
