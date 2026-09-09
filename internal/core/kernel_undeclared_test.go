package core

import (
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// TestAssert_WhenPredicateUndeclared_ShouldStoreButStayUnreadable pins the
// behaviour that made 82 production asserts invisible, so it is documented by a
// test rather than rediscovered.
//
// Assert returns nil and the fact is in the EDB. Query returns nothing, because
// the fixpoint only derives what the program declares. Both halves are
// individually correct; together they mean the caller is told it recorded
// something that no rule can ever read.
func TestAssert_WhenPredicateUndeclared_ShouldStoreButStayUnreadable(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	// Drive one evaluation so programInfo (and its Decls) is populated.
	if _, err := k.Query("critical_file(X)"); err != nil {
		t.Fatalf("warmup query: %v", err)
	}

	const undeclared = "zz_test_undeclared_predicate"
	if err := k.Assert(Fact{Predicate: undeclared, Args: []any{"a"}}); err != nil {
		t.Fatalf("Assert of an undeclared predicate should not error today: %v", err)
	}

	rows, err := k.Query(undeclared + "(X)")
	if err == nil && len(rows) != 0 {
		t.Fatalf("undeclared predicate became queryable (%d rows) — if the kernel now declares "+
			"dynamically, this test and the undeclared-assert budget both need revisiting", len(rows))
	}

	// The warning must have fired, and exactly once. The seen-set is the
	// observable: without it a per-turn predicate would warn on every assert
	// and the signal would be worse than none.
	sym := ast.PredicateSym{Symbol: undeclared, Arity: 1}
	if _, seen := k.undeclared.seen.Load(sym); !seen {
		t.Fatal("asserting an undeclared predicate produced no warning — the one signal a caller gets is missing")
	}

	if err := k.Assert(Fact{Predicate: undeclared, Args: []any{"b"}}); err != nil {
		t.Fatalf("second Assert: %v", err)
	}
	count := 0
	k.undeclared.seen.Range(func(key, _ any) bool {
		if key == sym {
			count++
		}
		return true
	})
	if count != 1 {
		t.Fatalf("predicate recorded %d times in the warn-once set, want 1", count)
	}
}

// TestAssert_WhenPredicateDeclared_ShouldNotWarn keeps the warning honest. A
// diagnostic that fires on correct code is noise, and noise is how a real
// signal gets filtered out.
func TestAssert_WhenPredicateDeclared_ShouldNotWarn(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if _, err := k.Query("critical_file(X)"); err != nil {
		t.Fatalf("warmup query: %v", err)
	}

	if err := k.Assert(Fact{Predicate: "critical_file", Args: []any{"undeclared_test.go"}}); err != nil {
		t.Fatalf("Assert: %v", err)
	}

	k.undeclared.seen.Range(func(key, _ any) bool {
		t.Errorf("declared predicate was reported as undeclared: %v", key)
		return true
	})
}

// TestAssert_WhenArityUndeclared_ShouldWarn covers the case that reads as the
// most confusing bug of the three: the predicate exists, the name is right, and
// the fact is still unreadable because the Decl bounds a different arity.
func TestAssert_WhenArityUndeclared_ShouldWarn(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if _, err := k.Query("critical_file(X)"); err != nil {
		t.Fatalf("warmup query: %v", err)
	}

	k.mu.RLock()
	_, declaredAtOne := k.programInfo.Decls[ast.PredicateSym{Symbol: "critical_file", Arity: 1}]
	_, declaredAtFour := k.programInfo.Decls[ast.PredicateSym{Symbol: "critical_file", Arity: 4}]
	k.mu.RUnlock()
	if !declaredAtOne || declaredAtFour {
		t.Skip("critical_file's declared arities changed; pick another single-arity predicate")
	}

	if err := k.Assert(Fact{Predicate: "critical_file", Args: []any{"a", "b", "c", "d"}}); err != nil {
		// A rejection here is also an acceptable outcome — it is the loud one.
		return
	}
	sym := ast.PredicateSym{Symbol: "critical_file", Arity: 4}
	if _, seen := k.undeclared.seen.Load(sym); !seen {
		t.Fatal("a fact asserted at an undeclared arity produced no warning")
	}
}
