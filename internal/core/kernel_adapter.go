// Package core provides the kernel adapter for autopoiesis integration.
// This file implements the KernelInterface required by the autopoiesis package,
// bridging between the autopoiesis types and the core kernel types.
package core

import (
	"codenerd/internal/autopoiesis"
)

// KernelAdapter wraps RealKernel to implement autopoiesis.KernelInterface.
// This adapter bridges the autopoiesis package to the core kernel without
// creating import cycles.
type KernelAdapter struct {
	kernel *RealKernel
}

// NewKernelAdapter creates an adapter that implements autopoiesis.KernelInterface.
func NewKernelAdapter(kernel *RealKernel) *KernelAdapter {
	return &KernelAdapter{kernel: kernel}
}

// AssertFact implements autopoiesis.KernelInterface.
// Converts autopoiesis.KernelFact to core.Fact and asserts to kernel.
func (ka *KernelAdapter) AssertFact(fact autopoiesis.KernelFact) error {
	coreFact := Fact{
		Predicate: fact.Predicate,
		Args:      fact.Args,
	}
	return ka.kernel.Assert(coreFact)
}

// QueryPredicate implements autopoiesis.KernelInterface.
// Queries the kernel and converts results to autopoiesis.KernelFact.
func (ka *KernelAdapter) QueryPredicate(predicate string) ([]autopoiesis.KernelFact, error) {
	facts, err := ka.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}

	result := make([]autopoiesis.KernelFact, len(facts))
	for i, f := range facts {
		result[i] = autopoiesis.KernelFact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return result, nil
}

// QueryBool implements autopoiesis.KernelInterface.
// Returns true if the query returns any facts.
func (ka *KernelAdapter) QueryBool(predicate string) bool {
	facts, err := ka.kernel.Query(predicate)
	if err != nil {
		return false
	}
	return len(facts) > 0
}

// RetractFact implements autopoiesis.KernelInterface.
func (ka *KernelAdapter) RetractFact(fact autopoiesis.KernelFact) error {
	coreFact := Fact{
		Predicate: fact.Predicate,
		Args:      fact.Args,
	}
	return ka.kernel.RetractFact(coreFact)
}

// Ensure KernelAdapter implements KernelInterface at compile time.
var _ autopoiesis.KernelInterface = (*KernelAdapter)(nil)
