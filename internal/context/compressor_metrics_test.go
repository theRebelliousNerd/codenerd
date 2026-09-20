package context

import (
	"testing"
)

// Age boundary mirrored from assertTurnAgeCategories in compressor_metrics.go:
// age = currentTurn - turn.TurnNumber; age <= 3 -> /recent, age <= 8 -> /mid,
// age <= 15 -> /old, otherwise -> /ancient. The masking rules in
// context_compilation.mg mask /old and /ancient only, so the masking boundary
// sits between age 8 (/mid, unmasked) and age 9 (/old, masked).

func collectMaskIDs(t *testing.T, comp *Compressor, pred string) map[string]bool {
	t.Helper()
	facts, err := comp.kernel.Query(pred)
	if err != nil {
		t.Fatalf("query %s: %v", pred, err)
	}
	out := make(map[string]bool, len(facts))
	for _, f := range facts {
		if len(f.Args) < 1 {
			continue
		}
		if id, ok := f.Args[0].(string); ok {
			out[id] = true
		}
	}
	return out
}

func TestObservationMaskingDerivation_MaskingBoundary(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	comp.turnNumber = 40
	// age 1 -> /recent, age 8 -> /mid, age 9 -> /old, age 39 -> /ancient.
	comp.recentTurns = []CompressedTurn{
		{TurnNumber: 39},
		{TurnNumber: 32},
		{TurnNumber: 31},
		{TurnNumber: 1},
	}
	comp.assertTurnAgeCategories(comp.recentTurns)

	masked := collectMaskIDs(t, comp, "should_mask_observation")
	preserved := collectMaskIDs(t, comp, "should_preserve_reasoning")

	if len(masked) == 0 {
		t.Fatal("should_mask_observation returned no rows during a real turn set")
	}
	if len(preserved) == 0 {
		t.Fatal("should_preserve_reasoning returned no rows during a real turn set")
	}

	// Masking boundary: /old and /ancient masked, /recent and /mid not.
	if !masked[turnMaskID(31)] {
		t.Errorf("age-9 turn %s (/old) must appear in should_mask_observation, got %v", turnMaskID(31), masked)
	}
	if !masked[turnMaskID(1)] {
		t.Errorf("age-39 turn %s (/ancient) must appear in should_mask_observation, got %v", turnMaskID(1), masked)
	}
	if masked[turnMaskID(39)] {
		t.Errorf("age-1 turn %s (/recent) must not appear in should_mask_observation, got %v", turnMaskID(39), masked)
	}
	if masked[turnMaskID(32)] {
		t.Errorf("age-8 turn %s (/mid, just below the boundary) must not appear in should_mask_observation, got %v", turnMaskID(32), masked)
	}

	// Invariant from schemas_context.mg: we mask observations, never reasoning.
	for _, n := range []int{39, 32, 31, 1} {
		if !preserved[turnMaskID(n)] {
			t.Errorf("turn %s must appear in should_preserve_reasoning, got %v", turnMaskID(n), preserved)
		}
	}
}

func TestObservationMaskingDerivation_CompressionPathQueriesReturnRows(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	comp.turnNumber = 40
	comp.recentTurns = []CompressedTurn{
		{TurnNumber: 39}, // age 1 -> /recent, must stay unmasked
		{TurnNumber: 1},  // age 39 -> /ancient, must be masked
	}

	// Real compression path: maskedObservationTurns asserts turn_age_category
	// for every known turn before running the :722/:732 queries.
	masked := comp.maskedObservationTurns()
	if !masked[turnMaskID(1)] {
		t.Errorf("ancient turn %s must be masked via the compression path, got %v", turnMaskID(1), masked)
	}
	if masked[turnMaskID(39)] {
		t.Errorf("recent turn %s must not be masked via the compression path, got %v", turnMaskID(39), masked)
	}

	// Exercise the derivation directly: both kernel predicates must return rows.
	maskFacts, err := comp.kernel.Query("should_mask_observation")
	if err != nil {
		t.Fatalf("query should_mask_observation: %v", err)
	}
	if len(maskFacts) == 0 {
		t.Fatal("should_mask_observation returned no rows during a real compression")
	}
	preserveFacts, err := comp.kernel.Query("should_preserve_reasoning")
	if err != nil {
		t.Fatalf("query should_preserve_reasoning: %v", err)
	}
	if len(preserveFacts) == 0 {
		t.Fatal("should_preserve_reasoning returned no rows during a real compression")
	}
	preserved := make(map[string]bool, len(preserveFacts))
	for _, f := range preserveFacts {
		if len(f.Args) >= 1 {
			if id, ok := f.Args[0].(string); ok {
				preserved[id] = true
			}
		}
	}
	for _, n := range []int{39, 1} {
		if !preserved[turnMaskID(n)] {
			t.Errorf("turn %s must appear in should_preserve_reasoning, got %v", turnMaskID(n), preserved)
		}
	}
}
