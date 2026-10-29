package core

import "fmt"

// settleEvaluationLocked records how an evaluation ended. It is the single
// place that decides whether k.store may be read.
//
// A failed evaluation leaves no fixpoint behind: the cone path edits k.store
// in place (removing and re-deriving the touched predicates), so a failure
// after its first removal leaves a half-rebuilt store, and a failed full
// evaluation leaves the previous store, which no longer matches the facts.
// Callers that stage with AssertWithoutEval and evaluate with EvaluatePending
// or Evaluate do not raise factsDirty, so without this a later read took
// ensureEvaluated's clean fast path and returned the unfinished store as a
// success. A failure therefore raises factsDirty (the next read retries the
// evaluation, which repairs the store or reports its error again) and latches
// evalErr, which readers check under the read lock, closing the window between
// ensureEvaluated and RLock in which another goroutine's evaluation can fail.
//
// Caller holds k.mu.
func (k *RealKernel) settleEvaluationLocked(err error) {
	k.evalErr = err
	if err != nil {
		k.factsDirty.Store(true)
	}
}

// readableLocked is nil when k.store is the result of a completed evaluation.
// Caller holds k.mu (read or write).
func (k *RealKernel) readableLocked() error {
	if k.evalErr != nil {
		return fmt.Errorf("kernel facts are unavailable, the last evaluation failed: %w", k.evalErr)
	}
	return nil
}
