package core

import (
	"errors"
	"strings"
	"testing"
)

// The reporting is installed as a deferred rewrite of a named return, which is
// the only way to cover every exit from a function that has several. It is also
// a shape with exactly two ways to be wrong, and both are silent: swallow a real
// error, or invent one on the success path. A caller writing
// `_ = kernel.Assert(...)` -- and there are around a hundred and sixty of those
// -- would notice neither.
func TestNoteMutationFailurePreservesTheCaller_sError(t *testing.T) {
	k := &RealKernel{}

	if got := k.noteMutationFailure("assert", "some_predicate", nil); got != nil {
		t.Errorf("nil error became %v; the success path must stay clean", got)
	}

	sentinel := errors.New("the fact could not be encoded")
	got := k.noteMutationFailure("assert", "some_predicate", sentinel)
	if !errors.Is(got, sentinel) {
		t.Errorf("got %v, want the caller's own error back; wrapping or replacing it here "+
			"breaks every errors.Is check upstream", got)
	}
}

// A batch names what it was carrying, because "assert batch FAILED" without the
// predicates tells an operator nothing they can act on. Bounded, because a
// failed ten-thousand-fact load must report what it was rather than filling the
// log with it.
func TestBatchPredicatesNamesAndBounds(t *testing.T) {
	if got := batchPredicates(nil); got != "(empty batch)" {
		t.Errorf("batchPredicates(nil) = %q, want \"(empty batch)\"", got)
	}

	// Duplicates collapse: a batch is usually many facts of few predicates, and
	// repeating one name a thousand times is the same failure as not bounding.
	dup := []Fact{{Predicate: "beta"}, {Predicate: "alpha"}, {Predicate: "beta"}}
	if got := batchPredicates(dup); got != "alpha,beta" {
		t.Errorf("batchPredicates(dup) = %q, want \"alpha,beta\"", got)
	}

	var many []Fact
	for _, n := range []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"} {
		many = append(many, Fact{Predicate: n})
	}
	got := batchPredicates(many)
	if !strings.Contains(got, "...") {
		t.Errorf("batchPredicates(10 predicates) = %q, want it truncated with an ellipsis", got)
	}
	if strings.Count(got, ",") > 8 {
		t.Errorf("batchPredicates(10 predicates) = %q, want at most 8 names plus the ellipsis", got)
	}
}
