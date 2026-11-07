package core

import (
	"errors"
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
// DRED: AGGREGATES MAINTAINED BY DELTA
// =============================================================================
// Recomputing a changed group reads all its rows, so one change to a group of
// n members costs n (the production dependent_count rule groups every importer
// of a file). A transform that is only fn:count() and integer fn:sum(X) needs
// no rows: the group's new output is its old output plus the change.
//
//   - The old output is the state. The store holds the head exactly as the
//     last evaluation of any kind left it, so there is nothing to keep beside
//     it and nothing to go stale across Clone(), a full or cone evaluation, a
//     policy change or a tainted store (each of which leaves a correct head, or
//     a fresh store). A group whose output cannot be read as such (more than
//     one row, a non-integer value, a count that does not fit the change) is
//     recomputed from its rows instead.
//   - The change is the set of rows that appeared or disappeared, exactly. A
//     single-atom body gives one row per fact, so a changed fact is a changed
//     row. A longer body gives one row per distinct tuple of its variables, so
//     a row with two derivations losing one must not count: the tuples the
//     change variants reach are candidates, and each is looked up in the old
//     and the new state (every variable bound, so every premise is a lookup);
//     only a tuple present in one and not the other changes its group.
//   - A group left with no rows has no output, never a zero count. With a
//     count in the head the row count is known; a sum alone has to ask the
//     new state whether any row is left, and only when a row was removed.
//
// Sums are int64 and wrap as the engine's do; a summand that is not an
// integer sends its group to the recompute, which reports it as the engine
// does. Float and duration sums, min, max and every other reducer keep the
// group recompute.

// aggReducer is one maintained let of a do-transform.
type aggReducer struct {
	out     ast.Variable
	sum     bool         // fn:sum(arg); else fn:count()
	arg     ast.Variable // the summed column
	headPos int          // where out sits in the head: the old value is read there
}

// rowColumns are the variables a single-atom body's rows carry, by the
// engine's row construction: its arguments that are plain variables.
func rowColumns(a ast.Atom) []ast.Variable {
	var cols []ast.Variable
	seen := make(map[ast.Variable]bool, len(a.Args))
	for _, term := range a.Args {
		if v, ok := term.(ast.Variable); ok && v.Symbol != "_" && !seen[v] {
			seen[v] = true
			cols = append(cols, v)
		}
	}
	return cols
}

// deltaReducers returns the maintained form of rule's transform, or nil when
// any part of it needs the group's rows: another reducer or function, an
// expression as a summand, a group or summed variable the rows do not carry,
// or a head that is not exactly the group's key and the reducers' results.
func (d *dredIndex) deltaReducers(rule dredRule, agg *dredAggregate, columns []ast.Variable) []aggReducer {
	c := rule.clause
	lets := c.Transform.Statements[1:]
	if len(lets) == 0 {
		return nil
	}
	carried := make(map[ast.Variable]bool, len(columns))
	for _, v := range columns {
		carried[v] = true
	}
	known := make(map[ast.Variable]bool, len(agg.groupVars)+len(lets))
	for _, v := range agg.groupVars {
		if !carried[v] {
			return nil
		}
		known[v] = true
	}
	reducers := make([]aggReducer, 0, len(lets))
	for _, stmt := range lets {
		if stmt.Var == nil || known[*stmt.Var] {
			return nil
		}
		red := aggReducer{out: *stmt.Var, headPos: -1}
		switch stmt.Fn.Function.Symbol {
		case symbols.Count.Symbol:
			if len(stmt.Fn.Args) != 0 {
				return nil
			}
		case symbols.Sum.Symbol:
			if len(stmt.Fn.Args) != 1 {
				return nil
			}
			arg, ok := stmt.Fn.Args[0].(ast.Variable)
			if !ok || !carried[arg] {
				return nil
			}
			red.sum, red.arg = true, arg
		default:
			return nil
		}
		for i, h := range c.Head.Args {
			if hv, ok := h.(ast.Variable); ok && hv == red.out {
				red.headPos = i
				break
			}
		}
		if red.headPos < 0 {
			return nil // its value never reaches the head: the engine still evaluates it
		}
		known[red.out] = true
		reducers = append(reducers, red)
	}
	for _, h := range c.Head.Args {
		if hv, ok := h.(ast.Variable); ok && !known[hv] {
			return nil
		}
	}
	return reducers
}

// aggGroup is the net change of one group's rows.
type aggGroup struct {
	key            []ast.Constant
	added, removed int64
	sums           []int64 // per reducer: summed added rows minus summed removed rows
	bad            bool    // a summand was not an integer: recompute the group
}

// aggChanges collects the row changes of one aggregate by group.
type aggChanges struct {
	agg   *dredAggregate
	byKey map[string]*aggGroup
	order []*aggGroup // in first-seen order
}

// fold records that row appeared (sign +1) or disappeared (-1).
func (c *aggChanges) fold(rule dredRule, row ast.ConstSubstList, sign int64) error {
	key := make([]ast.Constant, len(c.agg.groupVars))
	for i, v := range c.agg.groupVars {
		con, ok := row.Get(v).(ast.Constant)
		if !ok {
			return fmt.Errorf("dred: %s: group variable %s unbound", rule.head.Symbol, v.Symbol)
		}
		key[i] = con
	}
	ks := constsKey(key)
	g := c.byKey[ks]
	if g == nil {
		g = &aggGroup{key: key, sums: make([]int64, len(c.agg.reducers))}
		c.byKey[ks] = g
		c.order = append(c.order, g)
	}
	if sign > 0 {
		g.added++
	} else {
		g.removed++
	}
	for i, red := range c.agg.reducers {
		if !red.sum {
			continue
		}
		con, ok := row.Get(red.arg).(ast.Constant)
		if !ok || con.Type != ast.NumberType {
			g.bad = true
			continue
		}
		g.sums[i] += sign * con.NumValue
	}
	return nil
}

// constsKey is a collision-free string key of a tuple of constants.
func constsKey(cs []ast.Constant) string {
	var sb strings.Builder
	for _, c := range cs {
		s := c.String()
		fmt.Fprintf(&sb, "%d:%s|", len(s), s)
	}
	return sb.String()
}

// aggregateByDelta maintains one counts-and-sums rule's head from the rows
// that changed.
func (r *dredRun) aggregateByDelta(rule dredRule, oldCtx engine.QueryContext) error {
	k := r.k
	agg := rule.agg
	newCtx := r.ctx(k.store)
	changes := &aggChanges{agg: agg, byKey: make(map[string]*aggGroup)}
	var err error
	if agg.single {
		err = r.eachChange(rule, oldCtx, newCtx, func(sols []unionfind.UnionFind, sign int) error {
			for _, sol := range sols {
				if err := changes.fold(rule, sol.AsConstSubstList(), int64(sign)); err != nil {
					return err
				}
			}
			return nil
		})
	} else {
		err = r.tupleChanges(rule, oldCtx, newCtx, changes)
	}
	if err != nil {
		return err
	}
	if len(changes.order) == 0 {
		return nil
	}
	remover := k.store.(factstore.FactStoreWithRemove)
	r.dropScratchOf(rule.head.Symbol, remover)
	for _, g := range changes.order {
		if err := r.applyGroupDelta(rule, newCtx, g, remover); err != nil {
			return err
		}
	}
	return r.checkLimit()
}

// tupleChanges folds into changes the rows of a multi-premise body that
// appeared or disappeared: of the tuples the change variants reach, those the
// body derives in one state and not the other.
func (r *dredRun) tupleChanges(rule dredRule, oldCtx, newCtx engine.QueryContext, changes *aggChanges) error {
	agg := rule.agg
	clause := rule.clause
	seen := make(map[string]struct{})
	var candidates [][]ast.Constant
	err := r.eachChange(rule, oldCtx, newCtx, func(sols []unionfind.UnionFind, _ int) error {
		for _, sol := range sols {
			sub := sol.AsConstSubstList()
			tuple := make([]ast.Constant, len(agg.bodyVars))
			for i, v := range agg.bodyVars {
				c, ok := sub.Get(v).(ast.Constant)
				if !ok {
					return fmt.Errorf("dred: %s: row variable %s unbound", rule.head.Symbol, v.Symbol)
				}
				tuple[i] = c
			}
			ks := constsKey(tuple)
			if _, dup := seen[ks]; !dup {
				seen[ks] = struct{}{}
				candidates = append(candidates, tuple)
			}
		}
		return nil
	})
	if err != nil || len(candidates) == 0 {
		return err
	}
	bound := make(map[ast.Variable]bool, len(agg.bodyVars))
	for _, v := range agg.bodyVars {
		bound[v] = true
	}
	order := planOrder(clause, -1, bound, r.isDeferred)
	holds := func(ctx engine.QueryContext, tuple []ast.Constant) (bool, error) {
		sol, err := unionfind.UnifyTermsExtend(varsAsTerms(agg.bodyVars), constsAsTerms(tuple), unionfind.New())
		if err != nil {
			return false, fmt.Errorf("dred: %s: bind row: %w", rule.head.Symbol, err)
		}
		return existsSolution(ctx, clause, order, sol)
	}
	for _, tuple := range candidates {
		was, err := holds(oldCtx, tuple)
		if err != nil {
			return err
		}
		now, err := holds(newCtx, tuple)
		if err != nil {
			return err
		}
		if was == now {
			continue
		}
		var row ast.ConstSubstList
		for i, v := range agg.bodyVars {
			row = row.Extend(v, tuple[i])
		}
		sign := int64(1)
		if was {
			sign = -1
		}
		if err := changes.fold(rule, row, sign); err != nil {
			return err
		}
	}
	return nil
}

// existsSolution reports whether the premises of clause, run in order from
// the bindings of s, have a solution in ctx's state.
func existsSolution(ctx engine.QueryContext, clause ast.Clause, order []int, s unionfind.UnionFind) (bool, error) {
	var walk func(i int, s unionfind.UnionFind) (bool, error)
	walk = func(i int, s unionfind.UnionFind) (bool, error) {
		if i == len(order) {
			return true, nil
		}
		step, err := ctx.EvalPremise(clause.Premises[order[i]], s)
		if err != nil {
			return false, err
		}
		for _, next := range step {
			if ok, err := walk(i+1, next); ok || err != nil {
				return ok, err
			}
		}
		return false, nil
	}
	return walk(0, s)
}

// applyGroupDelta brings one group's output to its old value plus the change.
func (r *dredRun) applyGroupDelta(rule dredRule, newCtx engine.QueryContext, g *aggGroup, remover factstore.FactStoreWithRemove) error {
	agg := rule.agg
	recompute := func() error { return r.recomputeGroup(rule, newCtx, g.key, remover) }
	bound, old, err := r.groupOutput(rule, g.key)
	if err != nil {
		return err
	}
	if g.bad || len(old) > 1 {
		return recompute()
	}
	haveCount := false
	for _, red := range agg.reducers {
		haveCount = haveCount || !red.sum
	}
	// The old output is the group's old state: its sums, and its row count
	// when a count is in the head.
	oldVals := make([]int64, len(agg.reducers))
	var oldRows int64
	if len(old) == 1 {
		for i, red := range agg.reducers {
			c, ok := old[0].Args[red.headPos].(ast.Constant)
			if !ok || c.Type != ast.NumberType {
				return recompute()
			}
			oldVals[i] = c.NumValue
			if !red.sum {
				oldRows = c.NumValue
			}
		}
		if haveCount && oldRows < 1 {
			return recompute() // a count that no group can have: not this head's own output
		}
	} else if g.removed > 0 {
		return recompute() // rows left a group the store holds no output for
	}

	var newRows int64
	empty := false
	switch {
	case len(old) == 0:
		newRows = g.added
	case haveCount:
		if newRows = oldRows + g.added - g.removed; newRows < 0 {
			return recompute()
		}
		empty = newRows == 0
	case g.removed > 0:
		// Sums alone do not know the row count: ask the new state for one.
		has, err := r.groupHasRows(newCtx, rule, bound)
		if err != nil {
			return err
		}
		empty = !has
	}

	var fresh []ast.Atom
	if !empty {
		var subst ast.ConstSubstList
		for i, v := range agg.groupVars {
			subst = subst.Extend(v, g.key[i])
		}
		for i, red := range agg.reducers {
			n := newRows
			if red.sum {
				n = oldVals[i] + g.sums[i]
			}
			subst = subst.Extend(red.out, ast.Number(n))
		}
		fresh = append(fresh, rule.clause.Head.ApplySubst(subst).(ast.Atom))
	}
	r.aggDelta++
	r.replaceOutput(old, fresh, remover)
	return nil
}

// errRowFound ends a row scan at its first row.
var errRowFound = errors.New("row found")

// groupHasRows reports whether the group bound in sub has any row in ctx's
// state, without reading them all.
func (r *dredRun) groupHasRows(ctx engine.QueryContext, rule dredRule, sub unionfind.UnionFind) (bool, error) {
	clause := rule.clause
	if rule.agg.single {
		premise := clause.Premises[0].(ast.Atom)
		query, err := functional.EvalAtom(premise, sub)
		if err != nil {
			return false, err
		}
		err = ctx.Store.GetFacts(query, func(fact ast.Atom) error {
			if _, ok := factRow(premise, fact); ok {
				return errRowFound
			}
			return nil
		})
		if errors.Is(err, errRowFound) {
			return true, nil
		}
		return false, err
	}
	bound := make(map[ast.Variable]bool, len(rule.agg.groupVars))
	for _, v := range rule.agg.groupVars {
		bound[v] = true
	}
	return existsSolution(ctx, clause, planOrder(clause, -1, bound, r.isDeferred), sub)
}
