package prompt

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestTopKEligibleVectorScores_SkipsMandatoryAndBlocked pins F-JIT-TOPK:
// the vector tier must spend its k slots on atoms the kernel can actually
// select, never on mandatory atoms (selected regardless) or on atoms gated
// to another shard.
func TestTopKEligibleVectorScores_SkipsMandatoryAndBlocked(t *testing.T) {
	s := NewAtomSelector()
	s.SetMinScoreThreshold(0)

	cc := NewCompilationContext().WithShard("/coder", "", "")

	blockedIDs := []string{"blocked-0", "blocked-1", "blocked-2", "blocked-3", "blocked-4"}
	eligibleIDs := []string{"eligible-0", "eligible-1", "eligible-2", "eligible-3", "eligible-4", "eligible-5", "eligible-6"}

	var flesh []*PromptAtom
	scores := make(map[string]float64, 12)

	// The 5 highest scores belong to atoms the kernel cannot select via a
	// vector_hit: two mandatory, three gated to /reviewer.
	blockedScores := []float64{0.95, 0.94, 0.93, 0.92, 0.91}
	for i, id := range blockedIDs {
		atom := &PromptAtom{ID: id}
		if i < 2 {
			atom.IsMandatory = true
		} else {
			atom.ShardTypes = []string{"/reviewer"}
		}
		flesh = append(flesh, atom)
		scores[id] = blockedScores[i]
	}

	// Eligible atoms score strictly below every blocked atom.
	eligibleScores := []float64{0.80, 0.79, 0.78, 0.77, 0.76, 0.75, 0.74}
	for i, id := range eligibleIDs {
		flesh = append(flesh, &PromptAtom{ID: id})
		scores[id] = eligibleScores[i]
	}

	got := s.topKEligibleVectorScores(scores, flesh, cc, 3)

	if len(got) != 3 {
		t.Fatalf("expected exactly 3 kept scores, got %d: %v", len(got), got)
	}
	for _, id := range blockedIDs {
		if _, ok := got[id]; ok {
			t.Errorf("blocked atom %q must not consume a vector slot, kept=%v", id, got)
		}
	}
	for _, id := range []string{"eligible-0", "eligible-1", "eligible-2"} {
		if _, ok := got[id]; !ok {
			t.Errorf("expected highest eligible atom %q to be kept, kept=%v", id, got)
		}
	}
}

// TestTopKEligibleVectorScores_RelativeFloor pins F-JIT-TOPK's relative floor:
// with a tight cluster of low scores and two high outliers, only the outliers
// survive mean + 1.5*stddev even when k is large enough to keep everything.
func TestTopKEligibleVectorScores_RelativeFloor(t *testing.T) {
	s := NewAtomSelector()
	s.SetMinScoreThreshold(0)

	cc := NewCompilationContext().WithShard("/coder", "", "")

	var flesh []*PromptAtom
	scores := make(map[string]float64, 20)

	for i := 0; i < 18; i++ {
		id := "base-atom"
		if i < 10 {
			id = "base-atom-0" + string(rune('0'+i))
		} else {
			id = "base-atom-" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		}
		// Deterministic values in [0.50, 0.54]: 0.500, 0.505, ..., 0.540, repeating.
		score := 0.50 + float64(i%9)*0.005
		flesh = append(flesh, &PromptAtom{ID: id})
		scores[id] = score
	}
	flesh = append(flesh, &PromptAtom{ID: "outlier-high"})
	scores["outlier-high"] = 0.72
	flesh = append(flesh, &PromptAtom{ID: "outlier-low"})
	scores["outlier-low"] = 0.70

	got := s.topKEligibleVectorScores(scores, flesh, cc, 10)

	if len(got) != 2 {
		t.Fatalf("expected exactly the 2 outliers, got %d: %v", len(got), got)
	}
	if _, ok := got["outlier-high"]; !ok {
		t.Errorf("expected outlier-high (0.72) to be kept, kept=%v", got)
	}
	if _, ok := got["outlier-low"]; !ok {
		t.Errorf("expected outlier-low (0.70) to be kept, kept=%v", got)
	}
}

