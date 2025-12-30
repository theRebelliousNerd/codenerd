package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"time"
)

// Run executes the campaign until completion, pause, or failure.
func (o *Orchestrator) Run(ctx context.Context) error {
	runTimer := logging.StartTimer(logging.CategoryCampaign, "Run")

	o.mu.Lock()
	if o.campaign == nil {
		o.mu.Unlock()
		logging.Get(logging.CategoryCampaign).Error("Run called with no campaign loaded")
		return fmt.Errorf("no campaign loaded")
	}
	if o.isRunning {
		o.mu.Unlock()
		logging.Get(logging.CategoryCampaign).Warn("Campaign already running: %s", o.campaign.ID)
		return fmt.Errorf("campaign already running")
	}

	logging.Campaign("=== Starting campaign execution: %s ===", o.campaign.ID)
	logging.Campaign("Campaign: %s (type=%s, phases=%d, tasks=%d)",
		o.campaign.Title, o.campaign.Type, o.campaign.TotalPhases, o.campaign.TotalTasks)

	// Northstar alignment check at campaign start
	if o.northstarObserver != nil {
		if err := o.northstarObserver.StartCampaign(ctx, o.campaign.ID, o.campaign.Goal); err != nil {
			logging.Get(logging.CategoryCampaign).Warn("Northstar blocked campaign start: %v", err)
			o.mu.Unlock()
			return fmt.Errorf("northstar alignment failed: %w", err)
		}
	}

	// Normalize any dangling in-progress tasks/phases (e.g., after restart)
	o.resetInProgress()

	// Set up cancellation
	ctx, cancel := context.WithCancel(ctx)
	o.cancelFunc = cancel
	o.isRunning = true
	o.isPaused = false
	o.updateCampaignStatus(StatusActive)
	o.mu.Unlock()

	// Apply campaign-level timeout
	if o.config.CampaignTimeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, o.config.CampaignTimeout)
		defer timeoutCancel()
		logging.Campaign("Campaign timeout set: %v", o.config.CampaignTimeout)
	}

	// Start heartbeat/autosave loop for long-running durability.
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go o.runHeartbeatLoop(heartbeatCtx)

	defer func() {
		o.mu.Lock()
		o.isRunning = false
		o.cancelFunc = nil
		o.mu.Unlock()
		runTimer.StopWithInfo()
	}()

	// Main execution loop
	loopCount := 0
	for {
		loopCount++
		logging.CampaignDebug("Execution loop iteration %d", loopCount)

		select {
		case <-ctx.Done():
			logging.Campaign("Campaign execution cancelled: %v", ctx.Err())
			o.mu.Lock()
			o.updateCampaignStatus(StatusPaused)
			_ = o.saveCampaign()
			o.mu.Unlock()
			return ctx.Err()
		default:
		}

		// Check if paused
		o.mu.RLock()
		paused := o.isPaused
		o.mu.RUnlock()
		if paused {
			logging.CampaignDebug("Campaign paused, waiting...")
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 1. Query Mangle for current state
		currentPhase := o.getCurrentPhase()
		if currentPhase == nil {
			// Check if campaign is complete
			if o.isCampaignComplete() {
				logging.Campaign("=== Campaign completed successfully: %s ===", o.campaign.ID)
				logging.Campaign("Final stats: phases=%d/%d, tasks=%d/%d",
					o.campaign.CompletedPhases, o.campaign.TotalPhases,
					o.campaign.CompletedTasks, o.campaign.TotalTasks)

				// Northstar final observation
				if o.northstarObserver != nil {
					summary := fmt.Sprintf("phases=%d/%d, tasks=%d/%d",
						o.campaign.CompletedPhases, o.campaign.TotalPhases,
						o.campaign.CompletedTasks, o.campaign.TotalTasks)
					_ = o.northstarObserver.EndCampaign(ctx, true, summary)
				}

				o.mu.Lock()
				o.updateCampaignStatus(StatusCompleted)
				_ = o.saveCampaign()
				o.mu.Unlock()
				o.emitEvent("campaign_completed", "", "", "Campaign completed successfully", nil)
				return nil
			}

			// Check if blocked
			blockReason := o.getCampaignBlockReason()
			if blockReason != "" {
				logging.Get(logging.CategoryCampaign).Error("Campaign blocked: %s", blockReason)
				o.mu.Lock()
				o.updateCampaignStatus(StatusFailed)
				o.lastError = fmt.Errorf("campaign blocked: %s", blockReason)
				_ = o.saveCampaign()
				o.mu.Unlock()
				return o.lastError
			}

			// No current phase but not complete - start next eligible phase
			logging.CampaignDebug("No current phase, starting next eligible phase")
			if err := o.startNextPhase(ctx); err != nil {
				logging.Get(logging.CategoryCampaign).Warn("Failed to start next phase: %v", err)
				o.lastError = err
				continue
			}
			continue
		}

		logging.CampaignDebug("Current phase: %s (%s)", currentPhase.Name, currentPhase.ID)

		// 2. Page in context for current phase only on transition
		if o.contextPager != nil && currentPhase.ID != o.lastPhaseID {
			o.contextPager.ResetPhaseContext()
			if err := o.contextPager.ActivatePhase(ctx, currentPhase); err != nil {
				logging.Get(logging.CategoryCampaign).Warn("Context activation error: %v", err)
				o.emitEvent("context_error", currentPhase.ID, "", err.Error(), nil)
			}
			// Prefetch upcoming tasks for this phase
			var upcoming []Task
			for _, t := range currentPhase.Tasks {
				if t.Status == TaskPending {
					upcoming = append(upcoming, t)
				}
			}
			_ = o.contextPager.PrefetchNextTasks(ctx, upcoming, 3)
			o.lastPhaseID = currentPhase.ID
		}

		// 3. Execute the phase with parallelism + rolling checkpoints
		if err := o.runPhase(ctx, currentPhase); err != nil {
			logging.Get(logging.CategoryCampaign).Error("Phase execution error: %v", err)
			o.lastError = err
			if ctx.Err() != nil {
				return err
			}
		}
	}
}

// runHeartbeatLoop periodically emits progress, updates kernel heartbeat facts,
// and persists the campaign even when tasks are idle or blocked.
func (o *Orchestrator) runHeartbeatLoop(ctx context.Context) {
	heartbeatTicker := time.NewTicker(o.config.HeartbeatEvery)
	autosaveTicker := time.NewTicker(o.config.AutosaveEvery)
	defer heartbeatTicker.Stop()
	defer autosaveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			o.emitProgress()
			o.mu.RLock()
			campaignID := ""
			if o.campaign != nil {
				campaignID = o.campaign.ID
			}
			o.mu.RUnlock()
			if campaignID != "" && o.kernel != nil {
				_ = o.kernel.RetractFact(core.Fact{
					Predicate: "campaign_heartbeat",
					Args:      []interface{}{campaignID},
				})
				_ = o.kernel.Assert(core.Fact{
					Predicate: "campaign_heartbeat",
					Args:      []interface{}{campaignID, time.Now().Unix()},
				})
			}
		case <-autosaveTicker.C:
			o.mu.Lock()
			if o.campaign != nil {
				_ = o.saveCampaign()
			}
			o.mu.Unlock()
		}
	}
}
