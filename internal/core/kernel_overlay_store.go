package core

import (
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// =============================================================================
// OVERLAY STORE (KERNEL-PERF-06)
// =============================================================================
// A clone used to copy every atom of its parent's evaluated store before it
// could stage one projected fact: ~280 ms on a 345k-fact world kernel, almost
// all of a dream safety check (measured 2026-10-29). An overlayStore instead
// reads through to an immutable base and keeps its own writes on top: the
// atoms it added, and tombstones for base atoms it removed.
//
// Immutability of the base is the whole contract. A clone takes its parent's
// store as base only when the parent's store is plain, and marks the parent
// (storeShared): before the parent's next write it wraps that store in an
// overlay of its own (ownStoreLocked), so nobody writes to a base again. A
// parent whose store is already an overlay hands the clone its base and a
// copy of its writes (fork); the parent keeps writing to its own copy.
//
// Answers are the base's minus the tombstones plus the added atoms, and an
// atom is never in both: Add of a live base atom is a no-op, Add of a
// tombstoned one lifts the tombstone.

// overlayFlattenRatio bounds how far an overlay may drift from its base:
// when its writes exceed this fraction of the base, the next ownership check
// flattens it into a plain store (one copy, amortized over the writes).
const overlayFlattenRatio = 4

type overlayStore struct {
	base         factstore.FactStore
	added        factstore.FactStoreWithRemove
	removed      map[ast.PredicateSym]map[uint64][]ast.Atom
	removedCount int
}

var _ factstore.FactStoreWithRemove = (*overlayStore)(nil)

func newOverlayStore(base factstore.FactStore) *overlayStore {
	return &overlayStore{
		base:    base,
		added:   factstore.NewSimpleInMemoryStore(),
		removed: make(map[ast.PredicateSym]map[uint64][]ast.Atom),
	}
}

// fork is an independent overlay over the same base with a copy of o's
// writes. Read-only on o.
func (o *overlayStore) fork() (*overlayStore, bool) {
	out := newOverlayStore(o.base)
	if err := factstore.GetAllFacts(o.added, func(a ast.Atom) error {
		out.added.Add(a)
		return nil
	}); err != nil {
		return nil, false
	}
	for pred, byHash := range o.removed {
		copied := make(map[uint64][]ast.Atom, len(byHash))
		for h, atoms := range byHash {
			copied[h] = append([]ast.Atom(nil), atoms...)
		}
		out.removed[pred] = copied
	}
	out.removedCount = o.removedCount
	return out, true
}

// writes is how many atoms o holds apart from its base.
func (o *overlayStore) writes() int {
	return o.added.EstimateFactCount() + o.removedCount
}

func (o *overlayStore) tombstoned(a ast.Atom) bool {
	for _, t := range o.removed[a.Predicate][a.Hash()] {
		if t.Equals(a) {
			return true
		}
	}
	return false
}

func (o *overlayStore) liftTombstone(a ast.Atom) bool {
	byHash := o.removed[a.Predicate]
	h := a.Hash()
	for i, t := range byHash[h] {
		if t.Equals(a) {
			byHash[h] = append(byHash[h][:i], byHash[h][i+1:]...)
			if len(byHash[h]) == 0 {
				delete(byHash, h)
			}
			if len(byHash) == 0 {
				delete(o.removed, a.Predicate)
			}
			o.removedCount--
			return true
		}
	}
	return false
}

func (o *overlayStore) GetFacts(query ast.Atom, fn func(ast.Atom) error) error {
	tombs := o.removed[query.Predicate]
	if err := o.base.GetFacts(query, func(a ast.Atom) error {
		if len(tombs) > 0 && o.tombstoned(a) {
			return nil
		}
		return fn(a)
	}); err != nil {
		return err
	}
	return o.added.GetFacts(query, fn)
}

func (o *overlayStore) Contains(a ast.Atom) bool {
	return o.added.Contains(a) || (o.base.Contains(a) && !o.tombstoned(a))
}

func (o *overlayStore) ListPredicates() []ast.PredicateSym {
	seen := make(map[ast.PredicateSym]struct{})
	var out []ast.PredicateSym
	for _, list := range [][]ast.PredicateSym{o.base.ListPredicates(), o.added.ListPredicates()} {
		for _, p := range list {
			if _, dup := seen[p]; !dup {
				seen[p] = struct{}{}
				out = append(out, p)
			}
		}
	}
	return out
}

func (o *overlayStore) EstimateFactCount() int {
	return o.base.EstimateFactCount() - o.removedCount + o.added.EstimateFactCount()
}

func (o *overlayStore) Add(a ast.Atom) bool {
	if o.base.Contains(a) {
		return o.liftTombstone(a)
	}
	return o.added.Add(a)
}

func (o *overlayStore) Remove(a ast.Atom) bool {
	if o.added.Remove(a) {
		return true
	}
	if !o.base.Contains(a) || o.tombstoned(a) {
		return false
	}
	byHash := o.removed[a.Predicate]
	if byHash == nil {
		byHash = make(map[uint64][]ast.Atom)
		o.removed[a.Predicate] = byHash
	}
	byHash[a.Hash()] = append(byHash[a.Hash()], a)
	o.removedCount++
	return true
}

func (o *overlayStore) Merge(other factstore.ReadOnlyFactStore) {
	for _, pred := range other.ListPredicates() {
		_ = other.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
			o.Add(a)
			return nil
		})
	}
}

// flatten copies o's answers into a plain store of the kind evaluation uses.
func (o *overlayStore) flatten() factstore.FactStoreWithRemove {
	out := newEvaluationFactStore(o.EstimateFactCount())
	_ = factstore.GetAllFacts(o, func(a ast.Atom) error {
		out.Add(a)
		return nil
	})
	return out
}

// shareStoreLocked hands a clone a store over this kernel's evaluated store
// without copying it. Caller holds at least k.mu.RLock.
func (k *RealKernel) shareStoreLocked() (factstore.FactStoreWithRemove, bool) {
	if o, ok := k.store.(*overlayStore); ok {
		return o.fork()
	}
	// The plain store becomes a base: this kernel must not write to it
	// again (ownStoreLocked).
	k.storeShared.Store(true)
	return newOverlayStore(k.store), true
}

// ownStoreLocked makes k.store safe to write: a store a clone reads as its
// base is wrapped in an overlay first, and an overlay that has drifted far
// from its base is flattened. Every in-place store write calls it first.
// Caller holds k.mu.
func (k *RealKernel) ownStoreLocked() {
	if k.store == nil {
		return
	}
	if k.storeShared.Swap(false) {
		k.store = newOverlayStore(k.store)
		return
	}
	if o, ok := k.store.(*overlayStore); ok && o.writes()*overlayFlattenRatio > o.base.EstimateFactCount() {
		k.store = o.flatten()
	}
}
