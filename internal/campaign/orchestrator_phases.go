package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"fmt"
	"time"
)

// getCurrentPhase gets the current active phase from Mangle.
func (o *Orchestrator) getCurrentPhase() *Phase {
	facts, err := o.kernel.Query("current_phase")
	if err != nil {
		logging.CampaignDebug("Error querying current_phase: %v", err)
		return nil
	}
	if len(facts) == 0 {
		logging.CampaignDebug("No current_phase fact found")
		return nil
	}

	phaseID := types.ExtractString(facts[0].Args[0])
	logging.CampaignDebug("Current phase from kernel: %s", phaseID)

	// Find phase in campaign
	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phaseID {
			return &o.campaign.Phases[i]
		}
	}

	logging.CampaignDebug("Phase %s not found in campaign structure", phaseID)
	return nil
}

// getEligibleTasks returns all runnable tasks for the current phase.
func (o *Orchestrator) getEligibleTasks(phase *Phase) []*Task {
	if phase == nil {
		return nil
	}

	now := time.Now()
	tasks := make([]*Task, 0)

	facts, err := o.kernel.Query("eligible_task")
	if err != nil {
		logging.CampaignDebug("Error querying eligible_task: %v", err)
	}
	if len(facts) > 0 {
		logging.CampaignDebug("Found %d eligible_task facts from kernel", len(facts))
		for i := range phase.Tasks {
			for _, fact := range facts {
				taskID := types.ExtractString(fact.Args[0])
				if phase.Tasks[i].ID == taskID {
					tasks = append(tasks, &phase.Tasks[i])
					break
				}
			}
		}
	}

	// Fallback: when Mangle has no eligible_task facts (common after resume if
	// campaign_task facts were not re-derived), use in-memory dependency rules
	// so the phase does not spin forever with zero work.
	if len(tasks) == 0 {
		logging.CampaignDebug("No eligible_task facts for phase %s; using dependency fallback", phase.ID)
		completed := make(map[string]bool)
		for i := range phase.Tasks {
			if phase.Tasks[i].Status == TaskCompleted {
				completed[phase.Tasks[i].ID] = true
			}
		}
		// Also treat tasks completed in other phases as satisfied deps when IDs match.
		if o.campaign != nil {
			for pi := range o.campaign.Phases {
				for ti := range o.campaign.Phases[pi].Tasks {
					t := &o.campaign.Phases[pi].Tasks[ti]
					if t.Status == TaskCompleted {
						completed[t.ID] = true
					}
				}
			}
		}
		for i := range phase.Tasks {
			t := &phase.Tasks[i]
			if t.Status != TaskPending {
				continue
			}
			depsOK := true
			for _, dep := range t.DependsOn {
				if !completed[dep] {
					depsOK = false
					break
				}
			}
			if depsOK {
				tasks = append(tasks, t)
			}
		}
		logging.Campaign("Eligible fallback matched %d pending tasks for phase %s", len(tasks), phase.ID)
	} else {
		logging.CampaignDebug("Matched %d eligible tasks for phase %s", len(tasks), phase.ID)
	}

	// Respect retry backoff windows.
	filtered := make([]*Task, 0, len(tasks))
	skipped := 0
	for _, t := range tasks {
		if !t.NextRetryAt.IsZero() && t.NextRetryAt.After(now) {
			skipped++
			continue
		}
		filtered = append(filtered, t)
	}
	if skipped > 0 {
		logging.CampaignDebug("Filtered %d eligible tasks due to backoff", skipped)
	}
	return filtered
}

// getNextTask gets the next task to execute from Mangle.
func (o *Orchestrator) getNextTask(phase *Phase) *Task {
	if phase == nil {
		return nil
	}

	facts, err := o.kernel.Query("next_campaign_task")
	if err != nil {
		logging.CampaignDebug("Error querying next_campaign_task: %v", err)
		return nil
	}
	if len(facts) == 0 {
		logging.CampaignDebug("No next_campaign_task fact found")
		return nil
	}

	taskID := types.ExtractString(facts[0].Args[0])
	logging.CampaignDebug("Next task from kernel: %s", taskID)

	// Find task in phase
	for i := range phase.Tasks {
		if phase.Tasks[i].ID == taskID {
			return &phase.Tasks[i]
		}
	}

	logging.CampaignDebug("Task %s not found in phase %s", taskID, phase.ID)
	return nil
}

// isCampaignComplete checks if all phases are complete.
func (o *Orchestrator) isCampaignComplete() bool {
	completedCount := 0
	skippedCount := 0
	for _, phase := range o.campaign.Phases {
		if phase.Status == PhaseCompleted {
			completedCount++
		} else if phase.Status == PhaseSkipped {
			skippedCount++
		} else {
			logging.CampaignDebug("Campaign not complete: phase %s is %s", phase.ID, phase.Status)
			return false
		}
	}
	logging.CampaignDebug("Campaign complete check: completed=%d, skipped=%d, total=%d",
		completedCount, skippedCount, len(o.campaign.Phases))
	return true
}

// getCampaignBlockReason checks if campaign is blocked.
func (o *Orchestrator) getCampaignBlockReason() string {
	facts, err := o.kernel.Query("campaign_blocked")
	if err != nil {
		logging.CampaignDebug("Error querying campaign_blocked: %v", err)
		return ""
	}
	if len(facts) == 0 {
		return ""
	}

	reason := "unknown"
	if len(facts[0].Args) >= 2 {
		reason = types.ExtractString(facts[0].Args[1])
	}
	logging.CampaignDebug("Campaign blocked detected: %s", reason)
	return reason
}

