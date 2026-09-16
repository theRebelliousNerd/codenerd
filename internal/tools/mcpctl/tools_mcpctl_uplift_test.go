package mcpctl

import (
	"context"
	"encoding/json"
	"testing"

	"codenerd/internal/mcp"
)

// TestIntArg_CanonicalShapes pins the canonical delegation: max_items honors
// every numeric shape and falls back only on garbage or absence.
func TestIntArg_CanonicalShapes(t *testing.T) {
	cases := []struct {
		name string
		val  any
		want int
	}{
		{"int", 12, 12},
		{"whole float64", 12.0, 12},
		{"fraction truncates", 12.7, 12},
		{"decimal string honored", "8", 8},
		{"garbage falls back", "abc", 6},
		{"bool falls back", true, 6},
		{"nil falls back", nil, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{}
			if tc.val != nil {
				args["max_items"] = tc.val
			}
			if got := intArg(args, "max_items", 6); got != tc.want {
				t.Errorf("intArg(%#v) = %d; want %d", tc.val, got, tc.want)
			}
		})
	}
}

// TestExecuteProbe_ShapesAgreeThroughPlane pins that the argument shape does
// not change probe behavior end to end: max_items "1", 1, and 1.0 produce the
// same envelope through a bound (empty) control plane.
func TestExecuteProbe_ShapesAgreeThroughPlane(t *testing.T) {
	SetControlPlane(mcp.NewControlPlane(nil, nil, nil))
	defer SetControlPlane(nil)
	ctx := context.Background()

	var envelopes []string
	for _, v := range []any{"1", 1, 1.0} {
		out, err := executeProbe(ctx, map[string]any{"max_items": v})
		if err != nil {
			t.Fatalf("probe with max_items %#v failed: %v", v, err)
		}
		envelopes = append(envelopes, out)
	}
	for i := 1; i < len(envelopes); i++ {
		if envelopes[i] != envelopes[0] {
			t.Errorf("shape %d envelope differs:\n%s\nvs:\n%s", i, envelopes[i], envelopes[0])
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(envelopes[0]), &decoded); err != nil {
		t.Fatalf("probe result is not JSON: %v", err)
	}
	if decoded["success"] != true {
		t.Errorf("empty-plane probe should succeed, got: %s", envelopes[0])
	}
}
