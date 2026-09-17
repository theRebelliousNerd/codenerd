package core

import (
	"codeberg.org/TauCeti/mangle-go/ast"
	"strings"
	"testing"
)

// REMEDIATED: All 8 TEST_GAP items — see kernel_validation_gaps_test.go:
//   TestKernelValidationGap_ValidateLearnedRules_NilEmpty (Null/Empty)
//   TestKernelValidationGap_ValidateLearnedRule_SchemaValidatorNil (Null/Empty)
//   TestKernelValidationGap_CheckSyntax_LongLine (Type Coercion)
//   TestKernelValidationGap_CheckSyntax_InvalidUTF8 (Type Coercion)
//   TestKernelValidationGap_InfiniteLoopRisk_WhitespaceBypass (User Extremes)
//   TestKernelValidationGap_ValidateLearnedRulesContent_ManyRules (User Extremes)
//   TestKernelValidationGap_Concurrency_SetSchemasWhileValidate (State Conflicts)
//   TestKernelValidationGap_TOCTOU_ValidateLearnedRulesContent (State Conflicts)

func TestKernelValidation_Schema(t *testing.T) {
	k := setupMockKernel(t)

	// Define schema
	k.AppendPolicy("Decl valid_pred(Name).")
	k.Evaluate()

	// 1. Assert valid fact
	err := k.Assert(Fact{Predicate: "valid_pred", Args: []any{"ok"}})
	if err != nil {
		t.Errorf("Valid assert failed: %v", err)
	}

	// 2. Assert fact with a different arity.
	// Arity is part of predicate identity in Mangle: valid_pred/2 is a
	// different predicate from the declared valid_pred/1, so it is simply
	// undeclared (warn path) and must not error.
	err = k.Assert(Fact{Predicate: "valid_pred", Args: []any{"ok", "extra"}})
	if err != nil {
		t.Fatalf("Assert of valid_pred/2 against Decl valid_pred/1 (different arity = different, undeclared predicate) should succeed, got error: %v", err)
	}

	rows, qerr := k.Query("valid_pred")
	if qerr != nil {
		t.Fatalf("Query valid_pred failed: %v", qerr)
	}
	if len(rows) != 1 {
		t.Fatalf("Expected exactly 1 row for valid_pred, got %d: %v", len(rows), rows)
	}
	if rows[0].Predicate != "valid_pred" || len(rows[0].Args) != 1 || rows[0].Args[0] != "ok" {
		t.Fatalf("Unexpected row for valid_pred: %+v", rows[0])
	}
}

func TestKernelValidation_Types(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl typed_pred(Number) bound [/number].")
	k.Evaluate()

	// 1. Assert valid type
	if err := k.Assert(Fact{Predicate: "typed_pred", Args: []any{123}}); err != nil {
		t.Fatalf("Valid assert failed: %v", err)
	}

	// 2. Assert invalid type (String instead of Number).
	// AssertWithoutEval validates against the Decl at assert time
	// (kernel_facts.go validateAgainstDeclLocked) and refuses the
	// wrong-typed fact, so it never enters the store.
	err := k.AssertWithoutEval(Fact{Predicate: "typed_pred", Args: []any{"not_a_number"}})
	if err == nil {
		t.Fatalf("Expected type error asserting string into /number-bound typed_pred, got nil")
	}
	if !strings.Contains(err.Error(), "arg 0") {
		t.Fatalf("Expected error to name argument index, got: %v", err)
	}

	// The refused fact never entered the store, so Evaluate has
	// nothing to reject.
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate should succeed after refused assert, got: %v", err)
	}

	rows, qerr := k.Query("typed_pred")
	if qerr != nil {
		t.Fatalf("Query typed_pred failed: %v", qerr)
	}
	if len(rows) != 1 {
		t.Fatalf("Expected exactly 1 row for typed_pred, got %d: %v", len(rows), rows)
	}
	if v, ok := rows[0].Args[0].(int64); !ok || v != 123 {
		t.Fatalf("Unexpected row for typed_pred: %+v", rows[0])
	}
}

