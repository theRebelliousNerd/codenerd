package core

import (
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
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
	for _, a := range pending {
		ok, err := r.supported(ctx, a)
		if err != nil {
			return err
		}
		if ok {
			restore(a)
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
