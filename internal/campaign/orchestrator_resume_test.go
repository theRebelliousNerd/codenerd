package campaign

import (
	"strings"
	"testing"
)

func resumeFailedFixture() *Orchestrator {
	return &Orchestrator{
		campaign: &Campaign{
			ID:          "/campaign_resume",
			Status:      StatusFailed,
			BlockReason: "/all_tasks_blocked",
			Phases: []Phase{
				{
					ID: "phase_0", Order: 0, Status: PhaseFailed,
					Tasks: []Task{
						{
							ID: "task_fail", PhaseID: "phase_0", Type: TaskTypeFileModify,
							Status: TaskFailed, Order: 0, Description: "failed once",
							Attempts: []TaskAttempt{{Number: 1, Outcome: "/failure", Error: "boom"}},
						},
						{ID: "task_pending", PhaseID: "phase_0", Type: TaskTypeFileCreate, Status: TaskPending, Order: 1, Description: "waiting"},
						{ID: "task_active", PhaseID: "phase_0", Type: TaskTypeTestRun, Status: TaskInProgress, Order: 2, Description: "was running"},
					},
				},
			},
		},
	}
}

func TestPrepareResume_ResetsFailedTasks(t *testing.T) {
	o := resumeFailedFixture()
	// Zero-value config exercises the MaxRetries default (3).
	if err := o.PrepareResume(); err != nil {
		t.Fatalf("PrepareResume returned error: %v", err)
	}
	for _, task := range o.campaign.Phases[0].Tasks {
		if task.Status != TaskPending {
			t.Fatalf("task %s status = %s, want %s", task.ID, task.Status, TaskPending)
		}
	}
	if o.campaign.Status != StatusActive {
		t.Fatalf("campaign status = %s, want %s", o.campaign.Status, StatusActive)
	}
	if o.campaign.BlockReason != "" {
		t.Fatalf("BlockReason = %q, want empty", o.campaign.BlockReason)
	}
	if o.campaign.ResumeCount != 1 {
		t.Fatalf("ResumeCount = %d, want 1", o.campaign.ResumeCount)
	}
	if got := len(o.campaign.Phases[0].Tasks[0].Attempts); got != 1 {
		t.Fatalf("attempts history changed: len = %d, want 1", got)
	}
	if o.lastError != nil {
		t.Fatalf("lastError = %v, want nil", o.lastError)
	}
}

func TestPrepareResume_AtCapReturnsError(t *testing.T) {
	o := &Orchestrator{
		config: OrchestratorConfig{MaxRetries: 3},
		campaign: &Campaign{
			ID:          "/campaign_capped",
			Status:      StatusFailed,
			BlockReason: "/all_tasks_blocked",
			Phases: []Phase{
				{
					ID: "phase_0", Order: 0, Status: PhaseFailed,
					Tasks: []Task{
						{
							ID: "task_stuck", PhaseID: "phase_0", Type: TaskTypeFileModify,
							Status: TaskFailed, Order: 0, Description: "always fails",
							Attempts: []TaskAttempt{
								{Number: 1, Outcome: "/failure", Error: "boom1"},
								{Number: 2, Outcome: "/failure", Error: "boom2"},
								{Number: 3, Outcome: "/failure", Error: "boom3"},
							},
						},
					},
				},
			},
		},
	}
	err := o.PrepareResume()
	if err == nil {
		t.Fatal("expected attempt-cap error, got nil")
	}
	if !strings.Contains(err.Error(), "attempt cap") {
		t.Fatalf("error must mention %q, got %v", "attempt cap", err)
	}
	if o.campaign.Status != StatusFailed {
		t.Fatalf("campaign status = %s, want %s (unchanged)", o.campaign.Status, StatusFailed)
	}
	if got := len(o.campaign.Phases[0].Tasks[0].Attempts); got != 3 {
		t.Fatalf("attempts history changed: len = %d, want 3", got)
	}
}

func TestPrepareResume_PausedIsNoop(t *testing.T) {
	o := &Orchestrator{
		config: OrchestratorConfig{MaxRetries: 3},
		campaign: &Campaign{
			ID:          "/campaign_paused",
			Status:      StatusPaused,
			BlockReason: "/previous_block",
			Phases: []Phase{
				{
					ID: "phase_0", Order: 0, Status: PhaseInProgress,
					Tasks: []Task{
						{
							ID: "task_fail", PhaseID: "phase_0", Type: TaskTypeFileModify,
							Status: TaskFailed, Order: 0, Description: "failed once",
							Attempts: []TaskAttempt{{Number: 1, Outcome: "/failure", Error: "boom"}},
						},
					},
				},
			},
		},
	}
	if err := o.PrepareResume(); err != nil {
		t.Fatalf("PrepareResume on paused campaign returned error: %v", err)
	}
	if o.campaign.Status != StatusPaused {
		t.Fatalf("campaign status = %s, want %s (unchanged)", o.campaign.Status, StatusPaused)
	}
	if o.campaign.Phases[0].Tasks[0].Status != TaskFailed {
		t.Fatalf("task status = %s, want %s (unchanged)", o.campaign.Phases[0].Tasks[0].Status, TaskFailed)
	}
	if o.campaign.BlockReason != "/previous_block" {
		t.Fatalf("BlockReason = %q, want unchanged", o.campaign.BlockReason)
	}
	if o.campaign.ResumeCount != 0 {
		t.Fatalf("ResumeCount = %d, want 0 (unchanged)", o.campaign.ResumeCount)
	}
}
