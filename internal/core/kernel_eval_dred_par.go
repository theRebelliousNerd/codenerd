package core

import (
	"runtime"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"golang.org/x/sync/errgroup"
)

// dredParallelMinRows is the change size below which a phase's variants run
// on one goroutine: a one-file write is a few hundred microseconds of work,
// less than scheduling it across cores.
const dredParallelMinRows = 512

// dredVariant is one rule variant of a phase: premise pi of rule reads only delta.
type dredVariant struct {
	ctx     engine.QueryContext
	rule    int // index into dredIndex.rules
	clause  ast.Clause
	pi      int
	negated bool
	delta   *factstore.MultiIndexedArrayInMemoryStore
	order   []int // memoized premise order for this variant
	before  *dredBefore
}

// seedVariants lists a phase's variants over the lower strata's change: a
// positive premise reads pos (the phase's own sign: removals when
// over-deleting, inserts when inserting), a negated premise reads neg (the
// other sign). members are the stratum's variant-evaluated rules; only the
// premises of changed predicates are looked at (the trigger index).
func (r *dredRun) seedVariants(members map[int]struct{}, ctx engine.QueryContext, pos, neg deltaStores, inStratum func(ast.PredicateSym) bool) []dredVariant {
	out := r.idx.triggered(ctx, members, pos, false, inStratum, nil)
	return ordered(r.idx.triggered(ctx, members, neg, true, nil, out), &dredBefore{pos: pos, neg: neg})
}

// ordered gives every variant of one phase the phase's change, so a premise
// before its delta premise reads the state without it (dredBefore).
func ordered(vs []dredVariant, before *dredBefore) []dredVariant {
	for i := range vs {
		vs[i].before = before
	}
	return vs
}

// recursiveVariants lists a phase's variants over the stratum's own newest
// rows (its recursion): the positive premises reading them.
func (r *dredRun) recursiveVariants(members map[int]struct{}, ctx engine.QueryContext, cur deltaStores) []dredVariant {
	return ordered(r.idx.triggered(ctx, members, cur, false, nil, nil), &dredBefore{pos: cur})
}

// runVariants evaluates a phase's variants and hands their heads to apply in
// variant order. Evaluation only reads the store; apply, which writes it,
// runs after every variant has been evaluated, so variants of one phase may
// run in parallel and the outcome is the sequential one.
func (r *dredRun) runVariants(vs []dredVariant, apply func([]ast.Atom)) error {
	r.profile.variantsBuilt.Add(int64(len(vs)))
	rows := 0
	for _, v := range vs {
		rows += v.delta.EstimateFactCount()
	}
	results := make([][]ast.Atom, len(vs))
	timed := func(v dredVariant) ([]ast.Atom, error) {
		started := time.Now()
		heads, err := r.variant(v.ctx, v.clause, v.pi, v.negated, v.delta, v.order, v.before)
		r.profile.charge(v.clause.Head.Predicate.Symbol, time.Since(started))
		r.profile.derivations.Add(int64(len(heads)))
		return heads, err
	}
	if len(vs) < 2 || rows < dredParallelMinRows {
		for i, v := range vs {
			heads, err := timed(v)
			if err != nil {
				return err
			}
			results[i] = heads
		}
	} else {
		var g errgroup.Group
		g.SetLimit(runtime.GOMAXPROCS(0))
		for i, v := range vs {
			g.Go(func() error {
				heads, err := timed(v)
				results[i] = heads
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}
	for _, heads := range results {
		apply(heads)
	}
	return nil
}
