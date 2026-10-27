package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	coresys "codenerd/internal/system"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// tierClient is one configured LLM slot (main, worker or planner) in a CLI
// routing test. It counts every call and answers CompleteWithSystem, the
// plan proposal, with plan.
type tierClient struct {
	calls atomic.Int32
	plan  string
}

func (c *tierClient) Complete(context.Context, string) (string, error) {
	c.calls.Add(1)
	return "", nil
}

func (c *tierClient) CompleteWithSystem(context.Context, string, string) (string, error) {
	c.calls.Add(1)
	return c.plan, nil
}

func (c *tierClient) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan error) {
	c.calls.Add(1)
	text := make(chan string)
	errs := make(chan error, 1)
	close(text)
	errs <- errors.New("tierClient does not stream")
	close(errs)
	return text, errs
}

func (c *tierClient) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	c.calls.Add(1)
	return &types.LLMToolResponse{StopReason: "end_turn"}, nil
}

const tierPlanJSON = `{
  "title": "Tier routing plan",
  "confidence": 0.9,
  "phases": [{
    "name": "Phase 1", "order": 0, "category": "/scaffold",
    "description": "Write the routing note", "objective_type": "/create",
    "verification_method": "/none", "complexity": "/low",
    "tasks": [{
      "description": "Write Docs/tier.md", "type": "/file_create",
      "priority": "/normal", "order": 0,
      "artifacts": ["Docs/tier.md"], "write_set": ["Docs/tier.md"]
    }]
  }]
}`

// `nerd campaign start` plans on the planner slot when config.json configures
// one and on main otherwise; the worker slot never plans, and tasks run on the
// Cortex's task executor, the worker tier.
func TestCampaignStart_PlansOnThePlanningTier(t *testing.T) {
	tests := []struct {
		name        string
		withPlanner bool
	}{
		{name: "planner configured", withPlanner: true},
		{name: "no planner falls back to main", withPlanner: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The builder's northstar observer holds a shared guardian under
			// cwd/.nerd; reset the registry before TempDir cleanup.
			defer northstar.ResetGuardianRegistry()
			kern, err := core.NewRealKernel()
			if err != nil {
				t.Fatalf("NewRealKernel: %v", err)
			}
			main := &tierClient{plan: tierPlanJSON}
			worker := &tierClient{plan: tierPlanJSON}
			planner := &tierClient{plan: tierPlanJSON}
			tasks := &stubCampaignTaskExecutor{}
			cortex := &coresys.Cortex{
				Kernel:          kern,
				RealKernel:      kern,
				LLMClient:       main,
				WorkerLLMClient: worker,
				VirtualStore:    core.NewVirtualStoreWithConfig(tactile.NewDirectExecutor(), core.DefaultVirtualStoreConfig()),
				ShardManager:    coreshards.NewShardManager(),
				TaskExecutor:    tasks,
				Executor:        tactile.NewDirectExecutor(),
			}
			wantPlanning := main
			if tt.withPlanner {
				cortex.PlannerLLMClient = planner
				wantPlanning = planner
			}

			cfg, prompts, err := buildCampaignOrchestratorConfig(cortex, t.TempDir(), nil, nil)
			if err != nil {
				t.Fatalf("buildCampaignOrchestratorConfig: %v", err)
			}
			if cfg.PlanningLLMClient != perception.LLMClient(wantPlanning) {
				t.Fatalf("PlanningLLMClient = %p, want %p", cfg.PlanningLLMClient, wantPlanning)
			}
			if cfg.LLMClient != perception.LLMClient(main) {
				t.Fatalf("LLMClient = %p, want the main client %p", cfg.LLMClient, main)
			}
			if cfg.TaskExecutor != cortex.TaskExecutor {
				t.Fatal("tasks must run on the Cortex's task executor, the worker tier")
			}

			orchestrator, err := newCampaignOrchestrator(cfg, prompts)
			if err != nil {
				t.Fatalf("newCampaignOrchestrator: %v", err)
			}
			defer func() {
				if err := orchestrator.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
			res, err := orchestrator.Decompose(context.Background(), campaign.DecomposeRequest{
				Goal:         "Write the tier routing note",
				CampaignType: campaign.CampaignTypeCustom,
			})
			if err != nil {
				t.Fatalf("Decompose: %v", err)
			}
			if res == nil || res.Campaign == nil || len(res.Campaign.Phases) == 0 {
				t.Fatalf("Decompose returned no plan: %+v", res)
			}

			if wantPlanning.calls.Load() == 0 {
				t.Fatal("decomposition never called the planning slot")
			}
			if tt.withPlanner {
				if n := main.calls.Load(); n != 0 {
					t.Fatalf("with a planner configured, decomposition called main %d time(s)", n)
				}
			}
			if n := worker.calls.Load(); n != 0 {
				t.Fatalf("decomposition called the worker LLM slot %d time(s)", n)
			}
			// Step 0's shard consults are worker-tier tasks whose advice
			// reaches the planner's prompt.
			if tasks.runs.Load() == 0 {
				t.Fatal("pre-planning intelligence consulted no specialist on the task executor")
			}
		})
	}
}
