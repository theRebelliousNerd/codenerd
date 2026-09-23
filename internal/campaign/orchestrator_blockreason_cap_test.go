package campaign

import (
	"codenerd/internal/config"
	"codenerd/internal/core"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

// TestFailCampaign_PersistsBlockReasonFromTaskLoop covers the live 2026-09-04
// regression (campaign 5a2f4c8d second resume): the task-loop block path in
// runPhase marked the campaign failed without persisting BlockReason, leaving
// status /failed with an empty block_reason so resume could not prefer it.
func TestFailCampaign_PersistsBlockReasonFromTaskLoop(t *testing.T) {
	kernel := &MockKernel{}
	_ = kernel.Assert(core.Fact{
		Predicate: "campaign_blocked",
		Args:      []any{"campaign_block_taskloop", "/all_tasks_blocked"},
	})

	tmp := t.TempDir()
	o := &Orchestrator{
		kernel: kernel,
		campaign: &Campaign{
			ID:     "campaign_block_taskloop",
			Type:   CampaignTypeCustom,
			Title:  "Block Taskloop",
			Goal:   "persist block reason from task loop",
			Status: StatusActive,
			Phases: []Phase{
				{
					ID:         "phase_0",
					CampaignID: "campaign_block_taskloop",
					Name:       "Only Phase",
					Order:      0,
					Status:     PhaseInProgress,
					Tasks: []Task{
						{
							ID:          "task_stuck",
							PhaseID:     "phase_0",
							Description: "always fails",
							Status:      TaskFailed,
							Type:        TaskTypeFileModify,
							Priority:    PriorityNormal,
							Order:       0,
							Attempts: []TaskAttempt{
								{Number: 1, Outcome: "/failure", Error: "boom1"},
								{Number: 2, Outcome: "/failure", Error: "boom2"},
								{Number: 3, Outcome: "/failure", Error: "boom3"},
							},
						},
					},
				},
			},
			TotalPhases: 1,
			TotalTasks:  1,
		},
		workspace: tmp,
		nerdDir:   tmp + "/.nerd",
		policy:    testPolicy(nil),
	}

	err := o.runPhase(context.Background(), &o.campaign.Phases[0])
	if err == nil {
		t.Fatalf("expected phase-blocked error, got nil")
	}
	if !strings.Contains(err.Error(), "phase blocked:") {
		t.Fatalf("returned error must keep %q text, got %v", "phase blocked:", err)
	}
	if o.campaign.Status != StatusFailed {
		t.Fatalf("campaign status = %s, want %s", o.campaign.Status, StatusFailed)
	}
	if o.campaign.BlockReason == "" {
		t.Fatalf("BlockReason empty after task-loop block; want non-empty (e.g. /all_tasks_blocked)")
	}
	if o.campaign.BlockReason != "/all_tasks_blocked" {
		t.Fatalf("BlockReason = %q, want %q", o.campaign.BlockReason, "/all_tasks_blocked")
	}
}

// TestAttemptCap_TriggersReplanBeforeBlock proves the failure-driven replanner
// runs exactly once when a task reaches the attempt cap, before any block can
// fire, and never again for the same task: the kernel derives
// task_next_move(/replan) once, then /fail once task_replanned_at_cap holds.
func TestAttemptCap_TriggersReplanBeforeBlock(t *testing.T) {
	orch, kernel := newPolicyFailureOrchestrator(t, func(c *config.CampaignConfig) { c.MaxTaskAttempts = 1 }, Task{
		ID: "/task_mutate_1", Description: "Modify internal/foo/bar.go to add doc comment", Type: TaskTypeFileModify,
	})
	var replanCalls atomic.Int32
	orch.replanner = NewReplanner(kernel, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			replanCalls.Add(1)
			return `{"success": true, "change_summary": "attempt-cap replan ok", "retry_tasks": [], "skip_tasks": [], "add_tasks": [{"phase_id": "/phase_failure_policy", "description": "Add follow-up fix for attempt-cap failure", "type": "/file_modify", "priority": "/high", "before_task": ""}], "modify_dependencies": []}`, nil
		},
	}, "")

	live := failTask(orch, "/task_mutate_1", errors.New("attempt-cap failure"))
	if got := replanCalls.Load(); got != 1 {
		t.Fatalf("task hitting the cap must invoke Replanner exactly once, got %d", got)
	}
	if !live.ReplannedAtCap {
		t.Fatalf("task %s must record ReplannedAtCap after the cap replan", live.ID)
	}
	if live.Status != TaskFailed {
		t.Fatalf("task status = %s, want %s", live.Status, TaskFailed)
	}

	// A second failure of the same task must not trigger another replan.
	failTask(orch, "/task_mutate_1", errors.New("same task fails again"))
	if got := replanCalls.Load(); got != 1 {
		t.Fatalf("second failure of the same task must not replan again, got %d calls", got)
	}
}
