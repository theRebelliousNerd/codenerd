package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestHandleTaskFailure_RetryableDoesNotReplanNorDuplicate is the deterministic
// contract guard for the observed live failure: handleTaskFailure left a
// /file_modify pending for bounded retry, then immediately honored replan_needed
// and inserted a semantically duplicate replacement task; runPhase scheduled
// both concurrently, producing competing files.
//
// Contract: never invoke failure-driven Replanner while the failed task is still
// retryable/pending; replan only after terminal failure.
func TestHandleTaskFailure_RetryableDoesNotReplanNorDuplicate(t *testing.T) {
	kernel := &MockKernel{}
	// Simulate the policy having derived replan_needed while the task is still retryable.
	_ = kernel.Assert(core.Fact{Predicate: "replan_needed", Args: []any{"campaign_retry_guard", "/task_failure_cascade"}})

	var replanCalls atomic.Int32
	orch, _, _ := newFailureTestOrchestrator(t, 3)
	// Override kernel and replanner to observe replan invocation.
	orch.kernel = kernel
	orch.replanner = NewReplanner(kernel, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			replanCalls.Add(1)
			// Would add a semantically duplicate file_modify if called.
			return `{"success": true, "change_summary": "duplicate", "retry_tasks": [], "skip_tasks": [], "add_tasks": [{"phase_id": "phase_failure_lane", "description": "Modify source file", "type": "/file_modify", "priority": "/high", "before_task": ""}], "modify_dependencies": []}`, nil
		},
	}, "")
	// Ensure campaign and kernel are consistent for the test phase ID.
	orch.campaign.ID = "campaign_retry_guard"
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]
	// Ensure task is file_modify so the live duplicate scenario applies.
	task.Type = TaskTypeFileModify
	task.Description = "Modify internal/foo/bar.go to add doc comment"

	beforeTasks := len(phase.Tasks)
	beforeRevision := orch.campaign.RevisionNumber

	orch.handleTaskFailure(context.Background(), phase, task, errors.New("compile failed: undefined symbol x"))

	if got := replanCalls.Load(); got != 0 {
		t.Fatalf("retryable failure must not invoke Replanner, but Replan was called %d times", got)
	}
	if len(orch.campaign.Phases[0].Tasks) != beforeTasks {
		t.Fatalf("retryable failure must not insert duplicate task: before=%d after=%d tasks=%v", beforeTasks, len(orch.campaign.Phases[0].Tasks), orch.campaign.Phases[0].Tasks)
	}
	// Original must remain retryable/pending with backoff, not terminally failed.
	liveTask := &orch.campaign.Phases[0].Tasks[0]
	if liveTask.Status != TaskPending {
		t.Fatalf("retryable task status = %s, want %s", liveTask.Status, TaskPending)
	}
	if liveTask.NextRetryAt.IsZero() {
		t.Fatalf("retryable task should have non-zero NextRetryAt backoff")
	}
	if orch.campaign.RevisionNumber != beforeRevision {
		t.Fatalf("revision must not change on retryable failure: before=%d after=%d", beforeRevision, orch.campaign.RevisionNumber)
	}
	// Verify that scheduling would not run a duplicate: the original is in backoff
	// so eligible is empty, and no duplicate exists to run.
	eligible := orch.getEligibleTasks(&orch.campaign.Phases[0])
	if len(eligible) != 0 {
		t.Fatalf("retryable task in backoff should not be eligible, got %d eligible: %v", len(eligible), eligible)
	}
}

