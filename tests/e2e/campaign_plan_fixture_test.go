//go:build integration

package e2e_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/campaign"
	nerdconfig "codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
)

func registerCampaignFixtureCleanup(t *testing.T, orch *campaign.Orchestrator) {
	t.Helper()
	t.Cleanup(func() {
		if err := orch.Close(); err != nil {
			t.Errorf("close campaign fixture: %v", err)
		}
	})
}

// Use the same real admission boundary as the session fixtures, exposing its
// kernel for publication checks. These controls do not run or verify a task.
func newCampaignAdmissionFixture(t *testing.T) (*campaign.Orchestrator, *core.RealKernel) {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vs, llm := &campaignMockVirtualStore{}, &campaignMockLLMClient{}
	jit, cf, trans := &campaignMockJITCompiler{}, &campaignMockConfigFactory{}, &campaignMockTransducer{intentToReturn: "/fix"}
	exec := session.NewExecutor(kernel, vs, llm, jit, cf, trans)
	exec.SetConfig(session.DefaultExecutorConfig())
	spawner := session.NewSpawner(kernel, vs, llm, jit, cf, trans, session.DefaultSpawnerConfig())
	orch, err := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace: t.TempDir(), Kernel: kernel, LLMClient: llm, PlanningLLMClient: llm,
		Executor: tactile.NewDirectExecutor(), VirtualStore: &core.VirtualStore{},
		ShardManager: coreshards.NewShardManager(), TaskExecutor: session.NewJITExecutor(exec, spawner, trans),
		Campaign: nerdconfig.CampaignConfig{MaxParallelTasks: 3, MaxTaskAttempts: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	registerCampaignFixtureCleanup(t, orch)
	return orch, kernel
}

func TestCampaignSessionFixturePlanAdmission(t *testing.T) {
	for _, variant := range []string{"base", "appended", "replaced"} {
		t.Run(variant, func(t *testing.T) {
			orch, kernel := newCampaignAdmissionFixture(t)
			plan := createDummyCampaign("/admission_" + variant)
			if variant != "base" {
				task := campaign.Task{ID: "/extra", PhaseID: plan.Phases[0].ID,
					Description: "Fix file B", Status: campaign.TaskPending, Type: campaign.TaskTypeFileModify}
				if variant == "appended" {
					task.DependsOn = []string{plan.Phases[0].Tasks[0].ID}
					task.ContextFrom = []string{plan.Phases[0].Tasks[0].ID}
					plan.Phases[0].Tasks = append(plan.Phases[0].Tasks, task)
				} else {
					plan.Phases[0].Tasks = []campaign.Task{task}
				}
			}
			if err := orch.SetCampaign(plan); err != nil {
				t.Fatalf("valid fixture plan rejected: %v", err)
			}
			rows, err := kernel.Query("campaign_task")
			if err != nil || len(rows) != len(plan.Phases[0].Tasks) {
				t.Fatalf("admitted task rows = %v, %v", rows, err)
			}
			byID := make(map[string]core.Fact)
			for _, row := range rows {
				byID[fmt.Sprint(row.Args[0])] = row
			}
			for _, task := range plan.Phases[0].Tasks {
				want := []any{task.ID, plan.Phases[0].ID, task.Description, string(campaign.TaskPending), string(campaign.TaskTypeFileModify)}
				if !reflect.DeepEqual(byID[task.ID].Args, want) {
					t.Errorf("task fact = %v, want %v", byID[task.ID].Args, want)
				}
				if task.Type != campaign.TaskTypeFileModify || task.Status != campaign.TaskPending || len(task.WriteSet) != 0 {
					t.Errorf("admission changed task obligations or invented execution: %+v", task)
				}
			}
			for _, predicate := range []string{"task_dependency", "task_context_from"} {
				rows, err := kernel.Query(predicate)
				if err != nil {
					t.Fatal(err)
				}
				if variant == "appended" {
					if len(rows) != 1 || !reflect.DeepEqual(rows[0].Args, []any{"/extra", "/task_1"}) {
						t.Errorf("%s edge lost: %v", predicate, rows)
					}
				} else if len(rows) != 0 {
					t.Errorf("invented %s edges: %v", predicate, rows)
				}
			}
		})
	}
}

func TestCampaignSessionFixturePlanRejectsMalformedEdges(t *testing.T) {
	for _, defect := range []string{"missing_phase", "foreign_phase", "dangling_dependency", "dangling_context"} {
		t.Run(defect, func(t *testing.T) {
			orch, kernel := newCampaignAdmissionFixture(t)
			plan := createDummyCampaign("/reject_" + defect)
			// Supply a valid base independently of the repaired fixture, so the
			// negative controls also run unchanged against the original baseline.
			task := &plan.Phases[0].Tasks[0]
			task.PhaseID = plan.Phases[0].ID
			switch defect {
			case "missing_phase":
				task.PhaseID = ""
			case "foreign_phase":
				task.PhaseID = "/not_in_this_plan"
			case "dangling_dependency":
				task.DependsOn = []string{"/missing_task"}
			case "dangling_context":
				task.ContextFrom = []string{"/missing_task"}
			}
			err := orch.SetCampaign(plan)
			if err == nil || !strings.Contains(err.Error(), "plan not loaded") || !strings.Contains(err.Error(), "quarantined") {
				t.Fatalf("invalid plan was not quarantined: %v", err)
			}
			for _, predicate := range []string{"campaign", "campaign_phase", "campaign_task", "task_dependency", "task_context_from"} {
				rows, err := kernel.Query(predicate)
				if err != nil || len(rows) != 0 {
					t.Errorf("rejected plan published %s rows = %v, %v", predicate, rows, err)
				}
			}
		})
	}
}
