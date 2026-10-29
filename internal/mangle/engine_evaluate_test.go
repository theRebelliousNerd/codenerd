package mangle

import "testing"

// A fact written in the program is a clause until an evaluation runs; before
// that neither the fact store nor a query can see it. Evaluate is the
// exported way to materialise it (and everything else the program derives).
func TestEvaluateMaterialisesProgramFacts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	eng, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	if err := eng.LoadSchemaString("Decl span(N) bound [/number].\nDecl doubled(N) bound [/number].\nspan(8).\ndoubled(N) :- span(N).\n"); err != nil {
		t.Fatal(err)
	}
	if got := eng.QueryFacts("span"); len(got) != 0 {
		t.Fatalf("a program fact is in the store before any evaluation: %v", got)
	}
	if err := eng.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := eng.QueryFacts("span"); len(got) != 1 {
		t.Fatalf("after Evaluate the program fact must be in the store, got %v", got)
	}
	rows, err := eng.Query(t.Context(), "doubled(N)")
	if err != nil || len(rows.Bindings) != 1 {
		t.Fatalf("derived predicate after Evaluate: rows=%v err=%v", rows, err)
	}
}

func TestEvaluateWithoutASchemaIsAnError(t *testing.T) {
	eng, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	if err := eng.Evaluate(); err == nil {
		t.Fatal("Evaluate on an engine with no program must fail, not silently do nothing")
	}
}
