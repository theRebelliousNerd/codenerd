package core

import (
	"fmt"
	"reflect"
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
// A transform of only counts and integer sums skips the recompute: its output
// moves by the change (kernel_eval_dred_agg_delta.go).
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
	// reducers is set when the transform is nothing but counts and integer
	// sums: those are maintained by delta (kernel_eval_dred_agg_delta.go)
	// instead of recomputed per changed group.
	reducers []aggReducer
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
			agg.reducers = d.deltaReducers(rule, agg, rowColumns(a))
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
	agg.reducers = d.deltaReducers(rule, agg, agg.bodyVars)
	return agg
}

// aggregate maintains one do-transform rule's head: by delta when it is only
// counts and integer sums, else by recomputing the groups whose rows changed.
// It records the head's net change.
func (r *dredRun) aggregate(rule dredRule, oldCtx engine.QueryContext) error {
	if rule.agg.reducers != nil {
		return r.aggregateByDelta(rule, oldCtx)
	}
	k := r.k
	agg := rule.agg
	newCtx := r.ctx(k.store)

	keySym := ast.PredicateSym{Symbol: "__dred_group", Arity: len(agg.groupVars)}
	seen := newAtomSet()
	var keys [][]ast.Constant
	err := r.eachChange(rule, oldCtx, newCtx, func(sols []unionfind.UnionFind, _ int) error {
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
	})
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	// The engine groups by the printed key (transformer.groupKeyString), so
	// int64(1) and float64(1) are one group. Equality keeps them apart, and
	// the fresh fixpoint is what chooses which atom stands for the group.
	if err := r.printAlikeGroupKeys(rule, keys); err != nil {
		return err
	}
	remover := k.store.(factstore.FactStoreWithRemove)
	r.dropScratchOf(rule.head.Symbol, remover)
	for _, key := range keys {
		if err := r.recomputeGroup(rule, newCtx, key, remover); err != nil {
			return err
		}
	}
	return r.checkLimit()
}

// eachChange hands visit the solutions of every change variant of rule: the
// body's solutions with one premise reading only a change. sign is the
// change's direction for the row a positive premise's solution is (-1 a
// removed fact, read in the old state; +1 an inserted one, read in the new);
// for a negated premise it only hints at the direction.
func (r *dredRun) eachChange(rule dredRule, oldCtx, newCtx engine.QueryContext, visit func(sols []unionfind.UnionFind, sign int) error) error {
	clause := rule.clause
	// The concrete type, not the interface: a missing change is a nil
	// pointer, which as a ReadOnlyFactStore would not compare equal to nil.
	changed := func(ctx engine.QueryContext, i int, negated bool, delta *factstore.MultiIndexedArrayInMemoryStore, sign int) error {
		if delta == nil {
			return nil
		}
		sols, err := r.variantSolutions(ctx, clause, i, negated, delta, rule.orders[i])
		if err != nil {
			return err
		}
		return visit(sols, sign)
	}
	for _, pi := range rule.positives {
		p := clause.Premises[pi].(ast.Atom).Predicate
		if err := changed(oldCtx, pi, false, r.minus[p], -1); err != nil {
			return err
		}
		if err := changed(newCtx, pi, false, r.plus[p], +1); err != nil {
			return err
		}
	}
	for _, ni := range rule.negatives {
		p := clause.Premises[ni].(ast.NegAtom).Atom.Predicate
		if err := changed(oldCtx, ni, true, r.plus[p], -1); err != nil {
			return err
		}
		if err := changed(newCtx, ni, true, r.minus[p], +1); err != nil {
			return err
		}
	}
	return nil
}

