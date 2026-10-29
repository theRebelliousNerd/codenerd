package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// A schema that does not fit indented but fits compact is returned compact
// and still valid JSON; one that fits neither carries the marker.
func TestFullSchema_StaysValidJSONUnderTheBound(t *testing.T) {
	props := map[string]any{}
	for _, name := range []string{"alpha", "beta", "gamma", "delta", "epsilon"} {
		props[name] = map[string]any{"type": "string", "description": "the " + name + " value"}
	}
	raw, _ := json.Marshal(map[string]any{"type": "object", "properties": props})
	tool := &MCPTool{Name: "t", InputSchema: raw}
	pretty, _ := json.MarshalIndent(json.RawMessage(raw), "", "  ")
	got := FullSchema(tool, len(raw)+5)
	if len(pretty) <= len(raw)+5 {
		t.Skip("fixture fits indented")
	}
	if !json.Valid([]byte(got)) {
		t.Fatalf("FullSchema returned invalid JSON under a bound the compact form fits: %s", got)
	}
	if small := FullSchema(tool, 60); !strings.Contains(small, "truncated") {
		t.Fatalf("an oversized schema was shortened without the marker: %s", small)
	}
}
