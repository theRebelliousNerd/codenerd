package config

import (
	"strings"
	"testing"
)

func TestIntegrationsCheck_Reviews(t *testing.T) {
	c := IntegrationsConfig{Servers: map[string]MCPServerIntegration{
		"srv": {Enabled: true, Reviews: []MCPToolReview{
			{Tool: "ok", Effect: "write", SchemaHash: "h1"},
			{Tool: "", Effect: "read", SchemaHash: "h2"},
			{Tool: "nohash", Effect: "read"},
			{Tool: "badeffect", Effect: "teleport", SchemaHash: "h3"},
		}},
	}}
	problems := c.Check("integrations")
	var paths []string
	for _, p := range problems {
		paths = append(paths, p.Path)
	}
	joined := strings.Join(paths, " ")
	for _, want := range []string{"reviews[1].tool", "reviews[2]", "reviews[3]"} {
		if !strings.Contains(joined, want) {
			t.Errorf("problems %v lack %s", paths, want)
		}
	}
	if strings.Contains(joined, "reviews[0]") {
		t.Errorf("a valid review was refused: %v", paths)
	}
	if _, err := c.ResolveDefaultTimeout(); err != nil {
		t.Errorf("a bad review must not break the timeout fallback: %v", err)
	}
	got := c.ToMCPServerConfigs()["srv"].Reviews
	if len(got) != 4 || got[0].ToolID != "srv/ok" || got[0].SchemaHash != "h1" {
		t.Fatalf("mapped reviews = %+v", got)
	}
}
