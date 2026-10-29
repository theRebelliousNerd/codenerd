package tools

import (
	"encoding/json"
	"math"
	"testing"
)

// CoerceInt truncates fractional values toward zero. That is the contract for
// soft quantities (timeouts, limits, counts), pinned here and by shell's
// TestCoerceInt; exact addresses must use CoerceIntStrict instead.
func TestCoerceInt_TruncatesFractions(t *testing.T) {
	cases := []struct {
		in   any
		want int
		ok   bool
	}{
		{nil, 0, false},
		{42, 42, true},
		{int64(9), 9, true},
		{float64(4.7), 4, true},
		{float32(2.9), 2, true},
		{json.Number("15"), 15, true},
		{json.Number("3.8"), 3, true},
		{"123", 123, true},
		{"6.5", 6, true},
		{"", 0, false},
		{"notanumber", 0, false},
	}
	for _, c := range cases {
		got, ok := CoerceInt(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("CoerceInt(%v)=(%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// CoerceIntStrict accepts the same shapes but refuses fractional values: a
// line number or retained-output offset must fail closed, never silently
// address a neighbor of the requested location.
func TestCoerceIntStrict_RefusesFractions(t *testing.T) {
	cases := []struct {
		in   any
		want int
		ok   bool
	}{
		{nil, 0, false},
		{42, 42, true},
		{int64(9), 9, true},
		{float64(4.0), 4, true},
		{float64(4.7), 0, false},
		{float32(2.9), 0, false},
		{json.Number("15"), 15, true},
		{json.Number("3.8"), 0, false},
		{"123", 123, true},
		{"6.5", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := CoerceIntStrict(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("CoerceIntStrict(%v)=(%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}

	if _, ok := ArgIntStrict(map[string]any{"n": 1.5}, "n"); ok {
		t.Error("ArgIntStrict must refuse 1.5")
	}
	if v, ok := ArgIntStrict(map[string]any{"n": 2.0}, "n"); !ok || v != 2 {
		t.Errorf("ArgIntStrict(2.0)=(%d,%v), want (2,true)", v, ok)
	}
	if _, ok := ArgIntStrict(map[string]any{}, "n"); ok {
		t.Error("ArgIntStrict must report a missing key")
	}
	if _, ok := ArgIntStrict(nil, "n"); ok {
		t.Error("ArgIntStrict must report a nil map")
	}
}

// Both helpers refuse non-finite and out-of-range magnitudes rather than wrap
// them: a wrapped bound would silently address (or allocate for) a value no
// caller asked for. Ported up from codedom's applyCoerceInt, which carried the
// only overflow guards in the repo; the canonical helpers must be at least as
// safe as the copies they replaced.
func TestCoerceInt_RefusesNonFiniteAndOverflow(t *testing.T) {
	evil := []any{
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
		float32(math.Inf(1)),
		float64(uint64(math.MaxInt) + 1), // 2^63: float64(MaxInt) rounds here
		float64(math.MinInt) * 2,         // -2^64: exactly representable, out of range
		uint64(math.MaxInt) + 1,
		"1e300",
		json.Number("1e300"),
	}
	for _, v := range evil {
		if got, ok := CoerceInt(v); ok {
			t.Errorf("CoerceInt(%v) = (%d, true); want refusal", v, got)
		}
		if got, ok := CoerceIntStrict(v); ok {
			t.Errorf("CoerceIntStrict(%v) = (%d, true); want refusal", v, got)
		}
	}
	// The largest in-range magnitudes still convert exactly.
	if got, ok := CoerceInt(uint64(math.MaxInt)); !ok || got != math.MaxInt {
		t.Errorf("CoerceInt(maxuint-as-int) = (%d, %v); want (%d, true)", got, ok, math.MaxInt)
	}
	if got, ok := CoerceIntStrict(int64(math.MaxInt)); !ok || got != math.MaxInt {
		t.Errorf("CoerceIntStrict(int64 max) = (%d, %v); want (%d, true)", got, ok, math.MaxInt)
	}
}

// CoerceInt64 is the canonical 64-bit coercion for millisecond timestamps:
// epoch millis arrive as float64 (exact below 2^53), int64, or decimal strings,
// and every shape must read the same instant. Fractions truncate (sub-ms is
// meaningless); non-finite and out-of-range magnitudes fail closed.
func TestCoerceInt64_TimestampShapes(t *testing.T) {
	const epochMS = int64(1789544000000)
	cases := []struct {
		in   any
		want int64
		ok   bool
	}{
		{nil, 0, false},
		{epochMS, epochMS, true},
		{int(epochMS), epochMS, true},
		{float64(epochMS), epochMS, true},
		{float64(1500.9), 1500, true},
		{"1789544000000", epochMS, true},
		{json.Number("1789544000000"), epochMS, true},
		{"", 0, false},
		{"abc", 0, false},
		{true, 0, false},
		{math.NaN(), 0, false},
		{math.Inf(1), 0, false},
		{uint64(math.MaxInt64) + 1, 0, false},
		{float64(uint64(math.MaxInt64) + 1), 0, false},
		{"1e300", 0, false},
	}
	for _, c := range cases {
		got, ok := CoerceInt64(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("CoerceInt64(%v)=(%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
