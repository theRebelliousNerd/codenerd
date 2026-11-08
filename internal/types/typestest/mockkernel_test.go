package typestest

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/analysis"
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

func TestMockKernel_WhenLoadFactsCalled_ShouldStoreQueryableFacts(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.LoadFacts([]types.Fact{
		{Predicate: "tool_registered", Args: []any{"ripgrep"}},
		{Predicate: "tool_registered", Args: []any{"fd"}},
	}); err != nil {
		t.Fatalf("LoadFacts() error: %v", err)
	}
	if got := k.FactCount("tool_registered"); got != 2 {
		t.Fatalf("FactCount() = %d, want 2", got)
	}
	facts, err := k.Query("tool_registered")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("Query() returned %d facts, want 2", len(facts))
	}
}

func TestMockKernel_WhenQueryAllCalled_ShouldReturnCopyPerPredicate(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "a", Args: []any{"x"}},
		{Predicate: "b", Args: []any{"y"}},
	}); err != nil {
		t.Fatalf("AssertBatch() error: %v", err)
	}
	all, err := k.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll() error: %v", err)
	}
	if len(all) != 2 || len(all["a"]) != 1 || len(all["b"]) != 1 {
		t.Fatalf("QueryAll() = %v, want one fact under a and b", all)
	}
	all["a"][0].Args[0] = "mutated"
	again, err := k.Query("a")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(again) != 1 || again[0].Args[0] != "x" {
		t.Fatalf("QueryAll() leaked its backing array: %v", again)
	}
}

func TestMockKernel_WhenRetractCalled_ShouldRemovePredicate(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.Assert(types.Fact{Predicate: "shard_state", Args: []any{"coder"}}); err != nil {
		t.Fatalf("Assert() error: %v", err)
	}
	if err := k.Retract("shard_state"); err != nil {
		t.Fatalf("Retract() error: %v", err)
	}
	if got := k.FactCount("shard_state"); got != 0 {
		t.Fatalf("after Retract: %d facts, want 0", got)
	}
	facts, err := k.Query("shard_state")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("after Retract Query() = %v, want empty", facts)
	}
}

func TestMockKernel_WhenRetractFactCalled_ShouldMatchFirstArgOnly(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "p", Args: []any{"same", int64(1)}},
		{Predicate: "p", Args: []any{"same", int64(2)}},
		{Predicate: "p", Args: []any{"other", int64(3)}},
	}); err != nil {
		t.Fatalf("AssertBatch() error: %v", err)
	}
	if err := k.RetractFact(types.Fact{Predicate: "p", Args: []any{"same"}}); err != nil {
		t.Fatalf("RetractFact() error: %v", err)
	}
	if got := k.FactCount("p"); got != 1 {
		t.Fatalf("after RetractFact: %d facts, want 1", got)
	}
	rest, err := k.Query("p")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(rest) != 1 || rest[0].Args[0] != "other" {
		t.Fatalf("after RetractFact Query() = %v, want only the other fact", rest)
	}
}

func TestMockKernel_WhenRetractExactFactsBatchCalled_ShouldMatchAllArgsOnly(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "p", Args: []any{"same", int64(1)}},
		{Predicate: "p", Args: []any{"same", int64(2)}},
	}); err != nil {
		t.Fatalf("AssertBatch() error: %v", err)
	}
	if err := k.RetractExactFactsBatch([]types.Fact{{Predicate: "p", Args: []any{"same", int64(1)}}}); err != nil {
		t.Fatalf("RetractExactFactsBatch() error: %v", err)
	}
	if got := k.FactCount("p"); got != 1 {
		t.Fatalf("after RetractExactFactsBatch: %d facts, want 1", got)
	}
	rest, err := k.Query("p")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(rest) != 1 || rest[0].Args[1] != int64(2) {
		t.Fatalf("after RetractExactFactsBatch Query() = %v, want only the int64(2) fact", rest)
	}
}

