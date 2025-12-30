package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"strings"
	"time"
)

// assertCampaignConfigFacts publishes runtime configuration to the kernel for policy rules.
func (o *Orchestrator) assertCampaignConfigFacts() {
	if o.campaign == nil || o.kernel == nil {
		return
	}
	campaignID := o.campaign.ID
	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "campaign_config",
		Args:      []interface{}{campaignID},
	})

	maxRetries := o.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	threshold := o.config.ReplanThreshold
	if threshold <= 0 {
		threshold = 3
	}
	autoReplan := "/false"
	if o.config.AutoReplan {
		autoReplan = "/true"
	}
	checkpointOnFail := "/false"
	if o.config.CheckpointOnFail {
		checkpointOnFail = "/true"
	}

	_ = o.kernel.Assert(core.Fact{
		Predicate: "campaign_config",
		Args:      []interface{}{campaignID, maxRetries, threshold, autoReplan, checkpointOnFail},
	})
}

// updateFailedTaskCount recomputes failed task totals and asserts a computed count fact.
func (o *Orchestrator) updateFailedTaskCount() {
	if o.campaign == nil || o.kernel == nil {
		return
	}
	failedCount := 0
	o.mu.RLock()
	for _, phase := range o.campaign.Phases {
		for _, t := range phase.Tasks {
			if t.Status == TaskFailed {
				failedCount++
			}
		}
	}
	campaignID := o.campaign.ID
	o.mu.RUnlock()

	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "failed_campaign_task_count_computed",
		Args:      []interface{}{campaignID},
	})
	_ = o.kernel.Assert(core.Fact{
		Predicate: "failed_campaign_task_count_computed",
		Args:      []interface{}{campaignID, failedCount},
	})
}

// runPhaseCheckpoint runs the checkpoint for a phase.
func (o *Orchestrator) runPhaseCheckpoint(ctx context.Context, phase *Phase) (bool, string, error) {
	logging.Campaign("Running checkpoint for phase: %s", phase.Name)
	timer := logging.StartTimer(logging.CategoryCampaign, fmt.Sprintf("checkpoint(%s)", phase.Name))
	defer timer.Stop()

	allPassed := true
	var failedSummaries []string

	for _, obj := range phase.Objectives {
		if obj.VerificationMethod == VerifyNone {
			logging.CampaignDebug("Skipping verification (method=none) for objective: %s", obj.Description)
			continue
		}

		logging.CampaignDebug("Running verification: %s", obj.VerificationMethod)
		passed, details, err := o.checkpoint.Run(ctx, phase, obj.VerificationMethod)
		if err != nil {
			logging.Get(logging.CategoryCampaign).Error("Checkpoint error: %v", err)
			return false, "", err
		}

		if passed {
			logging.Campaign("Checkpoint PASSED: %s", obj.VerificationMethod)
		} else {
			logging.Get(logging.CategoryCampaign).Warn("Checkpoint FAILED: %s - %s", obj.VerificationMethod, details)
			allPassed = false
			// Keep summaries concise for replanning context.
			short := details
			if len(short) > 500 {
				short = short[:500] + "..."
			}
			failedSummaries = append(failedSummaries, fmt.Sprintf("%s: %s", obj.VerificationMethod, short))
		}

		checkpoint := Checkpoint{
			Type:      string(obj.VerificationMethod),
			Passed:    passed,
			Details:   details,
			Timestamp: time.Now(),
		}

		o.mu.Lock()
		for i := range o.campaign.Phases {
			if o.campaign.Phases[i].ID == phase.ID {
				o.campaign.Phases[i].Checkpoints = append(o.campaign.Phases[i].Checkpoints, checkpoint)
				break
			}
		}
		o.mu.Unlock()

		// Record in kernel
		o.kernel.Assert(core.Fact{
			Predicate: "phase_checkpoint",
			Args:      []interface{}{phase.ID, string(obj.VerificationMethod), passed, details, time.Now().Unix()},
		})
	}

	return allPassed, strings.Join(failedSummaries, " | "), nil
}

// emitProgress sends progress update to channel.
func (o *Orchestrator) emitProgress() {
	if o.progressChan == nil {
		return
	}

	select {
	case o.progressChan <- o.GetProgress():
	default:
		// Channel full, skip
	}
}

