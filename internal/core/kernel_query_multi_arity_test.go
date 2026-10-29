package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestQueryAndQueryAllAgreeAcrossArities asserts the invariant that Query and
// QueryAll agree for predicates declared at multiple arities.
//
// Before the fix, Query's non-pattern branch broke after the first matching
// PredicateSym (map iteration order), so it returned facts for only one arity
// (0 when the unpopulated arity was visited first). QueryAll iterated every
// Decls entry but reset the accumulator per PredicateSym, so it kept only the
// last arity's facts. With independent random iteration order, Query and
// QueryAll disagreed ~75% of the time and each returned the wrong count ~50%
// of the time when only one arity was populated. A loop defeats the remaining
// nondeterminism: 20 iterations make a false-pass before the fix astronomically
// unlikely, while after the fix every iteration agrees.
func TestQueryAndQueryAllAgreeAcrossArities(t *testing.T) {
	k := setupMockKernel(t)

	// Declare the same symbol at two different arities.
	k.AppendPolicy("Decl multi_arity_probe(Name).")
	k.AppendPolicy("Decl multi_arity_probe(Name, Value).")
	require.NoError(t, k.Evaluate(), "Evaluate after adding multi-arity decls")

	// Populate only arity 1 (single-arg). Arity 2 stays empty.
	facts := []Fact{
		{Predicate: "multi_arity_probe", Args: []any{"a"}},
		{Predicate: "multi_arity_probe", Args: []any{"b"}},
		{Predicate: "multi_arity_probe", Args: []any{"c"}},
	}
	require.NoError(t, k.LoadFacts(facts), "LoadFacts for populated arity")

	// LoadFacts marks factsDirty; the next Query/QueryAll will drive
	// ensureEvaluated. No explicit Evaluate needed, but it is harmless.
	expected := len(facts)

	// Loop to defeat map-iteration randomization. Before the fix Query picks
	// the first matching arity (break) and QueryAll keeps the last arity
	// (reset), with independent random order per call, so they disagree on
	// most iterations and each mismatches expected on ~50% of iterations.
	// After the fix both accumulate across all arities and return expected.
	for i := 0; i < 20; i++ {
		q, err := k.Query("multi_arity_probe")
		require.NoError(t, err, "Query failed iteration %d", i)

		qa, err := k.QueryAll()
		require.NoError(t, err, "QueryAll failed iteration %d", i)
		qaSlice := qa["multi_arity_probe"]

		if len(q) != len(qaSlice) {
			t.Fatalf("invariant violation iteration %d: Query returned %d, QueryAll returned %d, expected %d and Query==QueryAll (multi-arity accumulation)", i, len(q), len(qaSlice), expected)
		}
		if len(q) != expected {
			t.Fatalf("count mismatch iteration %d: Query returned %d, QueryAll returned %d, expected %d (populated arity count)", i, len(q), len(qaSlice), expected)
		}
	}
}

// TestQueryCallbackAndQueryAllAgreeAcrossArities verifies that QueryCallback
// also accumulates across arities. It shared the same break bug as Query.
func TestQueryCallbackAndQueryAllAgreeAcrossArities(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl multi_arity_probe2(Name).")
	k.AppendPolicy("Decl multi_arity_probe2(Name, Value).")
	require.NoError(t, k.Evaluate())

	facts := []Fact{
		{Predicate: "multi_arity_probe2", Args: []any{"x"}},
		{Predicate: "multi_arity_probe2", Args: []any{"y"}},
	}
	require.NoError(t, k.LoadFacts(facts))
	expected := len(facts)

	for i := 0; i < 20; i++ {
		var cbCount int
		err := k.QueryCallback("multi_arity_probe2", func(f Fact) error {
			cbCount++
			return nil
		})
		require.NoError(t, err, "QueryCallback iteration %d", i)

		qa, err := k.QueryAll()
		require.NoError(t, err)
		qaSlice := qa["multi_arity_probe2"]

		if cbCount != len(qaSlice) {
			t.Fatalf("iteration %d: QueryCallback %d vs QueryAll %d, expected %d", i, cbCount, len(qaSlice), expected)
		}
		if cbCount != expected {
			t.Fatalf("iteration %d: QueryCallback %d vs expected %d", i, cbCount, expected)
		}
	}
}
