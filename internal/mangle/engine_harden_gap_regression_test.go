package mangle

import (
	"context"
	"strings"
	"testing"
)

// Regression tests for the Mangle kernel hardening pass (fail closed with
// honest errors, no panics on edge-case inputs). Each test locks in one
// error-path gap: operations without a loaded schema must error, bad schemas
// must not poison later rebuilds, cancelled contexts must abort, and empty
// batches must be safe no-ops.

func TestHardenGap_AddFactsRequiresSchema(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if err := e.AddFact("test_pred", "a"); err == nil {
		t.Fatal("AddFact without schema should fail closed, got nil error")
	}
	if err := e.AddFacts([]Fact{{Predicate: "test_pred", Args: []any{"a"}}}); err == nil {
		t.Fatal("AddFacts without schema should fail closed, got nil error")
	}
}

func TestHardenGap_EvaluateRequiresSchema(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if err := e.Evaluate(); err == nil {
		t.Fatal("Evaluate without schema should fail closed, got nil error")
	}
	if err := e.RecomputeRules(); err == nil {
		t.Fatal("RecomputeRules without schema should fail closed, got nil error")
	}
}

func TestHardenGap_ReplaceRequiresSchema(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if err := e.ReplaceFactsForFile("some/file.go", []Fact{{Predicate: "p", Args: []any{"x"}}}); err == nil {
		t.Fatal("ReplaceFactsForFile without schema should fail closed, got nil error")
	}
	if err := e.ReplaceFactsForFileWithHash("some/file.go", nil, "abc"); err == nil {
		t.Fatal("ReplaceFactsForFileWithHash without schema should fail closed, got nil error")
	}
}

func TestHardenGap_BadSchemaFailClosed(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	bad := "this is ::: not valid mangle ((("
	if err := e.LoadSchemaString(bad); err == nil {
		t.Fatal("LoadSchemaString with invalid schema should error, got nil")
	}
	// Engine must still fail closed (not panic, not silently accept) after a
	// rejected schema fragment.
	if err := e.AddFact("p", "x"); err == nil {
		t.Fatal("AddFact after rejected schema should still fail closed without a valid schema")
	}
	if err := e.Evaluate(); err == nil {
		t.Fatal("Evaluate after rejected schema should still fail closed")
	}
	// A second bad load must also error rather than panic or corrupt state.
	if err := e.LoadSchemaString(bad); err == nil {
		t.Fatal("second LoadSchemaString with invalid schema should error, got nil")
	}
}

func TestHardenGap_CancelledContextAborts(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = e.AddFactsContext(ctx, []Fact{{Predicate: "p", Args: []any{"x"}}})
	if err == nil {
		t.Fatal("AddFactsContext with cancelled context should return error, got nil")
	}
	if !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "cancelled") && err != context.Canceled {
		t.Logf("cancelled-context error is honest but unexpected text: %v", err)
	}
}

func TestHardenGap_EmptyBatchIsNoOp(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	// Empty batches must not error and must not require a schema; they are
	// safe no-ops even on a fresh engine.
	if err := e.AddFacts(nil); err != nil {
		t.Fatalf("AddFacts(nil) should be a no-op, got: %v", err)
	}
	if err := e.AddFacts([]Fact{}); err != nil {
		t.Fatalf("AddFacts(empty) should be a no-op, got: %v", err)
	}
	if err := e.AddFactsContext(context.Background(), nil); err != nil {
		t.Fatalf("AddFactsContext(nil) should be a no-op, got: %v", err)
	}
}

func TestHardenGap_ReplaceEdgeCasesDoNotPanic(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	// Empty file path and nil facts must not panic; without a schema the
	// honest result is an error.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ReplaceFactsForFile(\"\", nil) panicked: %v", r)
			}
		}()
		_ = e.ReplaceFactsForFile("", nil)
	}()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ReplaceFactsForFileWithHash(\"\", nil, \"\") panicked: %v", r)
			}
		}()
		_ = e.ReplaceFactsForFileWithHash("", nil, "")
	}()
}

func TestHardenGap_NilFactArgFailsClosed(t *testing.T) {
	if _, err := convertValueToTypedTerm(nil, -1); err == nil {
		t.Fatal("convertValueToTypedTerm(nil) should fail closed, got nil error")
	} else if !strings.Contains(err.Error(), "nil") {
		t.Fatalf("nil-arg error should mention nil honestly, got: %v", err)
	}
}

