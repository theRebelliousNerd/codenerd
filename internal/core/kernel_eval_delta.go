package core

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/functional"
	"codeberg.org/TauCeti/mangle-go/unionfind"
)

// =============================================================================
// DELTA EVALUATION (DRed, insert half)
// =============================================================================
// The cone evaluator (kernel_eval_cone.go) re-derives a written predicate's
// whole cone: every row of every downstream predicate is removed and the
// engine's first round re-joins every rule over the full store. Over the
// production world (540k facts) one insert-only batch cost 8-16 s that way
// (measured 2026-11-06), because the world rules re-scan every file.
//
// A write that only inserts cannot make an old derived fact false through a
// rule that reads its inputs positively: such a rule is monotone, so its old
// output is still part of the new fixpoint and only derivations that use at
// least one new row are missing. Those are found semi-naively: for each rule
// and each positive premise over a predicate with new rows, evaluate the rule
// with that premise reading only the new rows (joined first, so the work is
// the delta, not the store) and every other premise reading the full store.
// New heads become the next round's delta, stratum by stratum, until nothing
// new is derived.
//
// What the argument does not cover is replaced instead, exactly as the cone
// path does it, after the insert pass so its inputs are already final:
//
//   - a rule that negates an affected predicate (a new row can withdraw a
//     derived one), aggregates (do-transforms: a count is replaced, not
//     added to), reads or writes time, keeps a lattice or reads a deferred
//     affected predicate (answered top-down, outside the materialized delta);
//   - every head with an external premise (volatile, re-derived every time);
//   - everything downstream of either.
//
// The replace set is downstream-closed, so no delta rule reads a predicate it
// replaces, and a replaced rule reads only predicates the delta pass has
// already brought to their fixpoint.
//
// A write set with any retraction takes the cone path: deletion needs the
// over-delete/re-derive half, which this file does not implement.

// errDeltaIneligible makes evaluate() try the cone path. It is not a fault.
var errDeltaIneligible = errors.New("delta evaluation not eligible")

// deltaRule is one rule's static shape for the delta pass.
type deltaRule struct {
	clause    ast.Clause
	head      ast.PredicateSym
	positives []int               // premise indexes that are non-builtin positive atoms
	negated   map[string]struct{} // predicate names it negates
	deferred  map[string]struct{} // deferred predicate names it reads
	intrinsic bool                // never delta-safe: transform, time, lattice or deferred head
}

// deltaIndex is the delta pass's view of one program, built once per
// coneIndex on first use and shared by every kernel compiled from it.
type deltaIndex struct {
	rules         []deltaRule
	byHead        map[string][]int // head name -> indexes into rules
	predToStratum map[ast.PredicateSym]int
	topDown       map[ast.PredicateSym][]ast.Clause // rules for deferred premises
	volatileDown  map[string]struct{}               // volatile heads and everything downstream

	mu    sync.Mutex
	plans map[string]*deltaPlan
}

// deltaPlan is the immutable split of one written-predicate set.
type deltaPlan struct {
	insert  map[string]struct{} // derived predicates brought up to date by delta
	replace map[string]struct{} // derived predicates re-derived as a cone
	strata  [][]int             // delta rules grouped by head stratum, in order
}

func (c *coneIndex) deltaIndex(decls map[ast.PredicateSym]*ast.Decl) *deltaIndex {
	c.deltaOnce.Do(func() { c.delta = buildDeltaIndex(c, decls) })
	return c.delta
}

