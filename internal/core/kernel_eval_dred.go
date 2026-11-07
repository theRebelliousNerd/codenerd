package core

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
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
// DRED: INCREMENTAL MAINTENANCE BY DELETE AND RE-DERIVE
// =============================================================================
// The cone evaluator (kernel_eval_cone.go) re-derives a written predicate's
// whole cone: every row of every downstream predicate is removed and the
// engine's first round re-joins every rule over the full store. Over the
// production world (540k facts) one write cost 4-16 s that way (measured
// 2026-11-06), because the world rules re-scan every file.
//
// DRed (Gupta, Mumick and Subrahmanian, 1993) maintains the derived facts
// instead, stratum by stratum, from the store's actual changes below:
//
//  1. Over-delete. A fact with any derivation that used a removed row is
//     deleted: for each rule and each premise over a changed predicate, the
//     variant whose premise reads only the change, every other premise the
//     OLD state. A positive premise reads the removed rows; a negated one the
//     inserted rows (a new row makes "!p" false). The old state is the store
//     with the lower strata's inserts hidden and removals shown (dredOldView),
//     so a derivation that used two removed rows is still found. Recursive
//     strata iterate over their own over-deleted rows.
//  2. Re-derive. An over-deleted fact that still has support in the NEW state
//     comes back: a rule body with the head bound, a program fact, or an EDB
//     row of a predicate both asserted and derived. Iterated to a fixpoint, so
//     support through another re-derived fact counts.
//  3. Insert. Semi-naively: a positive premise reads the inserted rows, a
//     negated one the removed rows (a gone row makes "!p" true), every other
//     premise the new state; recursive strata iterate over their own inserts.
//
// The stratum's net change (deleted and not back, inserted and not there
// before) is what the next stratum reads, so each variant joins the change
// first and the work is the change, not the store.
//
// An aggregation is maintained by group: only the groups whose body rows
// changed are recomputed (kernel_eval_dred_agg.go).
//
// Not maintained, but replaced after the pass exactly as the cone path does
// it, are temporal rules, lattices, deferred heads, rules reading a deferred
// affected predicate, aggregations of a shape the group recompute does not
// cover, heads fed by an external predicate (re-derived every time), and
// everything downstream of any of them. That set is downstream-closed: no maintained
// rule reads a replaced predicate, and a replaced rule reads only predicates
// the pass has already brought to their fixpoint.

// errDRedIneligible makes evaluate() try the cone path. It is not a fault.
var errDRedIneligible = errors.New("dred evaluation not eligible")

// errChangeTooLarge makes evaluate() run the full fixpoint without trying the
// cone: the change is most of the store. It is not a fault.
var errChangeTooLarge = errors.New("change is most of the store")

// dredMaxChangeRatio is the share of the EDB a change may be and still be
// maintained. DRed's cost follows the rows changed and the full fixpoint's the
// facts held; over the production policy they cross near 0.65 (measured
// 2026-11-06: 48K rows into a 50K-fact EDB, DRed 1.25 s vs full 0.66 s; into a
// 74K one, 0.81 s vs 0.87 s; 144K into 290K, 2.57 s vs 3.74 s). Above it the
// cone is worse still: it re-derives every downstream predicate, which was
// 15.8 s for boot's 540K-row world load, where a full fixpoint is a few.
const dredMaxChangeRatio = 0.6

// dredCutoverMinRows is the change size below which the ratio does not apply:
// a few thousand rows take DRed and the full fixpoint alike tens of
// milliseconds (6K rows over an empty world: 92 ms vs 90 ms), and DRed keeps
// the store.
const dredCutoverMinRows = 4096

// dredRule is one rule's static shape.
type dredRule struct {
	clause    ast.Clause
	head      ast.PredicateSym
	positives []int               // premise indexes: non-builtin positive atoms over materialized predicates
	negatives []int               // premise indexes: negated non-builtin atoms
	deferred  map[string]struct{} // deferred predicate names it reads (answered top-down)
	letVars   map[ast.Variable]struct{}
	agg       *dredAggregate // a maintained do-transform (kernel_eval_dred_agg.go)
	intrinsic bool           // never maintained: time, lattice, deferred head, unmaintainable aggregation
}

