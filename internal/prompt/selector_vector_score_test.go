package prompt

import "testing"

// The defect these guard (F-JIT-4, observed live 1,209 times in one day):
//
//	[kernel] rejecting fact that fails ToAtom: vector_hit -
//	unsupported arg type func() (float64, error) at index 1
//
// loadFleshAtomsKernel emitted vector_hit facts with the score formatted as a
// float in 0..1. That was wrong twice over:
//
//  1. Wrong type. schemas_prompts.mg:359 declares
//     `vector_hit(AtomID, Score) bound [/string, /number]`, and /number is
//     int64 in this Mangle fork, so every fact was rejected outright.
//  2. Wrong scale. jit_selection.mg:211 gates candidates on `Score > 30` and
//     says so itself — "sufficient similarity (> 30 on 0-100 scale)". A cosine
//     score never exceeds 1, so no candidate could ever have passed that gate
//     even if the type had been accepted.
//
// Together: Mangle flesh selection had never once seen a usable vector score,
// and the selector silently fell back to keyword matching on every turn.

func TestVectorScoreToPercent_ScalesToPolicyRange(t *testing.T) {
	cases := []struct {
		score float64
		want  int64
	}{
		{0.0, 0},
		{0.31, 31}, // just over the jit_selection.mg gate
		{0.5, 50},
		{0.875, 88}, // rounds, not truncates
		{0.999, 100},
		{1.0, 100},
	}

	for _, c := range cases {
		if got := vectorScoreToPercent(c.score); got != c.want {
			t.Errorf("vectorScoreToPercent(%v) = %d, want %d", c.score, got, c.want)
		}
	}
}

// The gate is `Score > 30`. A similarity of 0.30 must NOT pass and 0.31 must,
// or the policy's threshold silently shifts.
func TestVectorScoreToPercent_RespectsTheThirtyGate(t *testing.T) {
	if got := vectorScoreToPercent(0.30); got > 30 {
		t.Errorf("0.30 mapped to %d, which passes the > 30 gate it should not", got)
	}
	if got := vectorScoreToPercent(0.31); got <= 30 {
		t.Errorf("0.31 mapped to %d, which fails the > 30 gate it should pass", got)
	}
}

// Ordering must survive scaling, or conflict resolution (beats/2 compares two
// candidates' scores) silently changes its verdict.
func TestVectorScoreToPercent_PreservesOrdering(t *testing.T) {
	pairs := []struct{ lo, hi float64 }{
		{0.10, 0.20},
		{0.50, 0.51},
		{0.88, 0.89},
		{0.98, 1.00},
	}
	for _, p := range pairs {
		lo, hi := vectorScoreToPercent(p.lo), vectorScoreToPercent(p.hi)
		if lo >= hi {
			t.Errorf("ordering lost: %v->%d not < %v->%d", p.lo, lo, p.hi, hi)
		}
	}
}

// Some vector backends return dot products that stray outside 0..1. Clamping
// keeps the fact inside the declared 0-100 scale instead of asserting a value
// the policy's comparisons were never written for.
func TestVectorScoreToPercent_ClampsOutOfRange(t *testing.T) {
	if got := vectorScoreToPercent(1.4); got != 100 {
		t.Errorf("1.4 mapped to %d, want clamped 100", got)
	}
	if got := vectorScoreToPercent(-0.2); got != 0 {
		t.Errorf("-0.2 mapped to %d, want clamped 0", got)
	}
}

// NaN must become a definite number, not propagate into a fact where every
// comparison against it is false.
func TestVectorScoreToPercent_NaNBecomesZero(t *testing.T) {
	nan := 0.0
	nan = nan / nan //nolint:staticcheck // deliberately produce NaN
	if got := vectorScoreToPercent(nan); got != 0 {
		t.Errorf("NaN mapped to %d, want 0", got)
	}
}
