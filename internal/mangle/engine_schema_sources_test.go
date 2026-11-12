package mangle

import (
	"context"
	"testing"
)

// LoadSchemaSources builds one program from several sources with a single
// analysis, where LoadSchemaString in a loop re-analyzes the growing program
// once per fragment.
func TestLoadSchemaSources_AnalyzesOnceAcrossSources(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	err = e.LoadSchemaSources(
		SchemaSource{Name: "base", Content: rollbackBaseline},
		SchemaSource{Name: "later", Content: rollbackLater},
	)
	if err != nil {
		t.Fatalf("LoadSchemaSources: %v", err)
	}
	if got := e.WorkCounts().ProgramBuilds; got != 1 {
		t.Fatalf("ProgramBuilds = %d, want 1 for one batched load", got)
	}
	if err := e.AddFact("item", "a"); err != nil {
		t.Fatal(err)
	}
	res, err := e.Query(context.Background(), "later(X)")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("later(X) = %v, want one row derived across sources", res.Bindings)
	}
}

// A batch is admitted whole or not at all: a source that fails to parse, or a
// combined program that fails analysis, leaves the accepted program unchanged.
func TestLoadSchemaSources_RejectsWholeBatch(t *testing.T) {
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadSchemaString(rollbackBaseline); err != nil {
		t.Fatal(err)
	}
	before := snapshotEngine(e)

	if err := e.LoadSchemaSources(
		SchemaSource{Name: "later", Content: rollbackLater},
		SchemaSource{Name: "broken", Content: "Decl broken(X"},
	); err == nil {
		t.Fatal("parse error in one source was accepted")
	}
	if err := e.LoadSchemaSources(
		SchemaSource{Name: "later", Content: rollbackLater},
		SchemaSource{Name: "paradox", Content: rollbackParadox},
	); err == nil {
		t.Fatal("unstratifiable batch was accepted")
	}
	if after := snapshotEngine(e); after != before {
		t.Fatalf("rejected batch changed the program: before %+v after %+v", before, after)
	}
	if _, err := e.Query(context.Background(), "later(X)"); err == nil {
		t.Fatal("later/1 from a rejected batch is still queryable")
	}
}

// Every fixpoint run is counted, so a caller that asserts fact by fact under
// AutoEval is visible.
func TestWorkCounts_CountsEvaluationsPerAddFactsCall(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	e, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadSchemaString(rollbackBaseline); err != nil {
		t.Fatal(err)
	}
	start := e.WorkCounts().Evaluations
	if err := e.AddFacts([]Fact{{Predicate: "item", Args: []any{"a"}}, {Predicate: "item", Args: []any{"b"}}}); err != nil {
		t.Fatal(err)
	}
	if got := e.WorkCounts().Evaluations - start; got != 1 {
		t.Fatalf("one AddFacts batch ran %d evaluations, want 1", got)
	}
	if err := e.AddFact("item", "c"); err != nil {
		t.Fatal(err)
	}
	if err := e.AddFact("item", "d"); err != nil {
		t.Fatal(err)
	}
	if got := e.WorkCounts().Evaluations - start; got != 3 {
		t.Fatalf("evaluations after two more single adds = %d, want 3", got)
	}
}
