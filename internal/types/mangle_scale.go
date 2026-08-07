package types

import "math"

// PercentFromRatio converts a 0..1 ratio into the int64 percent scale that every
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
// The input scale is part of the signature on purpose. The previous version
// (PercentScale) accepted either scale and guessed with `if v < 1 { v *= 100 }`,
// which put a cliff at exactly 1.0: 0.9999 scaled to 100 while 1.0 — and
// 1.0000000000000002, which normalized float arithmetic produces routinely —
// scaled to 1. A tool with a perfect SuccessRate of 1.0 was therefore written to
// the kernel as 1, and tool_quality_* reads that against `SuccessRate > 50`, so
// a flawless tool scored as a 1% failure. Every caller in this repo passes a
// ratio; the dual-scale guess served no one and inverted the most important
// value in the range. Found by codeNERD reviewing this file.
//
// Values at or above 1 saturate to 100 rather than being reinterpreted, NaN and
// -Inf floor to 0, and the result is rounded half-away-from-zero.
func PercentFromRatio(r float64) int64 {
	if math.IsNaN(r) || r <= 0 {
		return 0
	}
	if r >= 1 {
		return 100
	}
	return int64(math.Round(r * 100))
}

// PercentClamp normalizes a value that is ALREADY on the 0..100 percent scale.
//
// Use this only when the caller's value is a percent; if it is a 0..1 ratio use
// PercentFromRatio. Mixing the two is the ambiguity this pair exists to remove.
func PercentClamp(p float64) int64 {
	if math.IsNaN(p) || p <= 0 {
		return 0
	}
	if p >= 100 {
		return 100
	}
	return int64(math.Round(p))
}
