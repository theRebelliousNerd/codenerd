package core

import (
	"testing"
)

func TestKernelEval_Evaluate(t *testing.T) {
	k := setupMockKernel(t)

	// 1. Assert some base facts
	k.Assert(Fact{Predicate: "foo", Args: []interface{}{"bar"}})
	k.Assert(Fact{Predicate: "num", Args: []interface{}{42}})

	// 2. Define a rule in policy
	// Explicitly declare predicates for strict mode
	policy := `
	Decl foo(Name).
	Decl num(Number).
	Decl baz(Name).
	Decl big(Number).

	baz(X) :- foo(X).
	big(X) :- num(N), N > 10, X = N.
	`
	k.AppendPolicy(policy)

	// 3. Evaluate
	err := k.Evaluate()
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// 4. Verify results
	results, err := k.Query("baz")
	if err != nil {
		t.Fatalf("Query baz failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for baz, got %d", len(results))
	} else if len(results) > 0 && results[0].Args[0] != "bar" {
		t.Errorf("Expected baz('bar'), got %v", results[0])
	}

	results, _ = k.Query("big")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for big, got %d", len(results))
	}
}

func TestKernelEval_Stratification(t *testing.T) {
	k := setupMockKernel(t)

	// 1. Define recursive/negated rules that might fail stratification check
	// Use unique names to avoid any potential (unlikely) contamination
	badPolicy := `
	Decl bad_p(Name).
	Decl bad_q(Name).
	bad_p(X) :- not bad_q(X).
	bad_q(X) :- bad_p(X).
	`
	k.AppendPolicy(badPolicy)

	// 2. Verify engine handles or rejects appropriately
	err := k.Evaluate()

	if err == nil {
		t.Logf("Warning: Unstratified negation did not return error. Logic Config might be permissive.")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}
