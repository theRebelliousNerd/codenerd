package mangle

import (
	"context"
	"testing"
	"time"
)

// A merge lattice replaces a row from inside the store's own scan. Over a
// read-locked store wrapper that is a self-deadlock; the engine evaluates over
// its plain store under its own exclusive lock instead.
const twoNodeLattice = `
Decl edge(A, B) bound [/name, /name].
Decl tie(P, O) bound [/name, /number].
Decl label(P, O)
    descr [fundep([P], [O]), merge([O], "label_min")]
    bound [/name, /number].
Decl label_min(A, B, M) descr [mode('+', '+', '-'), deferred()] bound [/number, /number, /number].
label_min(A, B, A) :- A <= B.
label_min(A, B, B) :- B < A.
edge(/a, /b). edge(/b, /a).
tie(/a, 1). tie(/b, 2).
label(A, O) :- edge(A, _), tie(A, O).
label(A, O) :- label(B, O), edge(A, B).
Decl rooted(P, R) bound [/name, /name].
rooted(P, R) :- label(P, O), tie(R, O).
`

func TestEngine_LatticeEvaluates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = false
	eng, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.LoadSchemaString(twoNodeLattice); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- eng.Evaluate() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Evaluate did not return: the lattice deadlocked on the store")
	}
	rows, err := eng.GetFacts("label")
	if err != nil {
		t.Fatal(err)
	}
	// Both nodes hold the component minimum, one row each: the lattice
	// replaced /b's own 2 with /a's 1 rather than keeping both.
	if len(rows) != 2 {
		t.Fatalf("label = %v, want one row per node", rows)
	}
	for _, r := range rows {
		if n, ok := r.Args[1].(int64); !ok || n != 1 {
			t.Fatalf("label = %v, want every node labelled 1", rows)
		}
	}
	res, err := eng.Query(context.Background(), "label(P, O)")
	if err != nil || len(res.Bindings) != 2 {
		t.Fatalf("Query label = %v, %v", res, err)
	}

	// An added edge to a smaller node replaces every label. What was derived
	// from the replaced rows goes with them: the lattice is non-monotone for
	// re-derivation, like negation.
	for _, f := range []Fact{
		{Predicate: "edge", Args: []any{"/b", "/c"}},
		{Predicate: "edge", Args: []any{"/c", "/b"}},
		{Predicate: "tie", Args: []any{"/c", int64(0)}},
	} {
		if err := eng.AddFact(f.Predicate, f.Args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := eng.Evaluate(); err != nil {
		t.Fatal(err)
	}
	rooted, err := eng.GetFacts("rooted")
	if err != nil {
		t.Fatal(err)
	}
	if len(rooted) != 3 {
		t.Fatalf("rooted = %v, want each of /a /b /c rooted at /c only", rooted)
	}
	for _, r := range rooted {
		if r.Args[1] != "/c" {
			t.Fatalf("rooted = %v: a conclusion from a replaced label survived", rooted)
		}
	}
}
