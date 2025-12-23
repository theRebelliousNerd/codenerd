package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// runPhase executes all tasks in a phase with bounded parallelism, checkpoints,
// and rolling-wave refinement of the next phase once complete.
func (o *Orchestrator) runPhase(ctx context.Context, phase *Phase) error {
	if phase == nil {
		return nil
	}

	phaseTimer := logging.StartTimer(logging.CategoryCampaign, fmt.Sprintf("runPhase(%s)", phase.Name))
	defer phaseTimer.StopWithInfo()

	logging.Campaign("Executing phase: %s (tasks=%d)", phase.Name, len(phase.Tasks))

	active := make(map[string]bool)
	results := make(chan taskResult, o.maxParallelTasks*2)

	for {
		// Respect cancellation
		select {
		case <-ctx.Done():
			logging.Campaign("Phase %s cancelled", phase.Name)
			return ctx.Err()
		default:
		}

		// Respect pause (no new work scheduled while paused)
		o.mu.RLock()
		paused := o.isPaused
		o.mu.RUnlock()
		if paused {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Drain any completed tasks
		for {
			select {
			case res := <-results:
				logging.CampaignDebug("Task result received: %s (err=%v)", res.taskID, res.err)
				delete(active, res.taskID)
			default:
				goto schedule
			}
		}

	schedule:
		// If phase is done and no active tasks, run checkpoint and finish
		if o.isPhaseComplete(phase) && len(active) == 0 {
			logging.Campaign("Phase %s complete, running checkpoint", phase.Name)
			allPassed, failedSummary, err := o.runPhaseCheckpoint(ctx, phase)
			if err != nil {
				logging.Get(logging.CategoryCampaign).Error("Checkpoint errored for phase %s: %v", phase.ID, err)
				o.emitEvent("checkpoint_failed", phase.ID, "", err.Error(), nil)
			}

			// If any verification failed, trigger a replan and keep the phase open.
			if !allPassed {
				logging.Get(logging.CategoryCampaign).Warn("Phase %s checkpoint failures: %s", phase.ID, failedSummary)
				o.emitEvent("checkpoint_failed", phase.ID, "", failedSummary, nil)

				// Seed a replan trigger so Replanner has a hard signal.
				if err := o.kernel.Assert(core.Fact{
					Predicate: "replan_trigger",
					Args:      []interface{}{o.campaign.ID, "/checkpoint_failed", time.Now().Unix()},
				}); err != nil {
					logging.CampaignWarn("failed to assert replan_trigger: %v", err)
				}

				if o.replanner != nil {
					if repErr := o.replanner.Replan(ctx, o.campaign, ""); repErr != nil {
						logging.Get(logging.CategoryCampaign).Error("Replan after checkpoint failure failed: %v", repErr)
						o.emitEvent("replan_failed", phase.ID, "", repErr.Error(), nil)
					} else {
						o.mu.Lock()
						if err := o.saveCampaign(); err != nil {
							logging.CampaignWarn("failed to save campaign after replan: %v", err)
						}
						o.mu.Unlock()
					}
				}

				// Return to main loop; policy will keep current phase active.
				return nil
			}

			logging.CampaignDebug("Compressing phase context: %s", phase.ID)
			if summary, count, compressedAt, err := o.contextPager.CompressPhase(ctx, phase); err != nil {
				logging.Get(logging.CategoryCampaign).Warn("Context compression error: %v", err)
				o.emitEvent("compression_error", phase.ID, "", err.Error(), nil)
			} else {
				logging.CampaignDebug("Phase compressed: atoms=%d, summary_len=%d", count, len(summary))
				o.mu.Lock()
				phase.CompressedSummary = summary
				phase.OriginalAtomCount = count
				phase.CompressedAt = compressedAt
				if err := o.saveCampaign(); err != nil {
					logging.CampaignWarn("failed to save campaign after compression: %v", err)
				}
				o.mu.Unlock()
			}
			o.completePhase(phase)
			o.triggerRollingWave(ctx, phase)
			return nil
		}

		var runnable []*Task

		// Calculate adaptive concurrency limit
		currentLimit := o.determineConcurrencyLimit(active, phase)
		logging.CampaignDebug("Concurrency: active=%d, limit=%d", len(active), currentLimit)

		// Schedule eligible tasks up to the concurrency limit
		if len(active) < currentLimit {
			runnable = o.getEligibleTasks(phase)
			for _, task := range runnable {
				if len(active) >= currentLimit {
					break
				}
				if active[task.ID] || task.Status != TaskPending {
					continue
				}
				logging.Campaign("Scheduling task: %s (type=%s)", task.Description[:min(50, len(task.Description))], task.Type)
				active[task.ID] = true
				o.updateTaskStatus(task, TaskInProgress)
				go o.runSingleTask(ctx, phase, task, results)
			}
		}

		// If nothing is running or eligible, we may be blocked
		if len(active) == 0 {
			if runnable == nil {
				runnable = o.getEligibleTasks(phase)
			}
			if len(runnable) == 0 {
				if reason := o.getCampaignBlockReason(); reason != "" {
					logging.Get(logging.CategoryCampaign).Error("Phase blocked: %s", reason)
					o.emitEvent("campaign_blocked", phase.ID, "", reason, nil)
					o.mu.Lock()
					o.updateCampaignStatus(StatusFailed)
					o.lastError = fmt.Errorf("phase blocked: %s", reason)
					if err := o.saveCampaign(); err != nil {
						logging.CampaignWarn("failed to save campaign after block: %v", err)
					}
					o.mu.Unlock()
					return fmt.Errorf("phase blocked: %s", reason)
				}
			}
		}

		// Wait for activity (completion or new eligibility)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case res := <-results:
			delete(active, res.taskID)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// triggerRollingWave refreshes downstream plans after a phase completes.
func (o *Orchestrator) triggerRollingWave(ctx context.Context, completedPhase *Phase) {
	logging.Campaign("Rolling-wave refinement triggered after phase: %s", completedPhase.Name)
	timer := logging.StartTimer(logging.CategoryCampaign, "triggerRollingWave")
	defer timer.Stop()

	// Optional: refresh the world model / holographic graph after edits.
	// We rely on the VirtualStore scopes to refresh after writes; this hook
	// keeps the policy facts in sync across phases.
	if o.virtualStore != nil {
		logging.CampaignDebug("Refreshing world model scope")
		// Best-effort scope refresh to update code graph facts
		_, _ = o.virtualStore.RouteAction(ctx, core.Fact{
			Predicate: "action",
			Args:      []interface{}{"/refresh_scope", o.workspace},
		})
	}

	if o.replanner != nil {
		logging.CampaignDebug("Refining next phase based on completed phase: %s", completedPhase.ID)
		if err := o.replanner.RefineNextPhase(ctx, o.campaign, completedPhase); err != nil {
			logging.Get(logging.CategoryCampaign).Warn("Rolling-wave refinement failed: %v", err)
			o.emitEvent("replan_failed", completedPhase.ID, "", err.Error(), nil)
			return
		}

		// Reload campaign facts after refinement to keep Mangle view up to date
		logging.CampaignDebug("Reloading campaign facts after refinement")
		o.kernel.Retract("campaign_phase")
		o.kernel.Retract("campaign_task")
		if err := o.kernel.LoadFacts(o.campaign.ToFacts()); err != nil {
			logging.CampaignWarn("failed to reload campaign facts after refinement: %v", err)
		}

		logging.Campaign("Rolling-wave refinement applied (revision=%d)", o.campaign.RevisionNumber)
		o.emitEvent("replan", completedPhase.ID, "", "Rolling-wave refinement applied", map[string]any{
			"revision": o.campaign.RevisionNumber,
		})
	}
}

// runSingleTask executes a task and sends the result back to the phase loop.
func (o *Orchestrator) runSingleTask(ctx context.Context, phase *Phase, task *Task, results chan<- taskResult) {
	// Apply task-level timeout
	if o.config.TaskTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.config.TaskTimeout)
		defer cancel()
	}

	taskTimer := logging.StartTimer(logging.CategoryCampaign, fmt.Sprintf("task(%s)", task.ID))

	logging.Campaign("Task started: %s (type=%s, phase=%s)", task.ID, task.Type, phase.Name)
	logging.CampaignDebug("Task description: %s", task.Description)

	o.emitEvent("task_started", phase.ID, task.ID, task.Description, nil)
	result, err := o.executeTask(ctx, task)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Task failed: %s - %v", task.ID, err)
		taskTimer.Stop()
		o.handleTaskFailure(ctx, phase, task, err)
		results <- taskResult{taskID: task.ID, err: err}
		return
	}

	taskTimer.StopWithInfo()
	logging.Campaign("Task completed: %s", task.ID)

	o.completeTask(task, result)
	o.emitEvent("task_completed", phase.ID, task.ID, "Task completed", result)
	o.applyLearnings(ctx, task, result)
	o.emitProgress()

	o.mu.Lock()
	o.saveCampaign()
	o.mu.Unlock()

	results <- taskResult{taskID: task.ID, result: result}
}

