package core

import (
	"math"
	"sync/atomic"
	"testing"

	"codenerd/internal/types"
)

// A kernel write that does not land must not be silent.
//
// The reason this is enforced at the boundary rather than at the call sites is
// arithmetic: 130 places in this repo write `_ = kernel.Assert(...)`. Each is
// defensible alone -- a progress fact is not worth failing a campaign over --
// and together they mean the executive can lose a fact 130 different ways with
// nothing to point at afterwards. In an agent whose executive is a Datalog
// kernel, a fact that does not land is a decision that will not be made, and
// the symptom is always "the agent just didn't do the thing".
func TestFailedAssertIsCountedNotSwallowed(t *testing.T) {
	cortex := NewCortexKernel("shard_a")
	shard := setupTestShard(t, "shard_a", []string{"owned_pred"})
	if err := cortex.RegisterShard(shard); err != nil {
		t.Fatalf("RegisterShard: %v", err)
	}

	before := atomic.LoadInt64(&cortex.mutationFailCount)

	// A NaN cannot be encoded as a Mangle constant. Before the encoder was
	// fixed it became the empty string and was asserted anyway; now it is an
	// error, which is only an improvement if somebody hears about it.
	err := cortex.Assert(types.Fact{
		Predicate: "owned_pred",
		Args:      []any{map[string]any{"score": math.NaN()}},
	})
	if err == nil {
		t.Fatal("Assert accepted a fact argument it cannot encode; " +
			"the kernel now holds something that is not the value it was given")
	}

	after := atomic.LoadInt64(&cortex.mutationFailCount)
	if after != before+1 {
		t.Errorf("mutationFailCount = %d, want %d: a failed assert was not counted, "+
			"so the stats line will report a healthy kernel that lost a fact", after, before+1)
	}
}

// The counter must not move when nothing failed, or it is noise rather than a
// signal and the stats line stops meaning anything.
func TestSuccessfulAssertDoesNotCountAsFailure(t *testing.T) {
	cortex := NewCortexKernel("shard_a")
	shard := setupTestShard(t, "shard_a", []string{"owned_pred"})
	if err := cortex.RegisterShard(shard); err != nil {
		t.Fatalf("RegisterShard: %v", err)
	}

	before := atomic.LoadInt64(&cortex.mutationFailCount)
	if err := cortex.Assert(types.Fact{Predicate: "owned_pred", Args: []any{"ok"}}); err != nil {
		t.Fatalf("Assert of a plain fact: %v", err)
	}
	if got := atomic.LoadInt64(&cortex.mutationFailCount); got != before {
		t.Errorf("mutationFailCount moved to %d on a successful assert (was %d)", got, before)
	}
}

// An empty kernel is the one case where routing genuinely has nowhere to go.
// It must be reported through the same path, so the count covers it too.
func TestAssertWithNoShardsIsCounted(t *testing.T) {
	cortex := NewCortexKernel("shard_a")

	err := cortex.Assert(types.Fact{Predicate: "anything", Args: []any{"x"}})
	if err == nil {
		t.Fatal("Assert on a kernel with no shards reported success")
	}
	if got := atomic.LoadInt64(&cortex.mutationFailCount); got != 1 {
		t.Errorf("mutationFailCount = %d, want 1", got)
	}
}