// TestTopKEligibleVectorScores_FewScoresUseAbsoluteFloorOnly pins F-JIT-TOPK's
// small-set rule: with fewer than 8 eligible scores the relative floor is
// skipped and only the absolute floor applies, so all four survive.
func TestTopKEligibleVectorScores_FewScoresUseAbsoluteFloorOnly(t *testing.T) {
	s := NewAtomSelector()
	s.SetMinScoreThreshold(0)

	cc := NewCompilationContext().WithShard("/coder", "", "")

	ids := []string{"few-0", "few-1", "few-2", "few-3"}
	scoresList := []float64{0.60, 0.55, 0.52, 0.51}

	var flesh []*PromptAtom
	scores := make(map[string]float64, 4)
	for i, id := range ids {
		flesh = append(flesh, &PromptAtom{ID: id})
		scores[id] = scoresList[i]
	}

	got := s.topKEligibleVectorScores(scores, flesh, cc, 4)

	if len(got) != 4 {
		t.Fatalf("expected all 4 scores with no relative floor, got %d: %v", len(got), got)
	}
	for _, id := range ids {
		if _, ok := got[id]; !ok {
			t.Errorf("expected eligible atom %q to be kept, kept=%v", id, got)
		}
	}
}

// TestTopKEligibleVectorScores_TieBreakIsDeterministic pins F-JIT-TOPK's
// tie-break: equal scores are ordered by atom ID ascending, so with k=3 out
// of 6 equal scores exactly the three smallest IDs survive, identically on
// every run (map iteration order must not leak into the result).
func TestTopKEligibleVectorScores_TieBreakIsDeterministic(t *testing.T) {
	s := NewAtomSelector()
	s.SetMinScoreThreshold(0)

	cc := NewCompilationContext().WithShard("/coder", "", "")

	// Deliberately unsorted insertion order: the kept set must still be the
	// IDs in ascending order.
	insertionOrder := []string{"tie-3", "tie-1", "tie-5", "tie-0", "tie-4", "tie-2"}

	var flesh []*PromptAtom
	scores := make(map[string]float64, len(insertionOrder))
	for _, id := range insertionOrder {
		flesh = append(flesh, &PromptAtom{ID: id})
		scores[id] = 0.60
	}

	want := map[string]bool{"tie-0": true, "tie-1": true, "tie-2": true}

	first := s.topKEligibleVectorScores(scores, flesh, cc, 3)
	if len(first) != 3 {
		t.Fatalf("expected exactly 3 kept scores, got %d: %v", len(first), first)
	}
	for id := range first {
		if !want[id] {
			t.Errorf("expected only the 3 smallest IDs %v, kept=%v", want, first)
			break
		}
	}
	for id := range want {
		if _, ok := first[id]; !ok {
			t.Errorf("expected smallest ID %q to be kept, kept=%v", id, first)
		}
	}

	for i := 0; i < 20; i++ {
		got := s.topKEligibleVectorScores(scores, flesh, cc, 3)
		if len(got) != len(first) {
			t.Fatalf("run %d: expected %d kept scores, got %d: %v", i, len(first), len(got), got)
		}
		for id := range first {
			if _, ok := got[id]; !ok {
				t.Fatalf("run %d: non-deterministic result, first=%v got=%v", i, first, got)
			}
		}
	}
}