// recomputeGroup replaces one group's output with the transform over the
// group's rows in the new state, and records the difference.
func (r *dredRun) recomputeGroup(rule dredRule, newCtx engine.QueryContext, key []ast.Constant, remover factstore.FactStoreWithRemove) error {
	clause := rule.clause
	bound, old, err := r.groupOutput(rule, key)
	if err != nil {
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
	r.aggRecomputed++
	r.replaceOutput(old, fresh, remover)
	return nil
}

// groupOutput binds one group's key and reads the group's output rows from
// the store.
// A program fact of the head in the group is never part of the output: the
// engine adds it before any rule runs and never replaces it, so a recompute
// must not remove it (it did: an adversarial review found the group recompute
// deleting it). hasProgramFact says one matched, and the delta path then
// recomputes instead, since the stored rows no longer tell the old output.
func (r *dredRun) groupOutput(rule dredRule, key []ast.Constant) (unionfind.UnionFind, []ast.Atom, error) {
	bound, old, _, err := r.groupOutputFacts(rule, key)
	return bound, old, err
}

func (r *dredRun) groupOutputFacts(rule dredRule, key []ast.Constant) (bound unionfind.UnionFind, old []ast.Atom, hasProgramFact bool, err error) {
	bound, err = unionfind.UnifyTermsExtend(varsAsTerms(rule.agg.groupVars), constsAsTerms(key), unionfind.New())
	if err != nil {
		return bound, nil, false, fmt.Errorf("dred: %s: bind group: %w", rule.head.Symbol, err)
	}
	pattern, err := functional.EvalAtom(rule.clause.Head, bound)
	if err != nil {
		return bound, nil, false, fmt.Errorf("dred: %s: group pattern: %w", rule.head.Symbol, err)
	}
	err = r.k.store.GetFacts(pattern, func(a ast.Atom) error {
		if r.idx.programFacts.has(a) {
			hasProgramFact = true
			return nil
		}
		old = append(old, a)
		return nil
	})
	return bound, old, hasProgramFact, err
}

// replaceOutput swaps a group's output rows old for fresh in the store and
// records what changed.
func (r *dredRun) replaceOutput(old, fresh []ast.Atom, remover factstore.FactStoreWithRemove) {
	k := r.k
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

// factRow is the row a fact gives a single-atom body, by the engine's own row
// construction (seminaivebottomup.go): a variable that occurs twice must match
// equal arguments, or the fact is no row.
func factRow(premise ast.Atom, fact ast.Atom) (ast.ConstSubstList, bool) {
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
				return nil, false
			}
			continue
		}
		row = row.Extend(v, c)
	}
	return row, true
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
			if row, ok := factRow(premise, fact); ok {
				rows = append(rows, row)
			}
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

// scratchRecord remembers, for one store, the aggregate heads whose scratch
// relations are already emptied. An emptied relation still holds the buckets
// its rows had, so scanning it again on every pass costs what it once held
// (a 10,000-member group made each one-row change cost 0.7 ms for it alone).
// The record is hygiene, not correctness: the cone path empties a head's
// scratch itself before it re-runs the head's rule (dropScratchRelationsLocked),
// and a full evaluation builds a new store, which the record does not match.
type scratchRecord struct {
	store factstore.FactStore
	heads map[string]struct{}
}

// done reports whether head's scratch is recorded empty in store, and records
// it as about to be.
func (s *scratchRecord) done(store factstore.FactStore, head string) bool {
	// A store kind that cannot be compared (a struct holding maps) is never
	// recognised again: its scratch is scanned each pass, which is slow, not wrong.
	if t := reflect.TypeOf(store); s.heads == nil || t != reflect.TypeOf(s.store) || !t.Comparable() || s.store != store {
		s.store, s.heads = store, make(map[string]struct{})
	}
	if _, ok := s.heads[head]; ok {
		return true
	}
	s.heads[head] = struct{}{}
	return false
}

// forget drops head from the record: its scratch is about to be rebuilt.
func (s *scratchRecord) forget(head string) { delete(s.heads, head) }

