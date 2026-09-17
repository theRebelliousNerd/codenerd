package campaign

import (
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/northstar"
	"codenerd/internal/tactile"
)

func TestNewOrchestrator_ClosesObserverOnInvalidConfig(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	obs := northstar.BuildCampaignObserver(ws, nil, nil)
	if obs == nil {
		t.Fatal("BuildCampaignObserver returned nil")
	}
	if got := northstar.GuardianRefCount(nerdDir); got != 1 {
		t.Fatalf("GuardianRefCount = %d, want 1", got)
	}
	_, err := NewOrchestrator(OrchestratorConfig{Workspace: ws, NorthstarObserver: obs})
	if err == nil {
		t.Fatal("NewOrchestrator succeeded with missing kernel, want error")
	}
	if got := northstar.GuardianRefCount(nerdDir); got != 0 {
		t.Fatalf("GuardianRefCount after failed NewOrchestrator = %d, want 0", got)
	}
}

func TestOrchestrator_CloseReleasesObserver(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	obs := northstar.BuildCampaignObserver(ws, nil, nil)
	if obs == nil {
		t.Fatal("BuildCampaignObserver returned nil")
	}
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("real kernel unavailable: %v", err)
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:         ws,
		Kernel:            kernel,
		LLMClient:         &MockLLMClient{},
		TaskExecutor:      &MockTaskExecutor{},
		Executor:          tactile.NewDirectExecutor(),
		VirtualStore:      &core.VirtualStore{},
		NorthstarObserver: obs,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	if got := northstar.GuardianRefCount(nerdDir); got != 1 {
		t.Fatalf("GuardianRefCount after NewOrchestrator = %d, want 1", got)
	}
	if err := orch.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := northstar.GuardianRefCount(nerdDir); got != 0 {
		t.Fatalf("GuardianRefCount after Close = %d, want 0", got)
	}
	if err := orch.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if got := northstar.GuardianRefCount(nerdDir); got != 0 {
		t.Fatalf("GuardianRefCount after second Close = %d, want 0", got)
	}
}