func buildDeltaIndex(c *coneIndex, decls map[ast.PredicateSym]*ast.Decl) *deltaIndex {
	idx := &deltaIndex{
		byHead:        make(map[string][]int),
		predToStratum: make(map[ast.PredicateSym]int),
		topDown:       make(map[ast.PredicateSym][]ast.Clause),
		volatileDown:  make(map[string]struct{}),
		plans:         make(map[string]*deltaPlan),
	}
	for i, stratum := range c.strata {
		for sym := range stratum {
			idx.predToStratum[sym] = i
		}
	}
	isDeferred := func(sym ast.PredicateSym) bool {
		d := decls[sym]
		return d != nil && d.DeferredPredicate()
	}
	for _, clause := range c.rules {
		if isDeferred(clause.Head.Predicate) {
			idx.topDown[clause.Head.Predicate] = append(idx.topDown[clause.Head.Predicate], clause)
		}
	}
	for _, clause := range c.rules {
		r := deltaRule{clause: clause, head: clause.Head.Predicate, negated: map[string]struct{}{}, deferred: map[string]struct{}{}}
		if d := decls[r.head]; d != nil {
			if d.DeferredPredicate() || len(d.FunDeps()) > 0 {
				r.intrinsic = true
			}
		}
		if clause.HeadTime != nil || (clause.Transform != nil && !clause.Transform.IsLetTransform()) {
			r.intrinsic = true
		}
		for i, premise := range clause.Premises {
			switch p := premise.(type) {
			case ast.Atom:
				if p.Predicate.IsBuiltin() {
					continue
				}
				if isDeferred(p.Predicate) {
					r.deferred[p.Predicate.Symbol] = struct{}{}
					continue
				}
				r.positives = append(r.positives, i)
			case ast.NegAtom:
				r.negated[p.Atom.Predicate.Symbol] = struct{}{}
			case ast.TemporalAtom, ast.TemporalLiteral:
				r.intrinsic = true
			}
		}
		idx.byHead[r.head.Symbol] = append(idx.byHead[r.head.Symbol], len(idx.rules))
		idx.rules = append(idx.rules, r)
	}
	for _, head := range c.volatile {
		idx.volatileDown[head] = struct{}{}
		for pred := range c.downstream(head) {
			idx.volatileDown[pred] = struct{}{}
		}
	}
	return idx
}

// plan splits the cone of a written set into its delta and replace parts.
func (d *deltaIndex) plan(c *coneIndex, written map[string]struct{}) *deltaPlan {
	key := planKey(written)
	d.mu.Lock()
	if p, ok := d.plans[key]; ok {
		d.mu.Unlock()
		return p
	}
	d.mu.Unlock()

	affected := make(map[string]struct{})
	for pred := range written {
		if _, derived := c.heads[pred]; derived {
			affected[pred] = struct{}{}
		}
		for head := range c.downstream(pred) {
			affected[head] = struct{}{}
		}
	}
	reads := func(set map[string]struct{}, name string) bool {
		if _, ok := set[name]; ok {
			return true
		}
		_, ok := written[name]
		return ok
	}
	replace := make(map[string]struct{}, len(d.volatileDown))
	for pred := range d.volatileDown {
		replace[pred] = struct{}{}
	}
	for head := range affected {
		for _, ri := range d.byHead[head] {
			r := d.rules[ri]
			unsafe := r.intrinsic
			for name := range r.negated {
				unsafe = unsafe || reads(affected, name)
			}
			for name := range r.deferred {
				unsafe = unsafe || reads(affected, name)
			}
			if unsafe {
				replace[head] = struct{}{}
				for down := range c.downstream(head) {
					replace[down] = struct{}{}
				}
				break
			}
		}
	}
	p := &deltaPlan{insert: make(map[string]struct{}), replace: replace}
	byStratum := make(map[int][]int)
	for head := range affected {
		if _, r := replace[head]; r {
			continue
		}
		p.insert[head] = struct{}{}
		for _, ri := range d.byHead[head] {
			s := d.predToStratum[d.rules[ri].head]
			byStratum[s] = append(byStratum[s], ri)
		}
	}
	order := make([]int, 0, len(byStratum))
	for s := range byStratum {
		order = append(order, s)
	}
	sort.Ints(order)
	for _, s := range order {
		rules := byStratum[s]
		sort.Ints(rules)
		p.strata = append(p.strata, rules)
	}

	d.mu.Lock()
	if len(d.plans) >= conePlanCacheMax {
		d.plans = make(map[string]*deltaPlan, 16)
	}
	d.plans[key] = p
	d.mu.Unlock()
	return p
}

// deltaIneligibilityLocked says why the next evaluate() cannot be a delta
// evaluation, or "" when it can. Caller holds k.mu.
func (k *RealKernel) deltaIneligibilityLocked() string {
	if why := k.coneIneligibilityLocked(); why != "" {
		return why
	}
	for pred := range k.dirtyPreds {
		if len(k.factIndex.delta.removed[pred]) > 0 {
			return "write set retracts"
		}
	}
	return ""
}