func TestKernelValidation_BoundedPredicateAcceptsDeclaredTypes(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl num_pred(Number) bound [/number].")
	k.AppendPolicy("Decl str_pred(Text) bound [/string].")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if err := k.Assert(Fact{Predicate: "num_pred", Args: []any{42}}); err != nil {
		t.Fatalf("num_pred should accept int, got error: %v", err)
	}
	if err := k.Assert(Fact{Predicate: "num_pred", Args: []any{int64(42)}}); err != nil {
		t.Fatalf("num_pred should accept int64, got error: %v", err)
	}
	if err := k.Assert(Fact{Predicate: "str_pred", Args: []any{"hello"}}); err != nil {
		t.Fatalf("str_pred should accept string, got error: %v", err)
	}

	err := k.Assert(Fact{Predicate: "num_pred", Args: []any{"not_a_number"}})
	if err == nil {
		t.Fatalf("Expected type error asserting string into /number-bound num_pred, got nil")
	}
	if !strings.Contains(err.Error(), "arg 0") {
		t.Fatalf("Expected error to name argument index, got: %v", err)
	}

	err = k.Assert(Fact{Predicate: "str_pred", Args: []any{42}})
	if err == nil {
		t.Fatalf("Expected type error asserting int into /string-bound str_pred, got nil")
	}
	if !strings.Contains(err.Error(), "arg 0") {
		t.Fatalf("Expected error to name argument index, got: %v", err)
	}
}

func TestKernelValidation_NameBoundIsNotCheckedAtAssert(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl name_pred(Name) bound [/name].")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if err := k.Assert(Fact{Predicate: "name_pred", Args: []any{"/atom"}}); err != nil {
		t.Fatalf("name_pred should accept atom-shaped string \"/atom\", got error: %v", err)
	}

	// checkBoundAgainstValue in kernel_undeclared.go deliberately has no
	// /name case: production code passes plain strings into /name slots
	// in many places and Fact.ToAtom decides the encoding, so rejecting
	// them at assert time would refuse working callers.
	if err := k.Assert(Fact{Predicate: "name_pred", Args: []any{"plain"}}); err != nil {
		t.Fatalf("name_pred should accept plain string at assert time (no /name check), got error: %v", err)
	}

	rows, qerr := k.Query("name_pred")
	if qerr != nil {
		t.Fatalf("Query name_pred failed: %v", qerr)
	}
	if len(rows) != 2 {
		t.Fatalf("Expected exactly 2 rows for name_pred, got %d: %v", len(rows), rows)
	}
}

func TestKernelValidation_UndeclaredPredicateAssertsButIsNotQueryable(t *testing.T) {
	k := setupMockKernel(t)
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if err := k.Assert(Fact{Predicate: "undeclared_pred", Args: []any{"hello"}}); err != nil {
		t.Fatalf("Assert of undeclared predicate should succeed (warn path), got error: %v", err)
	}

	rows, qerr := k.Query("undeclared_pred")
	if qerr != nil {
		t.Fatalf("Query undeclared_pred failed: %v", qerr)
	}
	if len(rows) != 0 {
		t.Fatalf("Expected 0 rows for undeclared_pred (undeclared facts are stored but no rule can read them), got %d: %v", len(rows), rows)
	}

	// Warn path recorded it: kernel_undeclared.go warnIfUndeclaredLocked stores
	// the symbol+arity in the undeclaredWarner seen-set (once per predicate).
	if _, ok := k.undeclared.seen.Load(ast.PredicateSym{Symbol: "undeclared_pred", Arity: 1}); !ok {
		t.Fatalf("Expected undeclared_pred/1 to be recorded in undeclared warn set")
	}
}

func TestKernelValidation_BatchRejectsBadFactKeepsRest(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl batch_num(N) bound [/number].")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	err := k.AssertBatch([]Fact{
		{Predicate: "batch_num", Args: []any{1}},
		{Predicate: "batch_num", Args: []any{"nope"}},
		{Predicate: "batch_num", Args: []any{3}},
	})
	if err == nil {
		t.Fatalf("Expected error from AssertBatch with type-violating fact, got nil")
	}
	if !strings.Contains(err.Error(), "rejected 1 of 3") {
		t.Fatalf("Expected error to contain %q, got: %v", "rejected 1 of 3", err)
	}

	rows, qerr := k.Query("batch_num")
	if qerr != nil {
		t.Fatalf("Query batch_num failed: %v", qerr)
	}
	if len(rows) != 2 {
		t.Fatalf("Expected exactly 2 rows for batch_num (good facts land), got %d: %v", len(rows), rows)
	}
}

func TestKernelValidation_BatchAllRejected(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl batch_num(N) bound [/number].")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	err := k.AssertBatch([]Fact{
		{Predicate: "batch_num", Args: []any{"bad1"}},
		{Predicate: "batch_num", Args: []any{"bad2"}},
	})
	if err == nil {
		t.Fatalf("Expected error from AssertBatch with all type-violating facts, got nil")
	}
	if !strings.Contains(err.Error(), "rejected all 2") {
		t.Fatalf("Expected error to contain %q, got: %v", "rejected all 2", err)
	}

	rows, qerr := k.Query("batch_num")
	if qerr != nil {
		t.Fatalf("Query batch_num failed: %v", qerr)
	}
	if len(rows) != 0 {
		t.Fatalf("Expected 0 rows for batch_num after all-rejected batch, got %d: %v", len(rows), rows)
	}
}
