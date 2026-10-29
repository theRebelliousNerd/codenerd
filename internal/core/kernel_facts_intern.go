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
	"unique"
)

// internFact returns a new Fact whose Predicate and string-typed Args
// have been interned. The original Fact is not modified. Args slice is
// only re-allocated when at least one argument is a string-shaped value
// that benefits from interning; otherwise the original slice header is
// re-used (the underlying array is never mutated).
func internFact(f Fact) Fact {
	f.Predicate = unique.Make(f.Predicate).Value()
	if len(f.Args) == 0 {
		return f
	}
	// Detect any string-shaped args before allocating a new slice.
	needCopy := false
	for _, arg := range f.Args {
		switch arg.(type) {
		case string, MangleAtom:
			needCopy = true
		}
		if needCopy {
			break
		}
	}
	if !needCopy {
		return f
	}
	newArgs := make([]any, len(f.Args))
	copy(newArgs, f.Args)
	for i, arg := range newArgs {
		switch v := arg.(type) {
		case string:
			newArgs[i] = unique.Make(v).Value()
		case MangleAtom:
			newArgs[i] = MangleAtom(unique.Make(string(v)).Value())
		}
	}
	f.Args = newArgs
	return f
}
