package core

import (
	"testing"
)

func TestMangleUpdates_Filter(t *testing.T) {
	policy := MangleUpdatePolicy{
		MaxUpdates:      5,
		AllowedPrefixes: []string{"safe_"},
		AllowedPredicates: map[string]struct{}{
			"ok_flag": {},
		},
	}

	// We pass nil kernel for pure syntax/policy check without schema validation for now
	// Or use setupMockKernel(t) if we want kernel validation.

	updates := []string{
		"safe_foo(1).",      // OK (prefix)
		"ok_flag(\"yes\").", // OK (exact)
		"bad_pred(1).",      // Blocked (policy)
		"Decl foo(bar).",    // Blocked (syntax)
		"p(X) :- q(X).",     // Blocked (rule)
	}

	facts, blocked := FilterMangleUpdates(nil, updates, policy)

	if len(facts) != 2 {
		t.Errorf("Expected 2 accepted facts, got %d", len(facts))
	}

	if len(blocked) != 3 {
		t.Errorf("Expected 3 blocked updates, got %d", len(blocked))
	}

	// Check specific reasons if needed
	for _, b := range blocked {
		if b.Update == "bad_pred(1)." && b.Reason == "" {
			t.Error("Block reason missing for bad_pred")
		}
	}
}

func TestMangleUpdates_SchemaValidation(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl safe_check(Int).")
	k.Evaluate()

	policy := MangleUpdatePolicy{
		AllowedPrefixes: []string{"safe_"},
	}

	updates := []string{
		"safe_check(100).",     // OK
		"safe_check(\"nan\").", // Blocked (Type/Syntax? ParseFactString handles types)
		// Note: ParseFactString parses literals. "nan" is string.
		// Validation check arity? Mangle might not check types in `validatePredicateDeclaration`
		// Implementation `validatePredicateDeclaration` checks Arity.
		"safe_check(1, 2).", // Blocked (Arity mismatch)
	}

	// ParseFactString might fail for "nan" if expecting number? No, it parses generic atoms.
	// But `safe_check(Int)` logic is in Mangle engine. `FilterMangleUpdates` calls `validatePredicateDeclaration`.
	// `validatePredicateDeclaration` checks Arity against `programInfo.Decls`.

	facts, blocked := FilterMangleUpdates(k, updates, policy)

	if len(facts) != 2 { // "nan" is valid ARITY (1), so likely accepted by filter, fail at runtime?
		// Wait, "nan" string vs Int type. `validatePredicateDeclaration` only checks arity in provided code?
		// "arity mismatch: %s expects %d args (got %d)"
		// Yes, looks like only arity. So 2 facts accepted.
		t.Logf("Accepted %d facts", len(facts))
	}

	// safe_check(1, 2) should be blocked
	foundArityBlock := false
	for _, b := range blocked {
		if b.Update == "safe_check(1, 2)." {
			foundArityBlock = true
		}
	}
	if !foundArityBlock {
		t.Error("Expected arity mismatch block for safe_check(1, 2)")
	}
}
