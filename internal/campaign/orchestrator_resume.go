package campaign

import (
	"codenerd/internal/logging"
	"fmt"
)

// defaultPrepareResumeMaxRetries mirrors applyOrchestratorDefaults
// (orchestrator_init.go): a zero MaxRetries means the default, not "no
// retries". PrepareResume applies it itself because resume-path
// orchestrators may be built as literals without going through the
// constructor.
const defaultPrepareResumeMaxRetries = 3

// PrepareResume re-arms a failed/blocked campaign so the next Run can make
// progress instead of tripping the terminal-failure guard immediately.
//
// A campaign that ends in `phase blocked: /all_tasks_blocked` is persisted
// with StatusFailed. Without a reset, the blocked task statuses derive
// `campaign_blocked` again on the first loop and the terminal-failure guard
// ends the run at once. PrepareResume resets every retryable task back to
// pending — unless it has reached the attempt cap — then flips the campaign
// back to active.
//
// Behaviour:
//   - No-op (nil) unless the campaign status is StatusFailed. Paused and
//     active campaigns need nothing and are left untouched.
//   - If the current phase — the first phase holding any non-completed task
//     — has zero pending or completed tasks after the reset, resume is
//     impossible and an error is returned naming the phase and the cap.
//   - Otherwise the campaign returns to StatusActive with BlockReason
//     cleared, ResumeCount incremented, and lastError cleared.
//
// Locking follows SetCampaign: the whole transition holds o.mu. Kernel facts
// are deliberately not touched here; the next Run reloads campaign facts via
// SetCampaign/LoadCampaign before the execution loop queries them.
func (o *Orchestrator) PrepareResume() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.campaign == nil {
		return fmt.Errorf("no campaign loaded")
	}
	if o.campaign.Status != StatusFailed {
		return nil
	}

	maxRetries := o.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultPrepareResumeMaxRetries
	}

	resetResumeTasks(o.campaign.Phases, maxRetries)

	target := findResumeTargetPhase(o.campaign.Phases)
	if target != nil && countResumableTasks(target) == 0 {
		return fmt.Errorf("resume impossible: every task in phase %s has reached the %d-attempt cap; re-plan the campaign", target.ID, maxRetries)
	}

	o.campaign.Status = StatusActive
	o.campaign.BlockReason = ""
	o.campaign.ResumeCount++
	o.lastError = nil
	return nil
}

// resetResumeTasks returns every retryable task to pending unless it has
// reached the attempt cap. Attempt history is kept untouched either way;
// at-cap tasks are marked failed with a warning naming the task.
func resetResumeTasks(phases []Phase, maxRetries int) {
	for pi := range phases {
		for ti := range phases[pi].Tasks {
			resetResumeTask(&phases[pi].Tasks[ti], maxRetries)
		}
	}
}

// resetResumeTask resets one task to pending when it is retryable and below
// the attempt cap. Tasks that already completed, were skipped, or never
// started are left alone.
func resetResumeTask(task *Task, maxRetries int) {
	if task.Status != TaskFailed && task.Status != TaskInProgress && task.Status != TaskBlocked {
		return
	}
	if len(task.Attempts) >= maxRetries {
		task.Status = TaskFailed
		logging.Get(logging.CategoryCampaign).Warn(
			"PrepareResume: task %s has reached the %d-attempt cap; leaving failed",
			task.ID, maxRetries)
		return
	}
	task.Status = TaskPending
}

// findResumeTargetPhase returns the first phase holding any non-completed
// task — the phase the next Run must make progress in. Nil when every task
// in the campaign already completed or was skipped.
func findResumeTargetPhase(phases []Phase) *Phase {
	for pi := range phases {
		if phaseHasRunnableWork(&phases[pi]) {
			return &phases[pi]
		}
	}
	return nil
}

// phaseHasRunnableWork reports whether the phase holds any task that is not
// completed or skipped.
func phaseHasRunnableWork(phase *Phase) bool {
	for ti := range phase.Tasks {
		s := phase.Tasks[ti].Status
		if s != TaskCompleted && s != TaskSkipped {
			return true
		}
	}
	return false
}

// countResumableTasks counts the phase tasks a resumed run can proceed with:
// pending or already completed.
func countResumableTasks(phase *Phase) int {
	resumable := 0
	for ti := range phase.Tasks {
		if phase.Tasks[ti].Status == TaskPending || phase.Tasks[ti].Status == TaskCompleted {
			resumable++
		}
	}
	return resumable
}
