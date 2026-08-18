package core

import "testing"

// After rebuild()/LoadFacts invalidate cachedAtoms, AssertBatch must not
// grow a short cache of only the new facts. Live log 2026-08-17:
// AssertBatch of 25 facts against an 8504-fact EDB produced atoms=25
// facts=8529 and a "cache desync" WARN on the next evaluate.
func TestAddFactIfNewLocked_DoesNotGrowInvalidCache(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	k.AppendPolicy("Decl cache_probe(Name).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	seed := make([]Fact, 40)
	for i := range seed {
		seed[i] = Fact{Predicate: "cache_probe", Args: []any{string(rune('a'+i%26)) + string(rune('0'+i/26))}}
	}
	if err := k.LoadFacts(seed); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	if k.cachedAtoms != nil {
		t.Fatalf("LoadFacts on initialized kernel must invalidate cachedAtoms, got len=%d", len(k.cachedAtoms))
	}

	if err := k.AssertBatch([]Fact{
		{Predicate: "cache_probe", Args: []any{"batch-a"}},
		{Predicate: "cache_probe", Args: []any{"batch-b"}},
	}); err != nil {
		t.Fatalf("AssertBatch: %v", err)
	}
	if k.cachedAtoms != nil {
		t.Fatalf("AssertBatch after invalidate grew a partial cache of %d atoms; want nil so evaluate rebuilds the full EDB", len(k.cachedAtoms))
	}

	if _, err := k.Query("cache_probe"); err != nil {
		t.Fatalf("Query after AssertBatch: %v", err)
	}
	if k.cachedAtoms == nil {
		t.Fatal("evaluate should have rebuilt cachedAtoms")
	}
	if len(k.cachedAtoms) != len(k.facts) {
		t.Fatalf("cache desync after evaluate: atoms=%d facts=%d", len(k.cachedAtoms), len(k.facts))
	}
}
