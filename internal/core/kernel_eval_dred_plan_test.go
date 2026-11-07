package core

import (
	"slices"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codenerd/internal/mangle"
)

func planClause(t *testing.T, src string) ast.Clause {
	t.Helper()
	unit, err := mangle.ParseUnit(strings.NewReader(src))
	if err != nil || len(unit.Clauses) != 1 {
		t.Fatalf("parse %q: %v (%d clauses)", src, err, len(unit.Clauses))
	}
	return unit.Clauses[0]
}

func noDeferred(ast.PredicateSym) bool { return false }

// The variant over new mandatory_atom rows binds M; the conflict premise M
// narrows goes next, and the candidate scan runs last, bound.
func TestPlanOrder_JoinsWhatTheDeltaBoundFirst(t *testing.T) {
	c := planClause(t, `conflict_loser(A) :- candidate_atom(A), atom_conflicts(A, M), mandatory_atom(M).`)
	if got := planOrder(c, 2, nil, noDeferred); !slices.Equal(got, []int{2, 1, 0}) {
		t.Fatalf("order = %v, want [2 1 0]", got)
	}
}

// Filters run as soon as their variables are bound; an equality runs as soon
// as it can bind its lone-variable side, so the atom after it is bound.
func TestPlanOrder_PlacesFiltersAndBindingEqualitiesEarly(t *testing.T) {
	c := planClause(t, `h(Z) :- p(X, Y), q(Z), Y > 3, Z = fn:plus(X, 1), !r(X).`)
	got := planOrder(c, -1, nil, noDeferred)
	if !slices.Equal(got, []int{0, 2, 3, 4, 1}) {
		t.Fatalf("order = %v, want [0 2 3 4 1]: p binds X,Y; Y>3, Z=..., !r(X) then; q(Z) bound last", got)
	}
}

// A bound head narrows the body for re-derivation.
func TestPlanOrder_StartsFromABoundHead(t *testing.T) {
	c := planClause(t, `h(A, B) :- big(X), link(A, X), link(X, B).`)
	got := planOrder(c, -1, map[ast.Variable]bool{{Symbol: "A"}: true, {Symbol: "B"}: true}, noDeferred)
	if got[0] != 1 && got[0] != 2 {
		t.Fatalf("order = %v: a link premise with a bound head variable must lead, not big(X)", got)
	}
}

// A body with a deferred premise keeps source order: its inputs follow modes.
func TestPlanOrder_DeferredBodyKeepsSourceOrder(t *testing.T) {
	c := planClause(t, `h(A) :- a(A), d(A, B), b(B).`)
	deferred := func(s ast.PredicateSym) bool { return s.Symbol == "d" }
	if got := planOrder(c, 2, nil, deferred); !slices.Equal(got, []int{2, 0, 1}) {
		t.Fatalf("order = %v, want the delta then source order [2 0 1]", got)
	}
}