// dredIndex is DRed's view of one program, built once per coneIndex on first
// use and shared by every kernel compiled from it.
type dredIndex struct {
	rules         []dredRule
	byHead        map[string][]int           // head name -> rule indexes
	bySym         map[ast.PredicateSym][]int // head symbol -> rule indexes
	predToStratum map[ast.PredicateSym]int
	topDown       map[ast.PredicateSym][]ast.Clause // rules answering deferred premises
	volatileDown  map[string]struct{}               // external-fed heads and their downstream
	programFacts  deltaStores                       // the program's own facts, by symbol

	mu    sync.Mutex
	plans map[string]*dredPlan
}

// dredPlan is the immutable split of one written-predicate set.
type dredPlan struct {
	maintain map[string]struct{} // derived predicates DRed maintains
	replace  map[string]struct{} // derived predicates re-derived as a cone
	strata   [][]int             // maintained rules grouped by head stratum, in order
	heads    []map[ast.PredicateSym]struct{}
	aggHeads []ast.PredicateSym // heads of the maintained aggregations
}

func (c *coneIndex) dredIndex(info *analysis.ProgramInfo) *dredIndex {
	c.dredOnce.Do(func() { c.dred = buildDRedIndex(c, info) })
	return c.dred
}

func buildDRedIndex(c *coneIndex, info *analysis.ProgramInfo) *dredIndex {
	decls := info.Decls
	idx := &dredIndex{
		byHead:        make(map[string][]int),
		bySym:         make(map[ast.PredicateSym][]int),
		predToStratum: make(map[ast.PredicateSym]int),
		topDown:       make(map[ast.PredicateSym][]ast.Clause),
		volatileDown:  make(map[string]struct{}),
		programFacts:  make(deltaStores),
		plans:         make(map[string]*dredPlan),
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
	for _, fact := range info.InitialFacts {
		if atom, err := functional.EvalAtom(fact, nil); err == nil {
			idx.programFacts.add(atom)
		}
	}
	for _, clause := range c.rules {
		r := dredRule{clause: clause, head: clause.Head.Predicate, deferred: map[string]struct{}{}}
		if d := decls[r.head]; d != nil && (d.DeferredPredicate() || len(d.FunDeps()) > 0) {
			r.intrinsic = true
		}
		if clause.HeadTime != nil {
			r.intrinsic = true
		}
		if t := clause.Transform; t != nil {
			r.letVars = make(map[ast.Variable]struct{})
			for ; t != nil; t = t.Next {
				for _, stmt := range t.Statements {
					if stmt.Var != nil {
						r.letVars[*stmt.Var] = struct{}{}
					}
				}
			}
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
				if !p.Atom.Predicate.IsBuiltin() {
					r.negatives = append(r.negatives, i)
				}
			case ast.TemporalAtom, ast.TemporalLiteral:
				r.intrinsic = true
			}
		}
		ri := len(idx.rules)
		idx.byHead[r.head.Symbol] = append(idx.byHead[r.head.Symbol], ri)
		idx.bySym[r.head] = append(idx.bySym[r.head], ri)
		idx.rules = append(idx.rules, r)
	}
	for i := range idx.rules {
		if t := idx.rules[i].clause.Transform; t != nil && !t.IsLetTransform() {
			idx.rules[i].agg = idx.aggregateShape(idx.rules[i])
			idx.rules[i].intrinsic = idx.rules[i].intrinsic || idx.rules[i].agg == nil
		}
	}
	for _, head := range c.volatile {
		idx.volatileDown[head] = struct{}{}
		for pred := range c.downstream(head) {
			idx.volatileDown[pred] = struct{}{}
		}
	}
	return idx
}

