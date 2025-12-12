package world

import "codenerd/internal/core"

// ApplyIncrementalResult updates the kernel with an incremental scan result.
// For Full results, it replaces all world predicates.
// For delta results, it retracts old facts (when available) and asserts new ones.
func ApplyIncrementalResult(kernel core.Kernel, res *IncrementalResult) error {
	if res == nil {
		return nil
	}

	if res.Full {
		if rk, ok := kernel.(*core.RealKernel); ok {
			_ = rk.RemoveFactsByPredicateSet(WorldPredicateSet())
		} else {
			for _, p := range WorldPredicates {
				_ = kernel.Retract(p)
			}
		}
		if len(res.NewFacts) == 0 {
			return nil
		}
		return kernel.LoadFacts(res.NewFacts)
	}

	if len(res.RetractFacts) > 0 {
		if rk, ok := kernel.(*core.RealKernel); ok {
			_ = rk.RetractExactFactsBatch(res.RetractFacts)
		}
	}

	// Refresh directory facts every scan.
	_ = kernel.Retract("directory")

	if len(res.NewFacts) == 0 {
		return nil
	}
	return kernel.LoadFacts(res.NewFacts)
}

