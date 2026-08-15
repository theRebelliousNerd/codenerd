package world

import "codenerd/internal/types"

// ApplyIncrementalResult updates the kernel with an incremental scan result.
// For Full results, it replaces the scanner-owned world predicates.
// For delta results, it retracts old facts (when available) and asserts new ones.
func ApplyIncrementalResult(kernel types.Kernel, res *IncrementalResult) error {
	if res == nil {
		return nil
	}

	if res.Full {
		// Scanner-owned predicates only. This used to clear the whole
		// WorldPredicates list, which included deep (code_defines/code_calls,
		// data flow) and LSP-projected predicates that a fast scan never
		// re-emits: a single rescan wiped the deep call graph and every LSP
		// diagnostic, and nothing restored them until a deep scan happened to
		// run. See the ownership matrix in world_predicates.go.
		_ = kernel.RemoveFactsByPredicateSet(ScannerReplaceSet())
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

	// Whole-snapshot derivations are recomputed in full by every delta scan, so
	// the previous generation is dropped rather than accumulated. project_language
	// in particular is single-valued: leaving the old atom in place next to the
	// new one makes every rule that reads it depend on iteration order.
	for _, pred := range SnapshotGlobalPredicates {
		_ = kernel.Retract(pred)
	}

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
