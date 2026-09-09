package campaign

import (
	"strings"
	"testing"

	"codenerd/internal/core"
)

// The campaign context-paging path had nine bare `cp.kernel.Assert(f)` calls in
// the fallback after a rejected AssertBatch. A paging pass that landed nothing
// was indistinguishable from one that landed everything, so a phase could run
// with the wrong activation profile and nothing anywhere said why.

func newAssertTestKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("new kernel: %v", err)
	}
	return k
}

func TestAssertFactsWithFallback_GoodFactsLandWhenTheBatchIsRejected(t *testing.T) {
	k := newAssertTestKernel(t)

	// dream_preference's second slot is bound /number; a fractional float is
	// rejected, which fails the whole batch.
	assertFactsWithFallback(k, []core.Fact{
		{Predicate: "dream_preference", Args: []any{"keeps the good one", int64(90)}},
		{Predicate: "dream_preference", Args: []any{"drops the bad one", 0.42}},
	}, "test batch")

	facts, err := k.Query("dream_preference")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("the per-fact fallback did not salvage the good fact: %v", facts)
	}
	if !strings.Contains(facts[0].String(), "keeps the good one") {
		t.Errorf("wrong fact survived: %v", facts[0])
	}
}

func TestAssertFactsWithFallback_NilKernelAndEmptyBatchAreNoOps(t *testing.T) {
	assertFactsWithFallback(nil, []core.Fact{{Predicate: "dream_preference", Args: []any{"x", int64(1)}}}, "nil kernel")
	assertFactsWithFallback(newAssertTestKernel(t), nil, "empty batch")
}
