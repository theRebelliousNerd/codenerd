package types

// KernelTransaction defines the interface for atomic kernel fact operations.
// Buffer retract/assert operations, then call Commit() to apply them atomically
// with a single rebuild/evaluate cycle instead of N separate rebuilds.
type KernelTransaction interface {
	Retract(predicate string)
	RetractFact(fact Fact)
	RetractExactFact(fact Fact)
	RetractPredicateSet(predicates map[string]struct{})
	Assert(fact Fact)
	Commit() error
}

// KernelTransactor is an optional interface for Kernel implementations
// that support atomic transactions. Use types.NewKernelTx() for a convenience
// wrapper that handles the type assertion and fallback automatically.
//
// Direct usage via type assertion:
//
//	if kt, ok := kernel.(types.KernelTransactor); ok {
//	    tx := kt.Transaction()
//	    tx.Retract("foo")
//	    tx.Assert(Fact{...})
//	    tx.Commit()
//	}
type KernelTransactor interface {
	Transaction() KernelTransaction
}

// KernelTx is a convenience wrapper that uses atomic transactions when
// the kernel supports KernelTransactor, falling back to direct (non-atomic)
// calls otherwise. This avoids code duplication at call sites and ensures
// test mocks work without implementing KernelTransactor.
//
// Usage:
//
//	tx := types.NewKernelTx(kernel)
//	tx.Retract("user_intent")
//	tx.Retract("pending_action")
//	tx.Assert(Fact{Predicate: "user_intent", Args: [...]})
//	if err := tx.Commit(); err != nil { ... }
type KernelTx struct {
	kernel Kernel
	tx     KernelTransaction // nil if kernel doesn't support transactions
}

// NewKernelTx creates a new transaction wrapper. If the kernel supports
// KernelTransactor, operations are buffered for atomic commit (single rebuild).
// Otherwise, operations execute immediately (non-atomic fallback for mocks).
func NewKernelTx(k Kernel) *KernelTx {
	kt := &KernelTx{kernel: k}
	if transactor, ok := k.(KernelTransactor); ok {
		kt.tx = transactor.Transaction()
	}
	return kt
}

// Retract queues removal of all facts with the given predicate.
func (t *KernelTx) Retract(predicate string) {
	if t.tx != nil {
		t.tx.Retract(predicate)
	} else {
		_ = t.kernel.Retract(predicate)
	}
}

// RetractFact queues removal of facts matching predicate + first argument.
func (t *KernelTx) RetractFact(fact Fact) {
	if t.tx != nil {
		t.tx.RetractFact(fact)
	} else {
		_ = t.kernel.RetractFact(fact)
	}
}

// RetractExactFact queues removal of facts matching predicate + all arguments.
func (t *KernelTx) RetractExactFact(fact Fact) {
	if t.tx != nil {
		t.tx.RetractExactFact(fact)
	} else {
		_ = t.kernel.RetractExactFactsBatch([]Fact{fact})
	}
}

// RetractPredicateSet queues removal of all facts in a predicate set.
func (t *KernelTx) RetractPredicateSet(predicates map[string]struct{}) {
	if t.tx != nil {
		t.tx.RetractPredicateSet(predicates)
	} else {
		_ = t.kernel.RemoveFactsByPredicateSet(predicates)
	}
}

// Assert queues a fact for insertion.
func (t *KernelTx) Assert(fact Fact) {
	if t.tx != nil {
		t.tx.Assert(fact)
	} else {
		_ = t.kernel.Assert(fact)
	}
}

// LoadFacts queues multiple facts for insertion. When transactional, each fact
// is buffered as an individual Assert. When non-transactional, delegates to
// the kernel's LoadFacts for batch efficiency.
func (t *KernelTx) LoadFacts(facts []Fact) {
	if t.tx != nil {
		for _, f := range facts {
			t.tx.Assert(f)
		}
	} else {
		_ = t.kernel.LoadFacts(facts)
	}
}

// Commit applies all buffered operations atomically with a single rebuild.
// For non-transactional kernels, this is a no-op (operations already applied).
func (t *KernelTx) Commit() error {
	if t.tx != nil {
		return t.tx.Commit()
	}
	return nil
}
