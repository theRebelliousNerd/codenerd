package mcp

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreServersAndToolsRoundTrip(t *testing.T) {
	store, err := NewMCPToolStore(filepath.Join(t.TempDir(), "mcp.db"), nil)
	if err != nil {
		t.Fatalf("NewMCPToolStore: %v", err)
	}
	ctx := context.Background()

	// Empty store returns no servers/tools (cross-boundary: store -> sqlite).
	if servers, err := store.GetAllServers(ctx); err != nil || len(servers) != 0 {
		t.Fatalf("empty GetAllServers=(%v,%v), want (empty,nil)", servers, err)
	}

	for _, id := range []string{"srv1", "srv2"} {
		s := &MCPServer{ID: id, Name: id, Endpoint: "http://" + id, Protocol: ProtocolHTTP, Status: ServerStatusConnected}
		if err := store.SaveServer(ctx, s); err != nil {
			t.Fatalf("SaveServer(%s): %v", id, err)
		}
	}
	servers, err := store.GetAllServers(ctx)
	if err != nil || len(servers) != 2 {
		t.Fatalf("GetAllServers after save=(%d,%v), want (2,nil)", len(servers), err)
	}

	if err := store.SaveTool(ctx, &MCPTool{ToolID: "t1", ServerID: "srv1", Name: "Tool1"}); err != nil {
		t.Fatalf("SaveTool: %v", err)
	}
	tools, err := store.GetToolsByServer(ctx, "srv1")
	if err != nil || len(tools) != 1 || tools[0].ToolID != "t1" {
		t.Fatalf("GetToolsByServer(srv1)=(%+v,%v), want 1 tool t1", tools, err)
	}
	if tools, err := store.GetToolsByServer(ctx, "nonexistent"); err != nil || len(tools) != 0 {
		t.Errorf("GetToolsByServer(nonexistent)=(%d,%v), want (0,nil)", len(tools), err)
	}
}
