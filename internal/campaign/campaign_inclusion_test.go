package campaign

import (
	"errors"
	"strings"
	"testing"

	"codenerd/internal/core"
)

func inclusionTestCampaign() *Campaign {
	return &Campaign{
		ID:    "camp-incl",
		Title: "inclusion",
		Phases: []Phase{{
			ID:         "phase-1",
			CampaignID: "camp-incl",
			Name:       "Build",
			Status:     PhasePending,
			Tasks: []Task{
				{ID: "task-1", PhaseID: "phase-1", Description: "first", Status: TaskPending, Type: TaskTypeFileCreate},
				{ID: "task-2", PhaseID: "phase-1", Description: "second", Status: TaskPending, Type: TaskTypeFileCreate, DependsOn: []string{"task-1"}},
			},
		}},
	}
}

// A replanned snapshot whose dependency names a task the plan does not hold
// is quarantined: the sync fails naming the edge and the kernel keeps the
// previous plan's rows.
func TestSyncCampaignFacts_DanglingDependencyIsQuarantined(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	good := inclusionTestCampaign()
	if err := syncCampaignFacts(kernel, nil, good, ""); err != nil {
		t.Fatalf("consistent plan rejected: %v", err)
	}

	bad := inclusionTestCampaign()
	bad.Phases[0].Tasks[1].DependsOn = []string{"task-ghost"}
	bad.Phases[0].Tasks = append(bad.Phases[0].Tasks,
		Task{ID: "task-3", PhaseID: "phase-1", Description: "third", Status: TaskPending, Type: TaskTypeFileCreate})
	err = syncCampaignFacts(kernel, good, bad, "replan")
	if !errors.Is(err, core.ErrInclusionViolation) {
		t.Fatalf("dangling dependency not quarantined: %v", err)
	}
	if !strings.Contains(err.Error(), "task-ghost") {
		t.Fatalf("error does not name the dangling edge: %v", err)
	}

	rows, err := kernel.Query("campaign_task")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("previous plan not preserved: %d campaign_task rows, want 2", len(rows))
	}
	deps, err := kernel.Query("task_dependency")
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0].Args[1] != "task-1" {
		t.Fatalf("previous dependency edge not preserved: %v", deps)
	}
}
