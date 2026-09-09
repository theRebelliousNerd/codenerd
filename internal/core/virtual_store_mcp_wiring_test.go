package core

import (
	"testing"

	"codenerd/internal/tools"
)

// mcpControlPlaneVerbs is the fixed model-facing MCP surface.
var mcpControlPlaneVerbs = []string{
	"mcp_map", "mcp_probe", "mcp_call", "mcp_expand", "mcp_context",
}

// TestHydrateModularTools_RegistersMCPControlPlaneOnBothRegistries proves the
// seam that decides whether MCP is reachable at all.
//
// The registration is deliberately split across two registries: the VirtualStore
// runs tools through its own, and session.Executor builds both the piggyback
// catalog and the native function-call definitions from tools.Global(). A tool
// present in only one of them is callable by one path and invisible to the
// other, which reads as an intermittent capability rather than a wiring bug.
func TestHydrateModularTools_RegistersMCPControlPlaneOnBothRegistries(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	if err := vs.HydrateModularTools(); err != nil {
		t.Fatalf("HydrateModularTools() error = %v", err)
	}

	local := vs.GetModularTools()
	if local == nil {
		t.Fatal("GetModularTools() = nil")
	}
	global := tools.Global()

	for _, name := range mcpControlPlaneVerbs {
		if !local.Has(name) {
			t.Errorf("%s missing from the VirtualStore registry; RouteAction cannot reach it", name)
		}
		if !global.Has(name) {
			t.Errorf("%s missing from tools.Global(); neither prompt path can describe it", name)
		}
		// A tool with no valid effect declaration fails closed at
		// LookupEffect, so an unregistered effect makes the tool
		// permanently unusable rather than merely undescribed.
		tool := global.Get(name)
		if tool == nil {
			continue
		}
		if _, err := tool.DeclaredEffect(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestHydrateModularTools_MCPSurfaceIsFixedSize pins the count.
//
// Five verbs regardless of how many MCP servers are connected is the whole
// economic argument. If this grows, the per-turn cost has started scaling with
// the catalog again.
func TestHydrateModularTools_MCPSurfaceIsFixedSize(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	if err := vs.HydrateModularTools(); err != nil {
		t.Fatalf("HydrateModularTools() error = %v", err)
	}

	local := vs.GetModularTools()
	count := 0
	for _, name := range local.Names() {
		if len(name) > 4 && name[:4] == "mcp_" {
			count++
		}
	}
	if count != len(mcpControlPlaneVerbs) {
		t.Errorf("registered %d mcp_* tools, want exactly %d", count, len(mcpControlPlaneVerbs))
	}
}
