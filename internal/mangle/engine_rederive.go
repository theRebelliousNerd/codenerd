package mangle

import (
	"codeberg.org/TauCeti/mangle-go/ast"
	mengine "codeberg.org/TauCeti/mangle-go/engine"
)

// =============================================================================
// EXACT RE-DERIVATION
// =============================================================================
// The store holds base and derived facts together, and evaluation runs over
// it. For rules without negation or aggregation, adding base facts can only add
// conclusions, so evaluating over the old derived facts is exact. Otherwise it
// is not:
//
//   - A conclusion derived before a fact that defeats it through negation
//     survives. Measured 2026-09-22: permitted(.env) derived when
//     touches_secret_path arrived after pending_action, and not when it arrived
//     first; the constitution rule was right and the engine was order-dependent.
//   - Removing a base fact removed nothing derived from it, whatever the rules:
//     ReplaceFactsForFile left every conclusion about the old contents in place.
//
// So every mutation records what it touched (markDirtyLocked), and the next
// evaluation (rederiveLocked) first clears the derived facts downstream of it
// whenever the change could have invalidated one -- any removal, or an addition
// whose downstream holds a non-monotone rule -- and then evaluates. The written
// set bounds the work: a large engine that only ever adds facts under positive
// rules evaluates exactly as it did before.

// ruleGraph is the rule-dependency graph of one program, by predicate name.
type ruleGraph struct {
	consumers   map[string][]string // body predicate -> heads of the rules reading it
	nonMonotone map[string]struct{} // heads of rules with a negated premise or a transform
}

func premiseNames(t ast.Term, out map[string]struct{}) {
	switch v := t.(type) {
	case ast.Atom:
		out[v.Predicate.Symbol] = struct{}{}
	case ast.NegAtom:
		out[v.Atom.Predicate.Symbol] = struct{}{}
	case ast.TemporalAtom:
		out[v.Atom.Predicate.Symbol] = struct{}{}
	case ast.TemporalLiteral:
		premiseNames(v.Literal, out)
	}
}

func isNegated(t ast.Term) bool {
	switch v := t.(type) {
	case ast.NegAtom:
		return true
	case ast.TemporalLiteral:
		return isNegated(v.Literal)
	}
	return false
}

func buildRuleGraph(rules []ast.Clause) ruleGraph {
	g := ruleGraph{consumers: make(map[string][]string), nonMonotone: make(map[string]struct{})}
	edges := make(map[string]map[string]struct{})
	for _, rule := range rules {
		head := rule.Head.Predicate.Symbol
		if rule.Transform != nil {
			g.nonMonotone[head] = struct{}{}
		}
		body := make(map[string]struct{})
		for _, premise := range rule.Premises {
			premiseNames(premise, body)
			if isNegated(premise) {
				g.nonMonotone[head] = struct{}{}
			}
		}
		for pred := range body {
			if edges[pred] == nil {
				edges[pred] = make(map[string]struct{})
			}
			edges[pred][head] = struct{}{}
		}
	}
	for pred, heads := range edges {
		for head := range heads {
			g.consumers[pred] = append(g.consumers[pred], head)
		}
	}
	return g
}

// downstream returns the given predicates and every head derived from them,
// directly or through other rules.
func (g ruleGraph) downstream(preds map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(preds))
	queue := make([]string, 0, len(preds))
	for pred := range preds {
		out[pred] = struct{}{}
		queue = append(queue, pred)
	}
	for len(queue) > 0 {
		pred := queue[0]
		queue = queue[1:]
		for _, head := range g.consumers[pred] {
			if _, seen := out[head]; !seen {
				out[head] = struct{}{}
				queue = append(queue, head)
			}
		}
	}
	return out
}

// markDirtyLocked records a mutation for the next rederiveLocked. removal says
// a base fact left the store. Caller holds e.mu.
func (e *Engine) markDirtyLocked(removal bool, preds ...string) {
	if e.dirtyPreds == nil {
		e.dirtyPreds = make(map[string]struct{}, len(preds))
	}
	for _, pred := range preds {
		e.dirtyPreds[pred] = struct{}{}
	}
	if removal {
		e.dirtyRemoval = true
	}
}

// rederiveLocked brings the derived facts up to date with the base facts and
// the program: it clears whatever the recorded mutations could have
// invalidated, then evaluates. Caller holds e.mu.
func (e *Engine) rederiveLocked() (mengine.Stats, error) {
	if e.programInfo != nil {
		switch {
		case e.dirtyAll:
			for sym := range e.programInfo.IdbPredicates {
				e.clearDerivedLocked(sym)
			}
		case len(e.dirtyPreds) > 0:
			cone := e.rules.downstream(e.dirtyPreds)
			invalidating := e.dirtyRemoval
			if !invalidating {
				for pred := range cone {
					if _, ok := e.rules.nonMonotone[pred]; ok {
						invalidating = true
						break
					}
				}
			}
			if invalidating {
				for sym := range e.programInfo.IdbPredicates {
					if _, ok := cone[sym.Symbol]; ok {
						e.clearDerivedLocked(sym)
					}
				}
			}
		}
	}
	stats, err := e.evalWithGasLimit()
	if err == nil {
		e.dirtyAll, e.dirtyRemoval, e.dirtyPreds = false, false, nil
	}
	return stats, err
}

// clearDerivedLocked removes the derived atoms of one rule-head predicate. A
// base fact asserted into it (idbBase) stays: it was never derived, and no
// evaluation would put it back. Caller holds e.mu.
func (e *Engine) clearDerivedLocked(sym ast.PredicateSym) {
	base := e.idbBase[sym]
	var atoms []ast.Atom
	_ = e.store.GetFacts(ast.NewQuery(sym), func(atom ast.Atom) error {
		if _, isBase := base[atom.String()]; !isBase {
			atoms = append(atoms, atom)
		}
		return nil
	})
	for _, atom := range atoms {
		e.baseStore.Remove(atom)
	}
}

// recordBaseLocked notes a base fact asserted into a rule-head predicate, so
// clearing that predicate's derived atoms leaves it in place.
func (e *Engine) recordBaseLocked(atom ast.Atom) {
	if e.programInfo == nil {
		return
	}
	if _, idb := e.programInfo.IdbPredicates[atom.Predicate]; !idb {
		return
	}
	if e.idbBase == nil {
		e.idbBase = make(map[ast.PredicateSym]map[string]struct{})
	}
	if e.idbBase[atom.Predicate] == nil {
		e.idbBase[atom.Predicate] = make(map[string]struct{})
	}
	e.idbBase[atom.Predicate][atom.String()] = struct{}{}
}

// forgetBaseLocked is recordBaseLocked's inverse, for a base fact removed.
func (e *Engine) forgetBaseLocked(atom ast.Atom) {
	if set := e.idbBase[atom.Predicate]; set != nil {
		delete(set, atom.String())
	}
}

// adoptProgramLocked installs a newly analyzed program. A predicate that was
// base-only and now heads a rule holds nothing but base facts, so they are
// recorded as such before anything is cleared; and the derived facts in the
// store came from the old program, so all of them are re-derived next time.
func (e *Engine) adoptProgramLocked(previous map[ast.PredicateSym]struct{}) {
	for sym := range e.programInfo.IdbPredicates {
		if _, was := previous[sym]; was {
			continue
		}
		_ = e.store.GetFacts(ast.NewQuery(sym), func(atom ast.Atom) error {
			e.recordBaseLocked(atom)
			return nil
		})
	}
	e.rules = buildRuleGraph(e.programInfo.Rules)
	e.dirtyAll = true
}
