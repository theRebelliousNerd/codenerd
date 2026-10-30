package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// A provider rejects null where the schema wants an object or an array, so a
// tool with no required arguments or no properties must not render either as
// null.
func TestInputSchema_NeverRendersNull(t *testing.T) {
	noop := func(context.Context, map[string]any) (string, error) { return "", nil }
	for name, tool := range map[string]*Tool{
		"no schema":     {Name: "a", Execute: noop},
		"optional only": {Name: "b", Execute: noop, Schema: ToolSchema{Properties: map[string]Property{"path": {Type: "string"}}}},
		"with required": {Name: "c", Execute: noop, Schema: ToolSchema{Required: []string{"path"}, Properties: map[string]Property{"path": {Type: "string"}}}},
	} {
		b, err := json.Marshal(tool.InputSchema())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "null") {
			t.Errorf("%s: schema carries a null: %s", name, b)
		}
		if !strings.Contains(string(b), `"properties":{`) {
			t.Errorf("%s: properties is not an object: %s", name, b)
		}
	}
	b, _ := json.Marshal((&Tool{Schema: ToolSchema{Required: []string{"path"}}}).InputSchema())
	if !strings.Contains(string(b), `"required":["path"]`) {
		t.Errorf("required arguments were dropped: %s", b)
	}
}
