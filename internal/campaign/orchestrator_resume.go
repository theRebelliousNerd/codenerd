package campaign

import (
	"codenerd/internal/logging"
	"fmt"
)

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
// Locking follows SetCampaign: the whole transition holds o.mu. The resumed
// state is synced into the kernel before returning: the kernel derives what
// blocks a campaign from its facts, and Run does not reload them. Until
// 2026-09-26 it said the next Run would, so a re-armed phase stayed
// /unverified in the kernel, campaign_blocked(/phase_unverified) derived on
// the first loop, and a campaign blocked on an unverified phase could never be
// resumed (campaign 7b853890).
func (o *Orchestrator) PrepareResume() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.campaign == nil {
		return fmt.Errorf("no campaign loaded")
	}
	if o.campaign.Status != StatusFailed {
		return nil
	}

	maxAttempts := o.policy.MaxTaskAttempts
	if maxAttempts < 1 {
		return fmt.Errorf("no campaign policy: campaign.max_task_attempts is %d", maxAttempts)
	}

	previous, err := cloneCampaign(o.campaign)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	resetResumeTasks(o.campaign.Phases, maxAttempts)
	rearmed := rearmUnverifiedPhases(o.campaign.Phases)

	target := findResumeTargetPhase(o.campaign.Phases)
	if target != nil && countResumableTasks(target) == 0 {
		return fmt.Errorf("resume impossible: every task in phase %s has reached the %d-attempt cap (campaign.max_task_attempts); re-plan the campaign", target.ID, maxAttempts)
	}

	o.campaign.Status = StatusActive
	o.campaign.BlockReason = ""
	o.campaign.ResumeCount++
	o.lastError = nil

	if o.kernel == nil {
		return nil
	}
	if err := syncCampaignFacts(o.kernel, previous, o.campaign, ""); err != nil {
		return fmt.Errorf("resume: the kernel was not given the resumed campaign: %w", err)
	}
	for _, id := range rearmed {
		// The fresh budget is the kernel's too: phase_ckpt_failures counts
		// the rows, and an old run left there would close the phase early.
		o.syncCheckpointFailures(id, 0)
	}
	return nil
}

// rearmUnverifiedPhases puts each phase whose checkpoint never passed back in
// progress with a fresh attempt budget. A resume is the operator's signal that
// the workspace may have changed, and the phase still owes its verification:
// it is never marked completed here -- only a passing checkpoint does that.
// It returns the IDs of the phases it re-armed.
func rearmUnverifiedPhases(phases []Phase) []string {
	var rearmed []string
	for i := range phases {
		if phases[i].Status == PhaseUnverified {
			phases[i].Status = PhaseInProgress
			phases[i].CheckpointFailures = 0
			rearmed = append(rearmed, phases[i].ID)
		}
	}
	return rearmed
}

// resetResumeTasks returns every retryable task to pending unless it has

// reached the attempt cap. Attempt history is kept untouched either way;
// at-cap tasks are marked failed with a warning naming the task.
func resetResumeTasks(phases []Phase, maxAttempts int) {
	for pi := range phases {
		for ti := range phases[pi].Tasks {
			resetResumeTask(&phases[pi].Tasks[ti], maxAttempts)
		}
	}
}

// resetResumeTask resets one task to pending when it is retryable and below
// the attempt cap. Tasks that already completed, were skipped, or never
// started are left alone.
func resetResumeTask(task *Task, maxAttempts int) {
	if task.Status != TaskFailed && task.Status != TaskInProgress && task.Status != TaskBlocked {
		return
	}
	if failedAttempts(task) >= maxAttempts {
		task.Status = TaskFailed
		logging.Get(logging.CategoryCampaign).Warn(
			"PrepareResume: task %s has reached the %d-attempt cap; leaving failed",
			task.ID, maxAttempts)
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

// failedAttempts counts a task's attempts that ended /failure -- the count
// campaign.max_task_attempts caps.
func failedAttempts(task *Task) int {
	n := 0
	for _, a := range task.Attempts {
		if a.Outcome == "/failure" {
			n++
		}
	}
	return n
}
