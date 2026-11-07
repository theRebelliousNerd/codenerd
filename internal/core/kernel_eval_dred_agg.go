package core

import (
	"fmt"
	"strings"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/functional"
	"codeberg.org/TauCeti/mangle-go/symbols"
	"codeberg.org/TauCeti/mangle-go/unionfind"
)

// =============================================================================
// DRED: AGGREGATIONS BY GROUP
// =============================================================================
// A do-transform (`|> do fn:group_by(G...), let N = fn:count()`) replaces its
// output rather than adding to it, so DRed's insert and delete do not apply.
// What does: only a group whose body rows changed can change its output. The
// changed groups are read off the change variants' solutions (removed rows
// and newly true negations against the old state, inserted rows and newly
// false negations against the new), and each is recomputed from the new
// state alone: its old output is replaced by the transform over its rows.
//
// The rows are the engine's, exactly. A single-atom body feeds the transform
// one row per matching fact. A longer body goes through the engine's rewrite
// (mangle-go rewrite.Rewrite), a scratch relation "<head><n>__tmp" whose rows
// are the distinct tuples of the variables of the body's atoms and
// equalities, so rows are deduplicated on those variables. The scratch
// relation itself is engine-private (QueryAll lists declared predicates
// only) and named in rewrite order, so DRed drops it for a head it maintains
// instead of keeping it; the cone and full paths rebuild it when they run.

// dredAggregate is a do-transform rule's shape for group maintenance.
type dredAggregate struct {
	groupVars []ast.Variable
	single    bool           // one atom premise: one row per matching fact
	bodyVars  []ast.Variable // longer bodies: the row's columns
}

// aggregateShape returns the group-maintenance shape of rule, or nil when the
// group recompute does not cover it: grouping by anything but plain head
// variables, a chained transform, a head with another rule, or a body that
// reads its own stratum or a deferred predicate.
func (d *dredIndex) aggregateShape(rule dredRule) *dredAggregate {
	c := rule.clause
	t := c.Transform
	if t == nil || t.IsLetTransform() || t.Next != nil || c.HeadTime != nil || len(rule.deferred) > 0 {
		return nil
	}
	if len(d.bySym[rule.head]) != 1 {
		return nil
	}
	do := t.Statements[0]
	if do.Var != nil || do.Fn.Function.Symbol != symbols.GroupBy.Symbol {
		return nil
	}
	for _, arg := range c.Head.Args {
		switch arg.(type) {
		case ast.Variable, ast.Constant:
		default:
			return nil
		}
	}
	headStratum := d.predToStratum[rule.head]
	sameStratum := func(sym ast.PredicateSym) bool {
		s, ok := d.predToStratum[sym]
		return ok && s == headStratum
	}
	for _, premise := range c.Premises {
		switch p := premise.(type) {
		case ast.Atom:
			if !p.Predicate.IsBuiltin() && sameStratum(p.Predicate) {
				return nil
			}
		case ast.NegAtom:
			if sameStratum(p.Atom.Predicate) {
				return nil
			}
		case ast.Eq, ast.Ineq:
		default:
			return nil
		}
	}
	agg := &dredAggregate{}
	for _, arg := range do.Fn.Args {
		v, ok := arg.(ast.Variable)
		if !ok || v.Symbol == "_" {
			return nil
		}
		inHead := false
		for _, h := range c.Head.Args {
			if hv, ok := h.(ast.Variable); ok && hv == v {
				inHead = true
				break
			}
		}
		if !inHead {
			return nil
		}
		agg.groupVars = append(agg.groupVars, v)
	}
	if len(c.Premises) == 1 {
		if a, ok := c.Premises[0].(ast.Atom); ok {
			if a.Predicate.IsBuiltin() {
				return nil
			}
			agg.single = true
			return agg
		}
	}
	vars := make(map[ast.Variable]bool)
	for _, premise := range c.Premises {
		switch p := premise.(type) {
		case ast.Atom:
			ast.AddVars(p, vars)
		case ast.Eq:
			ast.AddVars(p.Left, vars)
			ast.AddVars(p.Right, vars)
		}
	}
	delete(vars, ast.Variable{Symbol: "_"})
	for v := range vars {
		agg.bodyVars = append(agg.bodyVars, v)
	}
	return agg
}