// deltaStores holds the new rows per predicate symbol.
type deltaStores map[ast.PredicateSym]*factstore.MultiIndexedArrayInMemoryStore

func (d deltaStores) add(atom ast.Atom) {
	s := d[atom.Predicate]
	if s == nil {
		s = factstore.NewMultiIndexedArrayInMemoryStore()
		d[atom.Predicate] = s
	}
	s.Add(atom)
}

// evaluateDeltaLocked brings the store to the new fixpoint of an insert-only
// write set. It returns errDeltaIneligible when the cone path must run
// instead; any other error may leave rows added, and the caller runs the full
// path, which builds a fresh store.
func (k *RealKernel) evaluateDeltaLocked(stats *EvaluationStats) error {
	if k.deltaIneligibilityLocked() != "" {
		return errDeltaIneligible
	}
	didx := k.cone.deltaIndex(k.programInfo.Decls)
	plan := didx.plan(k.cone, k.dirtyPreds)
	if len(plan.insert) == 0 && len(plan.replace) >= len(k.cone.closure(k.dirtyPreds)) {
		return errDeltaIneligible // nothing to gain over the cone
	}
	k.ownStoreLocked()
	timer := logging.StartTimer(logging.CategoryKernel, "evaluate.delta")
	defer timer.Stop()
	started := time.Now()

	// Seed: the inserted EDB rows not already in the store.
	all := make(deltaStores)
	seeded := 0
	written := make([]string, 0, len(k.dirtyPreds))
	for pred := range k.dirtyPreds {
		written = append(written, pred)
		for key := range k.factIndex.delta.added[pred] {
			f, held := k.factIndex.bucket(pred)[key]
			if !held {
				continue
			}
			atom, err := k.edbAtomLocked(pred, key, f)
			if err != nil {
				return fmt.Errorf("delta: seed %s: %w", pred, err)
			}
			if k.store.Add(atom) {
				all.add(atom)
				seeded++
			}
		}
	}
	sort.Strings(written)

	limit := k.effectiveDerivedFactLimitLocked()
	ctx := engine.QueryContext{PredToRules: didx.topDown, PredToDecl: k.programInfo.Decls, Store: k.store}
	derived, variants := 0, 0
	for _, group := range plan.strata {
		// Round one joins every delta so far; later rounds only this
		// stratum's newest rows (the recursion inside it).
		frontier := make(deltaStores, len(all))
		for sym, s := range all {
			frontier[sym] = s
		}
		for len(frontier) > 0 {
			next := make(deltaStores)
			for _, ri := range group {
				r := didx.rules[ri]
				for _, pi := range r.positives {
					premise := r.clause.Premises[pi].(ast.Atom)
					delta := frontier[premise.Predicate]
					if delta == nil {
						continue
					}
					variants++
					heads, err := evalDeltaVariant(ctx, r.clause, pi, delta, limit)
					if err != nil {
						return fmt.Errorf("delta: %s: %w", r.head.Symbol, err)
					}
					for _, head := range heads {
						if k.store.Add(head) {
							next.add(head)
							all.add(head)
							derived++
						}
					}
					if limit > 0 && derived > limit {
						return fmt.Errorf("delta: derived fact limit %d exceeded", limit)
					}
				}
			}
			frontier = next
		}
	}
	deltaDuration := time.Since(started)

	var work coneWork
	var replacedRules, replacedStrata int
	if len(plan.replace) > 0 {
		cp := k.cone.plan(plan.replace)
		var err error
		if work, err = k.rederiveConeLocked(cp); err != nil {
			return err
		}
		replacedRules, replacedStrata = len(cp.rules), len(cp.strata)
	}
	*stats = EvaluationStats{
		InputFacts: stats.InputFacts, Mode: "delta", StoreKind: storeKind(k.store),
		PrepareDuration: work.prepare, EngineDuration: work.engine, DeltaDuration: deltaDuration,
		Touched: len(plan.insert) + len(plan.replace), RemovedFacts: work.removed, RestoredFacts: work.restored + seeded,
		ListedFacts: work.listed, Rules: replacedRules, Strata: replacedStrata,
		DeltaVariants: variants, DeltaFacts: derived,
	}
	logging.KernelDebug("evaluate.delta: written=%v insert=%d replace=%d seeded=%d derived=%d variants=%d",
		written, len(plan.insert), len(plan.replace), seeded, derived, variants)
	return nil
}

