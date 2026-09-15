package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// queryHasArg reports whether any fact of pred carries want in position idx.
func queryHasArg(t *testing.T, k *RealKernel, pred string, idx int, want string) bool {
	t.Helper()
	facts, err := k.Query(pred)
	require.NoError(t, err, "Query(%s) failed — the fixpoint aborted", pred)
	for _, f := range facts {
		if len(f.Args) > idx && argString(f.Args[idx]) == want {
			return true
		}
	}
	return false
}

func argString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// A Go float64 asserted into a slot declared /number used to reach the store as
// an ast.Float64. The pinned Mangle fork compares int64 only, so the comparison
// in `intelligence_missing_tests(Path) :- intelligence_test_coverage(Path, Coverage), Coverage < 30.`
// returned an error rather than false — and EvalStratifiedProgram propagates it,
// so evaluate() bailed before committing the store and the WHOLE kernel stopped
// deriving. Live symptom: "value 110 (4) is not a number" ~4x/2s, with no
// predicate named.
//
// The integral float must therefore be narrowed to ast.Number at the boundary.
func TestFactBoundary_IntegralFloatIsCoercedIntoNumberSlot(t *testing.T) {
	k, err := NewRealKernel()
	require.NoError(t, err)

	// float64, not int — this is what encoding/json and every ratio-producing
	// Go call site hands us.
	require.NoError(t, k.AssertBatch([]Fact{
		{Predicate: "intelligence_test_coverage", Args: []any{"under_tested.go", float64(10)}},
	}))

	require.True(t,
		queryHasArg(t, k, "intelligence_missing_tests", 0, "under_tested.go"),
		"Coverage 10 < 30 must derive intelligence_missing_tests; a Float64 in a /number slot makes the comparison error out instead")
}

// The property that actually matters: a poisoned fact must cost only itself.
// Before the fix one bad row took down every derivation in the kernel, so an
// unrelated, perfectly valid fact silently stopped producing its conclusions.
func TestFactBoundary_BadFactDoesNotStopUnrelatedDerivations(t *testing.T) {
	k, err := NewRealKernel()
	require.NoError(t, err)

	// A fractional ratio in a /number slot is a genuine schema violation: it
	// cannot be represented, so it is rejected rather than silently truncated.
	// The control fact below shares no predicate with it.
	_ = k.AssertBatch([]Fact{
		{Predicate: "intelligence_test_coverage", Args: []any{"fractional.go", 0.85}},
	})

	require.NoError(t, k.AssertBatch([]Fact{
		{Predicate: "intelligence_churn_hotspot", Args: []any{"churn_file.go", 15, "high churn"}},
	}))

	require.True(t,
		queryHasArg(t, k, "context_priority", 0, "churn_file.go"),
		"an unrelated derivation must survive a rejected fact — this is the whole-kernel-outage regression")

	// And the offending row must not have been quietly rounded into the store.
	require.False(t,
		queryHasArg(t, k, "intelligence_missing_tests", 0, "fractional.go"),
		"0.85 must be rejected, not truncated to 0 and treated as < 30 coverage")
}

// An integral float outside the int64 range cannot be narrowed: float64→int64
// conversion there is implementation-defined (amd64 yields MinInt64), so the
// fact must be rejected, not stored as a wildly wrong value.
func TestFactBoundary_OutOfRangeFloatIsRejected(t *testing.T) {
	k, err := NewRealKernel()
	require.NoError(t, err)

	for _, v := range []float64{1e300, -1e300} {
		err := k.Assert(Fact{Predicate: "intelligence_test_coverage", Args: []any{"huge.go", v}})
		require.Error(t, err, "out-of-range %v must be rejected", v)
		require.Contains(t, err.Error(), "outside the int64 range")
	}

	// A large but representable integral float still narrows exactly.
	require.NoError(t, k.Assert(Fact{Predicate: "intelligence_test_coverage", Args: []any{"big.go", float64(int64(1) << 62)}}))

	// The kernel keeps deriving around the rejections.
	require.NoError(t, k.AssertBatch([]Fact{
		{Predicate: "intelligence_test_coverage", Args: []any{"ok.go", float64(10)}},
	}))
	require.True(t,
		queryHasArg(t, k, "intelligence_missing_tests", 0, "ok.go"),
		"unrelated derivation must survive out-of-range rejections")
	require.False(t,
		queryHasArg(t, k, "intelligence_missing_tests", 0, "huge.go"),
		"rejected rows must not appear in derivations")
}

// Integers keep flowing untouched — the coercion must not disturb the common path.
func TestFactBoundary_IntegerArgsAreUnchanged(t *testing.T) {
	k, err := NewRealKernel()
	require.NoError(t, err)

	require.NoError(t, k.AssertBatch([]Fact{
		{Predicate: "intelligence_test_coverage", Args: []any{"well_tested.go", 95}},
	}))

	require.True(t,
		queryHasArg(t, k, "intelligence_well_tested", 0, "well_tested.go"),
		"Coverage 95 > 70 must derive intelligence_well_tested")
	require.False(t,
		queryHasArg(t, k, "intelligence_missing_tests", 0, "well_tested.go"),
		"95 is not below the 30 threshold")
}
