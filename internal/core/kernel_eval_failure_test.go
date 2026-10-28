package core

import (
	"fmt"
	"testing"
)

// failureKernel is an evaluated kernel whose derived-fact ceiling is low
// enough that a long edge chain cannot be closed: the cone fails after it has
// already removed the touched predicates, and the full fallback fails too.
func failureKernel(t *testing.T) *RealKernel {
	t.Helper()
	k := newConeKernel(t)
	if err := k.Assert(Fact{Predicate: "cn_edge", Args: []any{"/seed_a", "/seed_b"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("cn_reach"); err != nil {
		t.Fatalf("baseline query: %v", err)
	}
	return k
}

func stageChain(t *testing.T, k *RealKernel, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		f := Fact{Predicate: "cn_edge", Args: []any{fmt.Sprintf("/n%d", i), fmt.Sprintf("/n%d", i+1)}}
		if err := k.AssertWithoutEval(f); err != nil {
			t.Fatalf("stage %d: %v", i, err)
		}
	}
}

func assertEveryReadFails(t *testing.T, k *RealKernel, why string) {
	t.Helper()
	rows := 0
	if facts, err := k.Query("cn_reach"); err == nil {
		t.Fatalf("%s: Query returned %d rows as a success", why, len(facts))
	}
	if err := k.QueryCallback("cn_reach", func(Fact) error { rows++; return nil }); err == nil {
		t.Fatalf("%s: QueryCallback succeeded", why)
	}
	if rows != 0 {
		t.Fatalf("%s: QueryCallback exposed %d rows of a failed evaluation", why, rows)
	}
	if all, err := k.QueryAll(); err == nil {
		t.Fatalf("%s: QueryAll returned %d predicates as a success", why, len(all))
	}
}

// A new staged fact is the case the audit named: AssertWithoutEval leaves
// factsDirty down, so before the latch a read after the failed EvaluatePending
// took the clean fast path and returned the half-rebuilt store.
func TestFailedEvaluationOfStagedFactsIsNotReadable(t *testing.T) {
	k := failureKernel(t)
	k.SetDerivedFactLimit(3)
	stageChain(t, k, 12)

	if err := k.EvaluatePending(); err == nil {
		t.Fatal("the chain closure must exceed the derived-fact limit")
	}
	assertEveryReadFails(t, k, "after EvaluatePending")
	assertEveryReadFails(t, k, "on a second read, still failing")
}

// An explicit Evaluate that fails leaves the previous store, which is stale
// rather than partial, but it is no more a fixpoint of the facts.
func TestFailedExplicitEvaluateIsNotReadable(t *testing.T) {
	k := failureKernel(t)
	k.SetDerivedFactLimit(3)
	stageChain(t, k, 12)
	if err := k.Evaluate(); err == nil {
		t.Fatal("Evaluate must fail")
	}
	assertEveryReadFails(t, k, "after Evaluate")
}

// The reader that passed ensureEvaluated before another goroutine's
// evaluation failed holds a clean view of factsDirty. The read lock check
// refuses it all the same.
func TestReaderThatPassedTheCleanCheckStillRefusesAFailedStore(t *testing.T) {
	k := failureKernel(t)
	k.SetDerivedFactLimit(3)
	stageChain(t, k, 12)
	if err := k.EvaluatePending(); err == nil {
		t.Fatal("setup: evaluation must fail")
	}
	k.factsDirty.Store(false) // what the racing reader saw before the failure
	if _, err := k.Query("cn_reach"); err == nil {
		t.Fatal("a store left by a failed evaluation was read as a success")
	}
}

// Once the cause is gone the retry repairs the store, equals a fresh full
// fixpoint, and nothing stays latched.
func TestFailedEvaluationRecoversToTheFullFixpoint(t *testing.T) {
	k := failureKernel(t)
	k.SetDerivedFactLimit(3)
	stageChain(t, k, 12)
	if err := k.EvaluatePending(); err == nil {
		t.Fatal("setup: evaluation must fail")
	}
	k.SetDerivedFactLimit(0)
	got := coneSnapshot(t, k)

	fresh := k.Clone()
	if err := fresh.Evaluate(); err != nil {
		t.Fatal(err)
	}
	if want := coneSnapshot(t, fresh); got != want {
		t.Fatalf("recovered store differs from a fresh fixpoint:\n got: %s\nwant: %s", got, want)
	}
	if err := k.readableLocked(); err != nil {
		t.Fatalf("still latched after a successful evaluation: %v", err)
	}
}

// A write that succeeds must not be poisoned by an earlier, repaired failure,
// and a clone of a failed kernel evaluates itself instead of inheriting its store.
func TestCloneOfFailedKernelDoesNotInheritTheStore(t *testing.T) {
	k := failureKernel(t)
	k.SetDerivedFactLimit(3)
	stageChain(t, k, 12)
	if err := k.EvaluatePending(); err == nil {
		t.Fatal("setup: evaluation must fail")
	}
	clone := k.Clone()
	if _, err := clone.Query("cn_reach"); err == nil {
		t.Fatal("clone of a failed kernel returned the failed store")
	}
}

// Failure before any mutation (an unparsable policy) keeps the kernel
// refusing reads and then recovers; the latch is not permanent.
func TestFailureIsNotPermanentPoisoning(t *testing.T) {
	k := failureKernel(t)
	k.SetDerivedFactLimit(3)
	stageChain(t, k, 12)
	_ = k.EvaluatePending()
	k.SetDerivedFactLimit(0)
	if err := k.Assert(Fact{Predicate: "cn_edge", Args: []any{"/x", "/y"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Query("cn_reach"); err != nil {
		t.Fatalf("a repaired kernel must answer: %v", err)
	}
}