func TestMockKernel_WhenRemoveFactsByPredicateSetCalled_ShouldRemoveListedPredicates(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "a", Args: []any{"x"}},
		{Predicate: "b", Args: []any{"y"}},
		{Predicate: "c", Args: []any{"z"}},
	}); err != nil {
		t.Fatalf("AssertBatch() error: %v", err)
	}
	if err := k.RemoveFactsByPredicateSet(map[string]struct{}{"a": {}, "b": {}}); err != nil {
		t.Fatalf("RemoveFactsByPredicateSet() error: %v", err)
	}
	if got := k.FactCount("a"); got != 0 {
		t.Fatalf("FactCount(a) = %d, want 0", got)
	}
	if got := k.FactCount("b"); got != 0 {
		t.Fatalf("FactCount(b) = %d, want 0", got)
	}
	if got := k.FactCount("c"); got != 1 {
		t.Fatalf("FactCount(c) = %d, want 1", got)
	}
}

func TestMockKernel_WhenResetCalled_ShouldClearFacts(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.Assert(types.Fact{Predicate: "p", Args: []any{"x"}}); err != nil {
		t.Fatalf("Assert() error: %v", err)
	}
	k.Reset()
	if got := k.FactCount("p"); got != 0 {
		t.Fatalf("after Reset FactCount(p) = %d, want 0", got)
	}
	all, err := k.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll() error: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("after Reset QueryAll() = %v, want empty", all)
	}
}

func TestMockKernel_WhenFailureInjected_ShouldReturnErrorWithoutStateChange(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	k.QueryErr = errBoom
	if _, err := k.Query("p"); err == nil {
		t.Fatal("Query() with QueryErr set should fail")
	}
	if _, err := k.QueryAll(); err == nil {
		t.Fatal("QueryAll() with QueryErr set should fail")
	}

	k2 := NewMockKernel()
	k2.LoadFactsErr = errBoom
	if err := k2.LoadFacts([]types.Fact{{Predicate: "p", Args: []any{"x"}}}); err == nil {
		t.Fatal("LoadFacts() with LoadFactsErr set should fail")
	}
	if got := k2.FactCount("p"); got != 0 {
		t.Fatalf("failed LoadFacts applied facts: %d", got)
	}

	k3 := NewMockKernel()
	if err := k3.Assert(types.Fact{Predicate: "p", Args: []any{"x"}}); err != nil {
		t.Fatalf("Assert() error: %v", err)
	}
	k3.RetractErr = errBoom
	if err := k3.Retract("p"); err == nil {
		t.Fatal("Retract() with RetractErr set should fail")
	}
	if got := k3.FactCount("p"); got != 1 {
		t.Fatalf("failed Retract removed facts: %d", got)
	}
	if err := k3.RetractFact(types.Fact{Predicate: "p", Args: []any{"x"}}); err == nil {
		t.Fatal("RetractFact() with RetractErr set should fail")
	}
	if got := k3.FactCount("p"); got != 1 {
		t.Fatalf("failed RetractFact removed facts: %d", got)
	}

	k4 := NewMockKernel()
	k4.AssertErr = errBoom
	if err := k4.Assert(types.Fact{Predicate: "p", Args: []any{"x"}}); err == nil {
		t.Fatal("Assert() with AssertErr set should fail")
	}
	if got := k4.FactCount("p"); got != 0 {
		t.Fatalf("failed Assert stored facts: %d", got)
	}
}

func TestMockKernel_WhenGetProgramInfoCalled_ShouldReturnNilUnlessFnSet(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if got := k.GetProgramInfo(); got != nil {
		t.Fatalf("GetProgramInfo() = %v, want nil without ProgramInfoFn", got)
	}
}

func TestMockKernel_WhenUpdateSystemFactsAndAppendPolicyCalled_ShouldSucceed(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.UpdateSystemFacts(); err != nil {
		t.Fatalf("UpdateSystemFacts() error: %v", err)
	}
	k.AppendPolicy("parent(X, Y) :- child(X, Y).")
	if len(k.Calls) == 0 {
		t.Fatal("expected Calls to record UpdateSystemFacts and AppendPolicy")
	}
}