// updateTaskStatus updates task status in campaign and kernel.
func (o *Orchestrator) updateTaskStatus(task *Task, status TaskStatus) {
	logging.CampaignDebug("Task status update: %s -> %s", task.ID, status)
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.campaign.Phases {
		for j := range o.campaign.Phases[i].Tasks {
			if o.campaign.Phases[i].Tasks[j].ID == task.ID {
				o.campaign.Phases[i].Tasks[j].Status = status
				break
			}
		}
	}

	// Update kernel
	if err := o.kernel.RetractFact(core.Fact{
		Predicate: "campaign_task",
		Args:      []interface{}{task.ID},
	}); err != nil {
		logging.CampaignWarn("failed to retract campaign_task for %s: %v", task.ID, err)
	}
	if err := o.kernel.Assert(core.Fact{
		Predicate: "campaign_task",
		Args:      []interface{}{task.ID, task.PhaseID, task.Description, string(status), string(task.Type)},
	}); err != nil {
		logging.CampaignWarn("failed to assert campaign_task for %s: %v", task.ID, err)
	}
}

// completeTask marks a task as complete.
func (o *Orchestrator) completeTask(task *Task, result any) {
	o.updateTaskStatus(task, TaskCompleted)

	o.mu.Lock()
	o.campaign.CompletedTasks++
	o.mu.Unlock()

	// Record task result for learning and audit trail
	resultSummary := ""
	if result != nil {
		if data, err := json.Marshal(result); err == nil {
			resultSummary = string(data)
			// Truncate if too long
			if len(resultSummary) > 1000 {
				resultSummary = resultSummary[:1000] + "..."
			}
		}
	}
	o.kernel.Assert(core.Fact{
		Predicate: "task_result",
		Args:      []interface{}{task.ID, "/success", resultSummary},
	})

	// Store result for context injection into dependent tasks
	o.storeTaskResult(task.ID, resultSummary)
}

