package typestest

import (
	"testing"

	"codenerd/internal/types"
)

func TestMockKernel_WhenUsedWithNewKernelTx_ShouldNotPanic(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()

	// The reason this mock exists: types.NewKernelTx panics on a Kernel that
	// does not implement KernelTransactor, so every hand-rolled mock without
	// Transaction() turns "the code started batching" into a panic in an
	// unrelated package's test run.
	tx := types.NewKernelTx(k)
	tx.Retract("shard_state")
	tx.Assert(types.Fact{Predicate: "shard_state", Args: []any{"coder", types.MangleAtom("/running")}})

	if got := k.FactCount("shard_state"); got != 0 {
		t.Fatalf("transaction leaked before Commit: %d facts", got)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error: %v", err)
	}
	if got := k.FactCount("shard_state"); got != 1 {
		t.Fatalf("after Commit: %d facts, want 1", got)
	}
	if k.Commits != 1 {
		t.Fatalf("Commits = %d, want 1 (one atomic update, not one per op)", k.Commits)
	}
}

func TestMockKernel_WhenAssertingAnUnconvertibleFact_ShouldReturnTheKernelError(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	// A real kernel rejects a nil argument at ToAtom; a mock that accepted it
	// would let a broken assert site pass its own tests.
	if err := k.Assert(types.Fact{Predicate: "p", Args: []any{nil}}); err == nil {
		t.Fatal("expected an error for a nil fact argument")
	}
	if k.FactCount("p") != 0 {
		t.Fatal("rejected fact was stored anyway")
	}
}

func TestMockKernel_WhenFactsAsserted_ShouldQueryBackByPredicate(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "tool_registered", Args: []any{"ripgrep", int64(1)}},
		{Predicate: "tool_registered", Args: []any{"fd", int64(2)}},
		{Predicate: "other", Args: []any{"x"}},
	}); err != nil {
		t.Fatalf("AssertBatch() error: %v", err)
	}

	facts, err := k.Query("tool_registered")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("Query() returned %d facts, want 2", len(facts))
	}

	if err := k.RetractFact(types.Fact{Predicate: "tool_registered", Args: []any{"ripgrep"}}); err != nil {
		t.Fatalf("RetractFact() error: %v", err)
	}
	if got := k.FactCount("tool_registered"); got != 1 {
		t.Fatalf("after RetractFact: %d facts, want 1", got)
	}
}

func TestMockKernel_WhenCommitFails_ShouldNotApplyBufferedOps(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	k.CommitErr = errBoom
	tx := k.Transaction()
	tx.Assert(types.Fact{Predicate: "p", Args: []any{"x"}})
	if err := tx.Commit(); err == nil {
		t.Fatal("expected the injected commit error")
	}
	if k.FactCount("p") != 0 {
		t.Fatal("a failed commit applied its operations")
	}
}

type boomError struct{}

func (boomError) Error() string { return "boom" }

var errBoom = boomError{}
