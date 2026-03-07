package types

import "codenerd/internal/logging"

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

type KernelTx struct {
	tx KernelTransaction
}

// NewKernelTx creates a new transaction wrapper. The kernel MUST support
// KernelTransactor, allowing operations to be buffered for an atomic commit.
// The non-atomic fallback has been removed to ensure strict transactional integrity.
func NewKernelTx(k Kernel) *KernelTx {
	transactor, ok := k.(KernelTransactor)
	if !ok {
		logging.Get(logging.CategoryKernel).Warn("Kernel does not implement KernelTransactor; non-atomic fallback has been removed")
		panic("Kernel requires KernelTransactor for atomic transactions")
	}
	return &KernelTx{tx: transactor.Transaction()}
}

// Retract queues removal of all facts with the given predicate.
func (t *KernelTx) Retract(predicate string) {
	t.tx.Retract(predicate)
}

// RetractFact queues removal of facts matching predicate + first argument.
func (t *KernelTx) RetractFact(fact Fact) {
	t.tx.RetractFact(fact)
}

// RetractExactFact queues removal of facts matching predicate + all arguments.
func (t *KernelTx) RetractExactFact(fact Fact) {
	t.tx.RetractExactFact(fact)
}

// RetractPredicateSet queues removal of all facts in a predicate set.
func (t *KernelTx) RetractPredicateSet(predicates map[string]struct{}) {
	t.tx.RetractPredicateSet(predicates)
}

// Assert queues a fact for insertion.
func (t *KernelTx) Assert(fact Fact) {
	t.tx.Assert(fact)
}

// LoadFacts queues multiple facts for insertion. Each fact
// is buffered as an individual Assert.
func (t *KernelTx) LoadFacts(facts []Fact) {
	for _, f := range facts {
		t.tx.Assert(f)
	}
}

// Commit applies all buffered operations atomically with a single rebuild.
func (t *KernelTx) Commit() error {
	return t.tx.Commit()
}