func TestHardenGap_ListElementTypeFailsClosed(t *testing.T) {
	if _, err := convertValueToTypedTerm([]any{"ok", 42}, -1); err == nil {
		t.Fatal("convertValueToTypedTerm with non-string list element should fail closed, got nil error")
	} else if !strings.Contains(err.Error(), "unsupported list element") {
		t.Fatalf("list-element error should name the bad element honestly, got: %v", err)
	}
}

func TestHardenGap_UnmarshalableArgFailsClosed(t *testing.T) {
	if _, err := convertValueToTypedTerm(func() {}, -1); err == nil {
		t.Fatal("convertValueToTypedTerm(func) should fail closed, got nil error")
	} else if !strings.Contains(err.Error(), "unsupported fact argument type") {
		t.Fatalf("unmarshalable-arg error should be honest, got: %v", err)
	}
	if _, err := convertValueToTypedTerm(map[string]any{"f": func() {}}, -1); err == nil {
		t.Fatal("convertValueToTypedTerm(map with func) should fail closed, got nil error")
	} else if !strings.Contains(err.Error(), "unsupported fact argument type") {
		t.Fatalf("unmarshalable-map error should be honest, got: %v", err)
	}
}

func TestHardenGap_QueryFailClosedWithoutSchema(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if _, err := e.Query(context.Background(), "test_pred(X)"); err == nil {
		t.Fatal("Query without schema should fail closed, got nil error")
	}
	if _, err := e.Query(context.Background(), ""); err == nil {
		t.Fatal("Query with empty string should fail closed, got nil error")
	}
	if _, err := e.Query(context.Background(), "   "); err == nil {
		t.Fatal("Query with blank string should fail closed, got nil error")
	}
}

func TestHardenGap_GetFactsUnknownPredicateFailsClosed(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if _, err := e.GetFacts("no_such_pred_xyz"); err == nil {
		t.Fatal("GetFacts with unknown predicate should fail closed, got nil error")
	} else if !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("GetFacts error should say not declared honestly, got: %v", err)
	}
}

func TestHardenGap_ReplaceControlFactsFailClosed(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if err := e.ReplaceControlFacts(nil, "some_pred"); err == nil {
		t.Fatal("ReplaceControlFacts without schema should fail closed, got nil error")
	}
	if err := e.ReplaceControlFacts(nil); err == nil {
		t.Fatal("ReplaceControlFacts(nil) without schema should fail closed, got nil error")
	}
}

func TestHardenGap_ParseQueryShapeRejectsEmpty(t *testing.T) {
	for _, q := range []string{"", "   ", ".", "?."} {
		if _, err := parseQueryShape(q); err == nil {
			t.Fatalf("parseQueryShape(%q) should fail closed, got nil error", q)
		}
	}
}

func TestHardenGap_IsIdentifierEdgeCases(t *testing.T) {
	valid := []string{"foo", "_x", "a1_", "abc_def123"}
	for _, s := range valid {
		if !isIdentifier(s) {
			t.Errorf("isIdentifier(%q) = false, want true", s)
		}
	}
	invalid := []string{"", "Foo", "1abc", "has-dash", "has space", "foo.bar"}
	for _, s := range invalid {
		if isIdentifier(s) {
			t.Errorf("isIdentifier(%q) = true, want false", s)
		}
	}
}

func TestHardenGap_ClearResetFailClosed(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Clear() panicked: %v", r)
			}
		}()
		e.Clear()
	}()
	if _, err := e.Query(context.Background(), "p(X)"); err == nil {
		t.Fatal("Query after Clear without schema should fail closed, got nil error")
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Reset() panicked: %v", r)
			}
		}()
		e.Reset()
	}()
	if _, err := e.Query(context.Background(), "p(X)"); err == nil {
		t.Fatal("Query after Reset should fail closed, got nil error")
	}
	if err := e.AddFact("p", "x"); err == nil {
		t.Fatal("AddFact after Reset should fail closed without schema, got nil error")
	}
}

func TestHardenGap_WarmFromPersistenceNoOpWithoutPersistence(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("WarmFromPersistence panicked: %v", r)
			}
		}()
		if err := e.WarmFromPersistence(context.Background()); err != nil {
			t.Fatalf("WarmFromPersistence without persistence should be a no-op, got: %v", err)
		}
	}()
}
