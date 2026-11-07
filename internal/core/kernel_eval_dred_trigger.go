package core

import (
	"sort"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
)

// =============================================================================
// DRED: INVERTED TRIGGER INDEX
// =============================================================================
// A phase's variants are the (rule, premise) pairs whose premise reads a
// predicate that changed. Finding them by walking every rule of a stratum and
// every premise of each costs the program, however small the change. The index
// is compiled once per program, the other way round: a predicate symbol and a
// sign (a positive premise reads removed or inserted rows, a negated one the
// other sign) name the premises that read it. A phase then asks only about the
// predicates that changed, and its cost follows them.

// dredTrigKey names what a premise reads: a predicate, positively or negated.
type dredTrigKey struct {
	sym     ast.PredicateSym
	negated bool
}

// dredTrigger is one premise of one rule: premise pi of idx.rules[rule].
type dredTrigger struct {
	rule, pi int
}

// buildTriggers compiles the trigger index of every rule a stratum evaluates
// by variants. An aggregation recomputes by group, never by variant.
func (d *dredIndex) buildTriggers() {
	d.triggers = make(map[dredTrigKey][]dredTrigger)
	for ri, rule := range d.rules {
		if rule.agg != nil {
			continue
		}
		for _, pi := range rule.positives {
			key := dredTrigKey{sym: rule.clause.Premises[pi].(ast.Atom).Predicate}
			d.triggers[key] = append(d.triggers[key], dredTrigger{ri, pi})
		}
		for _, ni := range rule.negatives {
			key := dredTrigKey{sym: rule.clause.Premises[ni].(ast.NegAtom).Atom.Predicate, negated: true}
			d.triggers[key] = append(d.triggers[key], dredTrigger{ri, ni})
		}
	}
}

// triggered appends to into the variants over the rows of changed, restricted
// to the rules in members: the premises reading a changed predicate with the
// given sign, skipping the predicates skip names. Each call's variants come
// ordered by rule, then premise, as walking the rules would give.
func (d *dredIndex) triggered(ctx engine.QueryContext, members map[int]struct{}, changed deltaStores, negated bool, skip func(ast.PredicateSym) bool, into []dredVariant) []dredVariant {
	start := len(into)
	for sym, delta := range changed {
		if skip != nil && skip(sym) {
			continue
		}
		for _, t := range d.triggers[dredTrigKey{sym: sym, negated: negated}] {
			if _, ok := members[t.rule]; ok {
				into = append(into, dredVariant{ctx: ctx, rule: t.rule, clause: d.rules[t.rule].clause, pi: t.pi, negated: negated, delta: delta})
			}
		}
	}
	added := into[start:]
	sort.Slice(added, func(i, j int) bool {
		if added[i].rule != added[j].rule {
			return added[i].rule < added[j].rule
		}
		return added[i].pi < added[j].pi
	})
	return into
}
