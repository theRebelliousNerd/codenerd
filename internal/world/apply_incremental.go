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
	if !res.Full {
		// A full result carries its treatment rows in NewFacts and clears the
		// predicates through ScannerReplaceSet. Any other result replaces them
		// here, even when nothing else changed: a new or removed file in an
		// enforced scope is invisible to the change detection.
		if err := applyTreatmentFacts(kernel, res.TreatmentFacts); err != nil {
			return err
		}
	}
	// Repository facts follow the repository's history, not the source census,
	// so they apply whether or not any file changed.
	if err := applyRepoFacts(kernel, res); err != nil {
		return err
	}
	// The spec pairing follows the documents and the code it pairs, so it
	// applies whether or not the source census changed.
	if err := applySpecFacts(kernel, res); err != nil {
		return err
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
		if len(res.NewFacts) > 0 {
			if err := kernel.LoadFacts(toTypesFacts(res.NewFacts)); err != nil {
				return err
			}
		}
		if err := refreshImportCycles(kernel); err != nil {
			return err
		}
		return applyOrientationDelta(kernel, res)
	}

	// A rescanned file re-emits every fact it had, and most are unchanged:
	// adding one file to a package rescans each importer of it (652 files for
	// internal/core, 282k facts). Retracting all of them and asserting them
	// again made the kernel see the whole world change and run a full
	// fixpoint; only the facts that are not asserted again are retracted.
	if retract := notReasserted(res.RetractFacts, res.NewFacts); len(retract) > 0 {
		_ = kernel.RetractExactFactsBatch(toTypesFacts(retract))
	}
	edgesChanged := importEdgesChanged(res.RetractFacts, res.NewFacts)

	// Whole-snapshot derivations are recomputed in full by every delta scan, so
	// the previous generation is dropped rather than accumulated. project_language
	// in particular is single-valued: leaving the old atom in place next to the
	// new one makes every rule that reads it depend on iteration order.
	for _, pred := range SnapshotGlobalPredicates {
		_ = kernel.Retract(pred)
	}

	if len(res.NewFacts) > 0 {
		if err := kernel.LoadFacts(toTypesFacts(res.NewFacts)); err != nil {
			return err
		}
	}
	if edgesChanged {
		if err := refreshImportCycles(kernel); err != nil {
			return err
		}
	}
	return applyOrientationDelta(kernel, res)
}

// applyTreatmentFacts replaces the kernel's enforced-treatment rows with the
// walk's current set.
func applyTreatmentFacts(kernel types.Kernel, facts []Fact) error {
	for _, pred := range TreatmentPredicates {
		if len(facts) == 0 {
			// Nothing to assert: retract only what is there, so a scan of a
			// workspace without enforced scopes touches nothing.
			if held, err := kernel.Query(pred); err != nil || len(held) == 0 {
				continue
			}
		}
		_ = kernel.Retract(pred)
	}
	if len(facts) == 0 {
		return nil
	}
	return kernel.LoadFacts(toTypesFacts(facts))
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

// notReasserted is the retracted facts that the new facts do not assert
// again. Retracting a fact and loading an identical one leaves the kernel as
// skipping both does, since LoadFacts keeps a fact already held; skipping
// both spares the kernel a delete and a re-insert of the same row. Identity
// is the predicate and the Go-syntax form of the arguments, which tells
// apart every value the kernel could tell apart, so the kernel ends with
// exactly the facts it would have.
func notReasserted(retracted, added []Fact) []Fact {
	if len(retracted) == 0 || len(added) == 0 {
		return retracted
	}
	again := make(map[string]int, len(added))
	for _, f := range added {
		again[factIdentity(f)]++
	}
	out := make([]Fact, 0, len(retracted)/8)
	for _, f := range retracted {
		key := factIdentity(f)
		if again[key] > 0 {
			again[key]--
			continue
		}
		out = append(out, f)
	}
	return out
}

func factIdentity(f Fact) string {
	return f.Predicate + "\x00" + fmt.Sprintf("%#v", f.Args)
}

func toTypesFacts(worldFacts []Fact) []types.Fact {
	res := make([]types.Fact, len(worldFacts))
	for i, f := range worldFacts {
		res[i] = types.Fact(f)
	}
	return res
}
