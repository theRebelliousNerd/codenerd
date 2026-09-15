package mangle

import "testing"

func newHardenEngine(t *testing.T) *Engine {
	t.Helper()
	eng, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	return eng
}

func fragmentCount(eng *Engine) int {
	eng.mu.RLock()
	defer eng.mu.RUnlock()
	return len(eng.schemaFragments)
}

func TestLoadSchemaStringParseFailClosed(t *testing.T) {
	eng := newHardenEngine(t)
	// Baseline: a valid schema first, so Evaluate has something to run and
	// the fragment count measures the failed load, not an empty engine.
	if err := eng.LoadSchemaString("Decl item(X)."); err != nil {
		t.Fatalf("baseline schema failed: %v", err)
	}
	before := fragmentCount(eng)
	if err := eng.LoadSchemaString("Decl ::: not valid (((\n"); err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if after := fragmentCount(eng); after != before {
		t.Fatalf("parse failure kept fragment: %d before, %d after", before, after)
	}
	if err := eng.Evaluate(); err != nil {
		t.Fatalf("Evaluate after parse failure failed: %v", err)
	}
}