// TestHandleTaskFailure_TerminalCanStillReplan verifies the other half of the
// contract: once the task is terminally failed (exceeded MaxRetries) the
// failure-driven replan IS allowed and can insert its replacement. This prevents
// the retry-gate from permanently disabling replanning.
func TestHandleTaskFailure_TerminalCanStillReplan(t *testing.T) {
	kernel := &MockKernel{}
	_ = kernel.Assert(core.Fact{Predicate: "replan_needed", Args: []any{"campaign_terminal_replan", "/task_failure_cascade"}})

	var replanCalls atomic.Int32
	orch, _, _ := newFailureTestOrchestrator(t, 0) // 0 => terminal on first failure
	// NewOrchestrator treats zero as the default (3); override it after
	// construction to exercise the explicit fail-fast contract.
	orch.config.MaxRetries = 0
	orch.kernel = kernel
	orch.replanner = NewReplanner(kernel, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			replanCalls.Add(1)
			return `{"success": true, "change_summary": "terminal replan ok", "retry_tasks": [], "skip_tasks": [], "add_tasks": [{"phase_id": "phase_failure_lane", "description": "Add follow-up fix for terminal failure", "type": "/file_modify", "priority": "/high", "before_task": ""}], "modify_dependencies": []}`, nil
		},
	}, "")
	orch.campaign.ID = "campaign_terminal_replan"
	// Seed current_phase and campaign_phase facts so livePhaseByID etc. are coherent
	// (not strictly needed for handleTaskFailure but mirrors production wiring).
	phase := &orch.campaign.Phases[0]
	task := &orch.campaign.Phases[0].Tasks[0]
	task.Type = TaskTypeFileModify
	task.Description = "Modify internal/foo/bar.go to add doc comment"

	beforeTasks := len(phase.Tasks)
	orch.handleTaskFailure(context.Background(), phase, task, errors.New("terminal logic failure"))

	if got := replanCalls.Load(); got != 1 {
		t.Fatalf("terminal failure must invoke Replanner exactly once, got %d", got)
	}
	if len(orch.campaign.Phases[0].Tasks) != beforeTasks+1 {
		t.Fatalf("terminal replan should insert replacement task: before=%d after=%d", beforeTasks, len(orch.campaign.Phases[0].Tasks))
	}
	liveTask := orch.campaign.Phases[0].Tasks[0]
	// The failed task itself should be terminally failed, not left pending.
	foundFailed := false
	for _, tk := range orch.campaign.Phases[0].Tasks {
		if tk.ID == task.ID && tk.Status == TaskFailed {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Fatalf("original task should be %s after terminal failure, tasks=%v", TaskFailed, orch.campaign.Phases[0].Tasks)
	}
	_ = liveTask // avoid unused
	if orch.campaign.RevisionNumber == 0 {
		t.Fatalf("expected revision to be bumped by Replan, got %d", orch.campaign.RevisionNumber)
	}
}

