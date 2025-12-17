package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"strings"
	"time"
)

// handleTaskFailure handles task execution failure.
func (o *Orchestrator) handleTaskFailure(ctx context.Context, phase *Phase, task *Task, err error) {
	logging.Get(logging.CategoryCampaign).Warn("Handling task failure: %s - %v", task.ID, err)

	errorType := classifyTaskError(err)

	o.mu.Lock()
	markedFailed := false
	newStatus := TaskPending
	nextRetryAt := time.Time{}

	// Record attempt and update retry/backoff state
	for i := range o.campaign.Phases {
		for j := range o.campaign.Phases[i].Tasks {
			if o.campaign.Phases[i].Tasks[j].ID != task.ID {
				continue
			}

			attemptNum := len(o.campaign.Phases[i].Tasks[j].Attempts) + 1
			logging.CampaignDebug("Task %s attempt %d failed", task.ID, attemptNum)

			o.campaign.Phases[i].Tasks[j].Attempts = append(
				o.campaign.Phases[i].Tasks[j].Attempts,
				TaskAttempt{
					Number:    attemptNum,
					Outcome:   "/failure",
					Timestamp: time.Now(),
					Error:     err.Error(),
				},
			)
			o.campaign.Phases[i].Tasks[j].LastError = err.Error()

			maxRetries := o.config.MaxRetries
			if maxRetries <= 0 {
				maxRetries = 3
			}

			if attemptNum >= maxRetries {
				logging.Get(logging.CategoryCampaign).Error("Task %s exceeded max retries (%d), marking as failed", task.ID, maxRetries)
				o.campaign.Phases[i].Tasks[j].Status = TaskFailed
				o.campaign.Phases[i].Tasks[j].NextRetryAt = time.Time{}
				markedFailed = true
				newStatus = TaskFailed

				// Record in kernel
				_ = o.kernel.Assert(core.Fact{
					Predicate: "task_error",
					Args:      []interface{}{task.ID, fmt.Sprintf("max_retries_%d", maxRetries), err.Error()},
				})
			} else {
				// Backoff before retrying to avoid tight failure loops.
				backoff := o.computeRetryBackoff(errorType, attemptNum)
				nextRetryAt = time.Now().Add(backoff)
				o.campaign.Phases[i].Tasks[j].Status = TaskPending
				o.campaign.Phases[i].Tasks[j].NextRetryAt = nextRetryAt
				newStatus = TaskPending
			}
			break
		}
	}
	o.mu.Unlock()

	// Update kernel-visible task status for retries.
	o.updateTaskStatus(task, newStatus)

	// Record error taxonomy + retry window for policy/debugging.
	_ = o.kernel.Assert(core.Fact{
		Predicate: "task_error",
		Args:      []interface{}{task.ID, errorType, err.Error()},
	})
	if !nextRetryAt.IsZero() {
		_ = o.kernel.RetractFact(core.Fact{
			Predicate: "task_retry_at",
			Args:      []interface{}{task.ID},
		})
		_ = o.kernel.Assert(core.Fact{
			Predicate: "task_retry_at",
			Args:      []interface{}{task.ID, nextRetryAt.Unix()},
		})
	}

	o.emitEvent("task_failed", phase.ID, task.ID, err.Error(), nil)

	// Update computed failed-task count for Mangle replanning threshold rules.
	o.updateFailedTaskCount()

	// Optionally run checkpoint immediately after a task is fully failed.
	if markedFailed && o.config.CheckpointOnFail {
		if _, _, chkErr := o.runPhaseCheckpoint(ctx, phase); chkErr != nil {
			logging.Get(logging.CategoryCampaign).Warn("Checkpoint-on-fail error: %v", chkErr)
			o.emitEvent("checkpoint_failed", phase.ID, "", chkErr.Error(), nil)
		}
	}

	// Check if replan is needed
	facts, _ := o.kernel.Query("replan_needed")
	if len(facts) > 0 {
		logging.Campaign("Replan triggered due to task failures")
		o.emitEvent("replan_triggered", "", "", "Too many failures, triggering replan", nil)
		if repErr := o.replanner.Replan(ctx, o.campaign, task.ID); repErr != nil {
			logging.Get(logging.CategoryCampaign).Error("Replan failed: %v", repErr)
			o.emitEvent("replan_failed", "", "", repErr.Error(), nil)
		} else {
			o.mu.Lock()
			logging.Campaign("Campaign replanned, new revision: %d", o.campaign.RevisionNumber)
			_ = o.saveCampaign()
			o.mu.Unlock()
		}
	}

	// Persist failure updates for durability.
	o.mu.Lock()
	_ = o.saveCampaign()
	o.mu.Unlock()
}

// classifyTaskError uses heuristics to bucket errors into retry taxonomies.
func classifyTaskError(err error) string {
	if err == nil {
		return "/logic"
	}
	msg := strings.ToLower(err.Error())
	transientHints := []string{
		"timeout",
		"context deadline",
		"rate limit",
		"too many requests",
		"temporar",
		"connection",
		"unavailable",
		"network",
		"i/o",
	}
	for _, h := range transientHints {
		if strings.Contains(msg, h) {
			return "/transient"
		}
	}
	return "/logic"
}

// computeRetryBackoff returns exponential backoff based on attempt number.
func (o *Orchestrator) computeRetryBackoff(errorType string, attemptNum int) time.Duration {
	base := o.config.RetryBackoffBase
	if base <= 0 {
		base = 5 * time.Second
	}
	maxBackoff := o.config.RetryBackoffMax
	if maxBackoff <= 0 {
		maxBackoff = 5 * time.Minute
	}

	shift := attemptNum - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 10 {
		shift = 10
	}
	backoff := base * time.Duration(1<<shift)

	// Logic errors often benefit from faster replans; cap their backoff lower.
	if errorType == "/logic" && backoff > 30*time.Second {
		backoff = 30 * time.Second
	}
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}
