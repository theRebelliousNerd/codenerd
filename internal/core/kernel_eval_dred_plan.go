package core

import (
	"codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// DRED: BINDING-AWARE PREMISE ORDER
// =============================================================================
// A variant used to join its delta premise first and then the rest in source
// order. That order is written for the full fixpoint, not for a variant: in
//
//	conflict_loser(A) :- candidate_atom(A), atom_conflicts(A, M), mandatory_atom(M).
//
// the variant over new mandatory_atom rows binds M, and source order then
// scanned every candidate_atom with A unbound before atom_conflicts(A, M), which
// M already narrowed, could filter them. That one head cost 2.6 s of a 1.0 s
// parallel DRed pass over the JIT kernel (measured 2026-11-06).
//
// planOrder orders a body for a starting set of bound variables (the delta
// premise's, a bound head's, a group key's): greedily, the positive atom with
// the most bound arguments next; any other premise (negation, comparison,
// equality, built-in) as soon as every variable it mentions is bound, and an
// equality also as soon as it can bind one side from the other. A non-atom
// that is not ready blocks every premise after it, including the delta
// premise. The engine runs that premise there, and a later atom must not
// ground its variables first: `X = Y, !r(X), p(Y)` evaluates the negation
// while X is still a variable, and grounding X first would derive rows the
// fixpoint does not. X = Y itself grounds nothing, so it does not count as
// binding X for the negation that follows. What is still unplaced runs in
// source order. A body with a deferred premise (answered top-down by mode)
// keeps source order, delta premise first.

// planOrder returns the premise indexes of clause in evaluation order, the
// premise first first (-1: none), given variables bound before the body runs.
func planOrder(clause ast.Clause, first int, bound map[ast.Variable]bool, deferred func(ast.PredicateSym) bool) []int {
	n := len(clause.Premises)
	order := make([]int, 0, n)
	placed := make([]bool, n)
	have := make(map[ast.Variable]bool, len(bound)+4)
	for v := range bound {
		have[v] = true
	}
	place := func(i int) {
		order = append(order, i)
		placed[i] = true
		// The variant reads premise first as a positive lookup of the delta,
		// even when the clause writes a negation there, so that lookup grounds
		// its variables. Every other premise grounds only what it grounds.
		if i == first {
			ast.AddVars(clause.Premises[i], have)
			return
		}
		bindGround(clause.Premises[i], have)
	}
	for _, p := range clause.Premises {
		if a, ok := p.(ast.Atom); ok && deferred(a.Predicate) {
			// Source order, with the requested premise first when it is a
			// plain atom (the variant's delta).
			if first >= 0 {
				order = append(order, first)
			}
			for i := range clause.Premises {
				if i != first {
					order = append(order, i)
				}
			}
			return order
		}
	}
	// The delta premise grounds the variables of a positive lookup, so it may
	// lead. It may not jump a premise the engine still runs with a free
	// variable: that lookup would ground the variable first.
	if first >= 0 && !earlierUnready(clause, first, have) {
		place(first)
	}
	for len(order) < n {
		// Every non-atom premise whose variables are all bound goes now:
		// filters are cheapest early.
		progressed := false
		for i, p := range clause.Premises {
			if placed[i] || isPositiveAtom(p) {
				continue
			}
			if allBound(p, have) || bindsOneSide(p, have) {
				place(i)
				progressed = true
			}
		}
		if progressed {
			continue
		}
		// The earliest unready non-atom is where the engine evaluates it.
		// Positive atoms after it stay after it.
		block := n
		for i, p := range clause.Premises {
			if placed[i] || isPositiveAtom(p) {
				continue
			}
			if !allBound(p, have) && !bindsOneSide(p, have) {
				block = i
				break
			}
		}
		best, bestBound := -1, -1
		for i, p := range clause.Premises {
			if placed[i] || !isPositiveAtom(p) || i > block {
				continue
			}
			if b := boundArgs(p.(ast.Atom), have); b > bestBound {
				best, bestBound = i, b
			}
		}
		if best < 0 {
			for i := range clause.Premises {
				if !placed[i] {
					place(i)
					if i == block {
						break
					}
				}
			}
			if block < n {
				continue
			}
			break
		}
		place(best)
	}
	return order
}

// earlierUnready reports a non-atom before index that is not ready to run
// with the variables grounded so far.
func earlierUnready(clause ast.Clause, index int, have map[ast.Variable]bool) bool {
	for i, p := range clause.Premises {
		if i >= index {
			return false
		}
		if isPositiveAtom(p) || allBound(p, have) || bindsOneSide(p, have) {
			continue
		}
		return true
	}
	return false
}

// bindGround records the variables premise actually grounds. A positive atom
// grounds its arguments. An equality grounds a variable only when the other
// side is already ground: X = Y unifies two free variables and grounds
// neither, and marking both bound lets the negation after it run as if X
// were a constant. A negation, comparison, or built-in does not ground a
// variable for a later premise; the substitution does that when the premise
// runs, and pretending otherwise pulls that later premise forward.
func bindGround(premise ast.Term, have map[ast.Variable]bool) {
	if eq, ok := premise.(ast.Eq); ok {
		if v, ok := eq.Left.(ast.Variable); ok && v.Symbol != "_" && allBound(eq.Right, have) {
			have[v] = true
		}
		if v, ok := eq.Right.(ast.Variable); ok && v.Symbol != "_" && allBound(eq.Left, have) {
			have[v] = true
		}
		return
	}
	if !isPositiveAtom(premise) {
		return
	}
	ast.AddVars(premise, have)
}

// isPositiveAtom is a premise the planner may run with variables unbound: a
// non-built-in atom over plain arguments. An argument computed by a function
// needs its variables bound first, so such an atom waits like a filter.
func isPositiveAtom(p ast.Term) bool {
	a, ok := p.(ast.Atom)
	return ok && !a.Predicate.IsBuiltin() && plainAtom(a)
}

// bindsOneSide reports an equality that can run now and bind: one side a
// lone variable, the other fully bound (Z = fn:plus(X, 1) once X is bound).
func bindsOneSide(p ast.Term, have map[ast.Variable]bool) bool {
	eq, ok := p.(ast.Eq)
	if !ok {
		return false
	}
	if v, ok := eq.Left.(ast.Variable); ok && v.Symbol != "_" && allBound(eq.Right, have) {
		return true
	}
	if v, ok := eq.Right.(ast.Variable); ok && v.Symbol != "_" && allBound(eq.Left, have) {
		return true
	}
	return false
}

func allBound(p ast.Term, have map[ast.Variable]bool) bool {
	vars := make(map[ast.Variable]bool)
	ast.AddVars(p, vars)
	for v := range vars {
		if v.Symbol != "_" && !have[v] {
			return false
		}
	}
	return true
}

// boundArgs counts an atom's arguments that are constants or bound variables.
func boundArgs(a ast.Atom, have map[ast.Variable]bool) int {
	n := 0
	for _, arg := range a.Args {
		switch t := arg.(type) {
		case ast.Constant:
			n++
		case ast.Variable:
			if t.Symbol != "_" && have[t] {
				n++
			}
		}
	}
	return n
}