func TestMockKernel_WhenTransactionBuffersAllOpKinds_ShouldApplyAtomicallyOnCommit(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "gone", Args: []any{"x"}},
		{Predicate: "first", Args: []any{"keep"}},
		{Predicate: "first", Args: []any{"drop"}},
		{Predicate: "exact", Args: []any{"a", int64(1)}},
		{Predicate: "exact", Args: []any{"a", int64(2)}},
		{Predicate: "set_gone", Args: []any{"x"}},
		{Predicate: "set_kept", Args: []any{"y"}},
	}); err != nil {
		t.Fatalf("AssertBatch() error: %v", err)
	}
	tx := k.Transaction()
	tx.Retract("gone")
	tx.RetractFact(types.Fact{Predicate: "first", Args: []any{"drop"}})
	tx.RetractExactFact(types.Fact{Predicate: "exact", Args: []any{"a", int64(1)}})
	tx.RetractPredicateSet(map[string]struct{}{"set_gone": {}})
	tx.Assert(types.Fact{Predicate: "new_pred", Args: []any{"n"}})
	if got := k.FactCount("new_pred"); got != 0 {
		t.Fatalf("transaction leaked before Commit: %d facts", got)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error: %v", err)
	}
	if got := k.FactCount("gone"); got != 0 {
		t.Fatalf("FactCount(gone) = %d, want 0", got)
	}
	if got := k.FactCount("first"); got != 1 {
		t.Fatalf("FactCount(first) = %d, want 1", got)
	}
	if got := k.FactCount("exact"); got != 1 {
		t.Fatalf("FactCount(exact) = %d, want 1", got)
	}
	rest, err := k.Query("exact")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(rest) != 1 || rest[0].Args[1] != int64(2) {
		t.Fatalf("after tx Commit Query(exact) = %v, want only the int64(2) fact", rest)
	}
	if got := k.FactCount("set_gone"); got != 0 {
		t.Fatalf("FactCount(set_gone) = %d, want 0", got)
	}
	if got := k.FactCount("set_kept"); got != 1 {
		t.Fatalf("FactCount(set_kept) = %d, want 1", got)
	}
	if got := k.FactCount("new_pred"); got != 1 {
		t.Fatalf("FactCount(new_pred) = %d, want 1", got)
	}
	if k.Commits != 1 {
		t.Fatalf("Commits = %d, want 1", k.Commits)
	}
}

// noTxKernel is a types.Kernel that cannot transact: embedding the Kernel
// INTERFACE supplies every Kernel method while deliberately not promoting
// Transaction(), which is exactly the shape of the three production forwarding
// adapters that lose the capability.
type noTxKernel struct{ types.Kernel }

func TestNewKernelTx_WhenKernelCannotTransact_ShouldPanicNamingTheConcreteType(t *testing.T) {
	t.Parallel()
	k := noTxKernel{NewMockKernel()}

	if _, ok := types.TransactorOf(k); ok {
		t.Fatal("TransactorOf reported a transactor for a kernel that has no Transaction()")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewKernelTx did not panic on a non-transacting kernel")
		}
		msg, _ := r.(string)
		// The message has to name the wrapper, not the kernel behind it —
		// otherwise it sends whoever reads it to the wrong type.
		if !strings.Contains(msg, "noTxKernel") {
			t.Fatalf("panic message does not name the concrete type: %v", r)
		}
		if !strings.Contains(msg, "Transaction()") {
			t.Fatalf("panic message does not show the fix: %v", r)
		}
	}()
	types.NewKernelTx(k)
}

type boomError struct{}

func (boomError) Error() string { return "boom" }

var errBoom = boomError{}

func TestMockKernel_WhenRetractFactRemovesLastFact_ShouldDeletePredicateEntry(t *testing.T) {
	k := NewMockKernel()
	f := types.Fact{Predicate: "retract_last_pred", Args: []any{"a"}}
	if err := k.Assert(f); err != nil {
		t.Fatalf("Assert failed: %v", err)
	}
	if got := k.FactCount("retract_last_pred"); got != 1 {
		t.Fatalf("FactCount before RetractFact = %d, want 1", got)
	}
	if err := k.RetractFact(f); err != nil {
		t.Fatalf("RetractFact failed: %v", err)
	}
	if got := k.FactCount("retract_last_pred"); got != 0 {
		t.Fatalf("FactCount after RetractFact = %d, want 0", got)
	}
	got, err := k.Query("retract_last_pred")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Query after last RetractFact = %d facts, want 0", len(got))
	}
	all, err := k.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll failed: %v", err)
	}
	if _, ok := all["retract_last_pred"]; ok {
		t.Fatalf("QueryAll still contains predicate after last RetractFact was removed")
	}
}