// aggregate maintains one do-transform rule's head by recomputing the groups
// whose rows changed, and records the head's net change.
func (r *dredRun) aggregate(rule dredRule, oldCtx engine.QueryContext) error {
	k := r.k
	agg := rule.agg
	clause := rule.clause
	newCtx := r.ctx(k.store)

	keySym := ast.PredicateSym{Symbol: "__dred_group", Arity: len(agg.groupVars)}
	seen := newAtomSet()
	var keys [][]ast.Constant
	collect := func(sols []unionfind.UnionFind) error {
		for _, sol := range sols {
			sub := sol.AsConstSubstList()
			key := make([]ast.Constant, len(agg.groupVars))
			args := make([]ast.BaseTerm, len(agg.groupVars))
			for i, v := range agg.groupVars {
				c, ok := sub.Get(v).(ast.Constant)
				if !ok {
					return fmt.Errorf("dred: %s: group variable %s unbound", rule.head.Symbol, v.Symbol)
				}
				key[i], args[i] = c, c
			}
			if marker := (ast.Atom{Predicate: keySym, Args: args}); !seen.has(marker) {
				seen.add(marker)
				keys = append(keys, key)
			}
		}
		return nil
	}
	// The concrete type, not the interface: a missing change is a nil
	// pointer, which as a ReadOnlyFactStore would not compare equal to nil.
	changed := func(ctx engine.QueryContext, i int, negated bool, delta *factstore.MultiIndexedArrayInMemoryStore) error {
		if delta == nil {
			return nil
		}
		sols, err := r.variantSolutions(ctx, clause, i, negated, delta)
		if err != nil {
			return err
		}
		return collect(sols)
	}
	for _, pi := range rule.positives {
		p := clause.Premises[pi].(ast.Atom).Predicate
		if err := changed(oldCtx, pi, false, r.minus[p]); err != nil {
			return err
		}
		if err := changed(newCtx, pi, false, r.plus[p]); err != nil {
			return err
		}
	}
	for _, ni := range rule.negatives {
		p := clause.Premises[ni].(ast.NegAtom).Atom.Predicate
		if err := changed(oldCtx, ni, true, r.plus[p]); err != nil {
			return err
		}
		if err := changed(newCtx, ni, true, r.minus[p]); err != nil {
			return err
		}
	}
	if len(keys) == 0 {
		return nil
	}
	remover := k.store.(factstore.FactStoreWithRemove)
	r.dropScratchOf(rule.head.Symbol, remover)

	for _, key := range keys {
		bound, err := unionfind.UnifyTermsExtend(varsAsTerms(agg.groupVars), constsAsTerms(key), unionfind.New())
		if err != nil {
			return fmt.Errorf("dred: %s: bind group: %w", rule.head.Symbol, err)
		}
		pattern, err := functional.EvalAtom(clause.Head, bound)
		if err != nil {
			return fmt.Errorf("dred: %s: group pattern: %w", rule.head.Symbol, err)
		}
		var old []ast.Atom
		if err := k.store.GetFacts(pattern, func(a ast.Atom) error {
			old = append(old, a)
			return nil
		}); err != nil {
			return err
		}
		rows, err := r.groupRows(newCtx, rule, bound)
		if err != nil {
			return err
		}
		var fresh []ast.Atom
		if len(rows) > 0 {
			if err := engine.EvalTransformWithInputFacts(clause.Head, *clause.Transform, rows, nil,
				func(out ast.Atom, _ engine.TransformKind, _ []ast.Constant, _ []ast.Atom) bool {
					fresh = append(fresh, out)
					return true
				}); err != nil {
				return fmt.Errorf("dred: %s: %w", rule.head.Symbol, err)
			}
		}
		was, now := newAtomSet(), newAtomSet()
		for _, a := range old {
			was.add(a)
		}
		for _, a := range fresh {
			now.add(a)
		}
		for _, a := range old {
			if !now.has(a) && remover.Remove(a) {
				r.minus.add(a)
				r.overDeleted++
			}
		}
		for _, a := range fresh {
			if !was.has(a) && k.store.Add(a) {
				r.plus.add(a)
				r.inserted++
			}
		}
	}
	return r.checkLimit()
}

