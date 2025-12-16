package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
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

	phaseID := fmt.Sprintf("%v", facts[0].Args[0])
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

	facts, err := o.kernel.Query("eligible_task")
	if err != nil {
		logging.CampaignDebug("Error querying eligible_task: %v", err)
		return nil
	}
	if len(facts) == 0 {
		logging.CampaignDebug("No eligible_task facts found for phase %s", phase.ID)
		return nil
	}

	logging.CampaignDebug("Found %d eligible_task facts from kernel", len(facts))

	tasks := make([]*Task, 0, len(facts))
	for i := range phase.Tasks {
		for _, fact := range facts {
			taskID := fmt.Sprintf("%v", fact.Args[0])
			if phase.Tasks[i].ID == taskID {
				tasks = append(tasks, &phase.Tasks[i])
				break
			}
		}
	}
	logging.CampaignDebug("Matched %d eligible tasks for phase %s", len(tasks), phase.ID)

	// Respect retry backoff windows.
	now := time.Now()
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

	taskID := fmt.Sprintf("%v", facts[0].Args[0])
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
		reason = fmt.Sprintf("%v", facts[0].Args[1])
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

	phaseID := fmt.Sprintf("%v", facts[0].Args[0])
	logging.Campaign("Phase transition: starting phase %s", phaseID)

	// Find and update phase
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phaseID {
			logging.Campaign("=== Phase Started: %s (%s) ===", o.campaign.Phases[i].Name, phaseID)
			logging.CampaignDebug("Phase details: order=%d, tasks=%d, complexity=%s",
				o.campaign.Phases[i].Order, len(o.campaign.Phases[i].Tasks), o.campaign.Phases[i].EstimatedComplexity)

			o.campaign.Phases[i].Status = PhaseInProgress

			// Update kernel
			_ = o.kernel.RetractFact(core.Fact{
				Predicate: "campaign_phase",
				Args:      []interface{}{phaseID},
			})
			o.kernel.Assert(core.Fact{
				Predicate: "campaign_phase",
				Args: []interface{}{
					phaseID,
					o.campaign.ID,
					o.campaign.Phases[i].Name,
					o.campaign.Phases[i].Order,
					"/in_progress",
					o.campaign.Phases[i].ContextProfile,
				},
			})

			o.emitEvent("phase_started", phaseID, "", o.campaign.Phases[i].Name, nil)
			return nil
		}
	}

	logging.Get(logging.CategoryCampaign).Error("Phase not found: %s", phaseID)
	return fmt.Errorf("phase %s not found", phaseID)
}

// completePhase marks a phase as complete.
func (o *Orchestrator) completePhase(phase *Phase) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phase.ID {
			logging.Campaign("=== Phase Completed: %s (%s) ===", phase.Name, phase.ID)

			completedTasks := 0
			for _, t := range o.campaign.Phases[i].Tasks {
				if t.Status == TaskCompleted {
					completedTasks++
				}
			}
			logging.CampaignDebug("Phase stats: completed tasks=%d/%d", completedTasks, len(o.campaign.Phases[i].Tasks))

			o.campaign.Phases[i].Status = PhaseCompleted
			o.campaign.CompletedPhases++

			logging.Campaign("Campaign progress: phases=%d/%d",
				o.campaign.CompletedPhases, o.campaign.TotalPhases)

			// Update kernel
			_ = o.kernel.RetractFact(core.Fact{
				Predicate: "campaign_phase",
				Args:      []interface{}{phase.ID},
			})
			o.kernel.Assert(core.Fact{
				Predicate: "campaign_phase",
				Args: []interface{}{
					phase.ID,
					o.campaign.ID,
					phase.Name,
					phase.Order,
					"/completed",
					phase.ContextProfile,
				},
			})

			o.emitEvent("phase_completed", phase.ID, "", phase.Name, nil)
			_ = o.saveCampaign()
			break
		}
	}
}
