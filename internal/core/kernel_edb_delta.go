package core

import (
	"maps"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// =============================================================================
// EDB WRITE DELTA
// =============================================================================
// The cone evaluator re-derives a written predicate's cone over the store the
// last evaluation left. For a written predicate no rule derives, the store
// already holds every row the EDB still holds; only the rows written since
// differ. edbDelta is that difference, kept by the index every EDB write goes
// through (insert, remove), so preparing the cone costs the rows written, not
// the rows held: diffing a 162k-row world predicate against the store per
// write was ~0.3 s (measured 2026-10-29).
//
// The delta is reset when an evaluation succeeds (clearDirtyLocked), so it
// always spans exactly the writes since the store was last consistent. An
// index built by a rebuild has no history; its delta is unknown, and the next
// evaluation is a full one (coneIneligibilityLocked).

// edbDelta is the per-predicate difference between the EDB and the store's
// extensional rows.
type edbDelta struct {
	known   bool
	added   map[string]map[string]struct{}   // predicate -> canonical key now held, not in the store
	removed map[string]map[string]removedRow // predicate -> canonical key the store still holds
}

// removedRow is a retracted fact, with the atom it was admitted as when the
// index still remembered it.
type removedRow struct {
	fact Fact
	atom admittedAtom
}

func newEDBDelta(known bool) edbDelta {
	return edbDelta{
		known:   known,
		added:   make(map[string]map[string]struct{}),
		removed: make(map[string]map[string]removedRow),
	}
}

// noteInsert records a key that became held.
func (d *edbDelta) noteInsert(pred, key string) {
	set := d.added[pred]
	if set == nil {
		set = make(map[string]struct{})
		d.added[pred] = set
	}
	set[key] = struct{}{}
}

// noteRemove records a held key that was dropped. A key added since the last
// evaluation never reached the store and simply stops being added; any other
// key's stored row must be removed. A key removed, re-added and removed again
// keeps its first removed row: that is the atom the store holds.
func (d *edbDelta) noteRemove(pred, key string, f Fact, atom admittedAtom, hadAtom bool) {
	if set := d.added[pred]; set != nil {
		if _, pending := set[key]; pending {
			delete(set, key)
			if len(set) == 0 {
				delete(d.added, pred)
			}
			return
		}
	}
	rows := d.removed[pred]
	if rows == nil {
		rows = make(map[string]removedRow)
		d.removed[pred] = rows
	}
	if _, already := rows[key]; already {
		return
	}
	row := removedRow{fact: f}
	if hadAtom {
		row.atom = atom
	}
	rows[key] = row
}

// reset starts a new delta: the store now matches the EDB.
func (d *edbDelta) reset() {
	*d = newEDBDelta(true)
}

func (d edbDelta) clone() edbDelta {
	out := edbDelta{
		known:   d.known,
		added:   make(map[string]map[string]struct{}, len(d.added)),
		removed: make(map[string]map[string]removedRow, len(d.removed)),
	}
	for pred, set := range d.added {
		out.added[pred] = maps.Clone(set)
	}
	for pred, rows := range d.removed {
		out.removed[pred] = maps.Clone(rows)
	}
	return out
}

// applyExtensionalDeltaLocked brings the store's rows of pred, a predicate no
// rule derives, up to the EDB: it removes the rows retracted since the last
// evaluation and adds the rows asserted since. It returns how many atoms it
// removed and added. Caller holds k.mu; the delta is known
// (coneIneligibilityLocked).
func (k *RealKernel) applyExtensionalDeltaLocked(pred string) (removed, added int, err error) {
	remover := k.store.(factstore.FactStoreWithRemove)
	x := k.factIndex
	// The store is a set and the EDB is not: two host representations of one
	// value (a plain "/tmp" and types.MangleString("/tmp") under a /string
	// declaration) are two EDB rows on one store atom. Removing a row's atom
	// removes the support of every row that shares it, so an atom taken out
	// here is put back when a retained row still admits to it.
	// aliasCapable says whether any such pair can exist; for the ordinary
	// predicate it is false and the delta stays O(delta).
	aliasCapable := x.aliasRows[pred] > 0
	var taken map[string]ast.Atom
	for _, row := range x.delta.removed[pred] {
		atom := row.atom.atom
		if row.atom.program != k.programInfo || k.programInfo == nil {
			if atom, err = k.factToAtomLocked(row.fact); err != nil {
				return removed, added, err
			}
		}
		if remover.Remove(atom) {
			removed++
			if aliasCapable || hostAliasCapable(row.fact) {
				if taken == nil {
					taken = make(map[string]ast.Atom)
				}
				taken[atom.String()] = atom
			}
		}
	}
	bucket := x.bucket(pred)
	if len(taken) > 0 {
		for key, f := range bucket {
			atom, err := k.edbAtomLocked(pred, key, f)
			if err != nil {
				return removed, added, err
			}
			if _, shared := taken[atom.String()]; shared && k.store.Add(atom) {
				// The atom was never gone as far as the EDB is concerned.
				removed--
			}
		}
	}
	for key := range x.delta.added[pred] {
		f, held := bucket[key]
		if !held {
			continue
		}
		atom, err := k.edbAtomLocked(pred, key, f)
		if err != nil {
			return removed, added, err
		}
		if k.store.Add(atom) {
			added++
		}
	}
	return removed, added, nil
}

// edbAtomLocked returns the store atom of the fact held under key: the one
// admission remembered under this program, or a fresh conversion, then
// remembered. Caller holds k.mu.
func (k *RealKernel) edbAtomLocked(pred, key string, f Fact) (ast.Atom, error) {
	if atom, ok := k.factIndex.atomOf(pred, key, k.programInfo); ok {
		return atom, nil
	}
	atom, err := k.factToAtomLocked(f)
	if err != nil {
		return ast.Atom{}, err
	}
	k.factIndex.rememberAtom(pred, key, atom, k.programInfo)
	return atom, nil
}
