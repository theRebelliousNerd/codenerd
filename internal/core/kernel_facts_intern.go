// Package core - string interning for stored facts.
//
// internFact rewrites the predicate name and any string-typed arguments
// of a Fact to point at process-wide interned copies via the standard
// library's `unique` package. After interning, two Facts that share a predicate
// (or a repeated string argument) share the same underlying string
// memory. The dominant win is the Predicate field: a session that
// asserts thousands of `user_intent`, `next_action`, `permitted`,
// `selected_atom`, etc. facts collapses all those repeated names down
// to one canonical string per distinct value.
//
// unique.Handle / unique.Make guarantees:
//   - byte-equal inputs map to the same canonical value
//   - the interned value is garbage-collected once no Handle references
//     remain (so test/dream/scratch kernels don't leak forever)
//   - .Value() returns a regular string usable everywhere the original
//     was (no API change at call sites)
//
// We do NOT intern arbitrary `any` argument types — only `string` and
// `MangleAtom` (a string alias). Numbers, bools, and nested
// slices/maps fall through unchanged.
package core

import (
	"sync"
	"unique"
)

// predicateInternCache holds the canonical copy of each predicate name.
// The predicate vocabulary is bounded by the schema and every fact repeats
// one: unique.Make per fact cost a hash and a canonical-map lookup that
// allocated (11.0M allocations to load 150k facts, 2026-11-07), where a
// map[string]string lookup hashes the string in place. Arguments are not
// cached here: their vocabulary is unbounded (paths, ids), and a strong map
// would grow for the life of the process; they stay on unique, whose
// canonical copies are collected once no fact holds them.
var (
	predicateInternCacheMu sync.RWMutex
	predicateInternCache   = make(map[string]string)
)

func internPredicate(p string) string {
	predicateInternCacheMu.RLock()
	if v, ok := predicateInternCache[p]; ok {
		predicateInternCacheMu.RUnlock()
		return v
	}
	predicateInternCacheMu.RUnlock()
	v := unique.Make(p).Value()
	predicateInternCacheMu.Lock()
	if existing, ok := predicateInternCache[p]; ok {
		predicateInternCacheMu.Unlock()
		return existing
	}
	predicateInternCache[p] = v
	predicateInternCacheMu.Unlock()
	return v
}

// internFact returns a Fact whose predicate is the cached canonical name and
// whose string and MangleAtom arguments are unique's canonical copies, so
// facts that repeat a path share one string: 570k world facts hold 533 MB
// interned against 592 MB without argument interning. The Args slice is
// copied only from the first argument that changes, and never for a fact
// with no string-shaped argument. The original Fact is not modified.
func internFact(f Fact) Fact {
	f.Predicate = internPredicate(f.Predicate)
	if len(f.Args) == 0 {
		return f
	}
	var newArgs []any
	for i, arg := range f.Args {
		var interned any
		switch v := arg.(type) {
		case string:
			interned = unique.Make(v).Value()
		case MangleAtom:
			interned = MangleAtom(unique.Make(string(v)).Value())
		default:
			if newArgs != nil {
				newArgs[i] = arg
			}
			continue
		}
		if newArgs == nil {
			newArgs = make([]any, len(f.Args))
			copy(newArgs, f.Args[:i])
		}
		newArgs[i] = interned
	}
	if newArgs != nil {
		f.Args = newArgs
	}
	return f
}
