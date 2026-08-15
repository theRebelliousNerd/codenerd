package mcp

import (
	"testing"
)

func TestMCPClientManager_BasicInitialization(t *testing.T) {
	config := map[string]MCPServerConfig{
		"test_server": {
			ID:      "test_server",
			Enabled: true,
		},
	}
	manager := NewMCPClientManager(nil, nil, config)

	if manager == nil {
		t.Fatal("Expected manager to not be nil")
	}

	if manager.servers == nil {
		t.Error("Expected servers map to be initialized")
	}

	if manager.config == nil {
		t.Error("Expected config to be initialized")
	}
}

func TestMCPClientManager_SettersBasic(t *testing.T) {
	manager := NewMCPClientManager(nil, nil, nil)

	// Test SetToolSelectionConfig
	selectionConfig := ToolSelectionConfig{
		MaxFullTools: 10,
	}
	manager.SetToolSelectionConfig(selectionConfig)
	if manager.selection.MaxFullTools != 10 {
		t.Errorf("Expected MaxFullTools to be 10, got %d", manager.selection.MaxFullTools)
	}

	// Test SetOnToolDiscovered
	toolDiscoveredCalled := false
	manager.SetOnToolDiscovered(func(tool *MCPTool) {
		toolDiscoveredCalled = true
	})

	if manager.onToolDiscovered == nil {
		t.Fatal("Expected onToolDiscovered to not be nil")
	}

	manager.onToolDiscovered(nil)
	if !toolDiscoveredCalled {
		t.Error("Expected onToolDiscovered to be called")
	}

	// Test SetOnServerStatus
	serverStatusCalled := false
	manager.SetOnServerStatus(func(serverID string, status ServerStatus) {
		serverStatusCalled = true
	})

	if manager.onServerStatus == nil {
		t.Fatal("Expected onServerStatus to not be nil")
	}

	manager.onServerStatus("test_server", ServerStatusConnected)
	if !serverStatusCalled {
		t.Error("Expected onServerStatus to be called")
	}
}