// applyLearnings applies autopoiesis learnings from task execution.
func (o *Orchestrator) applyLearnings(ctx context.Context, task *Task, result any) {
	// Check for cancellation before applying learnings
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Query for learnings to apply
	facts, err := o.kernel.Query("promote_to_long_term")
	if err != nil {
		return
	}

	if len(facts) == 0 {
		return
	}

	logging.CampaignDebug("Applying %d learnings from task %s", len(facts), task.ID)

	// Summarize result for learning context
	resultContext := ""
	if result != nil {
		if data, err := json.Marshal(result); err == nil {
			resultContext = string(data)
			if len(resultContext) > 500 {
				resultContext = resultContext[:500] + "..."
			}
		}
	}

	o.mu.Lock()
	for _, fact := range facts {
		// Combine task description with result context for richer learning
		factStr := task.Description
		if resultContext != "" {
			factStr = fmt.Sprintf("%s [result: %s]", task.Description, resultContext)
		}
		learning := Learning{
			Type:      "/success_pattern",
			Pattern:   fmt.Sprintf("%v", fact.Args[0]),
			Fact:      factStr,
			AppliedAt: time.Now(),
		}
		o.campaign.Learnings = append(o.campaign.Learnings, learning)
		logging.CampaignDebug("Learning captured: %s", learning.Pattern)
	}
	o.mu.Unlock()

	logging.Campaign("Captured %d learnings (total=%d)", len(facts), len(o.campaign.Learnings))
}
