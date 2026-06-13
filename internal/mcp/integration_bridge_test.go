package mcp

import (
	"context"
	"testing"
)

// TestNewMCPIntegrationBridge builds the integration bridge against a real
// SQLite-backed tool store (temp workspace) and exercises its accessors and the
// unknown-server connect path.
func TestNewMCPIntegrationBridge(t *testing.T) {
	bridge, err := NewMCPIntegrationBridge(t.TempDir(), nil, nil, nil, map[string]MCPServerConfig{})
	if err != nil {
		t.Fatalf("NewMCPIntegrationBridge: %v", err)
	}

	if bridge.GetManager() == nil {
		t.Error("GetManager returned nil")
	}
	if bridge.GetStore() == nil {
		t.Error("GetStore returned nil")
	}
	if bridge.GetCompiler() == nil {
		t.Error("GetCompiler returned nil")
	}
	if bridge.GetRenderer() == nil {
		t.Error("GetRenderer returned nil")
	}

	// GetAdapter lazily creates and then caches an adapter per server ID.
	a1 := bridge.GetAdapter("srv-1")
	if a1 == nil {
		t.Fatal("GetAdapter returned nil")
	}
	if a2 := bridge.GetAdapter("srv-1"); a2 != a1 {
		t.Error("GetAdapter should return the cached adapter for the same server ID")
	}

	// Connecting to a server that isn't configured must error rather than panic.
	if err := bridge.ConnectServer(context.Background(), "does-not-exist"); err == nil {
		t.Error("ConnectServer to an unconfigured server should error")
	}
}
