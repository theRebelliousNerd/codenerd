package core

import "maps"

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
}

func newEDBIndex() *edbIndex {
	return &edbIndex{
		buckets: make(map[string]map[string]Fact),
		arities: make(map[string]map[int]struct{}),
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
	x.size--
	if len(bucket) == 0 {
		delete(x.buckets, pred)
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

func (x *edbIndex) len() int {
	return x.size
}

// clone copies the index structure; the facts and keys are shared.
func (x *edbIndex) clone() *edbIndex {
	out := &edbIndex{
		buckets: make(map[string]map[string]Fact, len(x.buckets)),
		arities: make(map[string]map[int]struct{}, len(x.arities)),
		size:    x.size,
	}
	for pred, bucket := range x.buckets {
		out.buckets[pred] = maps.Clone(bucket)
	}
	for pred, arities := range x.arities {
		out.arities[pred] = maps.Clone(arities)
	}
	return out
}
