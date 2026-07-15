package research

import (
	"encoding/json"
	"testing"
)

// TestArgInt is the F-RESEARCH-1 regression. LLM tool-call arguments are
// JSON-decoded, so numbers arrive as float64 (never int). The old
// args[key].(int) assertion silently failed in production, so max_docs /
// max_length / max_results were ignored and the default was always used.
// argInt must accept the numeric types that actually arrive at runtime.
func TestArgInt(t *testing.T) {
	cases := []struct {
		name string
		val  any
		want int
		ok   bool
	}{
		{"json number arrives as float64 (production path)", float64(50), 50, true},
		{"int literal (unit-test path)", 25, 25, true},
		{"int64 (mangle-sourced)", int64(7), 7, true},
		{"json.Number", json.Number("12"), 12, true},
		{"string is not numeric", "50", 0, false},
		{"bool is not numeric", true, 0, false},
		{"missing key", nil, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{}
			if tc.val != nil {
				args["n"] = tc.val
			}
			got, ok := argInt(args, "n")
			if got != tc.want || ok != tc.ok {
				t.Errorf("argInt(%#v) = (%d, %v); want (%d, %v)", tc.val, got, ok, tc.want, tc.ok)
			}
		})
	}
}
