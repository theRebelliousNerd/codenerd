package core

import (
	"math"
	"slices"
)

// =============================================================================
// KERNEL WRITES TO THE EDB
// =============================================================================
// Every write keeps the ordered storage (kernel_edb_facts.go) and the index
// (kernel_edb_index.go) in step. Callers hold k.mu for writing.

// storeFactLocked appends f under key to the EDB and the index. It reports
// false, storing nothing, when a fact with that key is already held.
func (k *RealKernel) storeFactLocked(key string, f Fact) bool {
	k.ensureFactIndexLocked()
	if k.factIndex.has(f.Predicate, key) {
		return false
	}
	pos := k.facts.append(f, key)
	k.factIndex.insert(key, f, pos)
	return true
}

// removeKeyLocked drops the fact held under key from the storage and the
// index: one slot, found through the index. It does not compact; callers that
// remove several facts call settleRemovalsLocked once at the end.
func (k *RealKernel) removeKeyLocked(pred, key string) bool {
	if k.factIndex == nil {
		return false
	}
	pos, held := k.factIndex.slotOf(pred, key)
	if !held {
		return false
	}
	k.facts.remove(pos)
	k.factIndex.remove(pred, key)
	return true
}

// settleRemovalsLocked reclaims tombstones once they outnumber a quarter of
// the slots, and tells the index where the survivors moved.
func (k *RealKernel) settleRemovalsLocked() {
	if !k.facts.needsCompaction() {
		return
	}
	k.facts.compact(func(f Fact, key string, pos int) {
		k.factIndex.setSlot(f.Predicate, key, pos)
	})
}

// retractMatchingLocked removes every held fact of pred that match accepts,
// walking pred's index bucket: the cost is pred's rows, not the EDB's. It
// returns how many facts it removed.
func (k *RealKernel) retractMatchingLocked(pred string, match func(Fact) bool) int {
	if k.factIndex == nil {
		return 0
	}
	var keys []string
	for key, f := range k.factIndex.eachRow(pred) {
		if match(f) {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		k.removeKeyLocked(pred, key)
	}
	k.settleRemovalsLocked()
	return len(keys)
}

// maxZeroVariants bounds the signed-zero spellings retractExactLocked tries
// before it walks the bucket instead.
const maxZeroVariants = 6

// signedZero is +0 or -0.
func signedZero(negative bool) float64 {
	if negative {
		return math.Copysign(0, -1)
	}
	return 0
}

// exactKeyVariants lists the canonical keys of every stored fact argsEqual
// could equate with f. For scalar arguments that is one key, except that
// argsEqual equates 0 with -0 while the canonical key spells them apart, so
// each zero float contributes both spellings. It reports false for facts it
// cannot bound (a composite or unkeyed argument, too many zeros); the caller
// then compares the predicate's rows.
func (k *RealKernel) exactKeyVariants(f Fact) ([]string, bool) {
	var zeros []int
	for i, a := range f.Args {
		if _, keyable := retractKey(a); !keyable {
			return nil, false
		}
		switch v := a.(type) {
		case float64:
			if v == 0 {
				zeros = append(zeros, i)
			}
		case float32:
			if v == 0 {
				zeros = append(zeros, i)
			}
		}
	}
	if len(zeros) > maxZeroVariants {
		return nil, false
	}
	keys := make([]string, 0, 1<<len(zeros))
	for mask := 0; mask < 1<<len(zeros); mask++ {
		args := f.Args
		if len(zeros) > 0 {
			args = slices.Clone(f.Args)
			for bit, i := range zeros {
				negative := mask&(1<<bit) != 0
				switch args[i].(type) {
				case float64:
					args[i] = signedZero(negative)
				case float32:
					args[i] = float32(signedZero(negative))
				}
			}
		}
		keys = append(keys, k.canonFact(Fact{Predicate: f.Predicate, Args: args}))
	}
	return keys, true
}

// retractExactLocked removes the facts whose arguments equal f's by argsEqual:
// an index lookup per spelling (exactKeyVariants) and one slot each, where it
// used to compare every fact of the EDB. The comparison itself still confirms
// each match, as it always did.
func (k *RealKernel) retractExactLocked(f Fact) int {
	if k.factIndex == nil {
		return 0
	}
	keys, bounded := k.exactKeyVariants(f)
	if !bounded {
		return k.retractMatchingLocked(f.Predicate, func(held Fact) bool {
			return argsSliceEqual(held.Args, f.Args)
		})
	}
	var matched []string
	for _, key := range keys {
		if held, ok := k.factIndex.row(f.Predicate, key); ok && argsSliceEqual(held.Args, f.Args) {
			matched = append(matched, key)
		}
	}
	for _, key := range matched {
		k.removeKeyLocked(f.Predicate, key)
	}
	k.settleRemovalsLocked()
	return len(matched)
}

// retractKeysLocked removes the facts held under the canonical keys of facts.
func (k *RealKernel) retractKeysLocked(facts []Fact) (removed int) {
	for _, f := range facts {
		if k.removeKeyLocked(f.Predicate, k.canonFact(f)) {
			removed++
		}
	}
	k.settleRemovalsLocked()
	return removed
}

// retractPlanLocked applies a transaction's compiled retractions. Exact ones
// are one index lookup each; the predicates retracted whole or by first
// argument walk only their own bucket.
func (k *RealKernel) retractPlanLocked(plan *retractionPlan) (removed int) {
	if k.factIndex == nil {
		return 0
	}
	for pred := range plan.scanned() {
		removed += k.retractMatchingLocked(pred, plan.removes)
	}
	for pred, pr := range plan.byPred {
		if plan.scans(pred) {
			continue
		}
		for _, args := range pr.allExact() {
			removed += k.retractExactLocked(Fact{Predicate: pred, Args: args})
		}
	}
	return removed
}

// indexFactsLocked builds a fresh index over the stored facts, reusing the
// canonical key each slot already carries.
func (k *RealKernel) indexFactsLocked() {
	k.factIndex = newEDBIndex()
	pos := 0
	for _, c := range k.facts.chunks {
		for _, s := range c.slots {
			if s.key != "" {
				k.factIndex.insert(s.key, s.fact, pos)
			}
			pos++
		}
	}
}
