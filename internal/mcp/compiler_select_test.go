package mcp

import (
	"context"
	"testing"
	"time"
)

func TestFallbackSelectTiersAndOrdering(t *testing.T) {
	c := NewJITToolCompiler(nil, nil, nil) // nil kernel forces the fallback path

	tools := []*MCPTool{
		{ToolID: "full", ShardAffinities: map[string]int{"coder": 100}}, // 100*7/10 = 70 -> Full
		{ToolID: "cond", ShardAffinities: map[string]int{"coder": 60}},  // 60*7/10  = 42 -> Condensed
		{ToolID: "min", ShardAffinities: map[string]int{"coder": 30}},   // 30*7/10  = 21 -> Minimal
		{ToolID: "out", ShardAffinities: map[string]int{"coder": 10}},   // 10*7/10  = 7  -> excluded
	}
	tcc := ToolCompilationContext{ShardType: "coder"}

	selected := c.selectTools(context.Background(), tcc, tools, nil, nil)
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected (out excluded), got %d: %+v", len(selected), selected)
	}
	// Sorted by final score descending: full, cond, min.
	want := []struct {
		id   string
		mode RenderMode
	}{
		{"full", RenderModeFull},
		{"cond", RenderModeCondensed},
		{"min", RenderModeMinimal},
	}
	for i, w := range want {
		if selected[i].ToolID != w.id || selected[i].RenderMode != w.mode {
			t.Errorf("selected[%d]=%s/%s, want %s/%s", i, selected[i].ToolID, selected[i].RenderMode, w.id, w.mode)
		}
	}
	for _, s := range selected {
		if s.ToolID == "out" {
			t.Error("below-threshold tool 'out' should be excluded")
		}
	}
}

func TestFallbackSelectStripsShardSlash(t *testing.T) {
	c := NewJITToolCompiler(nil, nil, nil)
	tools := []*MCPTool{{ToolID: "t", ShardAffinities: map[string]int{"coder": 100}}}
	// Leading-slash shard verbs ("/coder") must match the "coder" affinity key.
	selected := c.selectTools(context.Background(), ToolCompilationContext{ShardType: "/coder"}, tools, nil, nil)
	if len(selected) != 1 || selected[0].RenderMode != RenderModeFull {
		t.Fatalf("expected the /coder verb to resolve to a full-render tool, got %+v", selected)
	}
}

func TestSSEResolveURL(t *testing.T) {
	tr := NewSSETransport("http://example.com/api/", time.Second)
	cases := map[string]string{
		"events":             "http://example.com/api/events",
		"/abs":               "http://example.com/abs",
		"http://other.com/x": "http://other.com/x",
	}
	for in, want := range cases {
		if got := tr.resolveURL(in); got != want {
			t.Errorf("resolveURL(%q)=%q, want %q", in, got, want)
		}
	}
}
