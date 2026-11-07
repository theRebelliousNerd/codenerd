package core

import (
	"iter"
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
// The rows hold the stored Fact values themselves. Stored facts are immutable,
// so a clone shares them. Where a fact sits in the EDB's ordered storage
// (kernel_edb_facts.go) is kept apart, in slots, so a removal by canonical key
// reaches its slot without a scan.
//
// The per-predicate maps are persistent (persistent_map.go): a clone shares
// them whole and a write copies the path to its key, so staging one row of a
// 48K-row predicate on a clone costs a handful of nodes, where copying the
// predicate's Go map cost 4.7 ms (measured 2026-11-06).
type edbIndex struct {
	rows map[string]pmap[Fact] // predicate -> canonical key -> fact
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
	atoms map[string]pmap[admittedAtom]
	// aliasRows counts, per predicate, the held rows a second host
	// representation can share a store atom with (hostAliasCapable). A
	// predicate at zero cannot have two rows on one atom, so its extensional
	// delta needs no support check (kernel_edb_delta.go).
	aliasRows map[string]int
	// slots is each held fact's position in the EDB's ordered storage, by
	// predicate and key. Positions are stable until the storage compacts, which
	// reports every move (setSlot).
	slots map[string]pmap[int]
	// delta is what changed since the store was last evaluated
	// (kernel_edb_delta.go).
	delta edbDelta
	// content is the multiset hash of the held facts
	// (kernel_content_fingerprint.go).
	content [2]uint64
	// owner is the writer identity of the persistent maps: the nodes it
	// created are written in place, every other node is copied first. A clone
	// shares every node with its source and owns none; the source stops owning
	// its own at its next write. Whichever side writes a predicate first copies
	// the path to the key (copy-on-write).
	owner *pmapOwner
	// shared is raised by clone, which runs under the kernel's read lock and
	// so may not touch owner; the next write, under the write lock, takes a
	// new owner before it writes (writable).
	shared atomic.Bool
}

// writable returns the owner to write the persistent maps under. A clone
// shares them, so the owner changes after one is made: every node the index
// had created becomes foreign to it. Callers hold the kernel's write lock.
func (x *edbIndex) writable() *pmapOwner {
	if x.shared.Swap(false) || x.owner == nil {
		x.owner = new(pmapOwner)
	}
	return x.owner
}

// admittedAtom is a fact's store atom as admitted under one program.
type admittedAtom struct {
	atom    ast.Atom
	program *analysis.ProgramInfo
}

func newEDBIndex() *edbIndex {
	return &edbIndex{
		rows:      make(map[string]pmap[Fact]),
		arities:   make(map[string]map[int]struct{}),
		atoms:     make(map[string]pmap[admittedAtom]),
		aliasRows: make(map[string]int),
		slots:     make(map[string]pmap[int]),
		delta:     newEDBDelta(false),
	}
}

// slotOf returns where the fact held under key sits in the EDB's storage.
func (x *edbIndex) slotOf(pred, key string) (int, bool) {
	return x.slots[pred].get(key)
}

// setSlot records a held fact's new position after the storage compacted.
func (x *edbIndex) setSlot(pred, key string, pos int) {
	slots := x.slots[pred]
	slots.put(x.writable(), key, pos)
	x.slots[pred] = slots
}

// has reports whether a fact with this canonical key is held.
func (x *edbIndex) has(pred, key string) bool {
	return x.rows[pred].has(key)
}

// row returns the fact held under key.
func (x *edbIndex) row(pred, key string) (Fact, bool) {
	return x.rows[pred].get(key)
}

// rowCount is how many facts of pred are held.
func (x *edbIndex) rowCount(pred string) int {
	return x.rows[pred].len()
}

// eachRow yields the held facts of one predicate name with their canonical
// keys, in no order callers may rely on. The index must not be written while
// it runs.
func (x *edbIndex) eachRow(pred string) iter.Seq2[string, Fact] {
	return x.rows[pred].all()
}

// insert records f under key, stored at slot pos of the EDB's storage; it
// reports false when the key was already held.
func (x *edbIndex) insert(key string, f Fact, pos int) bool {
	if x.has(f.Predicate, key) {
		return false
	}
	o := x.writable()
	rows := x.rows[f.Predicate]
	rows.put(o, key, f)
	x.rows[f.Predicate] = rows
	slots := x.slots[f.Predicate]
	slots.put(o, key, pos)
	x.slots[f.Predicate] = slots
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
	f, ok := x.row(pred, key)
	if !ok {
		return
	}
	atom, hadAtom := x.atoms[pred].get(key)
	x.delta.noteRemove(pred, key, f, atom, hadAtom)
	o := x.writable()
	rows := x.rows[pred]
	rows.del(o, key)
	slots := x.slots[pred]
	slots.del(o, key)
	if rows.len() == 0 {
		delete(x.rows, pred)
		delete(x.slots, pred)
	} else {
		x.rows[pred] = rows
		x.slots[pred] = slots
	}
	if hostAliasCapable(f) {
		if x.aliasRows[pred]--; x.aliasRows[pred] <= 0 {
			delete(x.aliasRows, pred)
		}
	}
	x.forgetAtom(pred, key)
	x.subtractContent(f)
	x.size--
}

// rememberAtom records the atom the fact held under key was admitted as under
// program. A nil program (no Decls yet) records nothing.
func (x *edbIndex) rememberAtom(pred, key string, atom ast.Atom, program *analysis.ProgramInfo) {
	if program == nil || !x.has(pred, key) {
		return
	}
	memo := x.atoms[pred]
	memo.put(x.writable(), key, admittedAtom{atom: atom, program: program})
	x.atoms[pred] = memo
}

// atomOf returns the remembered atom of the fact held under key, if it was
// admitted under program.
func (x *edbIndex) atomOf(pred, key string, program *analysis.ProgramInfo) (ast.Atom, bool) {
	a, ok := x.atoms[pred].get(key)
	if !ok || program == nil || a.program != program {
		return ast.Atom{}, false
	}
	return a.atom, true
}

func (x *edbIndex) forgetAtom(pred, key string) {
	memo := x.atoms[pred]
	if !memo.has(key) {
		return
	}
	memo.del(x.writable(), key)
	if memo.len() == 0 {
		delete(x.atoms, pred)
		return
	}
	x.atoms[pred] = memo
}

// aritiesOf returns every arity ever held under pred. Read-only.
func (x *edbIndex) aritiesOf(pred string) map[int]struct{} {
	return x.arities[pred]
}

// clone shares every predicate's maps with x, copy-on-write: the outer maps
// are copied (one header per predicate) and neither index owns a node
// afterwards, so the first write to a predicate, on either side, copies the
// path to its key alone. The facts and keys are immutable and always shared.
// It runs under the kernel's read lock, so it only flags x as shared.
func (x *edbIndex) clone() *edbIndex {
	out := &edbIndex{
		rows:      maps.Clone(x.rows),
		arities:   make(map[string]map[int]struct{}, len(x.arities)),
		atoms:     maps.Clone(x.atoms),
		aliasRows: maps.Clone(x.aliasRows),
		slots:     maps.Clone(x.slots),
		delta:     x.delta.clone(),
		content:   x.content,
		size:      x.size,
	}
	if out.rows == nil {
		out.rows = make(map[string]pmap[Fact])
	}
	if out.slots == nil {
		out.slots = make(map[string]pmap[int])
	}
	if out.atoms == nil {
		out.atoms = make(map[string]pmap[admittedAtom])
	}
	x.shared.Store(true)
	for pred, arities := range x.arities {
		out.arities[pred] = maps.Clone(arities)
	}
	return out
}
