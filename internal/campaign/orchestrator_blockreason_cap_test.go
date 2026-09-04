package campaign

import (
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
		kernel:           kernel,
		campaign:         &Campaign{
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
		workspace:        tmp,
		nerdDir:          tmp + "/.nerd",
		maxParallelTasks: 3,
		config:           OrchestratorConfig{MaxRetries: 3},
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
// fire, and never again for the same task.
func TestAttemptCap_TriggersReplanBeforeBlock(t *testing.T) {
	kernel := &MockKernel{}
	// No replan_needed fact is seeded on purpose: the attempt-cap replan must
	// fire even when the policy has not derived one, otherwise a poison task
	// (e.g. pathless duplicate) blocks the phase with no replan at the cap.

	var replanCalls atomic.Int32
	orch, _, _ := newFailureTestOrchestrator(t, 0)
	// NewOrchestrator treats zero as the default (3); override after
	// construction to exercise the explicit fail-fast contract.
	orch.config.MaxRetries = 0
	orch.kernel = kernel
	orch.replanner = NewReplanner(kernel, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			replanCalls.Add(1)
			return `{"success": true, "change_summary": "attempt-cap replan ok", "retry_tasks": [], "skip_tasks": [], "add_tasks": [{"phase_id": "phase_failure_lane", "description": "Add follow-up fix for attempt-cap failure", "type": "/file_modify", "priority": "/high", "before_task": ""}], "modify_dependencies": []}`, nil
		},
	}, "")
	orch.campaign.ID = "campaign_attempt_cap"
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]
	task.Type = TaskTypeFileModify
	task.Description = "Modify internal/foo/bar.go to add doc comment"

	orch.handleTaskFailure(context.Background(), phase, task, errors.New("attempt-cap failure"))

	if got := replanCalls.Load(); got != 1 {
		t.Fatalf("task hitting the cap must invoke Replanner exactly once, got %d", got)
	}
	live := &orch.campaign.Phases[0].Tasks[0]
	// The failed task itself is terminally failed; the replacement (if any) is
	// appended. Find the original by ID.
	found := false
	for _, tk := range orch.campaign.Phases[0].Tasks {
		if tk.ID == task.ID {
			if !tk.ReplannedAtCap {
				t.Fatalf("original task %s must record ReplannedAtCap after cap replan", tk.ID)
			}
			if tk.Status != TaskFailed {
				t.Fatalf("original task status = %s, want %s", tk.Status, TaskFailed)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("original task %s missing after cap replan, tasks=%v", task.ID, orch.campaign.Phases[0].Tasks)
	}
	_ = live

	// A second failure of the same task must not trigger another replan.
	orch.handleTaskFailure(context.Background(), phase, task, errors.New("same task fails again"))
	if got := replanCalls.Load(); got != 1 {
		t.Fatalf("second failure of the same task must not replan again, got %d calls", got)
	}
}
