package core

import (
	"fmt"
	"testing"
)

const atomMemoPolicy = `
Decl am_row(X) bound [/string].
Decl am_seen(X) bound [/string].
am_seen(X) :- am_row(X).
`

func memoAtom(k *RealKernel, f Fact) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	_, ok := k.factIndex.atomOf(f.Predicate, k.canonFact(internFact(f)), k.programInfo)
	return ok
}

// Admission remembers each fact's atom under the program it was admitted for,
// and the cone evaluator adds and removes written rows by those atoms. An atom
// is never reused under another program (a retracted row whose atom is stale
// is converted again to find it in the store), and a retracted fact's atom is
// forgotten with it.
func TestEDBIndexRemembersAdmittedAtoms(t *testing.T) {
	k := newRealKernelBase()
	k.AppendPolicy(atomMemoPolicy)
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	a := Fact{Predicate: "am_row", Args: []any{"a"}}
	b := Fact{Predicate: "am_row", Args: []any{"b"}}
	if err := k.Assert(a); err != nil {
		t.Fatal(err)
	}
	if !memoAtom(k, a) {
		t.Fatal("admission did not remember the atom it converted")
	}

	// A new program: the remembered atom belongs to the old one.
	k.mu.RLock()
	oldProgram := k.programInfo
	k.mu.RUnlock()
	k.AppendPolicy("Decl am_other(X) bound [/string].\n")
	if _, err := k.Query("am_seen"); err != nil {
		t.Fatal(err)
	}
	// The full evaluation the new program forced converted every row again, and
	// remembered each atom under the program it converted it for: the memo
	// holds nothing admitted under the old one.
	k.mu.RLock()
	remembered, rememberedHeld := k.factIndex.atoms[a.Predicate][k.canonFact(internFact(a))]
	newProgram := k.programInfo
	k.mu.RUnlock()
	if newProgram == oldProgram {
		t.Fatal("appending policy did not produce a new program")
	}
	if rememberedHeld && remembered.program != newProgram {
		t.Fatal("an atom admitted under the previous program survived into the new one")
	}
	if !memoAtom(k, a) {
		t.Fatal("the full evaluation did not remember the atom it converted under the new program")
	}
	// The next write is admitted under the new program and remembered; its
	// DRed evaluation adds just that row (a stays in the store as the full evaluation
	// left it), and the answer is the full fixpoint's.
	if err := k.Assert(b); err != nil {
		t.Fatal(err)
	}
	seen, err := k.Query("am_seen")
	if err != nil {
		t.Fatal(err)
	}
	if got := k.LastEvaluation().Mode; got != "dred" {
		t.Fatalf("evaluation mode %q, want dred", got)
	}
	if len(seen) != 2 {
		t.Fatalf("am_seen = %v, want both rows", seen)
	}
	if !memoAtom(k, b) {
		t.Fatal("admission under the new program did not remember its atom")
	}

	clone := k.Clone()
	if !memoAtom(clone, b) {
		t.Fatal("a clone lost the parent's remembered atoms")
	}

	if err := k.RetractExactFact(b); err != nil {
		t.Fatal(err)
	}
	k.mu.RLock()
	_, held := k.factIndex.atoms["am_row"][k.canonFact(internFact(b))]
	k.mu.RUnlock()
	if held {
		t.Fatal("a retracted fact's atom is still remembered")
	}
	if !memoAtom(clone, b) {
		t.Fatal("retracting on the parent reached the clone's index")
	}
	seen, err = k.Query("am_seen")
	if err != nil || len(seen) != 1 {
		t.Fatalf("am_seen after retract = %v, %v; want one row", seen, err)
	}
	// a's remembered atom is from the previous program: the cone converts
	// the retracted row again to find it in the store.
	if err := k.RetractExactFact(a); err != nil {
		t.Fatal(err)
	}
	seen, err = k.Query("am_seen")
	if err != nil || len(seen) != 0 || k.LastEvaluation().Mode != "dred" {
		t.Fatalf("am_seen after retracting a = %v, %v (%s); want none, by dred", seen, err, k.LastEvaluation().Mode)
	}
}

// A write to a predicate no rule derives keeps the rows the EDB still holds in
// the store: only the new row is added and only the retracted one removed (the
// EDB write delta), and DRed (kernel_eval_dred.go) touches only the derived
// rows the change supported: an insert adds what the new row derives, a
// retract over-deletes what the old one did.
func TestConeKeepsUnchangedRowsOfAWrittenExtensionalPredicate(t *testing.T) {
	k := newRealKernelBase()
	k.AppendPolicy(atomMemoPolicy)
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if err := k.Assert(Fact{Predicate: "am_row", Args: []any{fmt.Sprintf("r%d", i)}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := k.Query("am_seen"); err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(Fact{Predicate: "am_row", Args: []any{"new"}}); err != nil {
		t.Fatal(err)
	}
	seen, err := k.Query("am_seen")
	if err != nil || len(seen) != 51 {
		t.Fatalf("am_seen = %d rows, %v; want 51", len(seen), err)
	}
	ev := k.LastEvaluation()
	if ev.Mode != "dred" || ev.RestoredFacts != 1 || ev.RemovedFacts != 0 || ev.DeltaFacts != 1 {
		t.Fatalf("%q restored %d, removed %d, inserted %d rows; want dred: 1 restored (the new row), 0 removed, 1 inserted (its am_seen)",
			ev.Mode, ev.RestoredFacts, ev.RemovedFacts, ev.DeltaFacts)
	}
	// Nothing was listed: neither am_row's 50 held rows nor am_seen's 50
	// derived ones. The insert's work is the one new row.
	if ev.ListedFacts != 0 {
		t.Fatalf("dred listed %d stored rows; want 0", ev.ListedFacts)
	}

	if err := k.RetractExactFact(Fact{Predicate: "am_row", Args: []any{"r7"}}); err != nil {
		t.Fatal(err)
	}
	seen, err = k.Query("am_seen")
	if err != nil || len(seen) != 50 {
		t.Fatalf("am_seen after retract = %d rows, %v; want 50", len(seen), err)
	}
	// The retract removes r7 and over-deletes the one am_seen row it
	// supported; the other 50 are never touched (the cone removed all 51).
	if ev := k.LastEvaluation(); ev.Mode != "dred" || ev.RemovedFacts != 1 || ev.OverDeleted != 1 || ev.Rederived != 0 || ev.ListedFacts != 0 {
		t.Fatalf("%q removed %d, over-deleted %d, re-derived %d, listed %d; want dred: 1 removed (r7), 1 over-deleted (its am_seen), none re-derived or listed",
			ev.Mode, ev.RemovedFacts, ev.OverDeleted, ev.Rederived, ev.ListedFacts)
	}
}
