package world

import "codenerd/internal/types"

// ApplyIncrementalResult updates the kernel with an incremental scan result.
// For Full results, it replaces all world predicates.
// For delta results, it retracts old facts (when available) and asserts new ones.
func ApplyIncrementalResult(kernel types.Kernel, res *IncrementalResult) error {
	if res == nil {
		return nil
	}

	if res.Full {
		_ = kernel.RemoveFactsByPredicateSet(WorldPredicateSet())
		if len(res.NewFacts) == 0 {
			return nil
		}
		// Convert world.Fact to types.Fact
		typeFacts := toTypesFacts(res.NewFacts)
		return kernel.LoadFacts(typeFacts)
	}

	if len(res.RetractFacts) > 0 {
		typeFacts := toTypesFacts(res.RetractFacts)
		_ = kernel.RetractExactFactsBatch(typeFacts)
	}

	// Refresh directory facts every scan.
	_ = kernel.Retract("directory")

	if len(res.NewFacts) == 0 {
		return nil
	}
	// Convert world.Fact to types.Fact
	typeFacts := toTypesFacts(res.NewFacts)
	return kernel.LoadFacts(typeFacts)
}

func toTypesFacts(worldFacts []Fact) []types.Fact {
	res := make([]types.Fact, len(worldFacts))
	for i, f := range worldFacts {
		res[i] = types.Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return res
}


