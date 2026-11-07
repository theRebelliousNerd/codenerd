package core

import (
	"maps"
	"sync/atomic"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
)

// edbIndex is the EDB's canonical identity set, bucketed by predicate name.
//
// It is the dedupe index (every write asks it whether a canonically equal fact
// is already held) and the access path the cone evaluator restores a touched
// predicate's rows from: before it, every cone evaluation walked the whole EDB
// to find the rows of the few predicates it re-derives
// (Docs/audits/mangle-kernel-performance.md §4).
//
// Buckets hold the stored Fact values themselves. Stored facts are immutable,
// so a clone shares them. Where a fact sits in the EDB's ordered storage
// (kernel_edb_facts.go) is kept apart, in slots, so a removal by canonical key
// reaches its slot without a scan.
type edbIndex struct {
	buckets map[string]map[string]Fact // predicate -> canonical key -> fact
	// arities records every arity ever indexed under a name. It never shrinks
	// (until the EDB is cleared): the store still holds the atom of a row
	// retracted since the last evaluation, and the cone evaluator has to know
	// that row's predicate symbol to remove it.
	arities map[string]map[int]struct{}
	size    int
	// atoms memoizes each held fact's admitted atom, by predicate and key,
	// with the program it was admitted under. The cone evaluator restores a
	// touched predicate from these instead of converting every row again:
	// after a write to a world predicate that conversion was the whole
	// preparation (0.8 s for 345k rows, measured 2026-10-29). An atom is
	// reused only under the program it was converted for, because
	// admission adapts arguments to that program's Decls.
	atoms map[string]map[string]admittedAtom
	// aliasRows counts, per predicate, the held rows a second host
	// representation can share a store atom with (hostAliasCapable). A
	// predicate at zero cannot have two rows on one atom, so its extensional
	// delta needs no support check (kernel_edb_delta.go).
	aliasRows map[string]int
	// slots is each held fact's position in the EDB's ordered storage, by
	// predicate and key. Positions are stable until the storage compacts, which
	// reports every move (setSlot).
	slots map[string]map[string]int
	// delta is what changed since the store was last evaluated
	// (kernel_edb_delta.go).
	delta edbDelta
	// content is the multiset hash of the held facts
	// (kernel_content_fingerprint.go).
	content [2]uint64
	// ownBuckets and ownAtoms name the inner maps this index may write. A
	// clone shares every inner map with its source and owns none, and so
	// does the source afterwards: whichever writes a predicate first copies
	// that predicate's map (copy-on-write). Cloning a 290K-row index used to
	// copy every map, 80% of a 24 ms kernel clone (measured 2026-11-06); a
	// dream check clones the kernel once per gated action.
	ownBuckets map[string]bool
	ownAtoms   map[string]bool
	ownSlots   map[string]bool
	// shared is raised by clone, which runs under the kernel's read lock and
	// so may not touch the ownership maps; the next write, under the write
	// lock, drops every ownership before it writes (settleShared).
	shared atomic.Bool
}

// settleShared gives up ownership of every inner map once a clone shares them.
// Callers hold the kernel's write lock.
func (x *edbIndex) settleShared() {
	if x.shared.Swap(false) {
		clear(x.ownBuckets)
		clear(x.ownAtoms)
		clear(x.ownSlots)
	}
}

// admittedAtom is a fact's store atom as admitted under one program.
type admittedAtom struct {
	atom    ast.Atom
	program *analysis.ProgramInfo
}

func newEDBIndex() *edbIndex {
	return &edbIndex{
		buckets:    make(map[string]map[string]Fact),
		arities:    make(map[string]map[int]struct{}),
		atoms:      make(map[string]map[string]admittedAtom),
		aliasRows:  make(map[string]int),
		slots:      make(map[string]map[string]int),
		delta:      newEDBDelta(false),
		ownBuckets: make(map[string]bool),
		ownAtoms:   make(map[string]bool),
		ownSlots:   make(map[string]bool),
	}
}

// writableBucket returns pred's bucket for writing, copying it first when it
// is shared with a clone; nil when pred holds nothing.
func (x *edbIndex) writableBucket(pred string) map[string]Fact {
	x.settleShared()
	bucket := x.buckets[pred]
	if bucket == nil || x.ownBuckets[pred] {
		return bucket
	}
	bucket = maps.Clone(bucket)
	x.buckets[pred] = bucket
	x.ownBuckets[pred] = true
	return bucket
}

// writableAtoms is writableBucket for the atom memo.
func (x *edbIndex) writableAtoms(pred string) map[string]admittedAtom {
	x.settleShared()
	memo := x.atoms[pred]
	if memo == nil || x.ownAtoms[pred] {
		return memo
	}
	memo = maps.Clone(memo)
	x.atoms[pred] = memo
	x.ownAtoms[pred] = true
	return memo
}

// writableSlots is writableBucket for the slot positions.
func (x *edbIndex) writableSlots(pred string) map[string]int {
	x.settleShared()
	pos := x.slots[pred]
	if pos == nil || x.ownSlots[pred] {
		return pos
	}
	pos = maps.Clone(pos)
	x.slots[pred] = pos
	x.ownSlots[pred] = true
	return pos
}

// slotOf returns where the fact held under key sits in the EDB's storage.
func (x *edbIndex) slotOf(pred, key string) (int, bool) {
	pos, ok := x.slots[pred][key]
	return pos, ok
}

