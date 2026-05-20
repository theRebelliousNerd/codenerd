package core

import (
	"context"
	"testing"
)

func TestTrace_CoverageHeuristics(t *testing.T) {
	k := setupMockKernel(t)

	// Append declarations to mock kernel
	k.AppendPolicy(`
	Decl my_impact(X).
	Decl my_permitted(X).
	Decl my_clarification(X).
	Decl my_action(X).
	Decl my_edb(X).
	`)

	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Assert facts
	k.Assert(Fact{Predicate: "is_edb_predicate", Args: []interface{}{"my_edb"}})
	k.Assert(Fact{Predicate: "rule_metadata", Args: []interface{}{"my_impact", "transitive_impact"}})
	k.Assert(Fact{Predicate: "rule_metadata", Args: []interface{}{"my_permitted", "permission_gate"}})
	k.Assert(Fact{Predicate: "rule_metadata", Args: []interface{}{"my_clarification", "focus_threshold"}})
	k.Assert(Fact{Predicate: "rule_metadata", Args: []interface{}{"my_action", "strategy_selector"}})

	k.Assert(Fact{Predicate: "dependency_link", Args: []interface{}{"caller1", "callee1", "import1"}})
	k.Assert(Fact{Predicate: "safe_action", Args: []interface{}{"action1"}})
	k.Assert(Fact{Predicate: "focus_resolution", Args: []interface{}{"ref1", "path1", "symbol1", int64(95)}})
	k.Assert(Fact{Predicate: "user_intent", Args: []interface{}{"id1", "cat1", "verb1", "target1", "constraint1"}})

	k.Assert(Fact{Predicate: "my_impact", Args: []interface{}{"caller1"}})
	k.Assert(Fact{Predicate: "my_permitted", Args: []interface{}{"action1"}})
	k.Assert(Fact{Predicate: "my_clarification", Args: []interface{}{"ref1"}})
	k.Assert(Fact{Predicate: "my_action", Args: []interface{}{"any"}})
	k.Assert(Fact{Predicate: "my_edb", Args: []interface{}{"val1"}})

	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate facts failed: %v", err)
	}

	ctx := context.Background()

	// 1. Test is_edb_predicate (my_edb)
	trace, err := k.TraceQuery(ctx, "my_edb")
	if err != nil {
		t.Fatalf("TraceQuery failed: %v", err)
	}
	if len(trace.RootNodes) != 1 || string(trace.RootNodes[0].Source) != "EDB" {
		t.Errorf("Expected EDB source, got: %+v", trace.RootNodes[0])
	}

	// 2. Test transitive_impact
	trace, err = k.TraceQuery(ctx, "my_impact")
	if err != nil {
		t.Fatalf("TraceQuery failed: %v", err)
	}
	if len(trace.RootNodes) != 1 || len(trace.RootNodes[0].Children) != 1 || trace.RootNodes[0].Children[0].Fact.Predicate != "dependency_link" {
		t.Errorf("Expected dependency_link child, got: %+v", trace.RootNodes[0])
	}

	// 3. Test permission_gate
	trace, err = k.TraceQuery(ctx, "my_permitted")
	if err != nil {
		t.Fatalf("TraceQuery failed: %v", err)
	}
	if len(trace.RootNodes) != 1 || len(trace.RootNodes[0].Children) != 1 || trace.RootNodes[0].Children[0].Fact.Predicate != "safe_action" {
		t.Errorf("Expected safe_action child, got: %+v", trace.RootNodes[0])
	}

	// 4. Test focus_threshold
	trace, err = k.TraceQuery(ctx, "my_clarification")
	if err != nil {
		t.Fatalf("TraceQuery failed: %v", err)
	}
	if len(trace.RootNodes) != 1 || len(trace.RootNodes[0].Children) != 1 || trace.RootNodes[0].Children[0].Fact.Predicate != "focus_resolution" {
		t.Errorf("Expected focus_resolution child, got: %+v", trace.RootNodes[0])
	}

	// 5. Test strategy_selector
	trace, err = k.TraceQuery(ctx, "my_action")
	if err != nil {
		t.Fatalf("TraceQuery failed: %v", err)
	}
	if len(trace.RootNodes) != 1 || len(trace.RootNodes[0].Children) != 1 || trace.RootNodes[0].Children[0].Fact.Predicate != "user_intent" {
		t.Errorf("Expected user_intent child, got: %+v", trace.RootNodes[0])
	}

	// 6. Test context canceled
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	// Since findPremises checks ctx.Err(), it should return nil and no children
	trace, err = k.TraceQuery(cCtx, "my_impact")
	if err != nil {
		t.Fatalf("TraceQuery failed: %v", err)
	}
	if len(trace.RootNodes) != 1 || len(trace.RootNodes[0].Children) != 0 {
		t.Errorf("Expected no children due to canceled context, got: %d", len(trace.RootNodes[0].Children))
	}

	// 7. Test query error
	_, err = k.TraceQuery(ctx, "")
	if err == nil {
		t.Error("expected error on invalid query syntax")
	}
}
