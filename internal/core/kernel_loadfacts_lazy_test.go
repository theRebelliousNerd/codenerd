package core

import (
	"testing"
)

// TestLoadFacts_LazyWhenInitialized verifies the fix for the quadratic
// scan pathology: LoadFacts on an already-initialized kernel must NOT run
// the fixpoint eagerly. It marks factsDirty and defers to ensureEvaluated
// on the next Query (correct-on-read). The boot path (uninitialized kernel)
// must still evaluate eagerly so the kernel is ready before any query.
func TestLoadFacts_LazyWhenInitialized(t *testing.T) {
	// Use a unique predicate pair so the test does not depend on the
	// embedded constitution's rules and cannot be polluted by other tests.
	const (
		basePred    = "lazy_base"
		derivedPred = "lazy_derived"
	)
	policy := `
Decl lazy_base(Name).
Decl lazy_derived(Name).
lazy_derived(X) :- lazy_base(X).
`

	t.Run("deferred when initialized", func(t *testing.T) {
		k, err := NewRealKernel()
		if err != nil {
			t.Fatalf("NewRealKernel: %v", err)
		}
		// Install the derivation rule and force a clean evaluation so the
		// kernel is initialized and factsDirty is false before the probe.
		k.AppendPolicy(policy)
		if err := k.Evaluate(); err != nil {
			t.Fatalf("initial Evaluate: %v", err)
		}
		// Ensure clean state: after Evaluate factsDirty must be false.
		// (Evaluate clears it; ensureEvaluated would also clear it.)
		if k.IsDirty() {
			// Force clear via a no-op query which triggers ensureEvaluated.
			if _, err := k.Query(basePred); err != nil {
				t.Fatalf("Query to clear dirty: %v", err)
			}
			if k.IsDirty() {
				t.Fatalf("expected clean kernel before probe, still dirty")
			}
		}
		// At this point the kernel is initialized.
		if !k.IsInitialized() {
			t.Fatalf("expected initialized kernel before probe")
		}

		// Load a fact that should derive lazy_derived("hello").
		// On an initialized kernel this must be lazy: no fixpoint yet,
		// factsDirty must be set, and cachedAtoms must have been invalidated.
		if err := k.LoadFacts([]Fact{{Predicate: basePred, Args: []any{"hello"}}}); err != nil {
			t.Fatalf("LoadFacts: %v", err)
		}
		if !k.IsDirty() {
			t.Fatalf("expected factsDirty=true after LoadFacts on initialized kernel (deferred evaluation), got false")
		}
		// The derived fact must NOT be visible without going through
		// ensureEvaluated — but the only correct way to observe it IS
		// through Query, which triggers ensureEvaluated. So we assert the
		// lazy half by checking dirty, then assert correct-on-read by querying.
		results, err := k.Query(derivedPred)
		if err != nil {
			t.Fatalf("Query(%s): %v", derivedPred, err)
		}
		if len(results) != 1 {
			t.Fatalf("Query(%s) = %d results, want 1", derivedPred, len(results))
		}
		if len(results[0].Args) == 0 || results[0].Args[0] != "hello" {
			t.Fatalf("Query(%s) = %v, want lazy_derived(\"hello\")", derivedPred, results[0])
		}
		if k.IsDirty() {
			t.Fatalf("expected factsDirty=false after Query triggered lazy evaluation, got true")
		}
		// Base fact must also be queryable.
		baseResults, err := k.Query(basePred)
		if err != nil {
			t.Fatalf("Query(%s): %v", basePred, err)
		}
		if len(baseResults) != 1 {
			t.Fatalf("Query(%s) = %d results, want 1", basePred, len(baseResults))
		}
	})

	t.Run("eager when uninitialized", func(t *testing.T) {
		k, err := NewRealKernel()
		if err != nil {
			t.Fatalf("NewRealKernel: %v", err)
		}
		k.AppendPolicy(policy)
		if err := k.Evaluate(); err != nil {
			t.Fatalf("initial Evaluate: %v", err)
		}
		// Make the kernel uninitialized but keep schemas/policy.
		// Clear retains policyDirty=false and the appended lazy rule.
		k.Clear()
		if k.IsInitialized() {
			t.Fatalf("expected uninitialized kernel after Clear")
		}
		// Ensure dirty is false before the probe (Clear does not set dirty,
		// but previous state was clean).
		if k.IsDirty() {
			t.Fatalf("expected clean dirty flag after Clear, got dirty")
		}

		// LoadFacts on an uninitialized kernel must evaluate eagerly
		// (boot semantics) so the kernel is ready before any query.
		if err := k.LoadFacts([]Fact{{Predicate: basePred, Args: []any{"world"}}}); err != nil {
			t.Fatalf("LoadFacts: %v", err)
		}
		// Eager path clears dirty and marks initialized.
		if k.IsDirty() {
			t.Fatalf("expected factsDirty=false after eager LoadFacts on uninitialized kernel, got true")
		}
		if !k.IsInitialized() {
			t.Fatalf("expected initialized=true after eager LoadFacts on uninitialized kernel")
		}
		// Derived fact must be immediately queryable (and Query must not need
		// to run a second fixpoint — but even if it does, result must be there).
		results, err := k.Query(derivedPred)
		if err != nil {
			t.Fatalf("Query(%s): %v", derivedPred, err)
		}
		if len(results) != 1 {
			t.Fatalf("Query(%s) = %d results, want 1", derivedPred, len(results))
		}
		if len(results[0].Args) == 0 || results[0].Args[0] != "world" {
			t.Fatalf("Query(%s) = %v, want lazy_derived(\"world\")", derivedPred, results[0])
		}
	})
}

// TestLoadFacts_DeferredIsCorrectOnRead_AdditionalBulk probes the bulk
// incremental-scan shape: multiple LoadFacts calls without interleaving
// queries must still be correct-on-read. Each LoadFacts marks dirty but
// does not evaluate; the first Query after the batch runs a single fixpoint.
func TestLoadFacts_DeferredIsCorrectOnRead_AdditionalBulk(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	policy := `
Decl bulk_base(Name).
Decl bulk_derived(Name).
bulk_derived(X) :- bulk_base(X).
`
	k.AppendPolicy(policy)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if k.IsDirty() {
		if _, err := k.Query("bulk_base"); err != nil {
			t.Fatalf("clear dirty: %v", err)
		}
	}

	// Simulate a scan that calls LoadFacts per file.
	factsBatch := [][]Fact{
		{{Predicate: "bulk_base", Args: []any{"a"}}},
		{{Predicate: "bulk_base", Args: []any{"b"}}},
		{{Predicate: "bulk_base", Args: []any{"c"}}},
	}
	for i, batch := range factsBatch {
		if err := k.LoadFacts(batch); err != nil {
			t.Fatalf("LoadFacts batch %d: %v", i, err)
		}
		if !k.IsDirty() {
			t.Fatalf("batch %d: expected dirty after deferred LoadFacts", i)
		}
	}
	// One query should materialize all three derivations with a single fixpoint.
	results, err := k.Query("bulk_derived")
	if err != nil {
		t.Fatalf("Query(bulk_derived): %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Query(bulk_derived) = %d results, want 3", len(results))
	}
}
