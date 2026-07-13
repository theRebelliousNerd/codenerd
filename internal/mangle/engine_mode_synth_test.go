package mangle

import (
	"context"
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
	// Bindings may be empty depending on store/mode eval path; the critical
	// contract is: no "no modes declared" error.
	t.Logf("bindings=%d", len(result.Bindings))
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
}
