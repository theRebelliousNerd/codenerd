package core

import (
	"runtime"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"golang.org/x/sync/errgroup"
)

// =============================================================================
// DRED: SEMI-NAIVE RE-DERIVATION
// =============================================================================
// Over-delete removed every fact that had a derivation through a removed row;
// many have another. Re-derivation puts those back, and it is a least fixpoint
// over the over-deleted set: a fact comes back only if a derivation of it holds
// in the new state, so an unsupported recursive cycle, whose members support
// only each other, never gets a first member and stays deleted.
//
// The old loop tested every still-pending fact again on every pass until a pass
// restored nothing, so a chain of n facts restoring one per pass cost n passes
// of up to n tests. The work now follows the restorations:
//
//  1. Seeds. Each over-deleted fact is tested once for support in the new
//     state: a program fact, an EDB row, or a rule body with the head bound.
//  2. Propagation. The facts restored by a round are the next round's frontier.
//     Only a rule premise that reads a frontier predicate can newly hold, so
//     the round runs the variants of exactly those premises (the trigger
//     index), the premise reading the frontier and every other premise the
//     store as it now is. A head they derive is restored only if it is still
//     among the over-deleted facts that have not come back; anything else
//     either never left the store or was never derivable before.
//
// A fact the seeds missed that a later restoration supports is found by the
// variant over that restoration: a derivation in the final state has a premise
// restored last, and that round's frontier held it.

// rederive restores the members of over that still have support in ctx's state
// (the store after over-delete). pending lists the over-deleted facts, none of
// them in the store; members are the stratum's variant-evaluated rules.
func (r *dredRun) rederive(members map[int]struct{}, ctx engine.QueryContext, over deltaStores, pending []ast.Atom) error {
	k := r.k
	frontier := make(deltaStores)
	restore := func(a ast.Atom) {
		if k.store.Add(a) {
			frontier.add(a)
			r.rederived++
		}
	}

	// Seeds: one support test per over-deleted fact. A fact restored here is
	// visible to the tests after it, which only saves rounds: every restored
	// fact is in the frontier, so nothing it supports is missed.
	if len(pending) > 0 {
		r.profile.rederiveRounds.Add(1)
	}
	if len(pending) < dredParallelMinRows {
		for _, a := range pending {
			ok, err := r.supported(ctx, a)
			if err != nil {
				return err
			}
			if ok {
				restore(a)
			}
		}
	} else {
		// A large over-delete: the tests are independent of one another (each
		// reads the state with every over-deleted fact gone), so they run on
		// every core and the supported facts are restored afterwards, in
		// order. A fact whose only support is another restored fact is not
		// found here; it is found by the frontier rounds below, as before.
		// Retracting a turn's handful of coder_target and modified rows
		// over-deleted about 3,300 context and impact facts and tested them
		// one at a time: 1.1-1.5 s of a turn boundary (kernel log, "6 changed
		// rows ... 3298 over-deleted ... re-derive 1.46s").
		supported, err := r.supportedAll(ctx, pending)
		if err != nil {
			return err
		}
		for i, a := range pending {
			if supported[i] {
				restore(a)
			}
		}
	}

	for len(frontier) > 0 {
		cur := frontier
		frontier = make(deltaStores)
		r.profile.rederiveRounds.Add(1)
		vs := r.recursiveVariants(members, ctx, cur)
		if err := r.runVariants(vs, func(heads []ast.Atom) {
			for _, h := range heads {
				if over.has(h) && !k.store.Contains(h) {
					restore(h)
				}
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

// supportedAll is supported for every atom, concurrently. The store is only
// read. The EDB rows of each predicate tested are loaded first: heldEDB fills
// a map, which the concurrent tests may then only read.
func (r *dredRun) supportedAll(ctx engine.QueryContext, atoms []ast.Atom) ([]bool, error) {
	warmed := make(map[string]struct{})
	for _, a := range atoms {
		pred := a.Predicate.Symbol
		if _, done := warmed[pred]; done {
			continue
		}
		warmed[pred] = struct{}{}
		if r.k.factIndex.aritiesOf(pred) != nil {
			if _, err := r.heldEDB(pred); err != nil {
				return nil, err
			}
		}
	}
	out := make([]bool, len(atoms))
	workers := runtime.GOMAXPROCS(0)
	chunk := max(len(atoms)/(workers*4), 16)
	var g errgroup.Group
	g.SetLimit(workers)
	for start := 0; start < len(atoms); start += chunk {
		end := min(start+chunk, len(atoms))
		g.Go(func() error {
			for i := start; i < end; i++ {
				ok, err := r.supported(ctx, atoms[i])
				if err != nil {
					return err
				}
				out[i] = ok
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}
