package mangle

import (
	"strings"
	"testing"
)

// A schema fragment that analyzes but fails stratification must leave the
// accepted program exactly as it was. rebuildProgramLocked used to publish the
// analyzed ProgramInfo before it stratified, so a rejected fragment left the
// new program next to the old strata, predicate index, query context and rule
// graph; evaluation and rederivation then read two different programs.
const (
	rollbackBaseline = `
Decl item(X) bound [/string].
Decl seen(X) bound [/string].
seen(X) :- item(X).
`
	// Positively bound, but the rule negates its own head: analysis accepts it,
	// stratification rejects it.
	rollbackParadox = `
Decl start(S) bound [/name].
Decl paradox(S) bound [/name].
paradox(X) :- start(X), !paradox(X).
`
	rollbackLater = `
Decl later(X) bound [/string].
later(X) :- seen(X).
`
)

type engineSnapshot struct {
	loaded   bool
	strata   int
	toStrat  int
	index    int
	idb      int
	hasQuery bool
}

func snapshotEngine(e *Engine) engineSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := engineSnapshot{
		loaded:   e.programInfo != nil,
		strata:   len(e.strata),
		toStrat:  len(e.predToStratum),
		index:    len(e.predicateIndex),
		hasQuery: e.queryContext != nil,
	}
	if e.programInfo != nil {
		s.idb = len(e.programInfo.IdbPredicates)
	}
	return s
}

func rollbackEngine(t *testing.T) *Engine {
	t.Helper()
	cfg := DefaultConfig()
	cfg.AutoEval = true
	e, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadSchemaString(rollbackBaseline); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestLoadSchemaString_StratificationRejectionKeepsTheAcceptedProgram(t *testing.T) {
	e := rollbackEngine(t)
	if err := e.AddFact("item", "a"); err != nil {
		t.Fatal(err)
	}
	before := snapshotEngine(e)

	err := e.LoadSchemaString(rollbackParadox)
	if err == nil || !strings.Contains(err.Error(), "stratification") {
		t.Fatalf("the paradox fragment must be rejected by stratification, got %v", err)
	}
	if got := snapshotEngine(e); got != before {
		t.Fatalf("a rejected fragment changed the accepted program:\n before %+v\n after  %+v", before, got)
	}
	// Repeated rejection changes nothing either.
	if err := e.LoadSchemaString(rollbackParadox); err == nil {
		t.Fatal("the paradox fragment is rejected every time")
	}
	if got := snapshotEngine(e); got != before {
		t.Fatalf("a second rejection changed the accepted program:\n before %+v\n after  %+v", before, got)
	}

	// The rejected predicates are not part of the program.
	if err := e.AddFact("start", "/x"); err == nil {
		t.Error("a predicate declared only by the rejected fragment must stay undeclared")
	}

	// Evaluation, new facts, Clear and rehydration all behave like an engine
	// that never saw the fragment.
	control := rollbackEngine(t)
	if err := control.AddFact("item", "a"); err != nil {
		t.Fatal(err)
	}
	for _, eng := range []*Engine{e, control} {
		if err := eng.AddFact("item", "b"); err != nil {
			t.Fatal(err)
		}
		if err := eng.Evaluate(); err != nil {
			t.Fatalf("Evaluate after a rejected load: %v", err)
		}
	}
	if got, want := len(e.QueryFacts("seen")), len(control.QueryFacts("seen")); got != want || got != 2 {
		t.Errorf("seen rows after rejection = %d, control engine has %d, want 2", got, want)
	}
	for _, eng := range []*Engine{e, control} {
		eng.Clear()
		if err := eng.AddFact("item", "c"); err != nil {
			t.Fatal(err)
		}
	}
	got, want := e.QueryFacts("seen"), control.QueryFacts("seen")
	if len(got) != 1 || len(got) != len(want) {
		t.Errorf("after Clear and rehydration seen = %v, control has %v", got, want)
	}

	// A later valid fragment still loads and derives through the old program.
	if err := e.LoadSchemaString(rollbackLater); err != nil {
		t.Fatalf("a valid fragment after a rejection must load: %v", err)
	}
	if err := e.Evaluate(); err != nil {
		t.Fatal(err)
	}
	if rows := e.QueryFacts("later"); len(rows) != 1 {
		t.Errorf("later should derive from the accepted program, got %v", rows)
	}
}

func TestLoadSchemaString_FirstLoadRejectionLeavesNoProgram(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true
	e, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadSchemaString(rollbackParadox); err == nil {
		t.Fatal("the paradox fragment must be rejected as a first load")
	}
	if got := snapshotEngine(e); got.loaded || got.hasQuery {
		t.Fatalf("a rejected first load must not publish a program: %+v", got)
	}
	if err := e.LoadSchemaString(rollbackBaseline); err != nil {
		t.Fatalf("a valid schema after a rejected first load must load: %v", err)
	}
}
