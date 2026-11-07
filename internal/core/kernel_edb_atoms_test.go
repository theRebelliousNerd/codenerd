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
	k.AppendPolicy("Decl am_other(X) bound [/string].\n")
	if _, err := k.Query("am_seen"); err != nil {
		t.Fatal(err)
	}
	if memoAtom(k, a) {
		t.Fatal("an atom admitted under the previous program is offered under the new one")
	}
	// The next write is admitted under the new program and remembered; its
	// delta evaluation adds just that row (a stays in the store as the full evaluation
	// left it), and the answer is the full fixpoint's.
	if err := k.Assert(b); err != nil {
		t.Fatal(err)
	}
	seen, err := k.Query("am_seen")
	if err != nil {
		t.Fatal(err)
	}
	if got := k.LastEvaluation().Mode; got != "delta" {
		t.Fatalf("evaluation mode %q, want delta", got)
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
	if err != nil || len(seen) != 0 || k.LastEvaluation().Mode != "cone" {
		t.Fatalf("am_seen after retracting a = %v, %v (%s); want none, by cone", seen, err, k.LastEvaluation().Mode)
	}
}

// A write to a predicate no rule derives keeps the rows the EDB still holds in
// the store: only the new row is added and only the retracted one removed (the
// EDB write delta). An insert keeps the derived rows too and derives only what
// the new row adds (kernel_eval_delta.go); a retract re-derives the cone.
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
	if ev.Mode != "delta" || ev.RestoredFacts != 1 || ev.RemovedFacts != 0 || ev.DeltaFacts != 1 {
		t.Fatalf("%q restored %d, removed %d, derived %d rows; want delta: 1 restored (the new row), 0 removed, 1 derived (its am_seen)",
			ev.Mode, ev.RestoredFacts, ev.RemovedFacts, ev.DeltaFacts)
	}
	// Nothing was listed: neither am_row's 50 held rows nor am_seen's 50
	// derived ones. The insert's work is the one new row.
	if ev.ListedFacts != 0 {
		t.Fatalf("delta listed %d stored rows; want 0", ev.ListedFacts)
	}

	if err := k.RetractExactFact(Fact{Predicate: "am_row", Args: []any{"r7"}}); err != nil {
		t.Fatal(err)
	}
	seen, err = k.Query("am_seen")
	if err != nil || len(seen) != 50 {
		t.Fatalf("am_seen after retract = %d rows, %v; want 50", len(seen), err)
	}
	if ev := k.LastEvaluation(); ev.Mode != "cone" || ev.RestoredFacts != 0 || ev.RemovedFacts != 52 {
		t.Fatalf("cone %q restored %d and removed %d rows; want 0 restored and 52 removed (r7 and 51 am_seen)",
			ev.Mode, ev.RestoredFacts, ev.RemovedFacts)
	}
}