func TestMockKernel_WhenProgramInfoFnSet_ShouldReturnFnResult(t *testing.T) {
	k := NewMockKernel()
	if got := k.GetProgramInfo(); got != nil {
		t.Fatalf("GetProgramInfo without ProgramInfoFn = %v, want nil", got)
	}
	want := &analysis.ProgramInfo{}
	k.ProgramInfoFn = func() *analysis.ProgramInfo { return want }
	got := k.GetProgramInfo()
	if got == nil {
		t.Fatalf("GetProgramInfo with ProgramInfoFn set = nil, want non-nil")
	}
	if got != want {
		t.Fatalf("GetProgramInfo did not return ProgramInfoFn result")
	}
}

func TestMockKernel_WhenRetractExactBatchRemovesLastFact_ShouldDeletePredicateEntry(t *testing.T) {
	k := NewMockKernel()
	f := types.Fact{Predicate: "retract_exact_last_pred", Args: []any{"a", "b"}}
	if err := k.Assert(f); err != nil {
		t.Fatalf("Assert failed: %v", err)
	}
	if err := k.RetractExactFactsBatch([]types.Fact{f}); err != nil {
		t.Fatalf("RetractExactFactsBatch failed: %v", err)
	}
	if got := k.FactCount("retract_exact_last_pred"); got != 0 {
		t.Fatalf("FactCount after RetractExactFactsBatch = %d, want 0", got)
	}
	got, err := k.Query("retract_exact_last_pred")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Query after last RetractExactFactsBatch = %d facts, want 0", len(got))
	}
	all, err := k.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll failed: %v", err)
	}
	if _, ok := all["retract_exact_last_pred"]; ok {
		t.Fatalf("QueryAll still contains predicate after last RetractExactFactsBatch was removed")
	}
}

func TestMockKernel_WhenQueryReturnsFacts_ShouldIsolateArgsBackingArray(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.Assert(types.Fact{Predicate: "iso_pred", Args: []any{"orig", int64(1)}}); err != nil {
		t.Fatalf("Assert() error: %v", err)
	}
	first, err := k.Query("iso_pred")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("Query() returned %d facts, want 1", len(first))
	}
	first[0].Args[0] = "mutated"
	first[0].Args[1] = int64(99)
	second, err := k.Query("iso_pred")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(second) != 1 || second[0].Args[0] != "orig" || second[0].Args[1] != int64(1) {
		t.Fatalf("Query() leaked Args backing array: %v", second)
	}
}

func TestMockKernel_WhenQueryReturnsNilArgs_ShouldPreserveNil(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.LoadFacts([]types.Fact{{Predicate: "nil_pred"}}); err != nil {
		t.Fatalf("LoadFacts() error: %v", err)
	}
	got, err := k.Query("nil_pred")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Query() returned %d facts, want 1", len(got))
	}
	if got[0].Args != nil {
		t.Fatalf("Query() Args = %v, want nil to preserve nil-ness", got[0].Args)
	}
}

func TestMockKernel_WhenQueryAllIsolatesArgs_ShouldNotLeakBackingArray(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.LoadFacts([]types.Fact{
		{Predicate: "qa_nil"},
		{Predicate: "qa_vals", Args: []any{"v"}},
	}); err != nil {
		t.Fatalf("LoadFacts() error: %v", err)
	}
	all, err := k.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll() error: %v", err)
	}
	if len(all["qa_nil"]) != 1 || all["qa_nil"][0].Args != nil {
		t.Fatalf("QueryAll() nil Args not preserved: %v", all["qa_nil"])
	}
	all["qa_vals"][0].Args[0] = "mutated"
	again, err := k.Query("qa_vals")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(again) != 1 || again[0].Args[0] != "v" {
		t.Fatalf("QueryAll() leaked Args backing array: %v", again)
	}
	againAll, err := k.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll() error: %v", err)
	}
	if againAll["qa_vals"][0].Args[0] != "v" {
		t.Fatalf("QueryAll() mutation leaked into store: %v", againAll)
	}
}

func TestMockKernel_WhenGetProgramInfoCalledRepeatedly_ShouldInvokeFnEachTime(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	calls := 0
	k.ProgramInfoFn = func() *analysis.ProgramInfo {
		calls++
		return &analysis.ProgramInfo{}
	}
	first := k.GetProgramInfo()
	second := k.GetProgramInfo()
	if first == nil || second == nil {
		t.Fatalf("GetProgramInfo() = %v, %v, want non-nil each call", first, second)
	}
	if first == second {
		t.Fatalf("GetProgramInfo() returned same pointer twice, want fresh Fn result per call")
	}
	if calls != 2 {
		t.Fatalf("ProgramInfoFn calls = %d, want 2", calls)
	}
}