// evalDeltaVariant evaluates clause with premise pi reading only delta and
// every other premise reading ctx.Store, and returns the heads it derives.
// The delta premise is joined first when that cannot unbind a later premise:
// a positive atom over plain terms binds variables and needs none.
func evalDeltaVariant(ctx engine.QueryContext, clause ast.Clause, pi int, delta factstore.ReadOnlyFactStore, limit int) ([]ast.Atom, error) {
	order := make([]int, 0, len(clause.Premises))
	if plainAtom(clause.Premises[pi].(ast.Atom)) {
		order = append(order, pi)
		for i := range clause.Premises {
			if i != pi {
				order = append(order, i)
			}
		}
	} else {
		for i := range clause.Premises {
			order = append(order, i)
		}
	}
	deltaCtx := ctx
	deltaCtx.Store = delta
	solutions := []unionfind.UnionFind{unionfind.New()}
	for _, i := range order {
		use := ctx
		if i == pi {
			use = deltaCtx
		}
		var out []unionfind.UnionFind
		for _, s := range solutions {
			step, err := use.EvalPremise(clause.Premises[i], s)
			if err != nil {
				return nil, err
			}
			out = append(out, step...)
			if limit > 0 && len(out) > limit {
				return nil, fmt.Errorf("solution limit %d exceeded", limit)
			}
		}
		solutions = out
		if len(solutions) == 0 {
			return nil, nil
		}
	}
	heads := make([]ast.Atom, 0, len(solutions))
	for _, sol := range solutions {
		head, err := functional.EvalAtom(clause.Head, sol)
		if err != nil {
			return nil, err
		}
		if clause.Transform == nil {
			heads = append(heads, head)
			continue
		}
		if err := engine.EvalTransformWithInputFacts(head, *clause.Transform, []ast.ConstSubstList{sol.AsConstSubstList()}, nil,
			func(out ast.Atom, _ engine.TransformKind, _ []ast.Constant, _ []ast.Atom) bool {
				heads = append(heads, out)
				return true
			}); err != nil {
			return nil, err
		}
	}
	return heads, nil
}

// plainAtom reports whether every argument is a variable or a constant.
func plainAtom(a ast.Atom) bool {
	for _, arg := range a.Args {
		switch arg.(type) {
		case ast.Variable, ast.Constant:
		default:
			return false
		}
	}
	return true
}

// rederiveConeLocked replaces the rows of plan's touched predicates and runs
// the engine over plan's rules: the cone evaluator's work, shared with the
// replace part of a delta evaluation.
func (k *RealKernel) rederiveConeLocked(plan *conePlan) (w coneWork, err error) {
	started := time.Now()
	w.removed, w.restored, w.listed, err = k.prepareConeStoreLocked(plan)
	if err != nil {
		return w, err
	}
	w.prepare = time.Since(started)
	// Run the engine even when the cone holds no rule: it is what re-adds a
	// program fact of a written predicate, removed above with the rest.
	sub := &analysis.ProgramInfo{
		EdbPredicates:    k.programInfo.EdbPredicates,
		IdbPredicates:    k.programInfo.IdbPredicates,
		InitialFacts:     k.programInfo.InitialFacts,
		InitialFactTimes: k.programInfo.InitialFactTimes,
		Rules:            plan.rules,
		Decls:            k.programInfo.Decls,
	}
	opts := append([]engine.EvalOption{engine.WithCreatedFactLimit(k.effectiveDerivedFactLimitLocked())},
		k.externalPredicateOptionsLocked()...)
	engineStarted := time.Now()
	if _, err := engine.EvalStratifiedProgramWithStats(sub, plan.strata, plan.predToStratum, k.store, opts...); err != nil {
		return w, fmt.Errorf("cone: fixpoint: %w", err)
	}
	w.engine = time.Since(engineStarted)
	return w, nil
}

// coneWork is what one cone re-derivation did and cost.
type coneWork struct {
	removed, restored, listed int
	prepare, engine           time.Duration
}