// TestFilterFleshAtoms_IncludesNonMandatorySkeletonCategory pins F-JIT-REACH:
// a non-mandatory skeleton-category atom competes in the flesh tier (while
// staying in the skeleton tier for depends_on resolution); a mandatory one
// stays skeleton-only.
func TestFilterFleshAtoms_IncludesNonMandatorySkeletonCategory(t *testing.T) {
	cc := NewCompilationContext().WithShard("/coder", "", "")

	nonMandatory := &PromptAtom{ID: "methodology/optional", Category: CategoryMethodology}
	mandatory := &PromptAtom{ID: "methodology/required", Category: CategoryMethodology, IsMandatory: true}
	atoms := []*PromptAtom{nonMandatory, mandatory}

	contains := func(list []*PromptAtom, id string) bool {
		for _, a := range list {
			if a.ID == id {
				return true
			}
		}
		return false
	}

	flesh := filterFleshAtoms(atoms, cc)
	if !contains(flesh, nonMandatory.ID) {
		t.Errorf("expected non-mandatory %q in flesh set, got %v", nonMandatory.ID, flesh)
	}
	if contains(flesh, mandatory.ID) {
		t.Errorf("expected mandatory %q excluded from flesh set, got %v", mandatory.ID, flesh)
	}

	skeleton := filterSkeletonAtoms(atoms, cc)
	if !contains(skeleton, nonMandatory.ID) {
		t.Errorf("expected non-mandatory %q to stay in skeleton set, got %v", nonMandatory.ID, skeleton)
	}
	if !contains(skeleton, mandatory.ID) {
		t.Errorf("expected mandatory %q in skeleton set, got %v", mandatory.ID, skeleton)
	}
}

// fleshRejectingKernel accepts the skeleton assert but rejects the flesh
// assert, forcing the flesh tier through its keyword-matching fallback
// (fallbackFleshSelection) over the flesh-eligible set. The flesh fact set is
// the only one containing vector_hit witnesses (they are built solely in
// buildFleshFacts), so the skeleton path is unaffected. This isolates tier
// membership: the target below can only be selected if filterFleshAtoms
// admits it.
type fleshRejectingKernel struct {
	*mockKernel
}

func (k *fleshRejectingKernel) AssertBatch(facts []any) error {
	for _, f := range facts {
		if s, ok := f.(string); ok && strings.Contains(s, "vector_hit") {
			return errors.New("reach test: flesh facts rejected")
		}
	}
	return k.mockKernel.AssertBatch(facts)
}

// TestSelectAtoms_NonMandatoryMethodologyAtomIsReachable pins F-JIT-REACH end
// to end: a non-mandatory methodology-category atom with the top vector score
// must be selected through SelectAtoms. Before the fix it is in neither
// tier's reachable set — the skeleton query has no vector facts and the flesh
// tier excludes skeleton categories — so this test fails against the
// unmodified filterFleshAtoms.
func TestSelectAtoms_NonMandatoryMethodologyAtomIsReachable(t *testing.T) {
	target := &PromptAtom{ID: "methodology/reachable-target", Category: CategoryMethodology}
	decoyA := &PromptAtom{ID: "flesh-decoy-a"}
	decoyB := &PromptAtom{ID: "flesh-decoy-b"}
	atoms := []*PromptAtom{target, decoyA, decoyB}

	cc := NewCompilationContext().WithShard("/coder", "", "")
	cc.SemanticQuery = "reachable methodology pattern"
	cc.SemanticTopK = 10

	sel := NewAtomSelector()
	sel.SetMinScoreThreshold(0)
	sel.SetVectorSearcher(&mockVectorSearcher{results: map[string]float64{
		target.ID: 0.95,
		decoyA.ID: 0.30,
		decoyB.ID: 0.31,
	}})
	sel.SetKernel(&fleshRejectingKernel{mockKernel: &mockKernel{}})

	got, err := sel.SelectAtoms(context.Background(), atoms, cc)
	if err != nil {
		t.Fatalf("SelectAtoms returned error: %v", err)
	}
	for _, sa := range got {
		if sa != nil && sa.Atom != nil && sa.Atom.ID == target.ID {
			return
		}
	}
	t.Fatalf("non-mandatory methodology atom %q was not selected (got %d atoms)", target.ID, len(got))
}
