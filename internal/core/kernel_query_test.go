package core

import (
	"testing"
)

// TODO: TEST_GAP: Null/Undefined/Empty - Verify Query with uninitialized kernel fails safely without panicking.
// TODO: TEST_GAP: Null/Undefined/Empty - Verify QueryAll safely handles k.programInfo == nil without panicking.
// TODO: TEST_GAP: Null/Undefined/Empty - Verify ParseFactString handles empty strings ("") or just a period (".") cleanly.
// TODO: TEST_GAP: Null/Undefined/Empty - Verify LoadFactsFromFile safely ignores empty files without crashing.
// TODO: TEST_GAP: Null/Undefined/Empty - Verify Query fails cleanly when given an empty predicate string.

// TODO: TEST_GAP: Type Coercion - Verify baseTermToValue fallback for unknown Mangle AST primitives.
// TODO: TEST_GAP: Type Coercion - Verify factMatchesPattern differentiates between String ("alice") and Name Atom (/alice).
// TODO: TEST_GAP: Type Coercion - Verify Query preserves numeric precision when converting between float64 and integer.

// TODO: TEST_GAP: User Request Extremes - Verify Query handles a pattern with a massive number of arguments (arity > 1000).
// TODO: TEST_GAP: User Request Extremes - Verify QueryAll can handle huge EDBs (e.g., 1,000,000 facts) without OOM or stall.
// TODO: TEST_GAP: User Request Extremes - Verify LoadFactsFromFile rejects or safely processes 500MB+ `.mg` files.
// TODO: TEST_GAP: User Request Extremes - Verify ParseFactString does not stack overflow on deeply nested logical recursion strings.

// TODO: TEST_GAP: State Conflicts - Verify concurrent reads (Query) don't starve write locks (UpdateSystemFacts).
// TODO: TEST_GAP: State Conflicts - Verify UpdateSystemFacts resolves cleanly when git commands hang (context cancellation).
// TODO: TEST_GAP: State Conflicts - Verify LoadFactsFromFile handles Time-of-Check to Time-of-Use (TOCTOU) file deletion gracefully.

func TestKernelQuery_Parse(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl foo(Name).")
	k.Evaluate()

	// 1. Parse valid query strings (explicitly WITHOUT final dot)
	err := k.AssertString(`foo("bar")`)
	if err != nil {
		t.Errorf("Failed to parse valid fact string: %v", err)
	}

	// 2. Parse invalid query strings
	err = k.AssertString(`foo(,,)`)
	if err == nil {
		t.Error("Expected error for invalid fact string, got nil")
	}
}

func TestKernelQuery_Execute(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy(`
	Decl test_parent(Name, Child).
	Decl test_ancestor(Name, Descendant).
	
	test_ancestor(X, Y) :- test_parent(X, Y).
	test_ancestor(X, Z) :- test_parent(X, Y), test_ancestor(Y, Z).
	`)
	k.Evaluate()

	k.Assert(Fact{Predicate: "test_parent", Args: []interface{}{"alice", "bob"}})
	k.Assert(Fact{Predicate: "test_parent", Args: []interface{}{"bob", "charlie"}})

	// 1. Execute complex query (recursive join)
	err := k.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	results, err := k.Query("test_ancestor")
	if err != nil {
		t.Fatalf("Query test_ancestor failed: %v", err)
	}

	// Expect: alice->bob, bob->charlie, alice->charlie
	if len(results) != 3 {
		t.Errorf("Expected 3 test_ancestor facts, got %d", len(results))
	}
}
