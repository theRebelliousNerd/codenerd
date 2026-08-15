package campaign

import (
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/tactile"
	"errors"
	"strings"
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

// A minimal valid configuration now includes a TaskExecutor.
//
// This test used to assert that a ShardManager alone was enough, which encoded
// the old "TaskExecutor OR ShardManager" contract. That assertion was wrong:
// ShardManager is only consulted for monitoring, so an orchestrator built that
// way could not execute a single task (spawnTask returns "taskExecutor not
// initialized") and could not run the shard-validation or Nemesis checkpoints.
// It described a configuration that constructs, not one that works.
func TestNewOrchestrator_AcceptsValidMinimalConfiguration(t *testing.T) {
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       &MockKernel{},
		LLMClient:    &MockLLMClient{},
		ShardManager: coreshards.NewShardManager(),
		TaskExecutor: &MockTaskExecutor{},
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

// A nil TaskExecutor must be refused at construction, whether it arrives as a
// nil interface or as a typed nil from an accessor whose backing field was
// never assigned (CampaignRunnerShard.TaskExecutor() does exactly that until
// the boot closure calls SetTaskExecutor).
//
// Without this, the field is present at every call site and still nil at
// runtime, which is how unverifiable campaigns shipped: both verification
// checkpoints have no executor to run on.
func TestNewOrchestrator_WhenTaskExecutorNil_ShouldRejectConfig(t *testing.T) {
	base := func() OrchestratorConfig {
		return OrchestratorConfig{
			Workspace:    t.TempDir(),
			Kernel:       &MockKernel{},
			LLMClient:    &MockLLMClient{},
			ShardManager: coreshards.NewShardManager(),
			Executor:     tactile.NewDirectExecutor(),
			VirtualStore: &core.VirtualStore{},
		}
	}

	t.Run("nil interface", func(t *testing.T) {
		cfg := base()
		cfg.TaskExecutor = nil
		_, err := NewOrchestrator(cfg)
		if err == nil {
			t.Fatal("expected nil TaskExecutor to be refused")
		}
		if !errors.Is(err, ErrNilDependency) {
			t.Fatalf("expected ErrNilDependency, got %v", err)
		}
		if !strings.Contains(err.Error(), "task_executor") {
			t.Fatalf("error must name the missing dependency so the fix is obvious; got %v", err)
		}
	})

	t.Run("typed nil", func(t *testing.T) {
		cfg := base()
		var typedNil *MockTaskExecutor
		cfg.TaskExecutor = typedNil
		_, err := NewOrchestrator(cfg)
		if err == nil {
			t.Fatal("expected typed-nil TaskExecutor to be refused: it is non-nil as an interface and unusable in practice")
		}
		if !errors.Is(err, ErrNilDependency) {
			t.Fatalf("expected ErrNilDependency, got %v", err)
		}
	})
}

// SetTaskExecutor must never downgrade a working orchestrator.
func TestSetTaskExecutor_WhenNil_ShouldKeepExistingExecutor(t *testing.T) {
	wired := &MockTaskExecutor{}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       &MockKernel{},
		LLMClient:    &MockLLMClient{},
		TaskExecutor: wired,
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
	})
	if err != nil {
		t.Fatalf("unexpected NewOrchestrator error: %v", err)
	}

	orch.SetTaskExecutor(nil)
	if orch.taskExecutor != wired {
		t.Fatal("nil SetTaskExecutor replaced a working executor")
	}
	var typedNil *MockTaskExecutor
	orch.SetTaskExecutor(typedNil)
	if orch.taskExecutor != wired {
		t.Fatal("typed-nil SetTaskExecutor replaced a working executor")
	}

	replacement := &MockTaskExecutor{}
	orch.SetTaskExecutor(replacement)
	if orch.taskExecutor != replacement {
		t.Fatal("a real executor should still replace the wired one")
	}
	// The checkpoint runner must follow, or verification keeps using the old one.
	if orch.checkpoint.taskExecutor != replacement {
		t.Fatal("checkpoint runner kept the previous executor; verification would run on a stale collaborator")
	}
}
