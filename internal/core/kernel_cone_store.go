package core

import (
	"fmt"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// prepareConeStoreLocked brings the store's rows of every touched predicate
// to what the cone's re-derivation starts from, and reports how many atoms it
// removed and put back. Per touched predicate:
//
//   - one a rule derives: every stored row goes, derived and extensional
//     alike, and the EDB's rows come back; the cone re-derives the rest;
//   - one no rule derives: the rows retracted since the last evaluation go
//     and the rows asserted since come in (the write delta,
//     kernel_edb_delta.go). The work is the rows written, not the rows held.
//
// A program fact of a derived predicate, removed with the rest, is re-added by
// the engine run that follows, as on the full path; one of an extensional
// predicate is never removed. Caller holds k.mu.
func (k *RealKernel) prepareConeStoreLocked(plan *conePlan) (removed, restored, listed int, err error) {
	remover := k.store.(factstore.FactStoreWithRemove)
	for _, pred := range plan.touched {
		_, derived := k.cone.heads[pred]
		var r, a, l int
		if derived {
			r, a, l, err = k.replaceStoredRowsLocked(pred, remover)
		} else {
			r, a, err = k.applyExtensionalDeltaLocked(pred)
		}
		if err != nil {
			return removed, restored, listed, fmt.Errorf("cone: %s: %w", pred, err)
		}
		removed += r
		restored += a
		listed += l
	}
	return removed, restored, listed, nil
}

// storedRowsLocked lists every row the store holds under pred's symbols.
func (k *RealKernel) storedRowsLocked(pred string) ([]ast.Atom, error) {
	var rows []ast.Atom
	for _, sym := range k.cone.symbolsOf(pred, k.factIndex.aritiesOf(pred)) {
		if err := k.store.GetFacts(ast.NewQuery(sym), func(a ast.Atom) error {
			rows = append(rows, a)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// heldAtomsLocked is every EDB row of pred as its store atom.
func (k *RealKernel) heldAtomsLocked(pred string) ([]ast.Atom, error) {
	bucket := k.factIndex.bucket(pred)
	atoms := make([]ast.Atom, 0, len(bucket))
	for key, f := range bucket {
		atom, err := k.edbAtomLocked(pred, key, f)
		if err != nil {
			return nil, err
		}
		atoms = append(atoms, atom)
	}
	return atoms, nil
}

// replaceStoredRowsLocked empties pred in the store and puts its EDB rows back.
func (k *RealKernel) replaceStoredRowsLocked(pred string, remover factstore.FactStoreWithRemove) (removed, restored, listed int, err error) {
	rows, err := k.storedRowsLocked(pred)
	if err != nil {
		return 0, 0, 0, err
	}
	held, err := k.heldAtomsLocked(pred)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, atom := range rows {
		remover.Remove(atom)
	}
	for _, atom := range held {
		k.store.Add(atom)
	}
	return len(rows), len(held), len(rows), nil
}
