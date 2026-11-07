package core

import (
	"math/rand"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
)

// The trigger index names exactly the variants a walk over every rule and
// premise of the stratum names, for any set of changed predicates.
func TestDRedTriggerIndexMatchesAWalkOfTheRules(t *testing.T) {
	k := newPolicyKernel(t, newRealKernelBase(), deltaPolicy)
	if err := k.Assert(dlEdge("/a", "/b")); err != nil {
		t.Fatal(err)
	}
	idx := k.cone.dredIndex(k.programInfo)
	members := make(map[int]struct{})
	var syms []ast.PredicateSym
	seen := make(map[ast.PredicateSym]bool)
	for ri, rule := range idx.rules {
		if rule.agg == nil {
			members[ri] = struct{}{}
		}
		for _, p := range rule.clause.Premises {
			var sym ast.PredicateSym
			switch p := p.(type) {
			case ast.Atom:
				sym = p.Predicate
			case ast.NegAtom:
				sym = p.Atom.Predicate
			default:
				continue
			}
			if !seen[sym] {
				seen[sym] = true
				syms = append(syms, sym)
			}
		}
	}
	if len(members) < 10 || len(syms) < 10 {
		t.Fatalf("the program has %d variant rules over %d predicates; the check would be vacuous", len(members), len(syms))
	}
	type ref struct {
		rule, pi int
		negated  bool
	}
	rng := rand.New(rand.NewSource(11))
	for round := 0; round < 200; round++ {
		pos, neg := make(deltaStores), make(deltaStores)
		for _, sym := range syms {
			row := ast.Atom{Predicate: sym, Args: make([]ast.BaseTerm, sym.Arity)}
			for i := range row.Args {
				row.Args[i] = ast.Number(int64(i))
			}
			if rng.Intn(3) == 0 {
				pos.add(row)
			}
			if rng.Intn(3) == 0 {
				neg.add(row)
			}
		}
		inStratum := func(sym ast.PredicateSym) bool { return sym.Symbol < "dl_f" }
		want := make(map[ref]bool)
		for ri := range members {
			rule := idx.rules[ri]
			for _, pi := range rule.positives {
				p := rule.clause.Premises[pi].(ast.Atom).Predicate
				if pos[p] != nil && !inStratum(p) {
					want[ref{ri, pi, false}] = true
				}
			}
			for _, ni := range rule.negatives {
				if neg[rule.clause.Premises[ni].(ast.NegAtom).Atom.Predicate] != nil {
					want[ref{ri, ni, true}] = true
				}
			}
		}
		r := &dredRun{idx: idx}
		got := make(map[ref]bool)
		for _, v := range r.seedVariants(members, engine.QueryContext{}, pos, neg, inStratum) {
			got[ref{v.rule, v.pi, v.negated}] = true
		}
		if len(got) != len(want) {
			t.Fatalf("round %d: index lists %d variants, a walk %d", round, len(got), len(want))
		}
		for w := range want {
			if !got[w] {
				t.Fatalf("round %d: the index misses variant %+v", round, w)
			}
		}
	}
}
