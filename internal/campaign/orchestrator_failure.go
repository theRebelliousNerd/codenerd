package campaign

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// reproDiagnosticDescriptionPrefix marks a repro task's description for a
// reader; the task is recognized by its inference reason, not by this text.
const reproDiagnosticDescriptionPrefix = "[diagnostic-repro]"

// reproInferenceReason is the inference reason a repro task carries, which the
// policy names it by (task_is_repro). The value predates the typed signals and
// stays as it was so repro tasks in persisted campaigns are still recognized.
const reproInferenceReason = "/logic_failure_repro_guard"

// handleTaskFailure records a failed attempt with its typed signals, asks the
// kernel for the task's next move (task_next_move, policy/campaign_decisions.mg)
// and does it. The move is never decided here: a kernel that derives none, or
// more than one, fails the task.
func (o *Orchestrator) handleTaskFailure(ctx context.Context, phase *Phase, task *Task, err error) {
	if task == nil {
		logging.Get(logging.CategoryCampaign).Warn("Handling task failure: <nil task> - %v", err)
		return
	}
	errStr := "unknown error"
	if err != nil {
		errStr = err.Error()
	}
	signals := failureSignals(err)
	logging.Get(logging.CategoryCampaign).Warn("Handling task failure: %s %v - %s", task.ID, signals, errStr)

	phaseID := ""
	if phase != nil {
		phaseID = phase.ID
	}

	// Record the attempt.
	o.mu.Lock()
	live, pi, _ := o.liveTaskLocked(task.ID)
	if live == nil {
		o.mu.Unlock()
		logging.Get(logging.CategoryCampaign).Warn("Task %s failed but is not in the campaign; nothing to record", task.ID)
		return
	}
	attempt := TaskAttempt{
		Number:    len(live.Attempts) + 1,
		Outcome:   "/failure",
		Timestamp: time.Now(),
		Error:     errStr,
		Signals:   signals,
	}
	live.Attempts = append(live.Attempts, attempt)
	live.LastError = errStr
	phaseID = o.campaign.Phases[pi].ID
	errFact, hasErrFact := taskErrorFact(live)
	o.mu.Unlock()

	o.assertTaskFacts(task.ID, attemptFacts(task.ID, attempt)...)
	if hasErrFact {
		_ = o.kernel.RetractFact(core.Fact{Predicate: "task_error", Args: []any{task.ID}})
		o.assertTaskFacts(task.ID, errFact)
	}

	// Ask the kernel what the task does next.
	move, moveErr := o.oneDerivedFor("task_next_move", task.ID)
	if moveErr != nil {
		logging.Get(logging.CategoryCampaign).Error("Task %s: %v; the task fails", task.ID, moveErr)
		move = "/fail"
	}
	logging.Campaign("Task %s attempt %d failed %v; next move %s", task.ID, attempt.Number, signals, move)

	// Do it.
	status := TaskFailed
	var nextRetryAt time.Time
	reproTaskID, reproInserted := "", false
	replan := false
	o.mu.Lock()
	if live, pi, ti := o.liveTaskLocked(task.ID); live != nil {
		switch move {
		case "/retry", "/retry_later", "/repro_first":
			// A refusal or an ended context is waited out on the full
			// backoff: retrying fast against a broker that just declined
			// spends attempts against a limit that has not moved.
			nextRetryAt = attempt.Timestamp.Add(o.computeRetryBackoff(attempt.Number, move == "/retry_later"))
			status = TaskPending
			live.NextRetryAt = nextRetryAt
			if move == "/repro_first" {
				// Last: inserting the repro task moves this task in its slice.
				reproTaskID, reproInserted = o.insertReproDiagnosticTaskLocked(pi, ti, attempt.Number, errStr)
			}
		case "/replan":
			live.NextRetryAt = time.Time{}
			live.ReplannedAtCap = true
			replan = true
		default: // "/fail"
			live.NextRetryAt = time.Time{}
		}
	}
	o.mu.Unlock()

	o.updateTaskStatus(task, status)
	if replan {
		o.assertTaskFacts(task.ID, core.Fact{Predicate: "task_replanned_at_cap", Args: []any{task.ID}})
	}
	_ = o.kernel.RetractFact(core.Fact{Predicate: "task_retry_at", Args: []any{task.ID}})
	if !nextRetryAt.IsZero() {
		o.assertTaskFacts(task.ID, core.Fact{Predicate: "task_retry_at", Args: []any{task.ID, nextRetryAt.Unix()}})
	}

	o.emitEvent(EventTaskFailed, phaseID, task.ID, errStr, map[string]any{
		"attempt": attempt.Number,
		"signals": signals,
		"move":    move,
	})
	if reproInserted {
		o.emitEvent(EventDiagnosticTaskInserted, phaseID, reproTaskID, "Inserted repro-test-first diagnostic task", map[string]any{
			"failed_task_id": task.ID,
			"signals":        signals,
		})
	}

	// Optionally run checkpoint immediately after a task is fully failed.
	if status == TaskFailed && o.policy.CheckpointOnTaskFailure {
		if _, _, chkErr := o.runPhaseCheckpoint(ctx, phase); chkErr != nil {
			logging.Get(logging.CategoryCampaign).Warn("Checkpoint-on-fail error: %v", chkErr)
			o.emitEvent(EventCheckpointFailed, phaseID, "", chkErr.Error(), nil)
		}
	}

	// The replanner runs only on a failed task, never on one still retrying:
	// observed live, a /file_modify left pending for retry was replaced by a
	// semantically duplicate task and runPhase scheduled both, producing
	// competing files. /replan comes once per task (task_replanned_at_cap), so
	// the planner can drop or retype a poison task and let the phase proceed.
	if replan {
		if o.replanner == nil {
			logging.Get(logging.CategoryCampaign).Warn("Replan needed but no replanner configured for task %s", task.ID)
		} else if repErr := o.replanner.Replan(ctx, o.campaign, task.ID); repErr != nil {
			logging.Get(logging.CategoryCampaign).Error("Attempt-cap replan failed for task %s: %v", task.ID, repErr)
			o.emitEvent(EventReplanFailed, "", "", repErr.Error(), nil)
		} else {
			o.mu.Lock()
			logging.Campaign("Campaign replanned at attempt cap for task %s, new revision: %d", task.ID, o.campaign.RevisionNumber)
			o.persistCampaign("attempt-cap replan")
			o.mu.Unlock()
		}
	}

	// Persist failure updates for durability.
	o.mu.Lock()
	o.persistCampaign("task failure")
	o.mu.Unlock()
}