func TestFilterFacts_WhenKeepSelectsSubset_ShouldPreserveOrderAndHandleEmpty(t *testing.T) {
	t.Parallel()
	facts := []types.Fact{
		{Predicate: "p", Args: []any{"a"}},
		{Predicate: "p", Args: []any{"b"}},
		{Predicate: "p", Args: []any{"c"}},
	}
	kept := filterFacts(facts, func(f types.Fact) bool {
		return f.Args[0] != "b"
	})
	if len(kept) != 2 || kept[0].Args[0] != "a" || kept[1].Args[0] != "c" {
		t.Fatalf("filterFacts() = %v, want [a c] in order", kept)
	}
	if len(facts) != 3 {
		t.Fatalf("filterFacts() modified input: %v", facts)
	}
	if out := filterFacts(nil, func(f types.Fact) bool { return true }); len(out) != 0 {
		t.Fatalf("filterFacts(nil) = %v, want empty", out)
	}
	if out := filterFacts(facts, func(f types.Fact) bool { return false }); len(out) != 0 {
		t.Fatalf("filterFacts(all-false) = %v, want empty", out)
	}
}

func TestMockKernel_WhenCommitErrSet_ShouldReturnErrorWithoutApplyingOps(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.Assert(types.Fact{Predicate: "stable", Args: []any{"x"}}); err != nil {
		t.Fatalf("Assert() error: %v", err)
	}
	k.CommitErr = errBoom
	tx := k.Transaction()
	tx.Assert(types.Fact{Predicate: "new_pred", Args: []any{"n"}})
	tx.Retract("stable")
	if err := tx.Commit(); err == nil {
		t.Fatal("Commit() with CommitErr set should fail")
	}
	if got := k.FactCount("new_pred"); got != 0 {
		t.Fatalf("failed Commit applied assert: %d facts", got)
	}
	if got := k.FactCount("stable"); got != 1 {
		t.Fatalf("failed Commit removed facts: %d", got)
	}
	if k.Commits != 0 {
		t.Fatalf("Commits = %d, want 0 after failed Commit", k.Commits)
	}
	k.CommitErr = nil
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() after clearing CommitErr error: %v", err)
	}
	if got := k.FactCount("new_pred"); got != 1 {
		t.Fatalf("FactCount(new_pred) = %d, want 1 after retry", got)
	}
	if k.Commits != 1 {
		t.Fatalf("Commits = %d, want 1", k.Commits)
	}
}

func TestMockKernel_WhenTxRetractPredicateSetBuffersCopy_ShouldNotObserveCallerMutation(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "txset_a", Args: []any{"x"}},
		{Predicate: "txset_b", Args: []any{"y"}},
	}); err != nil {
		t.Fatalf("AssertBatch() error: %v", err)
	}
	set := map[string]struct{}{"txset_a": {}}
	tx := k.Transaction()
	tx.RetractPredicateSet(set)
	set["txset_b"] = struct{}{}
	delete(set, "txset_a")
	if got := k.FactCount("txset_a"); got != 1 {
		t.Fatalf("transaction leaked before Commit: %d facts", got)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error: %v", err)
	}
	if got := k.FactCount("txset_a"); got != 0 {
		t.Fatalf("FactCount(txset_a) = %d, want 0 after buffered retract_set", got)
	}
	if got := k.FactCount("txset_b"); got != 1 {
		t.Fatalf("FactCount(txset_b) = %d, want 1 (caller mutation must not leak into tx)", got)
	}
}