// plan splits the cone of a written set into its maintained and replaced parts.
func (d *dredIndex) plan(c *coneIndex, written map[string]struct{}) *dredPlan {
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
	replace := make(map[string]struct{}, len(d.volatileDown))
	for pred := range d.volatileDown {
		replace[pred] = struct{}{}
	}
	for head := range affected {
		for _, ri := range d.byHead[head] {
			r := d.rules[ri]
			unsafe := r.intrinsic
			for name := range r.deferred {
				_, a := affected[name]
				_, w := written[name]
				unsafe = unsafe || a || w
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
	p := &dredPlan{maintain: make(map[string]struct{}), replace: replace}
	byStratum := make(map[int][]int)
	headsOf := make(map[int]map[ast.PredicateSym]struct{})
	for head := range affected {
		if _, r := replace[head]; r {
			continue
		}
		p.maintain[head] = struct{}{}
		for _, ri := range d.byHead[head] {
			if d.rules[ri].agg != nil {
				p.aggHeads = append(p.aggHeads, d.rules[ri].head)
			}
			sym := d.rules[ri].head
			s := d.predToStratum[sym]
			byStratum[s] = append(byStratum[s], ri)
			if headsOf[s] == nil {
				headsOf[s] = make(map[ast.PredicateSym]struct{})
			}
			headsOf[s][sym] = struct{}{}
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
		p.heads = append(p.heads, headsOf[s])
	}

	d.mu.Lock()
	if len(d.plans) >= conePlanCacheMax {
		d.plans = make(map[string]*dredPlan, 16)
	}
	d.plans[key] = p
	d.mu.Unlock()
	return p
}

// deltaStores holds rows per predicate symbol.
type deltaStores map[ast.PredicateSym]*factstore.MultiIndexedArrayInMemoryStore

func (d deltaStores) add(atom ast.Atom) bool {
	s := d[atom.Predicate]
	if s == nil {
		s = factstore.NewMultiIndexedArrayInMemoryStore()
		d[atom.Predicate] = s
	}
	return s.Add(atom)
}

func (d deltaStores) has(atom ast.Atom) bool {
	s := d[atom.Predicate]
	return s != nil && s.Contains(atom)
}

// each calls fn for every row, in no particular order.
func (d deltaStores) each(fn func(ast.Atom)) {
	for _, s := range d {
		_ = factstore.GetAllFacts(s, func(a ast.Atom) error {
			fn(a)
			return nil
		})
	}
}

// dredOldView reads the store as it was before this evaluation for every
// predicate whose change is recorded: inserted rows hidden, removed rows shown.
type dredOldView struct {
	cur         factstore.FactStore
	plus, minus deltaStores
}

func (v dredOldView) GetFacts(q ast.Atom, cb func(ast.Atom) error) error {
	plus, minus := v.plus[q.Predicate], v.minus[q.Predicate]
	if plus == nil && minus == nil {
		return v.cur.GetFacts(q, cb)
	}
	if err := v.cur.GetFacts(q, func(a ast.Atom) error {
		if plus != nil && plus.Contains(a) {
			return nil
		}
		return cb(a)
	}); err != nil {
		return err
	}
	if minus != nil {
		return minus.GetFacts(q, cb)
	}
	return nil
}

func (v dredOldView) Contains(a ast.Atom) bool {
	if m := v.minus[a.Predicate]; m != nil && m.Contains(a) {
		return true
	}
	if p := v.plus[a.Predicate]; p != nil && p.Contains(a) {
		return false
	}
	return v.cur.Contains(a)
}

func (v dredOldView) ListPredicates() []ast.PredicateSym { return v.cur.ListPredicates() }
func (v dredOldView) EstimateFactCount() int             { return v.cur.EstimateFactCount() }

// dredRun is one DRed evaluation's mutable state.
type dredRun struct {
	k           *RealKernel
	idx         *dredIndex
	plus, minus deltaStores // net changes of every predicate processed so far
	// EDB writes of predicates both asserted and derived, applied with their
	// stratum (their old rows must stay visible until it over-deletes).
	edbPlus, edbMinus deltaStores
	edbHeld           map[string]deltaStores // pred name -> EDB rows as atoms, built on demand
	scratchDropped    map[string]struct{}    // aggregate heads whose scratch relations are gone
	profile           dredProfile            // where the time went (kernel_eval_dred_profile.go)
	limit             int
	edbAdded          int // EDB rows the pass added to the store
	edbRemoved        int // EDB rows the pass removed from it
	overDeleted       int
	rederived         int
	inserted          int
	variants          atomic.Int64
}

// evaluateDRedLocked brings the store to the new fixpoint of the write set.
// It returns errDRedIneligible when the cone path must run instead; any other
// error leaves the store part-way, and the caller runs the full path.
func (k *RealKernel) evaluateDRedLocked(stats *EvaluationStats) error {
	if k.coneIneligibilityLocked() != "" {
		return errDRedIneligible
	}
	changedRows := k.factIndex.delta.rows()
	if changedRows >= dredCutoverMinRows && float64(changedRows) > dredMaxChangeRatio*float64(len(k.facts)) {
		return fmt.Errorf("%w: %d changed rows, %d held", errChangeTooLarge, changedRows, len(k.facts))
	}
	idx := k.cone.dredIndex(k.programInfo)
	plan := idx.plan(k.cone, k.dirtyPreds)
	if len(plan.maintain) == 0 {
		return errDRedIneligible // nothing to maintain: the cone is the same work
	}
	for _, sym := range plan.aggHeads {
		if k.factIndex.aritiesOf(sym.Symbol) != nil {
			// An asserted row beside a maintained aggregate: the group
			// recompute would not know it. Rare; the cone handles it.
			return errDRedIneligible
		}
	}
	k.ownStoreLocked()
	timer := logging.StartTimer(logging.CategoryKernel, "evaluate.dred")
	defer timer.Stop()
	started := time.Now()

	run := &dredRun{
		k: k, idx: idx, plus: make(deltaStores), minus: make(deltaStores),
		edbPlus: make(deltaStores), edbMinus: make(deltaStores), edbHeld: make(map[string]deltaStores),
		limit: k.effectiveDerivedFactLimitLocked(),
	}
	written := make([]string, 0, len(k.dirtyPreds))
	for pred := range k.dirtyPreds {
		written = append(written, pred)
		if _, derived := k.cone.heads[pred]; derived {
			if _, replaced := plan.replace[pred]; !replaced {
				if err := run.stageDerivedEDB(pred); err != nil {
					return err
				}
			}
			continue
		}
		gone, came, err := k.applyExtensionalDeltaLocked(pred)
		if err != nil {
			return fmt.Errorf("dred: %s: %w", pred, err)
		}
		kept := gone[:0]
		for _, a := range gone {
			// A program fact holds whatever the EDB does: retracting an
			// asserted copy of it removes nothing. (The cone path got this
			// from its engine run, which re-adds every program fact.)
			if idx.programFacts.has(a) {
				k.store.Add(a)
				continue
			}
			kept = append(kept, a)
		}
		gone = kept
		for _, a := range gone {
			run.minus.add(a)
		}
		for _, a := range came {
			run.plus.add(a)
		}
		run.edbRemoved += len(gone)
		run.edbAdded += len(came)
	}
	sort.Strings(written)

	for i, group := range plan.strata {
		if err := run.stratum(group, plan.heads[i]); err != nil {
			return err
		}
	}
	dredDuration := time.Since(started)

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
		InputFacts: stats.InputFacts, Mode: "dred", StoreKind: storeKind(k.store),
		PrepareDuration: work.prepare, EngineDuration: work.engine, DeltaDuration: dredDuration,
		Touched: len(plan.maintain) + len(plan.replace), RemovedFacts: work.removed + run.edbRemoved, RestoredFacts: work.restored + run.edbAdded,
		ListedFacts: work.listed, Rules: replacedRules, Strata: replacedStrata,
		DeltaVariants: int(run.variants.Load()), DeltaFacts: run.inserted, OverDeleted: run.overDeleted, Rederived: run.rederived,
		ChangedRows: changedRows, Profile: run.profile.summary(5),
	}
	logging.KernelDebug("evaluate.dred: written=%v maintain=%d replace=%d inserted=%d overdeleted=%d rederived=%d variants=%d",
		written, len(plan.maintain), len(plan.replace), run.inserted, run.overDeleted, run.rederived, run.variants.Load())
	return nil
}

// stageDerivedEDB records the EDB writes of pred, a predicate rules also
// derive, for its stratum to apply.
func (r *dredRun) stageDerivedEDB(pred string) error {
	k := r.k
	x := k.factIndex
	bucket := x.bucket(pred)
	var added, removed []ast.Atom
	for key := range x.delta.added[pred] {
		if f, held := bucket[key]; held {
			atom, err := k.edbAtomLocked(pred, key, f)
			if err != nil {
				return fmt.Errorf("dred: %s: %w", pred, err)
			}
			added = append(added, atom)
		}
	}
	for _, row := range x.delta.removed[pred] {
		atom := row.atom.atom
		if row.atom.program != k.programInfo || k.programInfo == nil {
			var err error
			if atom, err = k.factToAtomLocked(row.fact); err != nil {
				return fmt.Errorf("dred: %s: %w", pred, err)
			}
		}
		removed = append(removed, atom)
	}
	removed, added, _ = netChange(removed, added)
	for _, a := range added {
		r.edbPlus.add(a)
	}
	for _, a := range removed {
		r.edbMinus.add(a)
	}
	return nil
}

func (r *dredRun) ctx(store factstore.ReadOnlyFactStore) engine.QueryContext {
	return engine.QueryContext{PredToRules: r.idx.topDown, PredToDecl: r.k.programInfo.Decls, Store: store}
}

func (r *dredRun) checkLimit() error {
	if r.limit > 0 && r.overDeleted+r.inserted > r.limit {
		return fmt.Errorf("dred: derived fact limit %d exceeded", r.limit)
	}
	return nil
}

// stratum maintains the predicates heads, derived by the rules of group.
func (r *dredRun) stratum(group []int, heads map[ast.PredicateSym]struct{}) error {
	k := r.k
	inStratum := func(sym ast.PredicateSym) bool { _, ok := heads[sym]; return ok }
	oldCtx := r.ctx(dredOldView{cur: k.store, plus: r.plus, minus: r.minus})

	// Aggregates read only lower strata and nothing in this one reads them
	// (a layer's components are independent), so they go first, alone.
	plain := group[:0:0]
	for _, ri := range group {
		if rule := r.idx.rules[ri]; rule.agg != nil {
			started := time.Now()
			if err := r.aggregate(rule, oldCtx); err != nil {
				return err
			}
			d := time.Since(started)
			r.profile.addPhase(phaseAggregate, d)
			r.profile.charge(rule.head.Symbol, d)
			continue
		}
		plain = append(plain, ri)
	}
	group = plain

	// 1. Over-delete, against the old state.
	phaseStarted := time.Now()
	over := make(deltaStores)
	frontier := make(deltaStores)
	mark := func(atoms []ast.Atom) {
		for _, h := range atoms {
			if k.store.Contains(h) && over.add(h) {
				frontier.add(h)
				r.overDeleted++
			}
		}
	}
	for sym := range heads {
		if s := r.edbMinus[sym]; s != nil {
			var gone []ast.Atom
			_ = factstore.GetAllFacts(s, func(a ast.Atom) error { gone = append(gone, a); return nil })
			mark(gone)
		}
	}
	if err := r.runVariants(r.seedVariants(group, oldCtx, r.minus, r.plus, inStratum), mark); err != nil {
		return err
	}
	for len(frontier) > 0 {
		cur := frontier
		frontier = make(deltaStores)
		if err := r.runVariants(r.recursiveVariants(group, oldCtx, cur), mark); err != nil {
			return err
		}
		if err := r.checkLimit(); err != nil {
			return err
		}
	}
	remover := k.store.(factstore.FactStoreWithRemove)
	var pending []ast.Atom
	over.each(func(a ast.Atom) {
		remover.Remove(a)
		pending = append(pending, a)
	})
	r.profile.addPhase(phaseOverDelete, time.Since(phaseStarted))

	// 2. Re-derive what still has support in the new state.
	phaseStarted = time.Now()
	newCtx := r.ctx(k.store)
	for changed := true; changed && len(pending) > 0; {
		changed = false
		rest := pending[:0]
		for _, a := range pending {
			ok, err := r.supported(newCtx, a)
			if err != nil {
				return err
			}
			if ok {
				k.store.Add(a)
				r.rederived++
				changed = true
			} else {
				rest = append(rest, a)
			}
		}
		pending = rest
	}

	r.profile.addPhase(phaseRederive, time.Since(phaseStarted))

	// 3. Insert, against the new state.
	phaseStarted = time.Now()
	ins := make(deltaStores)
	frontier = make(deltaStores)
	add := func(atoms []ast.Atom) {
		for _, h := range atoms {
			if k.store.Add(h) {
				ins.add(h)
				frontier.add(h)
			}
		}
	}
	for sym := range heads {
		if s := r.edbPlus[sym]; s != nil {
			var came []ast.Atom
			_ = factstore.GetAllFacts(s, func(a ast.Atom) error { came = append(came, a); return nil })
			add(came)
		}
	}
	if err := r.runVariants(r.seedVariants(group, newCtx, r.plus, r.minus, inStratum), add); err != nil {
		return err
	}
	for len(frontier) > 0 {
		cur := frontier
		frontier = make(deltaStores)
		if err := r.runVariants(r.recursiveVariants(group, newCtx, cur), add); err != nil {
			return err
		}
		if err := r.checkLimit(); err != nil {
			return err
		}
	}

	r.profile.addPhase(phaseInsert, time.Since(phaseStarted))

	// The stratum's net change is what the strata above read.
	over.each(func(a ast.Atom) {
		if !k.store.Contains(a) {
			r.minus.add(a)
		}
	})
	ins.each(func(a ast.Atom) {
		if !over.has(a) {
			r.plus.add(a)
			r.inserted++
		}
	})
	return r.checkLimit()
}

// supported reports whether a, just over-deleted, still holds in the new
// state: as a program fact, an EDB row, or the head of a rule whose body holds.
func (r *dredRun) supported(ctx engine.QueryContext, a ast.Atom) (bool, error) {
	if r.idx.programFacts.has(a) {
		return true, nil
	}
	if r.k.factIndex.aritiesOf(a.Predicate.Symbol) != nil {
		held, err := r.heldEDB(a.Predicate.Symbol)
		if err != nil {
			return false, err
		}
		if held.has(a) {
			return true, nil
		}
	}
	for _, ri := range r.idx.bySym[a.Predicate] {
		started := time.Now()
		ok, err := r.derives(ctx, r.idx.rules[ri], a)
		r.profile.charge(a.Predicate.Symbol, time.Since(started))
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// heldEDB is every EDB row of pred as a store atom, built once per run.
func (r *dredRun) heldEDB(pred string) (deltaStores, error) {
	if held, ok := r.edbHeld[pred]; ok {
		return held, nil
	}
	atoms, err := r.k.heldAtomsLocked(pred)
	if err != nil {
		return nil, err
	}
	held := make(deltaStores)
	for _, a := range atoms {
		held.add(a)
	}
	r.edbHeld[pred] = held
	return held, nil
}

// derives reports whether rule derives a in ctx's state. The head is bound to
// a first (except variables a let defines), so the body is a lookup, not a scan.
func (r *dredRun) derives(ctx engine.QueryContext, rule dredRule, a ast.Atom) (bool, error) {
	var xs, ys []ast.BaseTerm
	for i, arg := range rule.clause.Head.Args {
		switch v := arg.(type) {
		case ast.Variable:
			if _, let := rule.letVars[v]; let || v.Symbol == "_" {
				continue
			}
		case ast.Constant:
		default:
			continue // an expression: checked on the derived head below
		}
		xs, ys = append(xs, arg), append(ys, a.Args[i])
	}
	sol, err := unionfind.UnifyTermsExtend(xs, ys, unionfind.New())
	if err != nil {
		return false, nil // the head cannot take a's values
	}
	bound := make(map[ast.Variable]bool, len(xs))
	for _, x := range xs {
		if v, ok := x.(ast.Variable); ok {
			bound[v] = true
		}
	}
	order := planOrder(rule.clause, -1, bound, r.isDeferred)
	// Existential: the first body solution whose head is a ends the search.
	// An evaluation error still ends it as an error, as before.
	var witness func(k int, s unionfind.UnionFind) (bool, error)
	witness = func(k int, s unionfind.UnionFind) (bool, error) {
		if k == len(order) {
			heads, err := headsOf(rule.clause, []unionfind.UnionFind{s})
			if err != nil {
				return false, err
			}
			for _, h := range heads {
				if h.Equals(a) {
					return true, nil
				}
			}
			return false, nil
		}
		step, err := ctx.EvalPremise(rule.clause.Premises[order[k]], s)
		if err != nil {
			return false, err
		}
		for _, next := range step {
			if ok, err := witness(k+1, next); ok || err != nil {
				return ok, err
			}
		}
		return false, nil
	}
	return witness(0, sol)
}

// isDeferred reports whether sym is answered top-down (deferred()).
func (r *dredRun) isDeferred(sym ast.PredicateSym) bool {
	d := r.k.programInfo.Decls[sym]
	return d != nil && d.DeferredPredicate()
}

// variant evaluates clause with premise pi reading only delta (a negated
// premise read as the positive atom it negates) and every other premise
// reading ctx.Store, and returns the heads it derives. The delta premise is
// joined first when that cannot leave a later premise unbound: an atom over
// plain terms binds variables and needs none.
func (r *dredRun) variant(ctx engine.QueryContext, clause ast.Clause, pi int, negated bool, delta factstore.ReadOnlyFactStore) ([]ast.Atom, error) {
	solutions, err := r.variantSolutions(ctx, clause, pi, negated, delta)
	if err != nil || len(solutions) == 0 {
		return nil, err
	}
	return headsOf(clause, solutions)
}

// variantSolutions is variant's body: the substitutions under which clause's
// premises hold with premise pi reading only delta.
func (r *dredRun) variantSolutions(ctx engine.QueryContext, clause ast.Clause, pi int, negated bool, delta factstore.ReadOnlyFactStore) ([]unionfind.UnionFind, error) {
	r.variants.Add(1)
	var deltaPremise ast.Atom
	if negated {
		deltaPremise = clause.Premises[pi].(ast.NegAtom).Atom
	} else {
		deltaPremise = clause.Premises[pi].(ast.Atom)
	}
	var order []int
	if plainAtom(deltaPremise) {
		order = planOrder(clause, pi, nil, r.isDeferred)
	} else {
		order = make([]int, 0, len(clause.Premises))
		for i := range clause.Premises {
			order = append(order, i)
		}
	}
	deltaCtx := ctx
	deltaCtx.Store = delta
	solutions := []unionfind.UnionFind{unionfind.New()}
	for _, i := range order {
		var out []unionfind.UnionFind
		for _, s := range solutions {
			var step []unionfind.UnionFind
			var err error
			if i == pi {
				step, err = deltaCtx.EvalPremise(deltaPremise, s)
			} else {
				step, err = ctx.EvalPremise(clause.Premises[i], s)
			}
			if err != nil {
				return nil, err
			}
			out = append(out, step...)
			if r.limit > 0 && len(out) > r.limit {
				return nil, fmt.Errorf("dred: solution limit %d exceeded", r.limit)
			}
		}
		if solutions = out; len(solutions) == 0 {
			return nil, nil
		}
	}
	return solutions, nil
}

// headsOf instantiates clause's head for each solution, through its let
// transform when it has one.
func headsOf(clause ast.Clause, solutions []unionfind.UnionFind) ([]ast.Atom, error) {
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
// replaced part of a DRed evaluation.
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
