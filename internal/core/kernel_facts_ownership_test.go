package core

import "testing"

// LoadFacts must not write into the caller's Args: the JIT selector shares one
// immutable Args slice per atom across concurrent compiles, and an in-place
// coercion here was a data race between their private kernel scopes.
func TestSanitizeFactForNumericPredicates_LeavesCallerArgsUntouched(t *testing.T) {
	args := []any{"atom/x", MangleAtom("/identity"), MangleAtom("/high"), int64(10), MangleAtom("/true")}
	original := append([]any(nil), args...)
	out := sanitizeFactForNumericPredicates(Fact{Predicate: "prompt_atom", Args: args})
	for i := range original {
		if args[i] != original[i] {
			t.Fatalf("caller arg %d rewritten in place: %v -> %v", i, original[i], args[i])
		}
	}
	if out.Args[2] == args[2] {
		t.Fatalf("priority atom was not coerced on the private copy: %v", out.Args[2])
	}
}
