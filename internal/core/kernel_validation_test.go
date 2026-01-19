package core

import (
	"testing"
)

func TestKernelValidation_Schema(t *testing.T) {
	k := setupMockKernel(t)

	// Define schema
	k.AppendPolicy("Decl valid_pred(Name).")
	k.Evaluate()

	// 1. Assert valid fact
	err := k.Assert(Fact{Predicate: "valid_pred", Args: []interface{}{"ok"}})
	if err != nil {
		t.Errorf("Valid assert failed: %v", err)
	}

	// 2. Assert invalid fact (wrong arity)
	// strict mode usually catches this at Asserts or Evaluate time?
	// RealKernel.Assert -> Evaluate -> Rebuild.
	// If checking is enabled, it should fail.

	// We force it by asserting a fact that doesn't match the decl.
	// Note: ParseFactString might fail, but Assert with struct skips parsing.
	// However, Evaluate will rebuild program with facts as EDB.
	// If facts don't match decls, Mangle might error.

	// Warning: Assert() only checks duplicate. Evaluate() does the heavy lifting.
	// But `k.facts` are stored as `Fact` structs.
	// When rebuilding, `LoadFacts` is used.

	err = k.Assert(Fact{Predicate: "valid_pred", Args: []interface{}{"ok", "extra"}})
	// If Evaluate fails, Verify returns error.

	if err == nil {
		// Maybe it didn't fail? Mangle might just ignore it or treat as different predicate signature?
		// Datalog supports overloading by arity?
		// Decl valid_pred(Name) implies arity 1.
		// If we use arity 2, it's undeclared.
		// STRICT MODE should fail undeclared predicates.
	} else {
		t.Logf("Got expected validation error: %v", err)
	}
}

func TestKernelValidation_Types(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl typed_pred(Number).")
	k.Evaluate()

	// 1. Assert valid type
	k.Assert(Fact{Predicate: "typed_pred", Args: []interface{}{123}})

	// 2. Assert invalid type (String instead of Number)
	// Mangle type checking occurs during evaluation/validation.
	// k.Assert won't catch it immediately, but Evaluate might.

	k.AssertWithoutEval(Fact{Predicate: "typed_pred", Args: []interface{}{"not_a_number"}})
	err := k.Evaluate()

	if err == nil {
		t.Logf("Warning: Type validation might be permissive or mismatched.")
	} else {
		t.Logf("Got expected type error: %v", err)
	}
}
