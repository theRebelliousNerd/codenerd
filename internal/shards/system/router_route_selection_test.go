package system

import (
	"testing"
	"time"
)

func TestTactileRouterShard_findRoute_PrefersExactMatchOnNormalizedAction(t *testing.T) {
	cfg := RouterConfig{
		DefaultRoutes: []ToolRoute{
			{ActionPattern: "foo", ToolName: "toolA", Timeout: 1 * time.Second},
			{ActionPattern: "foobar", ToolName: "toolB", Timeout: 1 * time.Second},
		},
		TickInterval:         1 * time.Second,
		IdleTimeout:          1 * time.Second,
		AllowUnmappedActions: false,
	}
	r := NewTactileRouterShardWithConfig(cfg)

	route, ok := r.findRoute("/foobar")
	if !ok {
		t.Fatalf("expected route for /foobar")
	}
	if route.ToolName != "toolB" {
		t.Fatalf("route.ToolName = %q, want %q", route.ToolName, "toolB")
	}
}

func TestTactileRouterShard_findRoute_PrefersPrefixOverContains(t *testing.T) {
	cfg := RouterConfig{
		DefaultRoutes: []ToolRoute{
			{ActionPattern: "bar", ToolName: "barTool", Timeout: 1 * time.Second},
			{ActionPattern: "foo", ToolName: "fooTool", Timeout: 1 * time.Second},
		},
		TickInterval:         1 * time.Second,
		IdleTimeout:          1 * time.Second,
		AllowUnmappedActions: false,
	}
	r := NewTactileRouterShardWithConfig(cfg)

	route, ok := r.findRoute("/foobar")
	if !ok {
		t.Fatalf("expected route for /foobar")
	}
	if route.ToolName != "fooTool" {
		t.Fatalf("route.ToolName = %q, want %q", route.ToolName, "fooTool")
	}
}

func TestTactileRouterShard_findRoute_RoutesFSRead(t *testing.T) {
	r := NewTactileRouterShard() // includes default routes

	route, ok := r.findRoute("/fs_read")
	if !ok {
		t.Fatalf("expected route for /fs_read")
	}
	if route.ToolName != "fs_read" {
		t.Fatalf("route.ToolName = %q, want %q", route.ToolName, "fs_read")
	}
}
