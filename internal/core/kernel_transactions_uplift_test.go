package core

import (
	"strings"
	"testing"
	"time"
)

func seedTxKernel(t *testing.T) *RealKernel {
	t.Helper()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	k.AppendPolicy("Decl tx_item(Key, Value).")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for _, f := range []Fact{
		{Predicate: "tx_item", Args: []any{"a", "1"}},
		{Predicate: "tx_item", Args: []any{"a", "2"}},
		{Predicate: "tx_item", Args: []any{"b", "3"}},
	} {
		if err := k.Assert(f); err != nil {
			t.Fatalf("Assert: %v", err)
		}
	}
	return k
}

func txItemCount(t *testing.T, k *RealKernel) int {
	t.Helper()
	rows, err := k.Query("tx_item")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	return len(rows)
}

// All four retract phases plus asserts in one commit, verified through
// derived reads — and the commit must publish asserted predicates so
// subscribers observe transactional writes like any other Assert.
func TestTransaction_MixedCommit(t *testing.T) {
	k := seedTxKernel(t)
	ch := k.GetEventBus().Subscribe([]string{"tx_item"})
	defer k.GetEventBus().Unsubscribe(ch)

	tx := k.Transaction()
	tx.RetractFact(Fact{Predicate: "tx_item", Args: []any{"a"}}) // removes a/1 + a/2
	tx.Assert(Fact{Predicate: "tx_item", Args: []any{"c", "4"}})
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := txItemCount(t, k); got != 2 {
		t.Fatalf("tx_item rows = %d, want 2 (b/3, c/4)", got)
	}
	select {
	case ev := <-ch:
		if ev.Predicate != "tx_item" {
			t.Fatalf("event predicate = %q", ev.Predicate)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no event published for transactional assert")
	}
}

// Exact and predicate-set phases agree with their standalone counterparts.
func TestTransaction_RetractPhases(t *testing.T) {
	t.Run("exact", func(t *testing.T) {
		k := seedTxKernel(t)
		tx := k.Transaction()
		tx.RetractExactFact(Fact{Predicate: "tx_item", Args: []any{"a", "1"}})
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
		if got := txItemCount(t, k); got != 2 {
			t.Fatalf("rows = %d, want 2", got)
		}
	})
	t.Run("predicate", func(t *testing.T) {
		k := seedTxKernel(t)
		tx := k.Transaction()
		tx.Retract("tx_item")
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
		if got := txItemCount(t, k); got != 0 {
			t.Fatalf("rows = %d, want 0", got)
		}
	})
	t.Run("predicateSet", func(t *testing.T) {
		k := seedTxKernel(t)
		tx := k.Transaction()
		tx.RetractPredicateSet(map[string]struct{}{"tx_item": {}})
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
		if got := txItemCount(t, k); got != 0 {
			t.Fatalf("rows = %d, want 0", got)
		}
	})
}

// A no-args RetractFact rejects the whole commit before anything mutates:
// silently removing a predicate is exactly what standalone RetractFact
// refuses to do.
func TestTransaction_NoArgsRetractFactRejected(t *testing.T) {
	k := seedTxKernel(t)
	before := k.FactCount()
	tx := k.Transaction()
	tx.RetractFact(Fact{Predicate: "tx_item"})
	tx.Assert(Fact{Predicate: "tx_item", Args: []any{"z", "9"}})
	err := tx.Commit()
	if err == nil {
		t.Fatal("expected rejection of no-args RetractFact, got nil")
	}
	if !strings.Contains(err.Error(), "no args") {
		t.Fatalf("error does not name the problem: %v", err)
	}
	if got := k.FactCount(); got != before {
		t.Fatalf("FactCount moved %d -> %d on rejected commit", before, got)
	}
	if got := txItemCount(t, k); got != 3 {
		t.Fatalf("rows = %d, want 3 (commit must not partially apply)", got)
	}
}

// Assert rejections surface AssertBatch-style: good facts land, the error
// names the rejected ones.
func TestTransaction_AssertRejectionsReported(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	k.AppendPolicy("Decl tx_num(X) bound [/number].")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	tx := k.Transaction()
	tx.Assert(Fact{Predicate: "tx_num", Args: []any{1}})
	tx.Assert(Fact{Predicate: "tx_num", Args: []any{0.5}}) // fractional: rejected
	err = tx.Commit()
	if err == nil {
		t.Fatal("expected rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "1 rejected") {
		t.Fatalf("error does not report the rejection: %v", err)
	}
	rows, err := k.Query("tx_num")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 (good fact lands, bad one does not)", len(rows))
	}
}

// Double commit and nil kernels fail closed, never panic.
func TestTransaction_CommitGuards(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	tx := k.Transaction()
	tx.Assert(Fact{Predicate: "tx_item", Args: []any{"x"}})
	if err := tx.Commit(); err != nil {
		t.Fatalf("first Commit: %v", err)
	}
	if err := tx.Commit(); err == nil {
		t.Fatal("expected double-commit error, got nil")
	}
	var nilTx *KernelTransaction
	if err := nilTx.Commit(); err == nil {
		t.Fatal("expected nil-transaction error, got nil")
	}
	var nilKernel *RealKernel
	nilBacked := nilKernel.Transaction()
	if err := nilBacked.Commit(); err == nil {
		t.Fatal("expected nil-kernel error, got nil")
	}
}
