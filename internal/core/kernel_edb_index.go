package core

import (
	"maps"

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
// Buckets hold the stored Fact values themselves, never positions in k.facts,
// which compaction shifts. Stored facts are immutable, so a clone shares them.
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
}

// admittedAtom is a fact's store atom as admitted under one program.
type admittedAtom struct {
	atom    ast.Atom
	program *analysis.ProgramInfo
}

func newEDBIndex() *edbIndex {
	return &edbIndex{
		buckets: make(map[string]map[string]Fact),
		arities: make(map[string]map[int]struct{}),
		atoms:   make(map[string]map[string]admittedAtom),
	}
}

// has reports whether a fact with this canonical key is held.
func (x *edbIndex) has(pred, key string) bool {
	_, ok := x.buckets[pred][key]
	return ok
}

// insert records f under key; it reports false when the key was already held.
func (x *edbIndex) insert(key string, f Fact) bool {
	bucket := x.buckets[f.Predicate]
	if bucket == nil {
		bucket = make(map[string]Fact)
		x.buckets[f.Predicate] = bucket
	}
	if _, dup := bucket[key]; dup {
		return false
	}
	bucket[key] = f
	x.forgetAtom(f.Predicate, key)
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
	bucket := x.buckets[pred]
	if _, ok := bucket[key]; !ok {
		return
	}
	delete(bucket, key)
	x.forgetAtom(pred, key)
	x.size--
	if len(bucket) == 0 {
		delete(x.buckets, pred)
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
	memo := x.atoms[pred]
	if memo == nil {
		memo = make(map[string]admittedAtom)
		x.atoms[pred] = memo
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
	memo := x.atoms[pred]
	if memo == nil {
		return
	}
	delete(memo, key)
	if len(memo) == 0 {
		delete(x.atoms, pred)
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

// clone copies the index structure; the facts and keys are shared.
func (x *edbIndex) clone() *edbIndex {
	out := &edbIndex{
		buckets: make(map[string]map[string]Fact, len(x.buckets)),
		arities: make(map[string]map[int]struct{}, len(x.arities)),
		atoms:   make(map[string]map[string]admittedAtom, len(x.atoms)),
		size:    x.size,
	}
	for pred, memo := range x.atoms {
		out.atoms[pred] = maps.Clone(memo)
	}
	for pred, bucket := range x.buckets {
		out.buckets[pred] = maps.Clone(bucket)
	}
	for pred, arities := range x.arities {
		out.arities[pred] = maps.Clone(arities)
	}
	return out
}
