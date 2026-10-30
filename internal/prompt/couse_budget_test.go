package prompt

import (
	"fmt"
	"testing"
)

// A long session of settled turns whose kernel-rendered atoms carry fresh
// content-hash ids kept adding resident atom and category entries: pending
// turns, selections and pairs were bounded, those two maps were not. The
// atom budget now bounds both, says so on the report, and keeps the
// accounting coherent: a refused atom contributes no pair, and the stable
// atoms keep counting.
func TestCoUseRecorderBoundsResidentAtomsAndCategories(t *testing.T) {
	r := NewCoUseRecorder()
	r.atomLimit = 8
	stable := NewPromptAtom("identity/stable", CategoryIdentity, "stable")
	for turn := 0; turn < 50; turn++ {
		fresh := NewPromptAtom(fmt.Sprintf("kernel/context/%08x", turn), CategoryContext, "brief")
		id := fmt.Sprintf("turn-%d", turn)
		r.ObserveAtoms(id, []*PromptAtom{stable, fresh})
		r.Settle(id, OutcomeSuccess)
	}

	r.mu.Lock()
	atoms, pairs, refused := len(r.atoms), len(r.pairs), r.refusedAtoms
	r.mu.Unlock()
	r.catMu.RLock()
	categories := len(r.categories)
	r.catMu.RUnlock()
	if atoms > 8 || categories > 8 {
		t.Fatalf("resident atoms = %d, categories = %d; want both within the budget of 8", atoms, categories)
	}
	if refused != 50-7 {
		t.Fatalf("refused atom observations = %d, want 43 (the fresh ids past the budget)", refused)
	}
	if pairs != 7 {
		t.Fatalf("pairs = %d, want 7: only pairs of two admitted atoms count", pairs)
	}

	rep := r.Report(CoUseParams{MinSupport: 1, UbiquityThreshold: 2}, r.Categories())
	if !rep.AtomsTruncated || rep.RefusedAtomObservations != 43 {
		t.Fatalf("report does not disclose the atom budget: truncated=%v refused=%d", rep.AtomsTruncated, rep.RefusedAtomObservations)
	}
	if rep.SuccessSelections != 50 {
		t.Fatalf("success selections = %d, want 50: refusing an atom does not drop its selection", rep.SuccessSelections)
	}
	for _, a := range rep.Atoms {
		if a.ID == "identity/stable" && a.Success != 50 {
			t.Fatalf("the stable atom stopped counting at the budget: %+v", a)
		}
	}
}

// The pair loop was quadratic in a selection's atoms with no bound of its
// own, under the mutex every compile's Observe takes. A selection past the
// per-selection pair bound now pairs its leading atoms only, tallies every
// atom, and the report counts the cells it skipped.
func TestCoUseRecorderBoundsPairsPerSelection(t *testing.T) {
	r := NewCoUseRecorder()
	r.pairLimit = 10 // five atoms pair fully (10 cells); a sixth would make 15
	var atoms []*PromptAtom
	for i := 0; i < 8; i++ {
		atoms = append(atoms, NewPromptAtom(fmt.Sprintf("flesh/a%d", i), CategoryDomain, "x"))
	}
	r.ObserveAtoms("turn", atoms)
	r.Settle("turn", OutcomeSuccess)

	r.mu.Lock()
	tallied, pairs, skipped := len(r.atoms), len(r.pairs), r.skippedPairs
	r.mu.Unlock()
	if tallied != 8 || pairs != 10 || skipped != 28-10 {
		t.Fatalf("atoms=%d pairs=%d skipped=%d; want 8 tallied, 10 pairs, 18 skipped", tallied, pairs, skipped)
	}
	if rep := r.Report(CoUseParams{MinSupport: 1, UbiquityThreshold: 2}, r.Categories()); rep.SkippedPairs != 18 {
		t.Fatalf("the report does not disclose the skipped pairs: %d", rep.SkippedPairs)
	}
}