// liveTaskLocked finds the campaign's own copy of a task by ID; callers hold
// o.mu. A replan or a repro insertion can move a task, so a caller's pointer
// is never trusted across an unlock.
func (o *Orchestrator) liveTaskLocked(taskID string) (*Task, int, int) {
	if o.campaign == nil {
		return nil, -1, -1
	}
	for i := range o.campaign.Phases {
		for j := range o.campaign.Phases[i].Tasks {
			if o.campaign.Phases[i].Tasks[j].ID == taskID {
				return &o.campaign.Phases[i].Tasks[j], i, j
			}
		}
	}
	return nil, -1, -1
}

// assertTaskFacts asserts what a failure changed. A dropped assert leaves the
// kernel deciding the next move on an attempt it never saw, so it is logged
// at Error.
func (o *Orchestrator) assertTaskFacts(taskID string, facts ...core.Fact) {
	if o.kernel == nil {
		return
	}
	for _, f := range facts {
		if err := o.kernel.Assert(f); err != nil {
			logging.Get(logging.CategoryCampaign).Error("Task %s: %s was not asserted: %v", taskID, f.Predicate, err)
		}
	}
}

func (o *Orchestrator) insertReproDiagnosticTaskLocked(phaseIdx, taskIdx, attemptNum int, lastErr string) (string, bool) {
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
			o.assertTaskFacts(failedTaskID, core.Fact{Predicate: "task_dependency", Args: []any{failedTaskID, existing}})
		}
		return existing, false
	}

	// The error's first line names the failure; the whole of it is the failed
	// task's LastError, which its next attempt reads.
	firstLine, _, _ := strings.Cut(strings.TrimSpace(lastErr), "\n")
	reproTaskID := fmt.Sprintf("%s/repro_%03d", failedTaskID, attemptNum)
	reproTask := Task{
		ID:              reproTaskID,
		PhaseID:         phase.Tasks[taskIdx].PhaseID,
		Description:     fmt.Sprintf("%s Reproduce the red suite %s failed on: run tests before its next change. Last error: %s", reproDiagnosticDescriptionPrefix, failedTaskID, firstLine),
		Status:          TaskPending,
		Type:            TaskTypeTestRun,
		Priority:        PriorityCritical,
		Order:           0,
		InferredFrom:    failedTaskID,
		InferenceConf:   1.0,
		InferenceReason: reproInferenceReason,
	}

	phase.Tasks = append([]Task{reproTask}, phase.Tasks...)
	for idx := range phase.Tasks {
		phase.Tasks[idx].Order = idx
		phase.Tasks[idx].PhaseID = phase.ID
	}

	for idx := range phase.Tasks {
		if phase.Tasks[idx].ID == failedTaskID {
			if ensureTaskDependsOn(&phase.Tasks[idx], reproTaskID) {
				o.assertTaskFacts(failedTaskID, core.Fact{Predicate: "task_dependency", Args: []any{failedTaskID, reproTaskID}})
			}
			break
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
		if t.InferredFrom != failedTaskID || !isReproDiagnosticTask(&t) {
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
	if slices.Contains(task.DependsOn, depID) {
		return false
	}
	task.DependsOn = append(task.DependsOn, depID)
	return true
}

func isReproDiagnosticTask(task *Task) bool {
	return task != nil && task.Type == TaskTypeTestRun && strings.TrimSpace(task.InferenceReason) == reproInferenceReason
}

// computeRetryBackoff returns the exponential backoff for an attempt:
// campaign.retry_backoff_base doubled per attempt up to
// campaign.retry_backoff_max. A retry with a reason to change something
// (/retry, /repro_first) is capped lower, at
// campaign.retry_with_reason_backoff_max; one that can only wait (/retry_later)
// keeps the full exponential.
func (o *Orchestrator) computeRetryBackoff(attemptNum int, waitOut bool) time.Duration {
	base := o.policy.RetryBackoffBase
	maxBackoff := o.policy.RetryBackoffMax

	shift := min(max(attemptNum-1, 0), 10)

	// Prevent overflow: check if multiplication will exceed max int64
	multiplier := time.Duration(1 << shift)
	var backoff time.Duration
	if base > 0 && multiplier > 0 && base > math.MaxInt64/multiplier {
		backoff = maxBackoff
	} else {
		backoff = base * multiplier
	}
	if backoff < 0 {
		backoff = maxBackoff
	}

	if !waitOut && backoff > o.policy.RetryWithReasonBackoffMax {
		backoff = o.policy.RetryWithReasonBackoffMax
	}
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}
