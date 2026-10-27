package chat

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/northstar"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	nerdsystem "codenerd/internal/system"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// chatTierLLM is one configured LLM slot in a chat routing test. It counts
// every call and answers CompleteWithSystem, the plan proposal, with plan.
type chatTierLLM struct {
	calls atomic.Int32
	plan  string
}

func (c *chatTierLLM) Complete(context.Context, string) (string, error) {
	c.calls.Add(1)
	return "", nil
}

func (c *chatTierLLM) CompleteWithSystem(context.Context, string, string) (string, error) {
	c.calls.Add(1)
	return c.plan, nil
}

func (c *chatTierLLM) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan error) {
	c.calls.Add(1)
	text := make(chan string)
	errs := make(chan error, 1)
	close(text)
	errs <- errors.New("chatTierLLM does not stream")
	close(errs)
	return text, errs
}

func (c *chatTierLLM) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	c.calls.Add(1)
	return &types.LLMToolResponse{StopReason: "end_turn"}, nil
}

// chatTierWorker is the worker tier: the observed task executor every chat
// campaign task runs on. It counts the tasks it runs.
type chatTierWorker struct {
	runs atomic.Int32
}

func (w *chatTierWorker) Execute(context.Context, session.TaskRequest) (string, error) {
	w.runs.Add(1)
	return "done", nil
}

func (w *chatTierWorker) ExecuteWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	return w.Execute(ctx, req)
}

func (w *chatTierWorker) ExecuteAsync(ctx context.Context, req session.TaskRequest) (string, error) {
	return w.Execute(ctx, req)
}

func (w *chatTierWorker) GetResult(string) (string, bool, error) { return "", false, nil }

func (w *chatTierWorker) WaitForResult(context.Context, string) (string, error) { return "", nil }

func (w *chatTierWorker) ExecuteObserved(ctx context.Context, req session.TaskRequest) (observation.Return, error) {
	out, err := w.Execute(ctx, req)
	return observation.Return{Output: out, Outcome: "/done"}, err
}

func (w *chatTierWorker) ExecuteObservedWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (observation.Return, error) {
	return w.ExecuteObserved(ctx, req)
}