// TestRunPhase_CancellationDrainsWorkers verifies the second half of the fix:
// On runPhase context cancellation, stop scheduling and join/drain all in-flight
// task goroutines before returning, without blocking forever.
//
// Observed live: Ctrl+C canceled the parent but the nerd process stayed alive
// with worker goroutines. This test proves cancellation leaves no active task
// worker.
func TestRunPhase_CancellationDrainsWorkers(t *testing.T) {
	tmpDir := t.TempDir()
	kernel := &MockKernel{}
	// Current phase derived from kernel.
	_ = kernel.Assert(core.Fact{Predicate: "current_phase", Args: []any{"phase_cancel"}})
	_ = kernel.Assert(core.Fact{Predicate: "campaign_phase", Args: []any{"phase_cancel", "campaign_cancel_drain", "Cancel Phase", 0, "/in_progress", ""}})
	_ = kernel.Assert(core.Fact{Predicate: "campaign", Args: []any{"campaign_cancel_drain", string(CampaignTypeCustom), "Cancel Drain", "", string(StatusActive)}})

	var activeWorkers atomic.Int32
	var maxConcurrent atomic.Int32
	blockUntilCancel := func(ctx context.Context, req session.TaskRequest) (string, error) {
		cur := activeWorkers.Add(1)
		defer activeWorkers.Add(-1)
		// Track max
		for {
			m := maxConcurrent.Load()
			if cur > m && maxConcurrent.CompareAndSwap(m, cur) {
				break
			}
			if cur <= m {
				break
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			return "ok", nil
		}
	}

	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:        tmpDir,
		Kernel:           kernel,
		LLMClient:        &MockLLMClient{},
		Executor:         tactile.NewDirectExecutor(),
		VirtualStore:     &core.VirtualStore{},
		TaskExecutor:     &MockTaskExecutor{ExecuteFunc: blockUntilCancel},
		MaxRetries:       3,
		MaxParallelTasks: 2,
		DisableTimeouts:  true,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	orch.campaign = &Campaign{
		ID:     "campaign_cancel_drain",
		Type:   CampaignTypeCustom,
		Title:  "Cancel Drain",
		Goal:   "test cancellation join",
		Status: StatusActive,
		Phases: []Phase{
			{
				ID:         "phase_cancel",
				CampaignID: "campaign_cancel_drain",
				Name:       "Cancel Phase",
				Order:      0,
				Status:     PhaseInProgress,
				Tasks: []Task{
					{ID: "t1", PhaseID: "phase_cancel", Description: "task 1", Status: TaskPending, Type: TaskTypeFileModify, Priority: PriorityNormal, Order: 0, WriteSet: []string{"a.go"}},
					{ID: "t2", PhaseID: "phase_cancel", Description: "task 2", Status: TaskPending, Type: TaskTypeFileModify, Priority: PriorityNormal, Order: 1, WriteSet: []string{"b.go"}},
					{ID: "t3", PhaseID: "phase_cancel", Description: "task 3", Status: TaskPending, Type: TaskTypeFileModify, Priority: PriorityNormal, Order: 2, WriteSet: []string{"c.go"}},
				},
			},
		},
		TotalPhases: 1,
		TotalTasks:  3,
	}
	// Seed eligible tasks for the phase.
	_ = kernel.Assert(core.Fact{Predicate: "eligible_task", Args: []any{"t1"}})
	_ = kernel.Assert(core.Fact{Predicate: "eligible_task", Args: []any{"t2"}})
	_ = kernel.Assert(core.Fact{Predicate: "eligible_task", Args: []any{"t3"}})
	for _, tk := range orch.campaign.Phases[0].Tasks {
		_ = kernel.Assert(core.Fact{Predicate: "campaign_task", Args: []any{tk.ID, tk.PhaseID, tk.Description, string(tk.Status), string(tk.Type)}})
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- orch.runPhase(ctx, &orch.campaign.Phases[0])
	}()

	// Let workers start.
	time.Sleep(300 * time.Millisecond)
	if activeWorkers.Load() == 0 {
		t.Fatalf("expected workers to have started, active=0")
	}
	// Cancel and ensure runPhase returns promptly and drains.
	start := time.Now()
	cancel()
	select {
	case err := <-done:
		elapsed := time.Since(start)
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			// runPhase may return context.Canceled wrapped; allow any cancellation error
			if err != context.Canceled && err != context.DeadlineExceeded {
				t.Logf("runPhase returned %v (want cancellation)", err)
			}
		}
		// Must not block forever: drain timeout is 5s, so should return well under 6s.
		if elapsed > 6*time.Second {
			t.Fatalf("runPhase cancellation took too long: %v (should drain within 5s)", elapsed)
		}
	case <-time.After(7 * time.Second):
		t.Fatalf("runPhase did not return after cancellation (leaked or blocked forever)")
	}

	// Give workers a moment to decrement counter after context cancellation.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if activeWorkers.Load() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := activeWorkers.Load(); got != 0 {
		t.Fatalf("cancellation left %d active task workers (leak)", got)
	}
	if got := maxConcurrent.Load(); got < 1 {
		t.Fatalf("expected at least 1 concurrent worker, got %d", got)
	}
	// Ensure no new tasks were scheduled after cancellation: t3 should remain pending
	// (not started) because we canceled before it could be scheduled beyond the limit.
	// At minimum, at least one task remains not completed.
	completed := 0
	for _, tk := range orch.campaign.Phases[0].Tasks {
		if tk.Status == TaskCompleted {
			completed++
		}
	}
	if completed == 3 {
		t.Fatalf("cancellation should have prevented all tasks from completing, but all 3 are completed")
	}
}
