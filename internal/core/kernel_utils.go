package core

import (
	"codenerd/internal/mangle/feedback"
	"codenerd/internal/types"
)

// =============================================================================
// AUTOPOIESIS BRIDGE (Formerly Kernel Adapter)
// =============================================================================

// AutopoiesisBridge wraps RealKernel to implement types.KernelInterface.
type AutopoiesisBridge struct {
	kernel *RealKernel
}

// NewAutopoiesisBridge creates an adapter that implements types.KernelInterface.
func NewAutopoiesisBridge(kernel *RealKernel) *AutopoiesisBridge {
	return &AutopoiesisBridge{kernel: kernel}
}

// AssertFact implements types.KernelInterface.
func (ab *AutopoiesisBridge) AssertFact(fact types.KernelFact) error {
	coreFact := Fact{
		Predicate: fact.Predicate,
		Args:      fact.Args,
	}
	return ab.kernel.Assert(coreFact)
}

// QueryPredicate implements types.KernelInterface.
func (ab *AutopoiesisBridge) QueryPredicate(predicate string) ([]types.KernelFact, error) {
	facts, err := ab.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}

	result := make([]types.KernelFact, len(facts))
	for i, f := range facts {
		result[i] = types.KernelFact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return result, nil
}

// QueryBool implements types.KernelInterface.
func (ab *AutopoiesisBridge) QueryBool(predicate string) bool {
	facts, err := ab.kernel.Query(predicate)
	if err != nil {
		return false
	}
	return len(facts) > 0
}

// RetractFact implements types.KernelInterface.
func (ab *AutopoiesisBridge) RetractFact(fact types.KernelFact) error {
	coreFact := Fact{
		Predicate: fact.Predicate,
		Args:      fact.Args,
	}
	return ab.kernel.RetractFact(coreFact)
}

// Ensure AutopoiesisBridge implements KernelInterface at compile time.
var _ types.KernelInterface = (*AutopoiesisBridge)(nil)

// Ensure RealKernel implements feedback.RuleValidator at compile time.
var _ feedback.RuleValidator = (*RealKernel)(nil)
