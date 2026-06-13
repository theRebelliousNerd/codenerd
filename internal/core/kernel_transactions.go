package core

import (
	"fmt"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// TRANSACTION API
// =============================================================================

// KernelTransaction buffers retract + assert operations and executes them
// atomically with a single rebuild(). This avoids the performance penalty
// of N separate retracts/asserts each triggering a full fixpoint evaluation.
//
// Usage:
//
//	tx := kernel.Transaction()
//	tx.Retract("user_intent")
//	tx.Retract("pending_action")
//	tx.Assert(Fact{Predicate: "user_intent", Args: [...]})
//	if err := tx.Commit(); err != nil { ... }
type KernelTransaction struct {
	kernel *RealKernel

	// Pending operations
	retractPredicates   []string            // Retract all facts of predicate
	retractFacts        []Fact              // Retract by predicate + first arg
	retractExactFacts   []Fact              // Retract by predicate + all args
	retractPredicateSet map[string]struct{} // Retract all facts for predicate set
	assertFacts         []Fact              // Facts to assert

	committed bool
}

// Transaction creates a new kernel transaction.
// Buffer retract/assert operations, then call Commit() to apply them atomically.
// Implements types.KernelTransactor.
func (k *RealKernel) Transaction() types.KernelTransaction {
	return &KernelTransaction{
		kernel:              k,
		retractPredicateSet: make(map[string]struct{}),
	}
}

// Retract queues removal of all facts with the given predicate.
func (tx *KernelTransaction) Retract(predicate string) {
	tx.retractPredicates = append(tx.retractPredicates, predicate)
}

// RetractFact queues removal of facts matching predicate + first argument.
func (tx *KernelTransaction) RetractFact(fact Fact) {
	tx.retractFacts = append(tx.retractFacts, fact)
}

// RetractExactFact queues removal of facts matching predicate + all arguments.
func (tx *KernelTransaction) RetractExactFact(fact Fact) {
	tx.retractExactFacts = append(tx.retractExactFacts, fact)
}

// RetractPredicateSet queues removal of all facts in a predicate set.
func (tx *KernelTransaction) RetractPredicateSet(predicates map[string]struct{}) {
	for p := range predicates {
		tx.retractPredicateSet[p] = struct{}{}
	}
}

// Assert queues a fact for insertion.
func (tx *KernelTransaction) Assert(fact Fact) {
	tx.assertFacts = append(tx.assertFacts, fact)
}

// Commit applies all buffered operations atomically under a single lock,
// then triggers exactly one rebuild()/evaluate().
func (tx *KernelTransaction) Commit() error {
	if tx.committed {
		return fmt.Errorf("transaction already committed")
	}
	tx.committed = true

	k := tx.kernel
	if k.simulateCommitErr != nil {
		return k.simulateCommitErr
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	timer := logging.StartTimer(logging.CategoryKernel, "Transaction.Commit")
	defer timer.Stop()

	mutated := false

	// Phase 1: Retracts (by full predicate)
	for _, pred := range tx.retractPredicates {
		if tx.retractByPredicateLocked(k, pred) {
			mutated = true
		}
	}

	// Phase 2: Retracts (by predicate set)
	if len(tx.retractPredicateSet) > 0 {
		for _, f := range k.facts {
			if _, ok := tx.retractPredicateSet[f.Predicate]; ok {
				mutated = true
				break
			}
		}
		if mutated {
			filtered := make([]Fact, 0, len(k.facts))
			for _, f := range k.facts {
				if _, ok := tx.retractPredicateSet[f.Predicate]; !ok {
					filtered = append(filtered, f)
				}
			}
			k.facts = filtered
		}
	}

	// Phase 3: Retracts (by predicate + first arg)
	for _, rf := range tx.retractFacts {
		if tx.retractFactLocked(k, rf) {
			mutated = true
		}
	}

	// Phase 4: Retracts (exact match)
	for _, rf := range tx.retractExactFacts {
		if tx.retractExactFactLocked(k, rf) {
			mutated = true
		}
	}

	// Rebuild index after all retracts
	if mutated {
		k.cachedAtoms = nil // Invalidate atom cache
		k.rebuildFactIndexLocked()
	}

	// Phase 5: Asserts
	assertCount := 0
	for _, f := range tx.assertFacts {
		f = sanitizeFactForNumericPredicates(f)
		if k.addFactIfNewLocked(f) {
			assertCount++
		}
	}

	totalOps := len(tx.retractPredicates) + len(tx.retractPredicateSet) +
		len(tx.retractFacts) + len(tx.retractExactFacts) + len(tx.assertFacts)

	logging.KernelDebug("Transaction.Commit: %d ops (%d retracts, %d asserts, %d new), single rebuild",
		totalOps,
		len(tx.retractPredicates)+len(tx.retractPredicateSet)+len(tx.retractFacts)+len(tx.retractExactFacts),
		len(tx.assertFacts),
		assertCount)

	// Phase 6: Single rebuild/evaluate
	if mutated || assertCount > 0 {
		if err := k.rebuild(); err != nil {
			logging.Get(logging.CategoryKernel).Error("Transaction.Commit: rebuild failed: %v", err)
			return err
		}
	}

	return nil
}

// retractByPredicateLocked removes all facts with a predicate. Caller holds k.mu.
func (tx *KernelTransaction) retractByPredicateLocked(k *RealKernel, predicate string) bool {
	n := 0
	retracted := false
	for _, f := range k.facts {
		if f.Predicate != predicate {
			k.facts[n] = f
			n++
		} else {
			retracted = true
		}
	}
	// Zero tail for GC
	for i := n; i < len(k.facts); i++ {
		k.facts[i] = Fact{}
	}
	k.facts = k.facts[:n]
	return retracted
}

// retractFactLocked removes facts matching predicate + first arg. Caller holds k.mu.
func (tx *KernelTransaction) retractFactLocked(k *RealKernel, fact Fact) bool {
	if len(fact.Args) == 0 {
		return tx.retractByPredicateLocked(k, fact.Predicate)
	}
	n := 0
	retracted := false
	for _, f := range k.facts {
		if f.Predicate == fact.Predicate && len(f.Args) > 0 && argsEqual(f.Args[0], fact.Args[0]) {
			retracted = true
		} else {
			k.facts[n] = f
			n++
		}
	}
	for i := n; i < len(k.facts); i++ {
		k.facts[i] = Fact{}
	}
	k.facts = k.facts[:n]
	return retracted
}

// retractExactFactLocked removes facts matching predicate + all args. Caller holds k.mu.
func (tx *KernelTransaction) retractExactFactLocked(k *RealKernel, fact Fact) bool {
	n := 0
	retracted := false
	for _, f := range k.facts {
		if f.Predicate == fact.Predicate && len(f.Args) == len(fact.Args) {
			match := true
			for j := range f.Args {
				if !argsEqual(f.Args[j], fact.Args[j]) {
					match = false
					break
				}
			}
			if match {
				retracted = true
				continue
			}
		}
		k.facts[n] = f
		n++
	}
	for i := n; i < len(k.facts); i++ {
		k.facts[i] = Fact{}
	}
	k.facts = k.facts[:n]
	return retracted
}