func TestMockKernel_WhenProgramInfoFnReentersKernel_ShouldNotDeadlock(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.Assert(types.Fact{Predicate: "reenter_pred", Args: []any{"x"}}); err != nil {
		t.Fatalf("Assert() error: %v", err)
	}
	k.ProgramInfoFn = func() *analysis.ProgramInfo {
		_ = k.FactCount("reenter_pred")
		if _, err := k.Query("reenter_pred"); err != nil {
			panic(err)
		}
		if _, err := k.QueryAll(); err != nil {
			panic(err)
		}
		return &analysis.ProgramInfo{}
	}
	done := make(chan *analysis.ProgramInfo, 1)
	go func() { done <- k.GetProgramInfo() }()
	select {
	case got := <-done:
		if got == nil {
			t.Fatalf("GetProgramInfo() with reentrant Fn = nil, want non-nil")
		}
		if got := k.FactCount("reenter_pred"); got != 1 {
			t.Fatalf("reentrant Fn observed FactCount = %d, want 1", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("GetProgramInfo() deadlocked when ProgramInfoFn re-entered the kernel")
	}
}

func TestFilterFacts_WhenOutputMutated_ShouldNotAffectInputAndPreserveNonNilEmpty(t *testing.T) {
	t.Parallel()
	facts := []types.Fact{
		{Predicate: "p", Args: []any{"a"}},
		{Predicate: "p", Args: []any{"b"}},
	}
	kept := filterFacts(facts, func(f types.Fact) bool { return true })
	if len(kept) != 2 {
		t.Fatalf("filterFacts(all-keep) = %d facts, want 2", len(kept))
	}
	kept[0] = types.Fact{Predicate: "p", Args: []any{"mutated"}}
	if facts[0].Args[0] != "a" {
		t.Fatalf("filterFacts() shares backing array with input: %v", facts)
	}
	none := filterFacts(facts, func(f types.Fact) bool { return false })
	if len(none) != 0 {
		t.Fatalf("filterFacts(all-drop) = %v, want empty", none)
	}
	if none == nil {
		t.Fatalf("filterFacts(all-drop) = nil, want non-nil empty slice")
	}
	nilOut := filterFacts(nil, func(f types.Fact) bool { return true })
	if len(nilOut) != 0 {
		t.Fatalf("filterFacts(nil) = %v, want empty", nilOut)
	}
	if nilOut == nil {
		t.Fatalf("filterFacts(nil) = nil, want non-nil empty slice")
	}
}

func TestMockKernel_WhenCommitOpFailsMidway_ShouldAbortAndNotCountCommit(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	tx := k.Transaction()
	tx.Assert(types.Fact{Predicate: "commit_ok", Args: []any{"x"}})
	tx.Assert(types.Fact{Predicate: "commit_bad", Args: []any{nil}})
	tx.Assert(types.Fact{Predicate: "commit_never", Args: []any{"y"}})
	if err := tx.Commit(); err == nil {
		t.Fatal("Commit() with an unconvertible buffered assert should fail")
	}
	if k.Commits != 0 {
		t.Fatalf("Commits = %d, want 0 after failed Commit", k.Commits)
	}
	if got := k.FactCount("commit_never"); got != 0 {
		t.Fatalf("failed Commit applied ops after the failing op: %d facts", got)
	}
}

func TestMockKernel_WhenCommitSucceeds_ShouldClearOpsSoSecondCommitIsNoop(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.Assert(types.Fact{Predicate: "clear_pred", Args: []any{"x"}}); err != nil {
		t.Fatalf("Assert() error: %v", err)
	}
	tx := k.Transaction()
	tx.Retract("clear_pred")
	if err := tx.Commit(); err != nil {
		t.Fatalf("first Commit() error: %v", err)
	}
	if got := k.FactCount("clear_pred"); got != 0 {
		t.Fatalf("after first Commit FactCount = %d, want 0", got)
	}
	if err := k.Assert(types.Fact{Predicate: "clear_pred", Args: []any{"reasserted"}}); err != nil {
		t.Fatalf("re-Assert() error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("second Commit() error: %v", err)
	}
	if got := k.FactCount("clear_pred"); got != 1 {
		t.Fatalf("second Commit re-applied cleared ops: FactCount = %d, want 1", got)
	}
}

func TestMockKernel_WhenGetProgramInfoCalled_ShouldNotAffectStoreOrCalls(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.Assert(types.Fact{Predicate: "keep_pred", Args: []any{"x"}}); err != nil {
		t.Fatalf("Assert() error: %v", err)
	}
	callsBefore := len(k.Calls)
	if got := k.GetProgramInfo(); got != nil {
		t.Fatalf("GetProgramInfo() without Fn = %v, want nil", got)
	}
	if got := k.FactCount("keep_pred"); got != 1 {
		t.Fatalf("GetProgramInfo() changed store: FactCount = %d, want 1", got)
	}
	if len(k.Calls) != callsBefore {
		t.Fatalf("GetProgramInfo() recorded %d calls, want %d (should be side-effect free)", len(k.Calls), callsBefore)
	}
	k.ProgramInfoFn = func() *analysis.ProgramInfo { return nil }
	if got := k.GetProgramInfo(); got != nil {
		t.Fatalf("GetProgramInfo() with nil-returning Fn = %v, want nil", got)
	}
	k.ProgramInfoFn = nil
	if got := k.GetProgramInfo(); got != nil {
		t.Fatalf("GetProgramInfo() after clearing Fn = %v, want nil", got)
	}
}

func TestMockKernel_WhenCommitEmpty_ShouldSucceedAndCountCommit(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	tx := k.Transaction()
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() with no ops error: %v", err)
	}
	if k.Commits != 1 {
		t.Fatalf("Commits = %d, want 1 after empty Commit", k.Commits)
	}
}

func TestMockKernel_WhenCommitOpHitsInjectedRetractErr_ShouldAbortWithoutCountingCommit(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "cr_a", Args: []any{"x"}},
		{Predicate: "cr_b", Args: []any{"y"}},
	}); err != nil {
		t.Fatalf("AssertBatch() error: %v", err)
	}
	k.RetractErr = errBoom
	tx := k.Transaction()
	tx.Retract("cr_a")
	tx.Assert(types.Fact{Predicate: "cr_c", Args: []any{"z"}})
	if err := tx.Commit(); err == nil {
		t.Fatal("Commit() with injected RetractErr should fail")
	}
	if k.Commits != 0 {
		t.Fatalf("Commits = %d, want 0 after failed Commit", k.Commits)
	}
	k.RetractErr = nil
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() after clearing RetractErr error: %v", err)
	}
	if got := k.FactCount("cr_a"); got != 0 {
		t.Fatalf("FactCount(cr_a) = %d, want 0 after retry", got)
	}
	if got := k.FactCount("cr_c"); got != 1 {
		t.Fatalf("FactCount(cr_c) = %d, want 1 after retry", got)
	}
	if k.Commits != 1 {
		t.Fatalf("Commits = %d, want 1", k.Commits)
	}
}

