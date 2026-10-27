package world

import (
	"fmt"

	"codenerd/internal/types"
)

// ApplyIncrementalResult updates the kernel with an incremental scan result.
// For Full results, it replaces the scanner-owned world predicates.
// For delta results, it retracts old facts (when available) and asserts new ones.
func ApplyIncrementalResult(kernel types.Kernel, res *IncrementalResult) error {
	if res == nil {
		return nil
	}
	if res.Orientation != nil && res.Orientation.Refreshed {
		if _, ok := types.TransactorOf(kernel); !ok {
			return fmt.Errorf("apply orientation refresh: kernel %T does not support transactions", kernel)
		}
	}
	if res.Unchanged {
		// The source world has no delta to apply. Orientation is independent:
		// publish its exact projection change without retracting world globals.
		return applyOrientationDelta(kernel, res)
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
			return applyOrientationDelta(kernel, res)
		}
		// Convert world.Fact to types.Fact
		typeFacts := toTypesFacts(res.NewFacts)
		if err := kernel.LoadFacts(typeFacts); err != nil {
			return err
		}
		return applyOrientationDelta(kernel, res)
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
		return applyOrientationDelta(kernel, res)
	}
	// Convert world.Fact to types.Fact
	typeFacts := toTypesFacts(res.NewFacts)
	if err := kernel.LoadFacts(typeFacts); err != nil {
		return err
	}
	return applyOrientationDelta(kernel, res)
}

func applyOrientationDelta(kernel types.Kernel, res *IncrementalResult) error {
	if res == nil || res.Orientation == nil || !res.Orientation.Refreshed {
		return nil
	}
	transactor, ok := types.TransactorOf(kernel)
	if !ok {
		return fmt.Errorf("apply orientation refresh: kernel %T does not support transactions", kernel)
	}
	return res.Orientation.Apply(transactor)
}

func toTypesFacts(worldFacts []Fact) []types.Fact {
	res := make([]types.Fact, len(worldFacts))
	for i, f := range worldFacts {
		res[i] = types.Fact(f)
	}
	return res
}
