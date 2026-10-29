package core

import "testing"

func TestSnapshotIdentity_AdvancesOnEveryWriteAndNotOnReads(t *testing.T) {
	k := newConeKernel(t)
	start := k.SnapshotIdentity()
	if start.Instance == 0 {
		t.Fatal("kernel has no instance identity")
	}
	if !start.Reusable {
		t.Fatal("a program without external premises must be reusable after evaluation")
	}
	if _, err := k.Query("cn_open"); err != nil {
		t.Fatal(err)
	}
	if got := k.SnapshotIdentity(); got != start {
		t.Fatalf("a read moved the identity: %+v -> %+v", start, got)
	}

	fact := Fact{Predicate: "cn_raised", Args: []any{"/a"}}
	if err := k.Assert(fact); err != nil {
		t.Fatal(err)
	}
	asserted := k.SnapshotIdentity()
	if asserted.Generation <= start.Generation {
		t.Fatalf("assert did not advance the generation: %d -> %d", start.Generation, asserted.Generation)
	}
	if err := k.RetractExactFact(fact); err != nil {
		t.Fatal(err)
	}
	if retracted := k.SnapshotIdentity(); retracted.Generation <= asserted.Generation {
		t.Fatalf("retract did not advance the generation: %d -> %d", asserted.Generation, retracted.Generation)
	}
}

func TestCloneWithIdentity_NamesTheClonedStateAndIsolatesInstances(t *testing.T) {
	k := newConeKernel(t)
	before := k.SnapshotIdentity()
	clone, id := k.CloneWithIdentity()
	if id != before {
		t.Fatalf("clone identity %+v differs from the parent state it copied %+v", id, before)
	}
	if err := clone.Assert(Fact{Predicate: "cn_raised", Args: []any{"/b"}}); err != nil {
		t.Fatal(err)
	}
	if got := k.SnapshotIdentity(); got != before {
		t.Fatalf("a write to the clone moved the parent: %+v -> %+v", before, got)
	}
	if clone.SnapshotIdentity().Instance == before.Instance {
		t.Fatal("a clone must not share the parent's instance identity")
	}
}

func TestSnapshotIdentity_ExternalPremisesAreNotReusable(t *testing.T) {
	k := setupMockKernel(t)
	// setupMockKernel's schema already declares query_learned as external.
	k.AppendPolicy(`
		Decl gen_learned(P).
		gen_learned(P) :- query_learned(P, _).
	`)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if k.SnapshotIdentity().Reusable {
		t.Fatal("a rule reading an external predicate re-derives untracked state; identity must not be reusable")
	}
}

func TestSnapshotIdentity_PendingProgramChangeIsNotReusable(t *testing.T) {
	k := newConeKernel(t)
	k.AppendPolicy(`Decl cn_extra(Id).`)
	if k.SnapshotIdentity().Reusable {
		t.Fatal("before the changed program is analysed its dependency index is stale")
	}
}
