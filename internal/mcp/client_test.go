package mcp

import (
	"testing"
)

func TestNewMCPClientManager_Initialization(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	if mgr == nil {
		t.Fatal("expected non-nil MCPClientManager")
	}
	if mgr.servers == nil {
		t.Error("expected servers map to be initialized")
	}
}

func TestSetToolSelectionConfig_Basic(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	cfg := DefaultToolSelectionConfig()
	cfg.MaxFullTools = 99
	mgr.SetToolSelectionConfig(cfg)
	if mgr.selection.MaxFullTools != 99 {
		t.Errorf("expected MaxFullTools to be 99, got %d", mgr.selection.MaxFullTools)
	}
}

func TestSetOnToolDiscovered_Basic(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	called := false
	mgr.SetOnToolDiscovered(func(tool *MCPTool) {
		called = true
	})

	if mgr.onToolDiscovered == nil {
		t.Fatal("expected onToolDiscovered to be set")
	}

	mgr.onToolDiscovered(&MCPTool{})
	if !called {
		t.Error("expected onToolDiscovered callback to be executed")
	}
}

func TestSetOnServerStatus_Basic(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	called := false
	mgr.SetOnServerStatus(func(serverID string, status ServerStatus) {
		called = true
	})

	if mgr.onServerStatus == nil {
		t.Fatal("expected onServerStatus to be set")
	}

	mgr.onServerStatus("test-server", ServerStatusConnected)
	if !called {
		t.Error("expected onServerStatus callback to be executed")
	}
}