// emitEvent sends an event to the event channel.
func (o *Orchestrator) emitEvent(eventType, phaseID, taskID, message string, data any) {
	if o.eventChan == nil {
		return
	}

	event := OrchestratorEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		PhaseID:   phaseID,
		TaskID:    taskID,
		Message:   message,
		Data:      data,
	}

	select {
	case o.eventChan <- event:
	default:
		// Channel full, skip
	}
}

// updateCampaignStatus sets the in-memory campaign status and refreshes the canonical kernel fact.
func (o *Orchestrator) updateCampaignStatus(status CampaignStatus) {
	if o.campaign == nil {
		return
	}

	o.campaign.Status = status
	campaignID := o.campaign.ID
	cType := string(o.campaign.Type)
	title := o.campaign.Title
	source := ""
	if len(o.campaign.SourceMaterial) > 0 {
		source = o.campaign.SourceMaterial[0]
	}

	_ = o.kernel.RetractFact(core.Fact{
		Predicate: "campaign",
		Args:      []interface{}{campaignID},
	})
	_ = o.kernel.Assert(core.Fact{
		Predicate: "campaign",
		Args:      []interface{}{campaignID, cType, title, source, string(status)},
	})
}

// determineConcurrencyLimit calculates the dynamic parallelism limit based on active workload.
func (o *Orchestrator) determineConcurrencyLimit(active map[string]bool, phase *Phase) int {
	// Base limit from config
	limit := o.maxParallelTasks

	// Check backpressure from spawn queue first
	if o.shardMgr != nil {
		if status := o.shardMgr.GetBackpressureStatus(); status != nil {
			if status.QueueUtilization > 0.8 {
				// High queue utilization, throttle down to single task
				logging.Campaign("Throttling concurrency: spawn queue at %.0f%%", status.QueueUtilization*100)
				return 1
			} else if status.QueueUtilization > 0.5 {
				// Moderate backpressure, reduce parallelism
				limit = limit / 2
				if limit < 1 {
					limit = 1
				}
				logging.CampaignDebug("Reducing concurrency due to spawn queue pressure (%.0f%%)", status.QueueUtilization*100)
			}
		}
	}

	// Count active task types
	var researchCount, refactorCount, testCount int
	for taskID := range active {
		// Find task in phase
		for _, t := range phase.Tasks {
			if t.ID == taskID {
				switch t.Type {
				case TaskTypeResearch, TaskTypeDocument:
					researchCount++
				case TaskTypeRefactor, TaskTypeIntegrate:
					refactorCount++
				case TaskTypeTestRun, TaskTypeVerify:
					testCount++
				}
				break
			}
		}
	}

	// Adaptive Logic:
	// 1. Refactoring is high-risk/CPU-heavy -> Throttle down
	if refactorCount > 0 {
		return 1 // Serial execution for refactoring to prevent race conditions/clobbering
	}

	// 2. Integration is complex -> Low parallelism
	// (Handled by Refactor count above if we treat them similar, or separate)

	// 3. Research/Tests are IO-bound -> Warning: Research spawns Shards which use memory.
	// We can scale up, but let's be conservative.
	if researchCount > 0 || testCount > 0 {
		// Boost limit for IO heavy work
		limit = o.maxParallelTasks * 2
		if limit > 10 {
			limit = 10
		}
	}

	return limit
}

// HandleNewRequirement processes a dynamic requirement injection from an external system (e.g., Autopoiesis).
// It wraps ReplanForNewRequirement.
func (o *Orchestrator) HandleNewRequirement(ctx context.Context, requirement string) error {
	logging.Campaign("New requirement received: %s", requirement[:min(100, len(requirement))])
	timer := logging.StartTimer(logging.CategoryCampaign, "HandleNewRequirement")
	defer timer.Stop()

	o.emitEvent("new_requirement_received", "", "", requirement, nil)

	// Pause temporarily to safely modify plan
	o.mu.Lock()
	wasPaused := o.isPaused
	o.isPaused = true
	logging.CampaignDebug("Temporarily pausing campaign to integrate new requirement")
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.isPaused = wasPaused
		logging.CampaignDebug("Resuming campaign after requirement integration")
		o.mu.Unlock()
	}()

	// Call the previously unwired Replanner method
	if err := o.replanner.ReplanForNewRequirement(ctx, o.campaign, requirement); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to integrate new requirement: %v", err)
		o.emitEvent("new_requirement_failed", "", "", err.Error(), nil)
		return err
	}

	logging.Campaign("New requirement successfully integrated into plan")
	o.emitEvent("new_requirement_integrated", "", "", "Plan updated with new requirement", nil)
	return nil
}
