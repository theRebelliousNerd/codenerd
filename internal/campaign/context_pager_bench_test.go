package campaign

import (
	"testing"

	"codenerd/internal/core"
)

// A dummy Kernel implementation for benchmarking.
type mockKernelForBench struct {
    core.Kernel
	assertCount int
	batchCount  int
}

func (m *mockKernelForBench) Assert(fact core.Fact) error {
	m.assertCount++
	return nil
}

func (m *mockKernelForBench) AssertBatch(facts []core.Fact) error {
	m.batchCount++
	return nil
}

func (m *mockKernelForBench) QueryAll() (map[string][]core.Fact, error) {
    res := make(map[string][]core.Fact)
    for _, pred := range []string{"dom_node", "attr", "geometry", "computed_style", "interactable", "visible_text"} {
        res[pred] = []core.Fact{{Predicate: pred}}
    }
    return res, nil
}

func (m *mockKernelForBench) Query(pred string) ([]core.Fact, error) {
    if pred == "context_profile" {
        return []core.Fact{{
            Predicate: "context_profile",
            Args: []any{"profile-123", "", "", ""},
        }}, nil
    }
    return nil, nil
}

func BenchmarkActivatePhaseSuppression(b *testing.B) {
	mk := &mockKernelForBench{}
	cp := &ContextPager{
		kernel: mk,
	}

	profile := &ContextProfile{
		ID:              "profile-123",
		RequiredSchemas: []string{}, // Forces all schemas to be suppressed
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		allSchemas := []string{
			"dom_node", "geometry", "interactable", "computed_style",
			"vector_recall",
		}

		suppressFacts := make([]core.Fact, 0, len(allSchemas))
		for _, schema := range allSchemas {
			if !contains(profile.RequiredSchemas, schema) {
				suppressFacts = append(suppressFacts, core.Fact{
					Predicate: "activation",
					Args:      []any{schema, -100},
				})
			}
		}
		if len(suppressFacts) > 0 {
			if err := cp.kernel.AssertBatch(suppressFacts); err != nil {
				for _, f := range suppressFacts {
					cp.kernel.Assert(f)
				}
			}
		}
	}
}

func BenchmarkPruneIrrelevant(b *testing.B) {
	mk := &mockKernelForBench{}
	cp := &ContextPager{
		kernel: mk,
	}

    profile := &ContextProfile{
        RequiredSchemas: []string{},
    }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cp.PruneIrrelevant(profile)
	}
}