func TestMockKernel_WhenRetractFactCalledWithNoArgs_ShouldReturnErrorWithoutRemovingFacts(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.Assert(types.Fact{Predicate: "p", Args: []any{"x"}}); err != nil {
		t.Fatalf("Assert() error: %v", err)
	}
	if err := k.RetractFact(types.Fact{Predicate: "p"}); err == nil {
		t.Fatal("RetractFact() with no args should fail closed")
	}
	if got := k.FactCount("p"); got != 1 {
		t.Fatalf("failed RetractFact removed facts: %d", got)
	}
}

func TestMockKernel_WhenAssertBatchPartiallyRejects_ShouldStoreGoodFactsAndReportCounts(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	err := k.AssertBatch([]types.Fact{
		{Predicate: "p", Args: []any{"a"}},
		{Predicate: "p", Args: []any{nil}},
		{Predicate: "p", Args: []any{"b"}},
	})
	if err == nil {
		t.Fatal("AssertBatch() with one unconvertible fact should fail")
	}
	if !strings.Contains(err.Error(), "rejected 1 of 3") {
		t.Fatalf("AssertBatch() error = %v, want rejected 1 of 3", err)
	}
	if got := k.FactCount("p"); got != 2 {
		t.Fatalf("FactCount(p) = %d, want 2 good facts stored", got)
	}
}

func TestMockKernel_WhenRetractExactAndPredicateSetHitRetractErr_ShouldFailWithoutStateChange(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "a", Args: []any{"x"}},
		{Predicate: "b", Args: []any{"y"}},
	}); err != nil {
		t.Fatalf("AssertBatch() error: %v", err)
	}
	k.RetractErr = errBoom
	if err := k.RetractExactFactsBatch([]types.Fact{{Predicate: "a", Args: []any{"x"}}}); err == nil {
		t.Fatal("RetractExactFactsBatch() with RetractErr set should fail")
	}
	if got := k.FactCount("a"); got != 1 {
		t.Fatalf("failed RetractExactFactsBatch removed facts: %d", got)
	}
	if err := k.RemoveFactsByPredicateSet(map[string]struct{}{"b": {}}); err == nil {
		t.Fatal("RemoveFactsByPredicateSet() with RetractErr set should fail")
	}
	if got := k.FactCount("b"); got != 1 {
		t.Fatalf("failed RemoveFactsByPredicateSet removed facts: %d", got)
	}
}

