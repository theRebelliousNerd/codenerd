package mangle

import (
	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// STRATUM LAYERS
// =============================================================================
// analysis.Stratify returns one stratum per strongly connected component of
// the rule dependency graph: 1,315 for the action kernel's program, most of
// them a single non-recursive predicate. mangle-go v0.7's
// EvalStratifiedProgramWithStats rebuilds, for every stratum i, a set of every
// predicate of strata 0..i-1 (seminaivebottomup.go, evalStrata; the set is
// handed to rewrite.Rewrite and then never read), so a call costs
// O(strata x predicates) before any rule runs. Measured 2026-10-29 on the
// production corpus over its boot EDB: 134 ms per full evaluation, of which the
// strata themselves took 15 ms.
//
// LayerStrata hands the engine fewer, larger strata. A layer is a set of
// components no member of which reads another member's component, directly
// or through the top-down evaluation the engine runs during a stratum. Each
// layer's components are therefore evaluated exactly as if they ran one after
// another: no rule of one sees a fact of another, so the semi-naive rounds,
// the delta rules (built only for premises of the same stratum) and the
// end-of-stratum do-transforms do the same work and derive the same facts.
//
// A component's layer is one more than the deepest layer it reads (longest
// path layering). Two kinds of read order it:
//
//   - a rule body premise, positive, negated, temporal or under a transform:
//     the dependency analysis.Stratify already sorted by;
//   - a top-down read: a deferred() premise and a lattice's merge predicate
//     (descr merge([...], "p")) are evaluated during the reading stratum,
//     against the store as it is then, so the components their rules read
//     keep the order they had in the original strata. analysis.Stratify does
//     not see these reads, so the original order between such components is
//     whatever its topological sort chose; layering keeps it, never flips it.
//
// Every edge runs from a lower original index to a higher one, so the layer
// graph is acyclic and every pair of components that interact keeps its
// original relative order. Components that do not interact may change order,
// which analysis.Stratify itself already does between runs (its components
// come out of map iteration).

// Stratify stratifies an analyzed program and layers its strata: the strata
// and predicate-to-stratum map every EvalStratifiedProgramWithStats call in
// codeNERD takes. It fails exactly when analysis.Stratify does.
func Stratify(info *analysis.ProgramInfo) ([]analysis.Nodeset, map[ast.PredicateSym]int, error) {
	strata, predToStratum, err := analysis.Stratify(analysis.Program{
		EdbPredicates: info.EdbPredicates,
		IdbPredicates: info.IdbPredicates,
		Rules:         info.Rules,
	})
	if err != nil {
		return nil, nil, err
	}
	layers, predToLayer := LayerStrata(info, strata, predToStratum)
	return layers, predToLayer, nil
}

// LayerStrata merges the strata of an analyzed, stratified program into
// layers in evaluation order. It returns the layers and the predicate-to-layer
// map, in the shape EvalStratifiedProgramWithStats takes.
func LayerStrata(info *analysis.ProgramInfo, strata []analysis.Nodeset, predToStratum map[ast.PredicateSym]int) ([]analysis.Nodeset, map[ast.PredicateSym]int) {
	if len(strata) == 0 {
		return strata, predToStratum
	}
	rulesByHead := make(map[ast.PredicateSym][]ast.Clause, len(strata))
	for _, rule := range info.Rules {
		rulesByHead[rule.Head.Predicate] = append(rulesByHead[rule.Head.Predicate], rule)
	}
	topDownOf := topDownDependencies(info, rulesByHead)

	// reads[s] holds the strata, other than s, that s must run after.
	reads := make([]map[int]struct{}, len(strata))
	order := func(a, b int) {
		if a == b {
			return
		}
		if a > b {
			a, b = b, a
		}
		if reads[b] == nil {
			reads[b] = make(map[int]struct{})
		}
		reads[b][a] = struct{}{}
	}
	premiseStrata := func(rule ast.Clause, visit func(int)) {
		for _, premise := range rule.Premises {
			if sym, ok := premiseSymbol(premise); ok {
				if t, ok := predToStratum[sym]; ok {
					visit(t)
				}
			}
		}
	}
	for s, stratum := range strata {
		for head := range stratum {
			for _, rule := range rulesByHead[head] {
				premiseStrata(rule, func(t int) { order(t, s) })
			}
			for td := range topDownClosure(head, topDownOf) {
				for _, rule := range rulesByHead[td] {
					premiseStrata(rule, func(t int) { order(t, s) })
				}
			}
		}
	}

	layerOf := make([]int, len(strata))
	layers := 0
	for s := range strata {
		layer := 0
		for t := range reads[s] {
			layer = max(layer, layerOf[t]+1)
		}
		layerOf[s] = layer
		layers = max(layers, layer+1)
	}
	out := make([]analysis.Nodeset, layers)
	for i := range out {
		out[i] = make(analysis.Nodeset)
	}
	predToLayer := make(map[ast.PredicateSym]int, len(predToStratum))
	for s, stratum := range strata {
		for sym := range stratum {
			out[layerOf[s]][sym] = struct{}{}
			predToLayer[sym] = layerOf[s]
		}
	}
	return out, predToLayer
}

// premiseSymbol names the predicate a rule premise reads, if it reads one.
func premiseSymbol(t ast.Term) (ast.PredicateSym, bool) {
	switch v := t.(type) {
	case ast.Atom:
		return v.Predicate, true
	case ast.NegAtom:
		return v.Atom.Predicate, true
	case ast.TemporalAtom:
		return v.Atom.Predicate, true
	case ast.TemporalLiteral:
		return premiseSymbol(v.Literal)
	}
	return ast.PredicateSym{}, false
}

// topDownDependencies names, per predicate, the predicates the engine
// evaluates top-down while deriving it: every deferred() predicate in one of
// its rule bodies, and its merge predicate when it is a lattice.
func topDownDependencies(info *analysis.ProgramInfo, rulesByHead map[ast.PredicateSym][]ast.Clause) map[ast.PredicateSym][]ast.PredicateSym {
	deferred := make(map[ast.PredicateSym]struct{})
	out := make(map[ast.PredicateSym][]ast.PredicateSym)
	for sym, decl := range info.Decls {
		if decl == nil {
			continue
		}
		if decl.DeferredPredicate() {
			deferred[sym] = struct{}{}
		}
		if len(decl.FunDeps()) == 1 {
			if _, merge := decl.MergePredicate(); merge.Symbol != "" {
				out[sym] = append(out[sym], merge)
			}
		}
	}
	for head, rules := range rulesByHead {
		for _, rule := range rules {
			for _, premise := range rule.Premises {
				if sym, ok := premiseSymbol(premise); ok {
					if _, isDeferred := deferred[sym]; isDeferred {
						out[head] = append(out[head], sym)
					}
				}
			}
		}
	}
	return out
}

// topDownClosure is every predicate evaluated top-down while deriving pred,
// transitively.
func topDownClosure(pred ast.PredicateSym, topDownOf map[ast.PredicateSym][]ast.PredicateSym) map[ast.PredicateSym]struct{} {
	direct := topDownOf[pred]
	if len(direct) == 0 {
		return nil
	}
	out := make(map[ast.PredicateSym]struct{}, len(direct))
	queue := append([]ast.PredicateSym(nil), direct...)
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if _, seen := out[p]; seen {
			continue
		}
		out[p] = struct{}{}
		queue = append(queue, topDownOf[p]...)
	}
	return out
}
