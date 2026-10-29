package main

import (
	"context"
	"path/filepath"
	"testing"

	"codenerd/internal/mcp"
)

// The defect these guard (F-MCP-1): `nerd mcp list`, `nerd mcp tools` and
// `nerd mcp status` queried mcp_server_registered and mcp_tool_capability.
// Both are declared in schemas_mcp.mg and neither has a producer anywhere in
// the repo — the only MCP fact ever asserted is mcp_tool_vector_score
// (internal/mcp/compiler.go:83). All three commands therefore reported zero
// unconditionally, and would have kept reporting zero with servers connected.
//
// They now read .nerd/mcp_tools.db, which internal/mcp/store.go already
// persists. That also makes them correct for a CLI invocation, which boots a
// fresh kernel holding no session facts.
//
// This is the verification I earlier said required a live MCP server. It does
// not: the store's own API populates it, so the read path can be exercised
// without touching the user's config or standing up a server.

func withWorkspace(t *testing.T, dir string) {
	t.Helper()
	saved := workspace
	workspace = dir
	t.Cleanup(func() { workspace = saved })
}

func TestReadMCPServers_EmptyWorkspaceIsNotAnError(t *testing.T) {
	withWorkspace(t, t.TempDir())

	servers, err := readMCPServers(context.Background())
	if err != nil {
		t.Fatalf("a workspace that never ran MCP must not error: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("got %d servers from a fresh workspace", len(servers))
	}
}

func TestReadMCPServers_SeesAPersistedServer(t *testing.T) {
	dir := t.TempDir()
	withWorkspace(t, dir)

	// Populate through the store's own API — the same call the bridge makes on
	// connect.
	store, err := mcp.NewMCPToolStore(filepath.Join(dir, ".nerd", "mcp_tools.db"), nil)
	if err != nil {
		t.Fatalf("NewMCPToolStore: %v", err)
	}
	err = store.SaveServer(context.Background(), &mcp.MCPServer{
		ID:       "code_graph",
		Name:     "Code Graph",
		Endpoint: "http://localhost:7777",
		Protocol: mcp.ProtocolHTTP,
		Status:   mcp.ServerStatusConnected,
	})
	if err != nil {
		t.Fatalf("SaveServer: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	servers, err := readMCPServers(context.Background())
	if err != nil {
		t.Fatalf("readMCPServers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want the one just persisted — this is the exact case the old kernel query could never see", len(servers))
	}
	if servers[0].ID != "code_graph" {
		t.Errorf("server ID = %q, want code_graph", servers[0].ID)
	}
	if servers[0].Status != mcp.ServerStatusConnected {
		t.Errorf("status = %q, want connected; mcp status counts on this", servers[0].Status)
	}
}

// The store must be read from the --workspace root, not the process's current
// directory, or `nerd -w <dir> mcp list` reports on the wrong workspace.
func TestMCPStorePath_HonoursWorkspace(t *testing.T) {
	dir := t.TempDir()
	withWorkspace(t, dir)

	want := filepath.Join(dir, ".nerd", "mcp_tools.db")
	if got := mcpStorePath(); got != want {
		t.Errorf("mcpStorePath() = %q, want %q", got, want)
	}
}

func TestReadMCPTools_EmptyWorkspaceIsNotAnError(t *testing.T) {
	withWorkspace(t, t.TempDir())

	tools, err := readMCPTools(context.Background())
	if err != nil {
		t.Fatalf("a workspace that never ran MCP must not error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("got %d tools from a fresh workspace", len(tools))
	}
}
