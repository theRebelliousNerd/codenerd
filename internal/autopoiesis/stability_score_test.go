package autopoiesis

import "testing"

// The defect this guards (F-OURO-2, observed live): schemas_state.mg declares
// Stability as /number and valid_transition compares it with
// `NextStability >= CurrStability`. This Mangle fork's comparison builtins are
// int64-only, so asserting a float64 makes the comparison FAIL rather than
// evaluate false:
//
//	transition query failed: value 1 (4) is not a number
//
// Every Ouroboros run died at that gate, which meant the transition validation
// the whole loop is built around had never once executed successfully.

func TestStabilityScore_ProducesIntegers(t *testing.T) {
	// The values actually asserted by ouroboros.go.
	for _, v := range []float64{0.0, 1.0, 0.85, 0.9} {
		got := stabilityScore(v)
		if float64(got) != float64(int64(got)) {
			t.Errorf("stabilityScore(%v) = %v, which is not integral", v, got)
		}
	}
}

// Scaling is only safe because it preserves ordering — the rules compare
// stabilities against each other, so any order-preserving map keeps their
// meaning. If this breaks, valid_transition silently changes verdict.
func TestStabilityScore_PreservesOrdering(t *testing.T) {
	cases := []struct{ lo, hi float64 }{
		{0.0, 1.0},
		{0.0, 0.01},
		{0.5, 0.51},
		{0.89, 0.9},
		{0.99, 1.0},
	}

	for _, c := range cases {
		lo, hi := stabilityScore(c.lo), stabilityScore(c.hi)
		if !(lo < hi) {
			t.Errorf("ordering lost: stabilityScore(%v)=%d not < stabilityScore(%v)=%d", c.lo, lo, c.hi, hi)
		}
	}
}

// Equal inputs must compare equal, or a transition to an identically-stable
// state is rejected when the rule says >= should accept it.
func TestStabilityScore_EqualInputsCompareEqual(t *testing.T) {
	if stabilityScore(0.85) != stabilityScore(0.85) {
		t.Error("identical stabilities did not map to the same score")
	}
	// The live path: baseline 0.0 vs a proposed state of equal stability.
	if !(stabilityScore(0.0) >= stabilityScore(0.0)) {
		t.Error("a transition to an equally-stable state would be rejected")
	}
}

// The concrete live scenario: baseline state 0.0, proposal at the model's
// confidence. valid_transition requires Next >= Curr.
func TestStabilityScore_ProposalAboveBaselineIsAccepted(t *testing.T) {
	baseline := stabilityScore(0.0)
	for _, confidence := range []float64{0.5, 0.8, 0.95, 1.0} {
		if !(stabilityScore(confidence) >= baseline) {
			t.Errorf("proposal at confidence %v would be rejected against a 0.0 baseline", confidence)
		}
	}
}

func TestStabilityScore_RoundsRatherThanTruncates(t *testing.T) {
	// 0.999 -> 100, not 99: truncation would make a near-perfect proposal
	// compare below an identical one recorded as 1.0.
	if got := stabilityScore(0.999); got != 100 {
		t.Errorf("stabilityScore(0.999) = %d, want 100 (rounded, not truncated)", got)
	}
}
