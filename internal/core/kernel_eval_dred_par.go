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

// dredVariant is one rule variant of a phase: premise pi reads only delta.
type dredVariant struct {
	ctx     engine.QueryContext
	clause  ast.Clause
	pi      int
	negated bool
	delta   *factstore.MultiIndexedArrayInMemoryStore
}

// seedVariants lists a phase's variants over the lower strata's change: a
// positive premise reads pos (the phase's own sign: removals when
// over-deleting, inserts when inserting), a negated premise reads neg (the
// other sign).
func (r *dredRun) seedVariants(group []int, ctx engine.QueryContext, pos, neg deltaStores, inStratum func(ast.PredicateSym) bool) []dredVariant {
	var out []dredVariant
	for _, ri := range group {
		rule := r.idx.rules[ri]
		for _, pi := range rule.positives {
			p := rule.clause.Premises[pi].(ast.Atom).Predicate
			if d := pos[p]; d != nil && !inStratum(p) {
				out = append(out, dredVariant{ctx, rule.clause, pi, false, d})
			}
		}
		for _, ni := range rule.negatives {
			if d := neg[rule.clause.Premises[ni].(ast.NegAtom).Atom.Predicate]; d != nil {
				out = append(out, dredVariant{ctx, rule.clause, ni, true, d})
			}
		}
	}
	return out
}

// recursiveVariants lists a phase's variants over the stratum's own newest
// rows (its recursion).
func (r *dredRun) recursiveVariants(group []int, ctx engine.QueryContext, cur deltaStores) []dredVariant {
	var out []dredVariant
	for _, ri := range group {
		rule := r.idx.rules[ri]
		for _, pi := range rule.positives {
			if d := cur[rule.clause.Premises[pi].(ast.Atom).Predicate]; d != nil {
				out = append(out, dredVariant{ctx, rule.clause, pi, false, d})
			}
		}
	}
	return out
}

// runVariants evaluates a phase's variants and hands their heads to apply in
// variant order. Evaluation only reads the store; apply, which writes it,
// runs after every variant has been evaluated, so variants of one phase may
// run in parallel and the outcome is the sequential one.
func (r *dredRun) runVariants(vs []dredVariant, apply func([]ast.Atom)) error {
	rows := 0
	for _, v := range vs {
		rows += v.delta.EstimateFactCount()
	}
	results := make([][]ast.Atom, len(vs))
	timed := func(v dredVariant) ([]ast.Atom, error) {
		started := time.Now()
		heads, err := r.variant(v.ctx, v.clause, v.pi, v.negated, v.delta)
		r.profile.charge(v.clause.Head.Predicate.Symbol, time.Since(started))
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