func TestMockKernel_WhenZeroValueKernelUsed_ShouldInitMapLazily(t *testing.T) {
	t.Parallel()
	var k MockKernel
	if err := k.Assert(types.Fact{Predicate: "p", Args: []any{"x"}}); err != nil {
		t.Fatalf("Assert() on zero value error: %v", err)
	}
	if got := k.FactCount("p"); got != 1 {
		t.Fatalf("FactCount(p) = %d, want 1", got)
	}
	if _, err := k.Query("p"); err != nil {
		t.Fatalf("Query() on zero value error: %v", err)
	}
	var k2 MockKernel
	if _, err := k2.Query("missing"); err != nil {
		t.Fatalf("Query() on empty zero value error: %v", err)
	}
	if _, err := k2.QueryAll(); err != nil {
		t.Fatalf("QueryAll() on empty zero value error: %v", err)
	}
}

func TestMockKernel_WhenQueryMissingPredicate_ShouldReturnNonNilEmpty(t *testing.T) {
	t.Parallel()
	k := NewMockKernel()
	facts, err := k.Query("missing")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("Query(missing) = %v, want empty", facts)
	}
	if facts == nil {
		t.Fatal("Query(missing) = nil, want non-nil empty slice")
	}
}

func TestMockKernel_WhenAssertBatchEmpty_ShouldReturnNilWithoutRecording(t *testing.T) {
	m := NewMockKernel()
	if err := m.AssertBatch(nil); err != nil {
		t.Fatalf("AssertBatch(nil) = %v, want nil", err)
	}
	if len(m.Calls) != 0 {
		t.Fatalf("AssertBatch(nil) recorded %d calls, want 0 (early return before record)", len(m.Calls))
	}
	if got := m.FactCount("p"); got != 0 {
		t.Fatalf("FactCount(p) = %d, want 0 after empty batch", got)
	}
	if err := m.AssertBatch([]types.Fact{}); err != nil {
		t.Fatalf("AssertBatch(empty) = %v, want nil", err)
	}
}

func TestMockKernel_WhenRetractFactWithArgs_ShouldSucceedAndFailClosedOnNoArgs(t *testing.T) {
	mkFact := func(predicate string, variant int) types.Fact {
		f := types.Fact{Predicate: predicate}
		rv := reflect.ValueOf(&f).Elem().FieldByName("Args")
		if !rv.IsValid() || !rv.CanSet() {
			t.Fatalf("Fact has no settable Args field")
		}
		elemType := rv.Type().Elem()
		var candidates []reflect.Value
		if variant == 0 {
			candidates = []reflect.Value{
				reflect.ValueOf(types.Atom("/a")),
				reflect.ValueOf("/a"),
				reflect.ValueOf(int64(1)),
				reflect.ValueOf(true),
			}
		} else {
			candidates = []reflect.Value{
				reflect.ValueOf(types.Atom("/b")),
				reflect.ValueOf("/b"),
				reflect.ValueOf(int64(2)),
				reflect.ValueOf(false),
			}
		}
		picked := reflect.Zero(elemType)
		for _, c := range candidates {
			if !c.IsValid() {
				continue
			}
			if c.Type().AssignableTo(elemType) {
				picked = c
				break
			}
			if c.Type().ConvertibleTo(elemType) {
				picked = c.Convert(elemType)
				break
			}
		}
		s := reflect.MakeSlice(rv.Type(), 1, 1)
		s.Index(0).Set(picked)
		rv.Set(s)
		return f
	}
	k := NewMockKernel()
	f1 := mkFact("testfact", 0)
	f2 := mkFact("testfact", 1)
	if err := k.LoadFacts([]types.Fact{f1, f2}); err != nil {
		t.Fatalf("LoadFacts failed: %v", err)
	}
	if got := k.FactCount("testfact"); got != 2 {
		t.Fatalf("FactCount = %d, want 2", got)
	}
	if err := k.RetractFact(f1); err != nil {
		t.Fatalf("RetractFact with args should succeed, got error: %v", err)
	}
	if got := k.FactCount("testfact"); got != 1 {
		t.Fatalf("FactCount after retract = %d, want 1", got)
	}
	empty := types.Fact{Predicate: "testfact"}
	if err := k.RetractFact(empty); err == nil {
		t.Fatalf("RetractFact with no args should fail closed, got nil error")
	}
	if got := k.FactCount("testfact"); got != 1 {
		t.Fatalf("FactCount after failed retract = %d, want 1", got)
	}
}