const chatTierPlanJSON = `{
  "title": "Chat tier plan",
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

// Chat campaigns plan on the planner slot when config.json configures one,
// on main otherwise, and on the chat's own client when no Cortex booted.
func TestCampaignPlanningClient(t *testing.T) {
	main, planner := &chatTierLLM{}, &chatTierLLM{}
	tests := []struct {
		name   string
		cortex *nerdsystem.Cortex
		want   perception.LLMClient
	}{
		{name: "planner configured", cortex: &nerdsystem.Cortex{LLMClient: main, PlannerLLMClient: planner}, want: planner},
		{name: "no planner falls back to main", cortex: &nerdsystem.Cortex{LLMClient: main}, want: main},
		{name: "no cortex uses the chat client", cortex: nil, want: main},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewTestModel(WithLLMClient(main))
			m.cortex = tt.cortex
			if got := m.campaignPlanningClient(); got != tt.want {
				t.Fatalf("campaignPlanningClient() = %p, want %p", got, tt.want)
			}
		})
	}
}

// The orchestrator every chat door builds decomposes on the planning tier;
// main plans only when no planner is configured, and tasks run on the
// session's task executor, the worker tier.
func TestNewCampaignOrchestrator_PlansOnThePlanningTier(t *testing.T) {
	tests := []struct {
		name        string
		withPlanner bool
	}{
		{name: "planner configured", withPlanner: true},
		{name: "no planner falls back to main", withPlanner: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The observer's shared guardian holds a store under the
			// workspace; reset the registry before TempDir cleanup.
			defer northstar.ResetGuardianRegistry()
			main := &chatTierLLM{plan: chatTierPlanJSON}
			planner := &chatTierLLM{plan: chatTierPlanJSON}
			worker := &chatTierWorker{}
			m := NewTestModel(WithRealKernel(t, t.TempDir()), WithLLMClient(main))
			m.cortex = &nerdsystem.Cortex{LLMClient: main}
			wantPlanning := main
			if tt.withPlanner {
				m.cortex.PlannerLLMClient = planner
				wantPlanning = planner
			}
			m.taskExecutor = worker
			m.executor = tactile.NewDirectExecutor()
			m.virtualStore = &core.VirtualStore{}
			m.scanner = world.NewScanner()
			m.shardMgr = coreshards.NewShardManager()

			orch, err := m.newCampaignOrchestrator(nil, nil, nil)
			if err != nil {
				t.Fatalf("newCampaignOrchestrator: %v", err)
			}
			defer func() {
				if err := orch.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
			res, err := orch.Decompose(context.Background(), campaign.DecomposeRequest{
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
			if n := worker.runs.Load(); n != 0 {
				t.Fatalf("decomposition ran %d task(s) on the worker", n)
			}
		})
	}
}

// `/campaign add` hands the requirement to the running campaign's
// orchestrator, which plans it on the planning tier; with no running campaign
// it is refused and nothing is planned.
func TestCampaignAddCommand_PlansTheRequirementOnThePlanningTier(t *testing.T) {
	defer northstar.ResetGuardianRegistry()
	main := &chatTierLLM{}
	planner := &chatTierLLM{}
	plannerReply := `{"new_tasks": [{"phase_order": 0, "description": "Research the added requirement", "type": "/research", "priority": "/high"}], "modified_tasks": [], "summary": "added"}`
	plannerComplete := &chatCompleteLLM{chatTierLLM: planner, reply: plannerReply}

	t.Run("no running campaign", func(t *testing.T) {
		m := NewTestModel(WithLLMClient(main))
		updated, cmd := m.handleCommand("/campaign add also research the routing")
		if cmd != nil {
			t.Fatal("a requirement with no running campaign must not start any work")
		}
		if got := lastChatMessage(updated.(Model)); !strings.Contains(got, "Requirement not added") {
			t.Fatalf("reply = %q, want a refusal", got)
		}
	})

	t.Run("running campaign", func(t *testing.T) {
		workspace := t.TempDir()
		m := NewTestModel(WithRealKernel(t, workspace), WithLLMClient(main))
		m.cortex = &nerdsystem.Cortex{LLMClient: main, PlannerLLMClient: plannerComplete}
		m.taskExecutor = &chatTierWorker{}
		m.executor = tactile.NewDirectExecutor()
		m.virtualStore = &core.VirtualStore{}
		m.scanner = world.NewScanner()
		m.shardMgr = coreshards.NewShardManager()
		orch, err := m.newCampaignOrchestrator(nil, nil, nil)
		if err != nil {
			t.Fatalf("newCampaignOrchestrator: %v", err)
		}
		defer func() {
			if err := orch.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		}()
		camp := &campaign.Campaign{
			ID: "/campaign_chat_add", Type: campaign.CampaignTypeCustom, Title: "chat add", Goal: "g",
			Status: campaign.StatusActive, TotalPhases: 1, TotalTasks: 1,
			Phases: []campaign.Phase{{
				ID: "/phase_chat_add", CampaignID: "/campaign_chat_add", Name: "p", Status: campaign.PhaseInProgress,
				Tasks: []campaign.Task{{ID: "/task_chat_add", PhaseID: "/phase_chat_add", Description: "research the plan", Type: campaign.TaskTypeResearch, Status: campaign.TaskPending}},
			}},
		}
		if err := orch.SetCampaign(camp); err != nil {
			t.Fatalf("SetCampaign: %v", err)
		}
		m.campaignOrch = orch
		m.activeCampaign = camp

		updated, cmd := m.handleCommand("/campaign add also research the routing")
		if cmd == nil {
			t.Fatalf("no work started; reply = %q", lastChatMessage(updated.(Model)))
		}
		msg := cmd()
		next, _ := updated.(Model).Update(msg)
		if got := lastChatMessage(next.(Model)); !strings.Contains(got, "Requirement added to the campaign plan") {
			t.Fatalf("reply = %q, want the requirement added", got)
		}
		if n := planner.calls.Load(); n != 1 {
			t.Fatalf("the requirement was planned on the planner %d time(s), want 1", n)
		}
		if n := main.calls.Load(); n != 0 {
			t.Fatalf("with a planner configured, the requirement called main %d time(s)", n)
		}
		if got := orch.GetProgress().TotalTasks; got != 2 {
			t.Fatalf("campaign has %d task(s), want the added one too", got)
		}
	})
}

// chatCompleteLLM is a planner slot whose Complete answers with reply, the
// replanner's call shape.
type chatCompleteLLM struct {
	*chatTierLLM
	reply string
}

func (c *chatCompleteLLM) Complete(context.Context, string) (string, error) {
	c.calls.Add(1)
	return c.reply, nil
}
