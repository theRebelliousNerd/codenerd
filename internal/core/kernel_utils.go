package core

import (
	"codenerd/internal/mangle/feedback"
	"codenerd/internal/types"
	"strings"
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

// AssertFactBatch implements types.KernelInterface.
// This is much faster than calling AssertFact in a loop because it only evaluates once.
func (ab *AutopoiesisBridge) AssertFactBatch(facts []types.KernelFact) error {
	if len(facts) == 0 {
		return nil
	}

	coreFacts := make([]Fact, len(facts))
	for i, f := range facts {
		coreFacts[i] = Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return ab.kernel.AssertBatch(coreFacts)
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

// Assert implements a string-based fact assertion (MCP-style).
func (ab *AutopoiesisBridge) Assert(fact string) error {
	trimmed := strings.TrimSpace(fact)
	trimmed = strings.TrimSuffix(trimmed, ".")
	parsed, err := ParseFactString(trimmed)
	if err != nil {
		return err
	}
	return ab.kernel.Assert(parsed)
}

// Query implements a string-based query that returns bindings (MCP-style).
func (ab *AutopoiesisBridge) Query(query string) ([]map[string]interface{}, error) {
	queryFact, err := ParseFactString(strings.TrimSuffix(strings.TrimSpace(query), "."))
	if err != nil {
		return nil, err
	}

	variableMap := make(map[int]string)
	for i, arg := range queryFact.Args {
		if s, ok := arg.(string); ok && strings.HasPrefix(s, "?") {
			variableMap[i] = s[1:]
		}
	}

	facts, err := ab.kernel.Query(query)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, len(facts))
	for _, f := range facts {
		binding := make(map[string]interface{})
		if len(variableMap) > 0 {
			for idx, varName := range variableMap {
				if idx < len(f.Args) {
					binding[varName] = f.Args[idx]
				}
			}
		}
		results = append(results, binding)
	}

	return results, nil
}

// Retract removes an exact fact using a string-based fact representation.
func (ab *AutopoiesisBridge) Retract(fact string) error {
	trimmed := strings.TrimSpace(fact)
	trimmed = strings.TrimSuffix(trimmed, ".")
	parsed, err := ParseFactString(trimmed)
	if err != nil {
		return err
	}
	return ab.kernel.RetractExactFact(parsed)
}

// AssertFact adds a fact using the KernelInterface representation.
func (k *RealKernel) AssertFact(fact types.KernelFact) error {
	return k.Assert(fact.ToFact())
}

// AssertFactBatch adds multiple facts and evaluates once (much faster than multiple AssertFact calls).
// Implements types.KernelInterface.
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

// Ensure AutopoiesisBridge implements KernelInterface at compile time.
var _ types.KernelInterface = (*AutopoiesisBridge)(nil)

// Ensure RealKernel implements feedback.RuleValidator at compile time.
var _ feedback.RuleValidator = (*RealKernel)(nil)