// dropScratchOf removes the engine's scratch relations of head ("<head><n>__tmp"):
// once DRed maintains the head they would only go stale.
func (r *dredRun) dropScratchOf(head string, remover factstore.FactStoreWithRemove) {
	if r.k.scratchGone.done(r.k.store, head) {
		return
	}
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

// printAlikeGroupKeys reports when a changed group shares its printed key
// with a different atom: another changed key, a row the head still holds,
// or a body row an equality-keyed recompute would not read.
func (r *dredRun) printAlikeGroupKeys(rule dredRule, keys [][]ast.Constant) error {
	numeric := false
	for _, key := range keys {
		if keyHasNumber(key) {
			numeric = true
			break
		}
	}
	if !numeric {
		return nil
	}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keysPrintAlike(keys[i], keys[j]) {
				return printAlikeGroupError(rule, keys[i])
			}
		}
	}
	indexes := groupHeadIndexes(rule)
	if err := r.k.store.GetFacts(ast.NewQuery(rule.head), func(a ast.Atom) error {
		got, ok := constantsAt(a.Args, indexes)
		if !ok {
			return nil
		}
		for _, key := range keys {
			if keysPrintAlike(key, got) {
				return printAlikeGroupError(rule, key)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	slots, covered := groupKeySlots(rule)
	if !covered {
		// The key is computed, not read off one premise, so a twin of it can
		// sit in the body unseen. A numeric key then cannot be split safely.
		return printAlikeGroupError(rule, keys[0])
	}
	for _, slot := range slots {
		if err := r.k.store.GetFacts(ast.NewQuery(slot.pred), func(a ast.Atom) error {
			got, ok := keyFromFact(a, slot.argToKey, len(rule.agg.groupVars))
			if !ok {
				return nil
			}
			for _, key := range keys {
				if keysPrintAlike(key, got) {
					return printAlikeGroupError(rule, key)
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// groupSlot is one positive premise that carries every group variable.
type groupSlot struct {
	pred     ast.PredicateSym
	argToKey map[int]int // premise arg index -> group-key index
}

func groupKeySlots(rule dredRule) ([]groupSlot, bool) {
	want := make(map[ast.Variable]int, len(rule.agg.groupVars))
	for i, v := range rule.agg.groupVars {
		want[v] = i
	}
	var slots []groupSlot
	for _, pi := range rule.positives {
		atom := rule.clause.Premises[pi].(ast.Atom)
		argToKey := make(map[int]int)
		seen := make(map[int]bool, len(want))
		for ai, arg := range atom.Args {
			v, ok := arg.(ast.Variable)
			if !ok {
				continue
			}
			ki, isGroup := want[v]
			if !isGroup {
				continue
			}
			argToKey[ai] = ki
			seen[ki] = true
		}
		if len(seen) != len(want) {
			continue
		}
		slots = append(slots, groupSlot{pred: atom.Predicate, argToKey: argToKey})
	}
	return slots, len(slots) > 0
}

func groupHeadIndexes(rule dredRule) []int {
	indexes := make([]int, len(rule.agg.groupVars))
	for i, v := range rule.agg.groupVars {
		for j, arg := range rule.clause.Head.Args {
			hv, ok := arg.(ast.Variable)
			if ok && hv == v {
				indexes[i] = j
				break
			}
		}
	}
	return indexes
}

func constantsAt(args []ast.BaseTerm, indexes []int) ([]ast.Constant, bool) {
	got := make([]ast.Constant, len(indexes))
	for i, ix := range indexes {
		if ix >= len(args) {
			return nil, false
		}
		c, ok := args[ix].(ast.Constant)
		if !ok {
			return nil, false
		}
		got[i] = c
	}
	return got, true
}

func keyFromFact(a ast.Atom, argToKey map[int]int, n int) ([]ast.Constant, bool) {
	indexes := make([]int, n)
	filled := make([]bool, n)
	for ai, ki := range argToKey {
		indexes[ki] = ai
		filled[ki] = true
	}
	for _, ok := range filled {
		if !ok {
			return nil, false
		}
	}
	return constantsAt(a.Args, indexes)
}

func keyHasNumber(key []ast.Constant) bool {
	for _, c := range key {
		if constantHasNumber(c) {
			return true
		}
	}
	return false
}

// constantHasNumber reports a number or float64 anywhere in c. A pair, list,
// map, or struct prints with the element text, so fn:pair(1, /k) and
// fn:pair(1.0, /k) are one engine group and two Mangle atoms.
func constantHasNumber(c ast.Constant) bool {
	switch c.Type {
	case ast.NumberType, ast.Float64Type:
		return true
	case ast.PairShape:
		fst, snd, err := c.PairValue()
		return err == nil && (constantHasNumber(fst) || constantHasNumber(snd))
	case ast.ListShape:
		seq, err := c.ListSeq()
		if err != nil {
			return false
		}
		for elem := range seq {
			if constantHasNumber(elem) {
				return true
			}
		}
		return false
	case ast.MapShape:
		found := false
		_, _ = c.MapValues(func(k, v ast.Constant) error {
			found = found || constantHasNumber(k) || constantHasNumber(v)
			return nil
		}, func() error { return nil })
		return found
	case ast.StructShape:
		found := false
		_, _ = c.StructValues(func(k, v ast.Constant) error {
			found = found || constantHasNumber(k) || constantHasNumber(v)
			return nil
		}, func() error { return nil })
		return found
	default:
		return false
	}
}

// keysPrintAlike reports two keys the engine treats as one group and Mangle
// equality treats as two.
func keysPrintAlike(a, b []ast.Constant) bool {
	if len(a) != len(b) {
		return false
	}
	same := true
	for i := range a {
		if a[i].String() != b[i].String() {
			return false
		}
		if !a[i].Equals(b[i]) {
			same = false
		}
	}
	return !same
}

func printAlikeGroupError(rule dredRule, key []ast.Constant) error {
	parts := make([]string, len(key))
	for i, c := range key {
		parts[i] = c.String()
	}
	return fmt.Errorf("%w: %s: group key %s prints like a different atom", errDRedFallsBack, rule.head.Symbol, strings.Join(parts, ","))
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
