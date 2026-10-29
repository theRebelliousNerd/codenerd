package core

import "math"

// retractionPlan is a transaction's buffered retractions compiled into sets,
// so Commit removes all of them in one pass over the EDB.
//
// Commit used to run one compaction pass per retraction: K exact retractions
// over N facts cost K full scans, and orientation's refresh queues one exact
// retraction per removed row (Docs/audits/mangle-kernel-performance.md §5).
// The plan makes the pass O(N) with a constant-time membership test per fact.
//
// Equality is not changed by the compilation. A retraction matches exactly the
// facts argsEqual / argsSliceEqual matched before, including their cross-type
// cases (int and int64, string and MangleAtom). Hash keys only select the
// candidates to confirm with those functions: retractKey gives every pair of
// values argsEqual can equate the same key, and values it cannot key (maps,
// slices, named types) are kept in a short list checked linearly, which is
// sound because argsEqual never equates a keyable value with an unkeyable one.
// Exact retractions are bucketed by arity and first argument; a key collision
// only adds a candidate to confirm. Keys are comparable structs, so testing a
// fact allocates nothing.
type retractionPlan struct {
	whole  map[string]struct{}              // predicates retracted entirely
	byPred map[string]*predicateRetractions // first-argument and exact conditions
}

type predicateRetractions struct {
	firstArg  map[valueKey][]any   // key of the first arg -> retracted first args
	exact     map[exactKey][][]any // arity and first-arg key -> retracted arg lists
	slowFirst []any                // first args with no key, checked linearly
	slowExact [][]any              // arg lists whose first arg has no key
}

// valueKey is a hash key under which argsEqual-equal scalars collide.
type valueKey struct {
	kind byte // 'n' nil, 's' string/name, 'b' bool, 'i' signed, 'u' unsigned, 'f' float
	s    string
	n    uint64
}

type exactKey struct {
	arity int
	first valueKey
}

// compileRetractions builds the plan of a transaction's retractions.
func compileRetractions(tx *KernelTransaction) *retractionPlan {
	p := &retractionPlan{
		whole:  make(map[string]struct{}, len(tx.retractPredicates)+len(tx.retractPredicateSet)),
		byPred: make(map[string]*predicateRetractions),
	}
	for _, pred := range tx.retractPredicates {
		p.whole[pred] = struct{}{}
	}
	for pred := range tx.retractPredicateSet {
		p.whole[pred] = struct{}{}
	}
	conditions := func(pred string) *predicateRetractions {
		pr := p.byPred[pred]
		if pr == nil {
			pr = &predicateRetractions{}
			p.byPred[pred] = pr
		}
		return pr
	}
	for _, rf := range tx.retractFacts {
		if len(rf.Args) == 0 {
			continue // Commit rejects these before compiling
		}
		pr := conditions(rf.Predicate)
		if key, ok := retractKey(rf.Args[0]); ok {
			if pr.firstArg == nil {
				pr.firstArg = make(map[valueKey][]any)
			}
			pr.firstArg[key] = append(pr.firstArg[key], rf.Args[0])
		} else {
			pr.slowFirst = append(pr.slowFirst, rf.Args[0])
		}
	}
	for _, rf := range tx.retractExactFacts {
		pr := conditions(rf.Predicate)
		if key, ok := retractArgsKey(rf.Args); ok {
			if pr.exact == nil {
				pr.exact = make(map[exactKey][][]any)
			}
			pr.exact[key] = append(pr.exact[key], rf.Args)
		} else {
			pr.slowExact = append(pr.slowExact, rf.Args)
		}
	}
	return p
}

// predicates names every predicate the plan can remove facts of.
func (p *retractionPlan) predicates() map[string]struct{} {
	out := make(map[string]struct{}, len(p.whole)+len(p.byPred))
	for pred := range p.whole {
		out[pred] = struct{}{}
	}
	for pred := range p.byPred {
		out[pred] = struct{}{}
	}
	return out
}

// keeps is the compaction predicate: a fact survives unless a retraction
// matches it.
func (p *retractionPlan) keeps(f Fact) bool {
	return !p.removes(f)
}

// removes reports whether any compiled retraction matches f.
func (p *retractionPlan) removes(f Fact) bool {
	if _, ok := p.whole[f.Predicate]; ok {
		return true
	}
	pr := p.byPred[f.Predicate]
	if pr == nil {
		return false
	}
	if len(f.Args) > 0 && (pr.firstArg != nil || pr.slowFirst != nil) {
		if key, ok := retractKey(f.Args[0]); ok {
			for _, v := range pr.firstArg[key] {
				if argsEqual(f.Args[0], v) {
					return true
				}
			}
		} else {
			for _, v := range pr.slowFirst {
				if argsEqual(f.Args[0], v) {
					return true
				}
			}
		}
	}
	if pr.exact != nil || pr.slowExact != nil {
		if key, ok := retractArgsKey(f.Args); ok {
			for _, args := range pr.exact[key] {
				if argsSliceEqual(f.Args, args) {
					return true
				}
			}
		} else {
			for _, args := range pr.slowExact {
				if argsSliceEqual(f.Args, args) {
					return true
				}
			}
		}
	}
	return false
}

// retractKey keys v so that argsEqual-equal values share a key. It reports
// false for values argsEqual compares structurally or by exact type (maps,
// slices, named types); those never equal a keyable value.
func retractKey(v any) (valueKey, bool) {
	switch t := v.(type) {
	case nil:
		return valueKey{kind: 'n'}, true
	case string:
		return valueKey{kind: 's', s: t}, true
	case MangleAtom:
		return valueKey{kind: 's', s: string(t)}, true
	case bool:
		if t {
			return valueKey{kind: 'b', n: 1}, true
		}
		return valueKey{kind: 'b'}, true
	case int:
		return valueKey{kind: 'i', n: uint64(int64(t))}, true
	case int8:
		return valueKey{kind: 'i', n: uint64(int64(t))}, true
	case int16:
		return valueKey{kind: 'i', n: uint64(int64(t))}, true
	case int32:
		return valueKey{kind: 'i', n: uint64(int64(t))}, true
	case int64:
		return valueKey{kind: 'i', n: uint64(t)}, true
	case uint:
		return valueKey{kind: 'u', n: uint64(t)}, true
	case uint8:
		return valueKey{kind: 'u', n: uint64(t)}, true
	case uint16:
		return valueKey{kind: 'u', n: uint64(t)}, true
	case uint32:
		return valueKey{kind: 'u', n: uint64(t)}, true
	case uint64:
		return valueKey{kind: 'u', n: t}, true
	case float32:
		return floatRetractKey(float64(t)), true
	case float64:
		return floatRetractKey(t), true
	default:
		return valueKey{}, false
	}
}

// floatRetractKey keys -0 with 0, which == (and so argsEqual) equates.
func floatRetractKey(v float64) valueKey {
	if v == 0 {
		return valueKey{kind: 'f'}
	}
	return valueKey{kind: 'f', n: math.Float64bits(v)}
}

// retractArgsKey buckets an argument list by its arity and first argument.
func retractArgsKey(args []any) (exactKey, bool) {
	if len(args) == 0 {
		return exactKey{}, true
	}
	first, ok := retractKey(args[0])
	return exactKey{arity: len(args), first: first}, ok
}
