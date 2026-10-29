package campaign

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

type BenchmarkMockKernel struct {
	types.Kernel
	Capability string
}

func (m *BenchmarkMockKernel) Query(predicate string) ([]core.Fact, error) {
	// If it's a specific query with arguments, we don't need to return the full dataset
	if predicate == fmt.Sprintf(`tool_registered("%s")`, m.Capability) || predicate == fmt.Sprintf(`has_capability("%s")`, m.Capability) {
		return []core.Fact{
			{
				Predicate: "dummy",
				Args:      []any{m.Capability},
			},
		}, nil
	}

	var facts []core.Fact
	// Return 1000 facts to simulate N+1 issue
	for i := 0; i < 1000; i++ {
		facts = append(facts, core.Fact{
			Predicate: "dummy",
			Args:      []any{"dummy_tool"},
		})
	}
	if predicate == "tool_registered" || predicate == "has_capability" {
		facts = append(facts, core.Fact{
			Predicate: predicate,
			Args:      []any{m.Capability},
		})
	}

	return facts, nil
}

func BenchmarkPollToolRegistration_Baseline(b *testing.B) {
	k := &BenchmarkMockKernel{Capability: "my_capability"}
	o := &Orchestrator{kernel: k}

	// We want to simulate the case inside the select -> ticker.C block
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		facts, _ := o.kernel.Query("tool_registered")
		found := false
		for _, fact := range facts {
			if len(fact.Args) > 0 {
				if toolName, ok := fact.Args[0].(string); ok && toolName == "my_capability" {
					found = true
					break
				}
			}
		}

		if !found {
			capFacts, _ := o.kernel.Query("has_capability")
			for _, fact := range capFacts {
				if len(fact.Args) > 0 {
					if cap, ok := fact.Args[0].(string); ok && cap == "my_capability" {
						break
					}
				}
			}
		}
	}
}

func BenchmarkPollToolRegistration_Optimized(b *testing.B) {
	k := &BenchmarkMockKernel{Capability: "my_capability"}
	o := &Orchestrator{kernel: k}
	capability := "my_capability"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		facts, err := o.kernel.Query(fmt.Sprintf(`tool_registered("%s")`, capability))
		if err == nil && len(facts) > 0 {
			continue // found
		}

		// Also check has_capability
		capFacts, capErr := o.kernel.Query(fmt.Sprintf(`has_capability("%s")`, capability))
		if capErr == nil && len(capFacts) > 0 {
			continue // found
		}
	}
}
