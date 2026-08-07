package types

import "math"

// PercentScale converts a ratio into the int64 percent scale that every
// numeric Mangle slot in this codebase expects.
//
// Why this exists rather than passing the float straight through:
//
// Fact.ToAtom maps a Go float64 to ast.Float64. The pinned Mangle fork
// implements <, <=, >, >= over int64 ONLY — builtin.go routes each one through
// getNumberValues, which rejects any constant whose Type != ast.NumberType.
// (getFloatValue exists but has no caller; there is no float comparison in the
// language.) The resulting error does not merely fail one rule: it propagates
// out of EvalStratifiedProgram, so RealKernel.evaluate() returns before it
// commits the new store and the ENTIRE kernel derives nothing. The log names
// only the value — "value 110 (4) is not a number" — never the predicate.
//
// Every numeric slot in the corpus is declared /number; there is not one
// /float64 bound anywhere. So a ratio must be scaled to an integer before it
// reaches a fact, and the policy thresholds are written on the same 0-100
// scale (Confidence > 70, AvgQuality < 50).
//
// Inputs are accepted on either scale, because callers disagree: a 0..1
// ratio is multiplied by 100, a value already >= 1 is treated as a percent.
// That ambiguity means 1.0 is read as 1%, not 100% — deliberate, since a
// 0..1 ratio is by far the common case and 1.0 is vanishingly rare in it.
// The result is clamped to [0, 100]; NaN and -Inf floor to 0.
func PercentScale(v float64) int64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 1 {
		v *= 100
	}
	switch {
	case v <= 0:
		return 0
	case v >= 100:
		return 100
	default:
		return int64(math.Round(v))
	}
}
