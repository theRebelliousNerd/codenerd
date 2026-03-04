package campaign

import (
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/tactile"
	"errors"
	"testing"
	"time"
)

func TestNewOrchestrator_RejectsMissingDependencies(t *testing.T) {
	_, err := NewOrchestrator(OrchestratorConfig{
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
	if !errors.Is(err, ErrNilDependency) {
		t.Fatalf("expected ErrNilDependency, got %v", err)
	}
}

func TestNewOrchestrator_RejectsNegativeConfigurationValues(t *testing.T) {
	_, err := NewOrchestrator(OrchestratorConfig{
		Workspace:        t.TempDir(),
		Kernel:           &MockKernel{},
		LLMClient:        &MockLLMClient{},
		ShardManager:     coreshards.NewShardManager(),
		Executor:         tactile.NewDirectExecutor(),
		VirtualStore:     &core.VirtualStore{},
		MaxParallelTasks: -1,
		CampaignTimeout:  -time.Second,
	})
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestNewOrchestrator_AcceptsValidMinimalConfiguration(t *testing.T) {
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       &MockKernel{},
		LLMClient:    &MockLLMClient{},
		ShardManager: coreshards.NewShardManager(),
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
	})
	if err != nil {
		t.Fatalf("unexpected NewOrchestrator error: %v", err)
	}
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
}
