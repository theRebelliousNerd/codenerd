package campaign

import (
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/tactile"
	"testing"
)

// NewOrchestrator keeps the collaborators it is given, and gives every door
// the workspace's specialist knowledge without being asked.
func TestOrchestrator_DependencyInjection(t *testing.T) {
	mockKernel := &MockKernel{}
	execution := &MockLLMClient{}
	planning := &MockLLMClient{}
	worker := &MockTaskExecutor{}
	workspace := t.TempDir()

	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:         workspace,
		Kernel:            mockKernel,
		LLMClient:         execution,
		PlanningLLMClient: planning,
		Executor:          tactile.NewDirectExecutor(),
		VirtualStore:      &core.VirtualStore{},
		ShardManager:      coreshards.NewShardManager(),
		TaskExecutor:      worker,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator() error = %v", err)
	}

	if orch.kernel != mockKernel {
		t.Error("kernel injection failed")
	}
	if orch.llmClient != execution || orch.planningClient != planning {
		t.Error("LLM client injection failed")
	}
	if orch.taskExecutor != worker || orch.checkpoint.taskExecutor != worker {
		t.Error("task executor injection failed")
	}
	local, ok := orch.specialistKnowledgeProvider.(*LocalSpecialistKnowledgeProvider)
	if !ok || local.workspace != workspace {
		t.Errorf("specialist knowledge provider = %#v, want the workspace's local provider", orch.specialistKnowledgeProvider)
	}
}
