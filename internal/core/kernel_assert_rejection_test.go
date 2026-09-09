package core

import (
	"strings"
	"testing"
)

// Assert returned nil for a fact the kernel had thrown away.
//
// addFactIfNewLocked answered a single bool for two opposite outcomes —
// "already present" and "rejected, and never going to be present" — and Assert
// read that bool as "duplicate". The most common rejection is a fractional
// float in a slot the Decl bounds /number, which coerceAtomToDeclLocked has to
// refuse because this Mangle fork's comparison builtins are int64-only. So the
// fact silently was not there and the caller was told it was.

func TestAssert_RejectedFactReturnsError(t *testing.T) {
	k := setupMockKernel(t)

	// schemas_dreamer.mg: Decl dream_preference(Content, Confidence) bound [/string, /number].
	err := k.Assert(Fact{Predicate: "dream_preference", Args: []any{"prefers table-driven tests", 0.85}})
	if err == nil {
		t.Fatal("Assert reported success for a fractional float in a /number slot")
	}
	if !strings.Contains(err.Error(), "dream_preference") {
		t.Errorf("error should name the predicate that was rejected, got: %v", err)
	}

	facts, qerr := k.Query("dream_preference")
	if qerr != nil {
		t.Fatalf("query: %v", qerr)
	}
	if len(facts) != 0 {
		t.Fatalf("the rejected fact is in the store after all: %v", facts)
	}
}

func TestAssert_DuplicateIsStillANoOpNotAnError(t *testing.T) {
	k := setupMockKernel(t)

	fact := Fact{Predicate: "dream_preference", Args: []any{"prefers table-driven tests", int64(85)}}
	if err := k.Assert(fact); err != nil {
		t.Fatalf("first assert: %v", err)
	}
	if err := k.Assert(fact); err != nil {
		t.Fatalf("re-asserting an existing fact must stay a no-op, got: %v", err)
	}
}

func TestAssertBatch_ReportsRejectedFactsAndKeepsTheRest(t *testing.T) {
	k := setupMockKernel(t)

	err := k.AssertBatch([]Fact{
		{Predicate: "dream_preference", Args: []any{"good", int64(90)}},
		{Predicate: "dream_preference", Args: []any{"bad", 0.42}},
	})
	if err == nil {
		t.Fatal("AssertBatch reported success while dropping a fact")
	}
	if !strings.Contains(err.Error(), "rejected") {
		t.Errorf("batch error should say a fact was rejected, got: %v", err)
	}

	facts, qerr := k.Query("dream_preference")
	if qerr != nil {
		t.Fatalf("query: %v", qerr)
	}
	if len(facts) != 1 {
		t.Fatalf("expected the one good fact to land, got %v", facts)
	}
}