// isPhaseComplete checks if all tasks in a phase are complete.
func (o *Orchestrator) isPhaseComplete(phase *Phase) bool {
	completedCount := 0
	skippedCount := 0
	for _, task := range phase.Tasks {
		if task.Status == TaskCompleted {
			completedCount++
		} else if task.Status == TaskSkipped {
			skippedCount++
		} else {
			logging.CampaignDebug("Phase %s not complete: task %s is %s", phase.ID, task.ID, task.Status)
			return false
		}
	}
	logging.CampaignDebug("Phase %s complete check: completed=%d, skipped=%d, total=%d",
		phase.ID, completedCount, skippedCount, len(phase.Tasks))
	return true
}

// startNextPhase starts the next eligible phase.
func (o *Orchestrator) startNextPhase(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryCampaign, "startNextPhase")
	defer timer.Stop()

	// Check for cancellation before starting phase transition
	select {
	case <-ctx.Done():
		logging.CampaignDebug("Phase transition cancelled")
		return ctx.Err()
	default:
	}

	facts, err := o.kernel.Query("phase_eligible")
	if err != nil || len(facts) == 0 {
		logging.CampaignDebug("No eligible phases found")
		return fmt.Errorf("no eligible phases")
	}

	phaseID := types.ExtractString(facts[0].Args[0])
	logging.Campaign("Phase transition: starting phase %s", phaseID)

	// Local copies of data needed for kernel assertions
	var targetPhaseName string
	var targetPhaseOrder int
	var targetPhaseCtxProf string
	var campaignID string
	var found bool

	o.mu.Lock()
	campaignID = o.campaign.ID
	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phaseID {
			targetPhaseName = o.campaign.Phases[i].Name
			targetPhaseOrder = o.campaign.Phases[i].Order
			targetPhaseCtxProf = o.campaign.Phases[i].ContextProfile
			found = true

			logging.Campaign("=== Phase Started: %s (%s) ===", targetPhaseName, phaseID)
			logging.CampaignDebug("Phase details: order=%d, tasks=%d, complexity=%s",
				targetPhaseOrder, len(o.campaign.Phases[i].Tasks), o.campaign.Phases[i].EstimatedComplexity)

			o.campaign.Phases[i].Status = PhaseInProgress
			break
		}
	}
	o.mu.Unlock()

	if !found {
		logging.Get(logging.CategoryCampaign).Error("Phase not found: %s", phaseID)
		return fmt.Errorf("phase %s not found", phaseID)
	}

	// Update kernel (slow I/O outside lock)
	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "campaign_phase",
		Args:      []any{phaseID},
	})
	o.kernel.Assert(core.Fact{
		Predicate: "campaign_phase",
		Args: []any{
			phaseID,
			campaignID,
			targetPhaseName,
			targetPhaseOrder,
			"/in_progress",
			targetPhaseCtxProf,
		},
	})

	// Northstar alignment check at phase transition
	if o.northstarObserver != nil {
		check, err := o.northstarObserver.OnPhaseStart(ctx, phaseID, targetPhaseName)
		if err != nil {
			logging.Campaign("Northstar blocked phase %s: %v", phaseID, err)
			return fmt.Errorf("northstar alignment failed: %w", err)
		}
		if check != nil {
			logging.Campaign("Northstar phase check: %s score=%.2f", check.Result, check.Score)
		}
	}

	o.emitEvent("phase_started", phaseID, "", targetPhaseName, nil)
	return nil
}

// completePhase marks a phase as complete.
func (o *Orchestrator) completePhase(phase *Phase) {
	var targetPhaseName string
	var targetPhaseOrder int
	var targetPhaseCtxProf string
	var campaignID string
	var completedTasks int
	var totalTasks int
	var found bool
	var isSuccess bool

	o.mu.Lock()
	campaignID = o.campaign.ID
	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phase.ID {
			targetPhaseName = o.campaign.Phases[i].Name
			targetPhaseOrder = o.campaign.Phases[i].Order
			targetPhaseCtxProf = o.campaign.Phases[i].ContextProfile
			found = true

			logging.Campaign("=== Phase Completed: %s (%s) ===", targetPhaseName, phase.ID)

			completedTasks = 0
			totalTasks = len(o.campaign.Phases[i].Tasks)
			for _, t := range o.campaign.Phases[i].Tasks {
				if t.Status == TaskCompleted {
					completedTasks++
				}
			}
			logging.CampaignDebug("Phase stats: completed tasks=%d/%d", completedTasks, totalTasks)

			o.campaign.Phases[i].Status = PhaseCompleted
			o.campaign.CompletedPhases++

			logging.Campaign("Campaign progress: phases=%d/%d",
				o.campaign.CompletedPhases, o.campaign.TotalPhases)

			isSuccess = (o.campaign.Phases[i].Status == PhaseCompleted)

			break
		}
	}
	o.mu.Unlock()

	if !found {
		return
	}

	// Update kernel (slow I/O outside lock)
	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "campaign_phase",
		Args:      []any{phase.ID},
	})
	o.kernel.Assert(core.Fact{
		Predicate: "campaign_phase",
		Args: []any{
			phase.ID,
			campaignID,
			targetPhaseName,
			targetPhaseOrder,
			"/completed",
			targetPhaseCtxProf,
		},
	})

	// Northstar observation on phase completion
	if o.northstarObserver != nil {
		summary := fmt.Sprintf("Completed %d/%d tasks", completedTasks, totalTasks)
		_ = o.northstarObserver.OnPhaseComplete(context.Background(), phase.ID, isSuccess, summary)
	}

	o.emitEvent("phase_completed", phase.ID, "", targetPhaseName, nil)

	o.mu.Lock()
	_ = o.saveCampaign()
	o.mu.Unlock()
}
