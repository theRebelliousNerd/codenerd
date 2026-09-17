package mangle

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestEngineQuery_SynthesizesModesWhenMissing ensures Decl without
// descr[mode(...)] is still queryable — root cause of CLI `nerd why`
// "predicate X has no modes declared".
func TestEngineQuery_SynthesizesModesWhenMissing(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Schema Decl with bound types but no mode descriptor (matches production schemas).
	schema := `Decl next_action(ActionType) bound [/name].`
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}

	if err := engine.AddFacts([]Fact{
		{Predicate: "next_action", Args: []any{"/scan"}},
	}); err != nil {
		t.Fatalf("AddFacts: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := engine.Query(ctx, "next_action(Var0)")
	if err != nil {
		t.Fatalf("Query without modes should synthesize default mode, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil QueryResult")
	}
	// The stored fact must be returned: no "no modes declared" error and
	// exactly one row binding Var0 to "/scan".
	if len(result.Bindings) != 1 {
		t.Fatalf("next_action(Var0) returned %d bindings, want 1: %v", len(result.Bindings), result.Bindings)
	}
	if got, want := result.Bindings[0]["Var0"], "/scan"; got != want {
		t.Fatalf("next_action(Var0) returned %v, want [{Var0:%q}]", result.Bindings, want)
	}
}

// TestProofTreeTracer_NoModesDeclaredRegression exercises TraceQuery path.
func TestProofTreeTracer_NoModesDeclaredRegression(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	engine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadSchemaString(`Decl permitted(Action) bound [/name].`); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	_ = engine.AddFacts([]Fact{{Predicate: "permitted", Args: []any{"/read"}}})

	tracer := NewProofTreeTracer(engine)
	tracer.IndexRules()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	trace, err := tracer.TraceQuery(ctx, "permitted(X)")
	if err != nil {
		t.Fatalf("TraceQuery: %v", err)
	}
	if trace == nil {
		t.Fatal("expected non-nil trace")
	}
	if len(trace.RootNodes) != 1 {
		t.Fatalf("expected 1 root node, got %d: %+v", len(trace.RootNodes), trace)
	}
	root := trace.RootNodes[0]
	if dump := fmt.Sprintf("%+v", root); !strings.Contains(dump, "/read") {
		t.Fatalf("root node does not mention /read: %s", dump)
	}
}