// groupRows is the transform's input for one group, bound in sub: the rows
// the engine would feed it.
func (r *dredRun) groupRows(ctx engine.QueryContext, rule dredRule, sub unionfind.UnionFind) ([]ast.ConstSubstList, error) {
	clause := rule.clause
	if rule.agg.single {
		premise := clause.Premises[0].(ast.Atom)
		query, err := functional.EvalAtom(premise, sub)
		if err != nil {
			return nil, err
		}
		var rows []ast.ConstSubstList
		err = ctx.Store.GetFacts(query, func(fact ast.Atom) error {
			// The engine's own row construction (seminaivebottomup.go): a
			// variable that occurs twice must match equal arguments.
			var row ast.ConstSubstList
			for i, term := range premise.Args {
				v, ok := term.(ast.Variable)
				if !ok || v.Symbol == "_" {
					continue
				}
				c, ok := fact.Args[i].(ast.Constant)
				if !ok {
					continue
				}
				if prev := row.Get(v); prev != nil {
					if !prev.Equals(c) {
						return nil
					}
					continue
				}
				row = row.Extend(v, c)
			}
			rows = append(rows, row)
			return nil
		})
		return rows, err
	}
	bound := make(map[ast.Variable]bool, len(rule.agg.groupVars))
	for _, v := range rule.agg.groupVars {
		bound[v] = true
	}
	solutions := []unionfind.UnionFind{sub}
	for _, i := range planOrder(clause, -1, bound, r.isDeferred) {
		premise := clause.Premises[i]
		var out []unionfind.UnionFind
		for _, s := range solutions {
			step, err := ctx.EvalPremise(premise, s)
			if err != nil {
				return nil, err
			}
			out = append(out, step...)
			if r.limit > 0 && len(out) > r.limit {
				return nil, fmt.Errorf("dred: %s: solution limit %d exceeded", rule.head.Symbol, r.limit)
			}
		}
		if solutions = out; len(solutions) == 0 {
			return nil, nil
		}
	}
	tupleSym := ast.PredicateSym{Symbol: "__dred_row", Arity: len(rule.agg.bodyVars)}
	seen := newAtomSet()
	rows := make([]ast.ConstSubstList, 0, len(solutions))
	for _, sol := range solutions {
		sub := sol.AsConstSubstList()
		var row ast.ConstSubstList
		tuple := make([]ast.BaseTerm, len(rule.agg.bodyVars))
		for i, v := range rule.agg.bodyVars {
			c, ok := sub.Get(v).(ast.Constant)
			if !ok {
				return nil, fmt.Errorf("dred: %s: row variable %s unbound", rule.head.Symbol, v.Symbol)
			}
			row = row.Extend(v, c)
			tuple[i] = c
		}
		if marker := (ast.Atom{Predicate: tupleSym, Args: tuple}); !seen.has(marker) {
			seen.add(marker)
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// dropScratchOf removes the engine's scratch relations of head ("<head><n>__tmp"):
// once DRed maintains the head they would only go stale.
func (r *dredRun) dropScratchOf(head string, remover factstore.FactStoreWithRemove) {
	if r.scratchDropped == nil {
		r.scratchDropped = make(map[string]struct{})
	}
	if _, done := r.scratchDropped[head]; done {
		return
	}
	r.scratchDropped[head] = struct{}{}
	for _, sym := range r.k.store.ListPredicates() {
		base, ok := strings.CutSuffix(sym.Symbol, ast.InternalPredicateSuffix)
		if !ok {
			continue
		}
		if n, found := strings.CutPrefix(base, head); !found || n == "" || strings.Trim(n, "0123456789") != "" {
			continue
		}
		var rows []ast.Atom
		_ = r.k.store.GetFacts(ast.NewQuery(sym), func(a ast.Atom) error {
			rows = append(rows, a)
			return nil
		})
		for _, a := range rows {
			remover.Remove(a)
		}
	}
}

func varsAsTerms(vs []ast.Variable) []ast.BaseTerm {
	out := make([]ast.BaseTerm, len(vs))
	for i, v := range vs {
		out[i] = v
	}
	return out
}

func constsAsTerms(cs []ast.Constant) []ast.BaseTerm {
	out := make([]ast.BaseTerm, len(cs))
	for i, c := range cs {
		out[i] = c
	}
	return out
}
