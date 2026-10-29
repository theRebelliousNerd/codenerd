package campaign

import (
	"slices"
	"testing"
	"time"

	"codenerd/internal/core"
)

// External audit N02 (2026-09-19): getEligibleTasks scheduled from an
// in-memory dependency scan whenever the kernel's eligible_task answer was
// empty or its query failed, so every reason the policy holds a task back --
// a write-set conflict, ordering, backoff -- was overridden the moment it held
// back everything. The kernel is the only scheduler now; these run on the
// shipped corpus because a MockKernel derives nothing and would schedule
// nothing either way.

func eligibilityCampaign(tasks ...Task) *Campaign {
	for i := range tasks {
		tasks[i].PhaseID = "/phase_elig"
		if tasks[i].Priority == "" {
			tasks[i].Priority = PriorityNormal
		}
		if tasks[i].Type == "" {
			tasks[i].Type = TaskTypeFileModify
		}
	}
	return &Campaign{
		ID:     "/campaign_elig",
		Type:   CampaignTypeFeature,
		Title:  "eligibility",
		Goal:   "schedule by the kernel",
		Status: StatusActive,
		Phases: []Phase{{
			ID:         "/phase_elig",
			CampaignID: "/campaign_elig",
			Name:       "work",
			Status:     PhaseInProgress,
			Tasks:      tasks,
		}},
	}
}

func eligibleIDs(tasks []*Task) []string {
	var ids []string
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	slices.Sort(ids)
	return ids
}

func realKernelFor(t *testing.T, c *Campaign, load bool) core.Kernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	if load {
		if err := k.LoadFacts(c.ToFacts()); err != nil {
			t.Fatalf("load campaign facts: %v", err)
		}
	}
	return k
}

// A pending task whose write target an in-progress task holds is withheld by
// task_conflict_active. The kernel holds both tasks, so its empty answer is a
// decision, and the orchestrator must not schedule the task anyway.
func TestGetEligibleTasks_AKernelThatHoldsTheTasksAndSchedulesNoneIsObeyed(t *testing.T) {
	shared := []TaskArtifact{{Type: "/source_file", Path: "internal/foo/foo.go"}}
	c := eligibilityCampaign(
		Task{ID: "/task_writing", Status: TaskInProgress, Artifacts: shared},
		Task{ID: "/task_waiting", Status: TaskPending, Artifacts: shared},
	)
	o := &Orchestrator{kernel: realKernelFor(t, c, true), campaign: c}

	if got := eligibleIDs(o.getEligibleTasks(&c.Phases[0])); len(got) != 0 {
		t.Fatalf("scheduled %v while /task_writing holds their write target; the kernel scheduled none", got)
	}
}

// Nothing in a campaign moved current_time, so a retry scheduled after the
// chat last refreshed the clock stayed in backoff for the rest of the run.
// The orchestrator feeds the kernel the wall clock before it asks.
func TestGetEligibleTasks_ARetryIsJudgedOnTheWallClock(t *testing.T) {
	now := time.Now()
	c := eligibilityCampaign(
		Task{ID: "/task_due", Status: TaskPending, NextRetryAt: now.Add(-5 * time.Second)},
		Task{ID: "/task_later", Status: TaskPending, NextRetryAt: now.Add(time.Hour)},
		Task{ID: "/task_fresh", Status: TaskPending},
	)
	k := realKernelFor(t, c, true)
	// The clock as the chat's last turn left it, an hour before the retry came due.
	if err := k.Assert(core.Fact{Predicate: "current_time", Args: []any{now.Add(-time.Hour).Unix()}}); err != nil {
		t.Fatalf("assert stale clock: %v", err)
	}
	o := &Orchestrator{kernel: k, campaign: c}

	got := eligibleIDs(o.getEligibleTasks(&c.Phases[0]))
	if want := []string{"/task_due", "/task_fresh"}; !slices.Equal(got, want) {
		t.Fatalf("eligible = %v, want %v: /task_due's retry is past on the wall clock, /task_later's is not", got, want)
	}
}

// The case the fallback was written for: a kernel that holds none of the
// phase's tasks (a resume whose facts were not loaded). That is missing state,
// not a decision -- the orchestrator reloads the campaign and asks again.
func TestGetEligibleTasks_AKernelMissingTheCampaignIsReloadedAndAsked(t *testing.T) {
	c := eligibilityCampaign(Task{ID: "/task_resumed", Status: TaskPending})
	o := &Orchestrator{kernel: realKernelFor(t, c, false), campaign: c}

	if got := eligibleIDs(o.getEligibleTasks(&c.Phases[0])); !slices.Equal(got, []string{"/task_resumed"}) {
		t.Fatalf("eligible = %v, want [/task_resumed] after the campaign's facts are reloaded", got)
	}
}