// setSlot records a held fact's new position after the storage compacted.
func (x *edbIndex) setSlot(pred, key string, pos int) {
	x.writableSlots(pred)[key] = pos
}

// has reports whether a fact with this canonical key is held.
func (x *edbIndex) has(pred, key string) bool {
	_, ok := x.buckets[pred][key]
	return ok
}

// insert records f under key, stored at slot pos of the EDB's storage; it
// reports false when the key was already held.
func (x *edbIndex) insert(key string, f Fact, pos int) bool {
	if _, dup := x.buckets[f.Predicate][key]; dup {
		return false
	}
	bucket := x.writableBucket(f.Predicate)
	if bucket == nil {
		bucket = make(map[string]Fact)
		x.buckets[f.Predicate] = bucket
		x.ownBuckets[f.Predicate] = true
	}
	bucket[key] = f
	slots := x.writableSlots(f.Predicate)
	if slots == nil {
		slots = make(map[string]int)
		x.slots[f.Predicate] = slots
		x.ownSlots[f.Predicate] = true
	}
	slots[key] = pos
	if hostAliasCapable(f) {
		x.aliasRows[f.Predicate]++
	}
	x.forgetAtom(f.Predicate, key)
	x.delta.noteInsert(f.Predicate, key)
	x.addContent(f)
	arities := x.arities[f.Predicate]
	if arities == nil {
		arities = make(map[int]struct{}, 1)
		x.arities[f.Predicate] = arities
	}
	arities[len(f.Args)] = struct{}{}
	x.size++
	return true
}

// remove drops the fact held under key.
func (x *edbIndex) remove(pred, key string) {
	f, ok := x.buckets[pred][key]
	if !ok {
		return
	}
	atom, hadAtom := x.atoms[pred][key]
	x.delta.noteRemove(pred, key, f, atom, hadAtom)
	bucket := x.writableBucket(pred)
	delete(bucket, key)
	slots := x.writableSlots(pred)
	delete(slots, key)
	if hostAliasCapable(f) {
		if x.aliasRows[pred]--; x.aliasRows[pred] <= 0 {
			delete(x.aliasRows, pred)
		}
	}
	x.forgetAtom(pred, key)
	x.subtractContent(f)
	x.size--
	if len(bucket) == 0 {
		delete(x.buckets, pred)
		delete(x.ownBuckets, pred)
		delete(x.slots, pred)
		delete(x.ownSlots, pred)
	}
}

// rememberAtom records the atom the fact held under key was admitted as under
// program. A nil program (no Decls yet) records nothing.
func (x *edbIndex) rememberAtom(pred, key string, atom ast.Atom, program *analysis.ProgramInfo) {
	if program == nil {
		return
	}
	if _, held := x.buckets[pred][key]; !held {
		return
	}
	memo := x.writableAtoms(pred)
	if memo == nil {
		memo = make(map[string]admittedAtom)
		x.atoms[pred] = memo
		x.ownAtoms[pred] = true
	}
	memo[key] = admittedAtom{atom: atom, program: program}
}

// atomOf returns the remembered atom of the fact held under key, if it was
// admitted under program.
func (x *edbIndex) atomOf(pred, key string, program *analysis.ProgramInfo) (ast.Atom, bool) {
	a, ok := x.atoms[pred][key]
	if !ok || program == nil || a.program != program {
		return ast.Atom{}, false
	}
	return a.atom, true
}

func (x *edbIndex) forgetAtom(pred, key string) {
	if _, held := x.atoms[pred][key]; !held {
		return
	}
	memo := x.writableAtoms(pred)
	delete(memo, key)
	if len(memo) == 0 {
		delete(x.atoms, pred)
		delete(x.ownAtoms, pred)
	}
}

// bucket returns the held facts of one predicate name. Read-only.
func (x *edbIndex) bucket(pred string) map[string]Fact {
	return x.buckets[pred]
}

// aritiesOf returns every arity ever held under pred. Read-only.
func (x *edbIndex) aritiesOf(pred string) map[int]struct{} {
	return x.arities[pred]
}

// clone shares every bucket and atom memo with x, copy-on-write: the outer
// maps are copied (one entry per predicate) and neither index owns an inner
// map afterwards, so the first write to a predicate, on either side, copies
// that predicate alone. The facts and keys are immutable and always shared.
// It runs under the kernel's read lock, so it only flags x as shared.
func (x *edbIndex) clone() *edbIndex {
	out := &edbIndex{
		buckets:    maps.Clone(x.buckets),
		arities:    make(map[string]map[int]struct{}, len(x.arities)),
		atoms:      maps.Clone(x.atoms),
		aliasRows:  maps.Clone(x.aliasRows),
		slots:      maps.Clone(x.slots),
		delta:      x.delta.clone(),
		content:    x.content,
		size:       x.size,
		ownBuckets: make(map[string]bool),
		ownAtoms:   make(map[string]bool),
		ownSlots:   make(map[string]bool),
	}
	if out.slots == nil {
		out.slots = make(map[string]map[string]int)
	}
	if out.buckets == nil {
		out.buckets = make(map[string]map[string]Fact)
	}
	if out.atoms == nil {
		out.atoms = make(map[string]map[string]admittedAtom)
	}
	x.shared.Store(true)
	for pred, arities := range x.arities {
		out.arities[pred] = maps.Clone(arities)
	}
	return out
}
