package core

import (
	"codenerd/internal/mangle/feedback"
	"codenerd/internal/types"
)

// AssertFact adds a fact using the KernelFact representation.
func (k *RealKernel) AssertFact(fact types.KernelFact) error {
	return k.Assert(fact.ToFact())
}

// AssertFactBatch adds multiple facts and evaluates once (much faster than multiple AssertFact calls).
func (k *RealKernel) AssertFactBatch(facts []types.KernelFact) error {
	// Convert to core.Fact slice
	coreFacts := make([]Fact, len(facts))
	for i, f := range facts {
		coreFacts[i] = f.ToFact()
	}
	return k.AssertBatch(coreFacts)
}

// QueryPredicate queries for facts matching a predicate string.
func (k *RealKernel) QueryPredicate(predicate string) ([]types.KernelFact, error) {
	facts, err := k.Query(predicate)
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

// QueryBool returns true when any facts match a predicate string.
func (k *RealKernel) QueryBool(predicate string) bool {
	facts, err := k.Query(predicate)
	if err != nil {
		return false
	}
	return len(facts) > 0
}


// Ensure RealKernel implements feedback.RuleValidator at compile time.
var _ feedback.RuleValidator = (*RealKernel)(nil)
