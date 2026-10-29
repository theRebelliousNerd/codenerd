package store

import (
	"testing"
	"time"

	"codenerd/internal/types"
)

// TestFactArgs_WhenInt64ExceedsFloat64Precision_ShouldRoundTripExactly is the
// regression test for a silent corruption with a very indirect symptom.
//
// encoding/json decodes every JSON number into an `any` as float64, and float64
// represents integers exactly only up to 2^53 (~9.0e15). A Unix nanosecond
// timestamp is around 1.79e18, so decodeFactArgs was rounding every one it read
// back: 1786773933859876776 in, 1786773933859876864 out.
//
// Nobody would have noticed by reading a timestamp. What made it matter is that
// these persisted rows are what the incremental scanner reads back to build
// RETRACTION facts for changed and deleted files — and a retraction removes a
// fact only if it matches argument-for-argument. A rounded ModTime produced a
// retraction that matched nothing, so superseded file_topology rows accumulated
// in the kernel forever while the scanner reported success. The visible symptom
// was "deleting a file does not remove its facts", several layers away from a
// JSON decoder.
//
// The boundary matters more than the magnitude here, so this walks values on
// both sides of 2^53 rather than picking one big number.
func TestFactArgs_WhenInt64ExceedsFloat64Precision_ShouldRoundTripExactly(t *testing.T) {
	const twoTo53 = int64(1) << 53

	cases := []struct {
		name string
		v    int64
	}{
		{"zero", 0},
		{"small", 42},
		{"negative", -12345},
		{"just below 2^53", twoTo53 - 1},
		{"exactly 2^53", twoTo53},
		{"just above 2^53", twoTo53 + 1},
		{"unix nanoseconds", time.Date(2026, 8, 15, 4, 5, 6, 123456789, time.UTC).UnixNano()},
		{"max int64", 1<<63 - 1},
		{"min int64", -1 << 63},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := encodeFactArgs([]any{tc.v})
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			decoded, err := decodeFactArgs(encoded)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(decoded) != 1 {
				t.Fatalf("decoded %d args, want 1", len(decoded))
			}
			got, ok := decoded[0].(int64)
			if !ok {
				t.Fatalf("decoded arg is %T, want int64", decoded[0])
			}
			if got != tc.v {
				t.Errorf("round trip changed the value: %d -> %d (delta %d). "+
					"A fact rebuilt from this row no longer matches the fact that was "+
					"asserted, so retracting it silently does nothing.",
					tc.v, got, got-tc.v)
			}
		})
	}
}

// TestFactArgs_ShouldPreserveEveryArgumentType keeps the fix from narrowing the
// codec: the retraction path compares whole facts, so a type that changes on the
// way back out (an atom decoded as a string, say) breaks matching just as
// thoroughly as a rounded number.
func TestFactArgs_ShouldPreserveEveryArgumentType(t *testing.T) {
	in := []any{
		"a/path.go",
		types.MangleAtom("/go"),
		int64(1786773933859876776),
		3.5,
		true,
		nil,
	}
	encoded, err := encodeFactArgs(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := decodeFactArgs(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("decoded %d args, want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("arg %d: %#v (%T) -> %#v (%T)", i, in[i], in[i], out[i], out[i])
		}
	}
}
