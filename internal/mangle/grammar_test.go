package mangle

import (
	"testing"
)

func TestNewAtomValidator(t *testing.T) {
	v := NewAtomValidator()

	if v == nil {
		t.Fatal("NewAtomValidator() returned nil")
	}

	if v.ValidPredicates == nil {
		t.Fatal("ValidPredicates map is nil")
	}

	if v.ValidNameConstants == nil {
		t.Fatal("ValidNameConstants map is nil")
	}

	// Verify core predicates are loaded
	expectedPredicates := []string{
		"user_intent",
		"focus_resolution",
	}

	for _, p := range expectedPredicates {
		if _, ok := v.ValidPredicates[p]; !ok {
			t.Errorf("Expected core predicate %q not found in ValidPredicates", p)
		}
	}

	// Verify arity for a known predicate
	if spec, ok := v.ValidPredicates["user_intent"]; ok {
		if spec.Arity != 5 {
			t.Errorf("Expected user_intent arity to be 5, got %d", spec.Arity)
		}
	}

	// Verify core name constants are loaded
	expectedConstants := []string{
		"/query",
		"/mutation",
		"/explain",
		"/go",
		"/error",
		"/function",
	}

	for _, c := range expectedConstants {
		if !v.ValidNameConstants[c] {
			t.Errorf("Expected core name constant %q not found in ValidNameConstants", c)
		}
	}
}
