package mangle

import (
	"sort"
	"strings"
	"testing"
)

// Behavioral proofs for the validator uplift: nested arity, sorted output,
// and string-safe rule splitting.

// A Decl whose argument list nests commas must count top-level args only.
func TestValidatorNestedArityCountsTopLevel(t *testing.T) {
	sv := NewSchemaValidator("Decl foo(bar(1,2), X).\nDecl simple(A, B).\n", "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got := sv.GetArity("foo"); got != 2 {
		t.Fatalf("foo arity = %d, want 2 (nested commas overcounted)", got)
	}
	if got := sv.GetArity("simple"); got != 2 {
		t.Fatalf("simple arity = %d, want 2", got)
	}
	// And the head side must agree with the decl side on the same input.
	if err := sv.CheckArity("foo", 2); err != nil {
		t.Fatalf("CheckArity(foo, 2) failed: %v", err)
	}
}

// The availability list is sorted so error output is deterministic and human
// usable even with hundreds of predicates.
func TestValidatorAvailablePredicatesSorted(t *testing.T) {
	sv := NewSchemaValidator("Decl zebra(A).\nDecl apple(A).\nDecl mango(A).\n", "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	got := sv.GetDeclaredPredicates()
	if !sort.StringsAreSorted(got) {
		t.Fatalf("predicates not sorted: %v", got)
	}
}

// ":-" inside a string literal must not split the rule: the head fragment
// before a literal ":-" is not a body.
func TestValidatorRuleWithSeparatorInString(t *testing.T) {
	sv := NewSchemaValidator("Decl outer(X).\nDecl inner(X).\n", "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	// "a:-b" is a string argument, not a rule separator: whole line is a fact.
	if err := sv.ValidateLearnedRule(`outer("a:-b").`); err != nil {
		t.Fatalf("fact with :- in string rejected: %v", err)
	}
	// Real separator still splits: undefined body predicate must be caught.
	err := sv.ValidateLearnedRule(`outer(X) :- nosuchpred(X).`)
	if err == nil || !strings.Contains(err.Error(), "nosuchpred") {
		t.Fatalf("undefined body predicate not caught: %v", err)
	}
}

func TestSplitRuleBody(t *testing.T) {
	cases := []struct {
		in       string
		wantBody string
		wantRule bool
	}{
		{`a(X) :- b(X).`, ` b(X).`, true},
		{`a("x:-y").`, ``, false},
		{`a(X) :- b("p:-q"), c(X).`, ` b("p:-q"), c(X).`, true},
		{`fact.`, ``, false},
		{`a(X) :- b(X) :- c(X).`, ` b(X) :- c(X).`, true}, // first separator wins
	}
	for _, tc := range cases {
		body, isRule := splitRuleBody(tc.in)
		if body != tc.wantBody || isRule != tc.wantRule {
			t.Errorf("splitRuleBody(%q) = (%q, %v), want (%q, %v)",
				tc.in, body, isRule, tc.wantBody, tc.wantRule)
		}
	}
}

func TestCountTopLevelArgs(t *testing.T) {
	cases := map[string]int{
		``:                 0,
		`X`:                1,
		`A, B, C`:          3,
		`bar(1,2), X`:      2,
		`f(g(h(1,2)),3),X`: 2,
	}
	for in, want := range cases {
		if got := countTopLevelArgs(in); got != want {
			t.Errorf("countTopLevelArgs(%q) = %d, want %d", in, got, want)
		}
	}
}
