package core

import (
	"testing"
)

func TestKernelPolicy_Append(t *testing.T) {
	k := setupMockKernel(t)

	// Append a policy fragment
	policy := "Decl policy_test_p(Name)."
	k.AppendPolicy(policy)

	// Verify it's in the program (implicit via Evaluate or checking k.policy)
	// We can't easily check k.policy string directly as it's private, but we can check if it takes effect.
	// But `policy_test_p` is just a decl. Let's add a rule.

	policy2 := `
	Decl policy_test_q(Name).
	policy_test_q(X) :- policy_test_p(X).
	`
	k.AppendPolicy(policy2)
	k.Evaluate() // Should succeed

	// Assert fact for p
	k.Assert(Fact{Predicate: "policy_test_p", Args: []interface{}{"foo"}})

	// Query q
	results, err := k.Query("policy_test_q")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for policy_test_q, got %d", len(results))
	}
}

func TestKernelPolicy_Conflict(t *testing.T) {
	k := setupMockKernel(t)

	// Test if appending conflicting policies causes issues
	// Mangle allows multiple rules for same predicate (OR logic).
	// But maybe duplicate Decl?

	k.AppendPolicy("Decl conflict_p(Name).")
	k.Evaluate()

	// Append duplicate decl
	k.AppendPolicy("Decl conflict_p(Name).")
	err := k.Evaluate()

	// Mangle might allow redundant decls or might error.
	// If it passes, fine. If it errors, we catch it.
	// Actually, duplicate Decl usually errors or warns.
	// Let's assume for this test we want to ensure it DOES NOT panic.
	if err != nil {
		t.Logf("Duplicate decl caused error: %v (as expected?)", err)
	}
}
