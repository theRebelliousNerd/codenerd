package mangle

import "testing"

func rederiveEngine(t *testing.T, program string) *Engine {
	t.Helper()
	cfg := DefaultConfig()
	cfg.AutoEval = true
	e, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadSchemaString(program); err != nil {
		t.Fatal(err)
	}
	return e
}

// The E9 probe, in miniature: a permission defeated through negation must not
// depend on which fact arrived first. It did: permitted(.env) survived when
// the secret-path measurement was asserted after the pending action.
func TestEngine_ANegatedDefeatDoesNotDependOnArrivalOrder(t *testing.T) {
	const program = `
Decl pending(T) bound [/string].
Decl secret(T) bound [/string].
Decl permitted(T) bound [/string].
permitted(T) :- pending(T), !secret(T).
`
	for _, order := range [][]Fact{
		{{Predicate: "secret", Args: []any{".env"}}, {Predicate: "pending", Args: []any{".env"}}},
		{{Predicate: "pending", Args: []any{".env"}}, {Predicate: "secret", Args: []any{".env"}}},
	} {
		e := rederiveEngine(t, program)
		for _, f := range order {
			if err := e.AddFacts([]Fact{f}); err != nil {
				t.Fatal(err)
			}
		}
		got := e.QueryFacts("permitted")
		if len(got) != 0 {
			t.Errorf("%s then %s: permitted = %v, want nothing", order[0].Predicate, order[1].Predicate, got)
		}
	}
}

// Removing a base fact removes what was derived from it, under positive rules
// too: ReplaceFactsForFile used to leave every conclusion about a file's old
// contents standing.
func TestEngine_ARemovedFactTakesItsConclusionsWithIt(t *testing.T) {
	e := rederiveEngine(t, `
Decl defines(File, Sym) bound [/string, /string].
Decl known(Sym) bound [/string].
known(S) :- defines(_, S).
`)
	if err := e.ReplaceFactsForFile("a.go", []Fact{{Predicate: "defines", Args: []any{"a.go", "Old"}}}); err != nil {
		t.Fatal(err)
	}
	if err := e.ReplaceFactsForFile("a.go", []Fact{{Predicate: "defines", Args: []any{"a.go", "New"}}}); err != nil {
		t.Fatal(err)
	}
	got := e.QueryFacts("known")
	if len(got) != 1 || got[0].Args[0] != "New" {
		t.Fatalf("known = %v, want only New: the conclusion about the old contents survived", got)
	}
}

// A fact asserted into a predicate that rules also derive is base, not
// derived: clearing the derived atoms for a re-derivation leaves it.
func TestEngine_ABaseFactInARuleHeadSurvivesRederivation(t *testing.T) {
	e := rederiveEngine(t, `
Decl flag(X) bound [/string].
Decl gate(X) bound [/string].
Decl blocked(X) bound [/string].
flag(X) :- gate(X), !blocked(X).
`)
	if err := e.AddFacts([]Fact{{Predicate: "flag", Args: []any{"asserted"}}}); err != nil {
		t.Fatal(err)
	}
	if err := e.AddFacts([]Fact{{Predicate: "gate", Args: []any{"g"}}}); err != nil {
		t.Fatal(err)
	}
	if err := e.AddFacts([]Fact{{Predicate: "blocked", Args: []any{"g"}}}); err != nil {
		t.Fatal(err)
	}
	got := e.QueryFacts("flag")
	if len(got) != 1 || got[0].Args[0] != "asserted" {
		t.Fatalf("flag = %v, want the asserted fact and nothing derived from the blocked gate", got)
	}
}
