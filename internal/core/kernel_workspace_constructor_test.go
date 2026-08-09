package core

import (
	"path/filepath"
	"testing"
)

func TestNewRealKernelWithWorkspaceInitializesRuntimeDependencies(t *testing.T) {
	workspace := t.TempDir()
	kernel, err := NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace() error = %v", err)
	}

	wantWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if got := kernel.GetWorkspace(); got != wantWorkspace {
		t.Fatalf("GetWorkspace() = %q, want %q", got, wantWorkspace)
	}
	if kernel.GetEventBus() == nil {
		t.Fatal("GetEventBus() = nil; workspace constructor must match the default runtime constructor")
	}
}
