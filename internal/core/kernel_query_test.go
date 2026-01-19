package core

import (
	"testing"
)

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
