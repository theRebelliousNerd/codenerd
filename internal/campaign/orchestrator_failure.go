package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	logicFailureEscalationWindowAttempts = 3
	logicFailureEscalationMinFailures    = 2
	logicFailureEscalationMaxLoopAge     = 20 * time.Minute

	reproDiagnosticDescriptionPrefix = "[diagnostic-repro]"
)

// handleTaskFailure handles task execution failure.
func (o *Orchestrator) handleTaskFailure(ctx context.Context, phase *Phase, task *Task, err error) {
	logging.Get(logging.CategoryCampaign).Warn("Handling task failure: %s - %v", task.ID, err)

	errorType := classifyTaskError(err)
	phaseID := ""
	if phase != nil {
		phaseID = phase.ID
	}

	o.mu.Lock()
	markedFailed := false
	newStatus := TaskPending
	nextRetryAt := time.Time{}
	logicEscalated := false
	logicEscalationReason := ""
	reproTaskID := ""
	reproTaskInserted := false

	// Record attempt and update retry/backoff state
taskSearch:
	for i := range o.campaign.Phases {
		for j := range o.campaign.Phases[i].Tasks {
			if o.campaign.Phases[i].Tasks[j].ID != task.ID {
				continue
			}

			attemptNum := len(o.campaign.Phases[i].Tasks[j].Attempts) + 1
			logging.CampaignDebug("Task %s attempt %d failed", task.ID, attemptNum)

			attemptedAt := time.Now()
			o.campaign.Phases[i].Tasks[j].Attempts = append(
				o.campaign.Phases[i].Tasks[j].Attempts,
				TaskAttempt{
					Number:    attemptNum,
					Outcome:   "/failure",
					Timestamp: attemptedAt,
					Error:     err.Error(),
				},
			)
			o.campaign.Phases[i].Tasks[j].LastError = err.Error()
			phaseID = o.campaign.Phases[i].ID

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
				nextRetryAt = attemptedAt.Add(backoff)
				o.campaign.Phases[i].Tasks[j].Status = TaskPending
				o.campaign.Phases[i].Tasks[j].NextRetryAt = nextRetryAt
				newStatus = TaskPending

				if errorType == "/logic" &&
					isMutatingTaskType(o.campaign.Phases[i].Tasks[j].Type) &&
					!isReproDiagnosticTask(&o.campaign.Phases[i].Tasks[j]) {
					shouldEscalate, reason := shouldEscalateLogicFailure(o.campaign.Phases[i].Tasks[j].Attempts, attemptedAt)
					if shouldEscalate {
						logicEscalated = true
						logicEscalationReason = reason
						reproTaskID, reproTaskInserted = o.insertReproDiagnosticTaskLocked(i, j, attemptNum, err, reason)
					}
				}
			}
			break taskSearch
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
	if logicEscalated {
		_ = o.kernel.Assert(core.Fact{
			Predicate: "task_error",
			Args:      []interface{}{task.ID, "/logic_failure_escalated", logicEscalationReason},
		})
		_ = o.kernel.Assert(core.Fact{
			Predicate: "task_error",
			Args:      []interface{}{task.ID, "/repro_test_first_required", reproTaskID},
		})
	}
	if !nextRetryAt.IsZero() {
		_ = o.kernel.RetractFact(core.Fact{
			Predicate: "task_retry_at",
			Args:      []interface{}{task.ID},
		})
		_ = o.kernel.Assert(core.Fact{
			Predicate: "task_retry_at",
			Args:      []interface{}{task.ID, nextRetryAt.Unix()},
		})
	} else {
		_ = o.kernel.RetractFact(core.Fact{
			Predicate: "task_retry_at",
			Args:      []interface{}{task.ID},
		})
	}

	o.emitEvent("task_failed", phaseID, task.ID, err.Error(), nil)
	if logicEscalated {
		o.emitEvent("logic_failure_escalated", phaseID, task.ID, "Deterministic logic escalation triggered", map[string]interface{}{
			"reason":                logicEscalationReason,
			"repro_task_id":         reproTaskID,
			"repro_task_inserted":   reproTaskInserted,
			"window_attempts":       logicFailureEscalationWindowAttempts,
			"window_min_failures":   logicFailureEscalationMinFailures,
			"window_max_loop_age_s": int(logicFailureEscalationMaxLoopAge.Seconds()),
		})
	}
	if reproTaskInserted {
		o.emitEvent("diagnostic_task_inserted", phaseID, reproTaskID, "Inserted repro-test-first diagnostic task", map[string]interface{}{
			"failed_task_id": task.ID,
			"reason":         logicEscalationReason,
		})
	}

	// Update computed failed-task count for Mangle replanning threshold rules.
	o.updateFailedTaskCount()

	// Optionally run checkpoint immediately after a task is fully failed.
	if markedFailed && o.config.CheckpointOnFail {
		if _, _, chkErr := o.runPhaseCheckpoint(ctx, phase); chkErr != nil {
			logging.Get(logging.CategoryCampaign).Warn("Checkpoint-on-fail error: %v", chkErr)
			o.emitEvent("checkpoint_failed", phaseID, "", chkErr.Error(), nil)
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

func shouldEscalateLogicFailure(attempts []TaskAttempt, now time.Time) (bool, string) {
	if len(attempts) == 0 {
		return false, ""
	}
	if now.IsZero() {
		now = time.Now()
	}

	start := len(attempts) - logicFailureEscalationWindowAttempts
	if start < 0 {
		start = 0
	}
	window := attempts[start:]

	logicFailures := 0
	oldestLoopFailure := time.Time{}
	for _, attempt := range attempts {
		if attempt.Outcome != "/failure" || attempt.Timestamp.IsZero() {
			continue
		}
		if oldestLoopFailure.IsZero() || attempt.Timestamp.Before(oldestLoopFailure) {
			oldestLoopFailure = attempt.Timestamp
		}
	}

	for _, attempt := range window {
		if attempt.Outcome != "/failure" {
			continue
		}
		if classifyTaskAttempt(attempt) != "/logic" {
			continue
		}
		logicFailures++
	}

	if logicFailures >= logicFailureEscalationMinFailures {
		return true, fmt.Sprintf("logic_failures_%d_of_last_%d", logicFailures, len(window))
	}

	if !oldestLoopFailure.IsZero() && now.Sub(oldestLoopFailure) >= logicFailureEscalationMaxLoopAge {
		return true, fmt.Sprintf("logic_loop_age_exceeded_%ds", int(now.Sub(oldestLoopFailure).Seconds()))
	}

	return false, ""
}

func classifyTaskAttempt(attempt TaskAttempt) string {
	if strings.TrimSpace(attempt.Error) == "" {
		return "/logic"
	}
	return classifyTaskError(errors.New(attempt.Error))
}

func (o *Orchestrator) insertReproDiagnosticTaskLocked(phaseIdx, taskIdx, attemptNum int, originalErr error, reason string) (string, bool) {
	if o == nil || o.campaign == nil {
		return "", false
	}
	if phaseIdx < 0 || phaseIdx >= len(o.campaign.Phases) {
		return "", false
	}
	phase := &o.campaign.Phases[phaseIdx]
	if taskIdx < 0 || taskIdx >= len(phase.Tasks) {
		return "", false
	}

	failedTaskID := phase.Tasks[taskIdx].ID
	if existing := findActiveReproTaskID(phase.Tasks, failedTaskID); existing != "" {
		if ensureTaskDependsOn(&phase.Tasks[taskIdx], existing) {
			if o.kernel != nil {
				_ = o.kernel.Assert(core.Fact{
					Predicate: "task_dependency",
					Args:      []interface{}{failedTaskID, existing},
				})
			}
		}
		return existing, false
	}

	errSummary := "logic failure"
	if originalErr != nil && strings.TrimSpace(originalErr.Error()) != "" {
		errSummary = strings.TrimSpace(originalErr.Error())
	}
	if len(errSummary) > 220 {
		errSummary = errSummary[:220] + "..."
	}

	reproTaskID := fmt.Sprintf("%s/repro_%03d", failedTaskID, attemptNum)
	reproTask := Task{
		ID:              reproTaskID,
		PhaseID:         phase.Tasks[taskIdx].PhaseID,
		Description:     fmt.Sprintf("%s Reproduce failing loop for %s (%s): run tests before next mutation. Last error: %s", reproDiagnosticDescriptionPrefix, failedTaskID, reason, errSummary),
		Status:          TaskPending,
		Type:            TaskTypeTestRun,
		Priority:        PriorityCritical,
		Order:           0,
		InferredFrom:    failedTaskID,
		InferenceConf:   1.0,
		InferenceReason: "/logic_failure_repro_guard",
	}

	phase.Tasks = append([]Task{reproTask}, phase.Tasks...)
	for idx := range phase.Tasks {
		phase.Tasks[idx].Order = idx
		phase.Tasks[idx].PhaseID = phase.ID
	}

	originalIdx := -1
	for idx := range phase.Tasks {
		if phase.Tasks[idx].ID == failedTaskID {
			originalIdx = idx
			break
		}
	}
	if originalIdx >= 0 {
		if ensureTaskDependsOn(&phase.Tasks[originalIdx], reproTaskID) {
			if o.kernel != nil {
				_ = o.kernel.Assert(core.Fact{
					Predicate: "task_dependency",
					Args:      []interface{}{failedTaskID, reproTaskID},
				})
			}
		}
	}

	o.campaign.TotalTasks++
	if o.kernel != nil {
		_ = o.kernel.LoadFacts(reproTask.ToFacts())
	}

	return reproTaskID, true
}

func findActiveReproTaskID(tasks []Task, failedTaskID string) string {
	for _, t := range tasks {
		if t.InferredFrom != failedTaskID {
			continue
		}
		if t.Type != TaskTypeTestRun || !isReproDiagnosticTask(&t) {
			continue
		}
		if t.Status == TaskPending || t.Status == TaskInProgress {
			return t.ID
		}
	}
	return ""
}

func ensureTaskDependsOn(task *Task, depID string) bool {
	if task == nil || depID == "" {
		return false
	}
	for _, dep := range task.DependsOn {
		if dep == depID {
			return false
		}
	}
	task.DependsOn = append(task.DependsOn, depID)
	return true
}

func isReproDiagnosticTask(task *Task) bool {
	if task == nil {
		return false
	}
	if task.Type != TaskTypeTestRun {
		return false
	}
	if strings.HasPrefix(strings.TrimSpace(task.Description), reproDiagnosticDescriptionPrefix) {
		return true
	}
	return strings.TrimSpace(task.InferenceReason) == "/logic_failure_repro_guard"
}

// classifyTaskError uses heuristics to bucket errors into retry taxonomies.
func classifyTaskError(err error) string {
	if err == nil {
		return "/logic"
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "/transient"
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return "/logic"
	}

	for _, hint := range transientErrorHints {
		if strings.Contains(msg, hint) {
			return "/transient"
		}
	}
	return "/logic"
}

var transientErrorHints = []string{
	"timeout",
	"timed out",
	"context deadline",
	"context canceled",
	"context cancelled",
	"temporar",
	"rate limit",
	"too many requests",
	"resource exhausted",
	"try again",
	"connection reset",
	"connection refused",
	"connection",
	"unavailable",
	"network",
	"tls handshake timeout",
	"broken pipe",
	"eof",
	"i/o timeout",
	"i/o",
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
