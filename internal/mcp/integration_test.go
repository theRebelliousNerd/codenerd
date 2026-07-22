package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMCPIntegrationBridge_Success(t *testing.T) {
	workspace := t.TempDir()
	serverConfigs := map[string]MCPServerConfig{
		"srv1": {ID: "srv1"},
	}

	bridge, err := NewMCPIntegrationBridge(workspace, nil, nil, nil, serverConfigs)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer bridge.Close()

	if bridge == nil {
		t.Fatal("expected non-nil bridge")
	}

	if bridge.GetManager() == nil {
		t.Error("expected non-nil manager")
	}
	if bridge.GetStore() == nil {
		t.Error("expected non-nil store")
	}
	if bridge.GetCompiler() == nil {
		t.Error("expected non-nil compiler")
	}
	if bridge.GetRenderer() == nil {
		t.Error("expected non-nil renderer")
	}
}

func TestNewMCPIntegrationBridge_StoreError(t *testing.T) {
	workspace := t.TempDir()

	// Create .nerd as a regular file, which will cause creating mcp_tools.db inside it to fail
	nerdFilePath := filepath.Join(workspace, ".nerd")
	f, err := os.Create(nerdFilePath)
	if err != nil {
		t.Fatalf("failed to create dummy .nerd file: %v", err)
	}
	f.Close()

	serverConfigs := map[string]MCPServerConfig{}

	bridge, err := NewMCPIntegrationBridge(workspace, nil, nil, nil, serverConfigs)
	if err == nil {
		bridge.Close()
		t.Fatal("expected error due to invalid db path, got nil")
	}

	if bridge != nil {
		t.Error("expected nil bridge when error occurs")
	}
}
