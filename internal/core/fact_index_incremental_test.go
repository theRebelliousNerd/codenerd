package core

import (
	"fmt"
	"reflect"
	"testing"
)

// Retraction drops removed facts' keys from the dedupe index instead of
// rebuilding it; the result must be the index a full rebuild produces.
func TestFactIndexFollowsRetractionsIncrementally(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	var facts []Fact
	for i := 0; i < 30; i++ {
		facts = append(facts, Fact{Predicate: "code_calls", Args: []any{fmt.Sprintf("A%d", i), fmt.Sprintf("B%d", i)}})
	}
	if err := k.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	check := func(stage string) {
		t.Helper()
		k.mu.Lock()
		got := k.factIndex.buckets
		gotSize := k.factIndex.len()
		k.rebuildFactIndexLocked()
		want := k.factIndex.buckets
		wantSize := k.factIndex.len()
		k.mu.Unlock()
		if !reflect.DeepEqual(got, want) || gotSize != wantSize {
			t.Fatalf("%s: incremental index has %d facts, a rebuild has %d", stage, gotSize, wantSize)
		}
	}
	check("after load")

	if err := k.RetractExactFact(facts[3]); err != nil {
		t.Fatal(err)
	}
	check("after exact retract")

	tx := k.Transaction()
	tx.RetractExactFact(facts[4])
	tx.RetractFact(Fact{Predicate: "code_calls", Args: []any{"A5"}})
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check("after transaction")

	if err := k.Retract("code_calls"); err != nil {
		t.Fatal(err)
	}
	check("after predicate retract")

	// A retracted fact is admissible again: its key left the index.
	if err := k.Assert(facts[3]); err != nil {
		t.Fatal(err)
	}
	rows, err := k.Query("code_calls")
	if err != nil || len(rows) != 1 {
		t.Fatalf("re-asserted fact: rows=%v err=%v; want exactly it", rows, err)
	}
	check("after re-assert")
}
