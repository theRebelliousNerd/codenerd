package campaign

import (
	"context"
	"fmt"
	"testing"
	"time"

	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// stubLLM implements perception.LLMClient for unit tests.
type stubLLM struct{}

func (s *stubLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return "ok", nil
}
func (s *stubLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "ok", nil
}
func (s *stubLLM) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "ok", StopReason: "end_turn"}, nil
}

func TestOrchestrator_AssertsCampaignConfigFacts(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	orch := NewOrchestrator(OrchestratorConfig{
		Workspace:        t.TempDir(),
		Kernel:           kernel,
		LLMClient:        &stubLLM{},
		ShardManager:     coreshards.NewShardManager(),
		MaxRetries:       5,
		ReplanThreshold:  2,
		AutoReplan:       true,
		CheckpointOnFail: true,
	})

	now := time.Now()
	c := &Campaign{
		ID:        "/campaign_test",
		Type:      CampaignTypeCustom,
		Title:     "Test",
		Goal:      "Goal",
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("SetCampaign() error = %v", err)
	}

	facts, err := kernel.Query("campaign_config")
	if err != nil {
		t.Fatalf("Query(campaign_config) error = %v", err)
	}
	if len(facts) == 0 {
		t.Fatalf("expected campaign_config fact, got none")
	}

	found := false
	for _, f := range facts {
		if len(f.Args) < 5 {
			continue
		}
		if fmt.Sprintf("%v", f.Args[0]) != c.ID {
			continue
		}
		found = true
		if fmt.Sprintf("%v", f.Args[1]) != "5" || fmt.Sprintf("%v", f.Args[2]) != "2" {
			t.Fatalf("unexpected config args: %v", f.Args)
		}
		if f.Args[3] != "/true" || f.Args[4] != "/true" {
			t.Fatalf("unexpected boolean config args: %v", f.Args)
		}
	}
	if !found {
		t.Fatalf("campaign_config for %s not found: %v", c.ID, facts)
	}
}

func TestKernel_ReplanNeededRespectsCampaignConfig(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	now := time.Now()
	c := Campaign{
		ID:        "/campaign_test",
		Type:      CampaignTypeCustom,
		Title:     "Test",
		Goal:      "Goal",
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_ = kernel.LoadFacts(c.ToFacts())

	_ = kernel.Assert(core.Fact{
		Predicate: "campaign_config",
		Args:      []interface{}{c.ID, 3, 2, "/true", "/false"},
	})
	_ = kernel.Assert(core.Fact{
		Predicate: "failed_campaign_task_count_computed",
		Args:      []interface{}{c.ID, 2},
	})

	facts, _ := kernel.Query("replan_needed")
	hasCascade := false
	for _, f := range facts {
		if len(f.Args) >= 2 &&
			fmt.Sprintf("%v", f.Args[0]) == c.ID &&
			fmt.Sprintf("%v", f.Args[1]) == "task_failure_cascade" {
			hasCascade = true
		}
	}
	if !hasCascade {
		t.Fatalf("expected task_failure_cascade replan_needed, got %v", facts)
	}

	// AutoReplan disabled should suppress cascade rule
	kernel2, _ := core.NewRealKernel()
	_ = kernel2.LoadFacts(c.ToFacts())
	_ = kernel2.Assert(core.Fact{
		Predicate: "campaign_config",
		Args:      []interface{}{c.ID, 3, 1, "/false", "/false"},
	})
	_ = kernel2.Assert(core.Fact{
		Predicate: "failed_campaign_task_count_computed",
		Args:      []interface{}{c.ID, 5},
	})
	facts2, _ := kernel2.Query("replan_needed")
	for _, f := range facts2 {
		if len(f.Args) >= 2 &&
			fmt.Sprintf("%v", f.Args[0]) == c.ID &&
			fmt.Sprintf("%v", f.Args[1]) == "task_failure_cascade" {
			t.Fatalf("did not expect cascade replan when autoReplan disabled, got %v", facts2)
		}
	}
}

func TestContextPager_ResetPhaseContextClearsFacts(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	cp := NewContextPager(kernel, &stubLLM{})
	_ = kernel.Assert(core.Fact{
		Predicate: "activation",
		Args:      []interface{}{"file_pattern(\"**/*\")", 100},
	})
	_ = kernel.Assert(core.Fact{
		Predicate: "phase_context_atom",
		Args:      []interface{}{"/phase1", "file_topology(\"x\",_,_,_,_)", 120},
	})

	cp.ResetPhaseContext()

	act, _ := kernel.Query("activation")
	if len(act) != 0 {
		t.Fatalf("expected activation facts cleared, got %v", act)
	}
	phaseAtoms, _ := kernel.Query("phase_context_atom")
	if len(phaseAtoms) != 0 {
		t.Fatalf("expected phase_context_atom facts cleared, got %v", phaseAtoms)
	}
}
