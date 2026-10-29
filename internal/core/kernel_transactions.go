package core

import (
	"errors"
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
//
// Validation runs before any mutation: a no-args RetractFact would otherwise
// silently remove a whole predicate (RealKernel.RetractFact rejects those
// outright). Assert rejections are reported AssertBatch-style — the good
// facts still land, but the caller learns which ones did not — and asserted
// predicates are published on the event bus like any other Assert.
func (tx *KernelTransaction) Commit() error {
	if tx == nil || tx.kernel == nil {
		return fmt.Errorf("transaction commit: nil transaction or kernel")
	}
	if tx.committed {
		return fmt.Errorf("transaction already committed")
	}
	tx.committed = true

	k := tx.kernel
	if k.simulateCommitErr != nil {
		return k.simulateCommitErr
	}

	for _, rf := range tx.retractFacts {
		if len(rf.Args) == 0 {
			return fmt.Errorf("transaction rejected: retractFact %s has no args (would remove the whole predicate)", rf.Predicate)
		}
	}

	k.mu.Lock()

	timer := logging.StartTimer(logging.CategoryKernel, "Transaction.Commit")
	defer timer.Stop()

	// Phase 1: every retraction, compiled into one plan and applied in one
	// pass over the EDB (kernel_retract_plan.go). A plan whose predicates the
	// EDB holds no rows of removes nothing, so the pass is skipped.
	mutated := false
	plan := compileRetractions(tx)
	k.ensureFactIndexLocked()
	for pred := range plan.predicates() {
		if len(k.factIndex.bucket(pred)) > 0 {
			mutated = k.compactFactsLocked(plan.keeps) > 0
			break
		}
	}

	// The dedupe index followed each removal (compactFactsLocked).
	if mutated {
		k.cachedAtoms = nil // Invalidate atom cache
	}

	// Phase 2: Asserts. Rejections are collected, not swallowed: the good
	// facts still land but the caller is told which ones did not.
	assertCount := 0
	assertedPredicates := make(map[string]struct{})
	var rejected []error
	for _, f := range tx.assertFacts {
		f = sanitizeFactForNumericPredicates(f)
		added, addErr := k.addFactIfNewLockedErr(f)
		if addErr != nil {
			rejected = append(rejected, addErr)
			continue
		}
		if added {
			assertCount++
			assertedPredicates[f.Predicate] = struct{}{}
		}
	}

	totalOps := len(tx.retractPredicates) + len(tx.retractPredicateSet) +
		len(tx.retractFacts) + len(tx.retractExactFacts) + len(tx.assertFacts)

	logging.KernelDebug("Transaction.Commit: %d ops (%d retracts, %d asserts, %d new), single rebuild",
		totalOps,
		len(tx.retractPredicates)+len(tx.retractPredicateSet)+len(tx.retractFacts)+len(tx.retractExactFacts),
		len(tx.assertFacts),
		assertCount)

	// Phase 3: Single rebuild/evaluate
	if mutated || assertCount > 0 {
		written := make(map[string]struct{}, len(assertedPredicates)+4)
		for pred := range assertedPredicates {
			written[pred] = struct{}{}
		}
		for _, pred := range tx.retractPredicates {
			written[pred] = struct{}{}
		}
		for pred := range tx.retractPredicateSet {
			written[pred] = struct{}{}
		}
		for _, rf := range tx.retractFacts {
			written[rf.Predicate] = struct{}{}
		}
		for _, rf := range tx.retractExactFacts {
			written[rf.Predicate] = struct{}{}
		}
		if err := k.rebuild(predicateNames(written)...); err != nil {
			k.mu.Unlock()
			logging.Get(logging.CategoryKernel).Error("Transaction.Commit: rebuild failed: %v", err)
			return err
		}
	}
	k.mu.Unlock()

	// Publish AFTER releasing lock — one event per asserted predicate,
	// matching standalone Assert.
	if k.eventBus != nil {
		for pred := range assertedPredicates {
			k.eventBus.Publish(pred)
		}
	}
	if len(rejected) > 0 {
		return fmt.Errorf("Transaction.Commit added %d of %d facts; %d rejected: %w",
			assertCount, len(tx.assertFacts), len(rejected), errors.Join(rejected...))
	}

	return nil
}
