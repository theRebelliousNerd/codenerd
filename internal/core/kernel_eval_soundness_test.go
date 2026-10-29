package core

import (
	"sort"
	"testing"
)

// TestKernelEvalUnDerivesUnderNegation pins the one property a kernel
// evaluator cannot trade away: a derived fact whose negated premise becomes
// true must stop being derived.
//
// This is the seam-S23 regression. The differential path kept one persistent
// store that accumulated EDB *and* IDB facts across evaluate() calls and
// re-ran the semi-naive bottom-up evaluator over it. Semi-naive evaluation
// over a store that already holds previously derived facts is monotone — it
// can only add — so `s23_open(/o1)`, once derived, survived the assertion of
// `s23_discharged(/o1)` forever. The full path rebuilds the store from the
// EDB on every evaluate and is sound.
func TestKernelEvalUnDerivesUnderNegation(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy(`
	Decl s23_raised(Id).
	Decl s23_discharged(Id).
	Decl s23_open(Id).

	s23_open(X) :- s23_raised(X), !s23_discharged(X).
	`)

	if err := k.Assert(Fact{Predicate: "s23_raised", Args: []any{"/o1"}}); err != nil {
		t.Fatalf("Assert s23_raised: %v", err)
	}
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate after raise: %v", err)
	}
	open, err := k.Query("s23_open")
	if err != nil {
		t.Fatalf("Query s23_open: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("after raise: s23_open = %v, want exactly 1 fact", open)
	}

	// The negated premise becomes true. The derived fact must disappear.
	if err := k.Assert(Fact{Predicate: "s23_discharged", Args: []any{"/o1"}}); err != nil {
		t.Fatalf("Assert s23_discharged: %v", err)
	}
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate after discharge: %v", err)
	}
	open, err = k.Query("s23_open")
	if err != nil {
		t.Fatalf("Query s23_open after discharge: %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("after discharge: s23_open = %v, want no facts — a derived fact "+
			"whose negated premise became true was never retracted", open)
	}
}

// TestKernelEvalReplacesAggregateResult pins the aggregation half of the same
// property: an aggregate must be replaced when its input grows, not added
// beside its previous value. On the differential path the second evaluate
// derived s23_item_count(2) into a store that still held s23_item_count(1),
// so the kernel reported two mutually contradictory counts at once and every
// `Count >= N` rule downstream fired on the stale one.
func TestKernelEvalReplacesAggregateResult(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy(`
	Decl s23_item(Name).
	Decl s23_item_count(Count).

	s23_item_count(N) :-
	    s23_item(_) |>
	    do fn:group_by(),
	    let N = fn:count().
	`)

	counts := func(stage string) []int64 {
		t.Helper()
		if err := k.Evaluate(); err != nil {
			t.Fatalf("Evaluate (%s): %v", stage, err)
		}
		rows, err := k.Query("s23_item_count")
		if err != nil {
			t.Fatalf("Query s23_item_count (%s): %v", stage, err)
		}
		got := make([]int64, 0, len(rows))
		for _, r := range rows {
			if len(r.Args) != 1 {
				t.Fatalf("%s: s23_item_count arity = %d, want 1 (%v)", stage, len(r.Args), r)
			}
			n, ok := r.Args[0].(int64)
			if !ok {
				t.Fatalf("%s: s23_item_count arg %#v is %T, want int64", stage, r.Args[0], r.Args[0])
			}
			got = append(got, n)
		}
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		return got
	}

	if err := k.Assert(Fact{Predicate: "s23_item", Args: []any{"/a"}}); err != nil {
		t.Fatalf("Assert /a: %v", err)
	}
	if got := counts("one item"); len(got) != 1 || got[0] != 1 {
		t.Fatalf("one item: s23_item_count = %v, want [1]", got)
	}

	if err := k.Assert(Fact{Predicate: "s23_item", Args: []any{"/b"}}); err != nil {
		t.Fatalf("Assert /b: %v", err)
	}
	if got := counts("two items"); len(got) != 1 || got[0] != 2 {
		t.Fatalf("two items: s23_item_count = %v, want exactly [2] — the previous "+
			"aggregate result was added to rather than replaced", got)
	}
}
