package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"fmt"
	"strings"
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

	// Deterministic risk scoring and gate enforcement (once per run). The kernel
	// grades the gate results; this only enforces what it derived, and the
	// returned error carries the whole evaluation so CLI and chat can render it
	// instead of leaving it buried in CategoryCampaign logs.
	if eval, err := o.runRiskPreflight(ctx); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Risk gate blocked campaign start: %v", err)
		o.emitEvent(EventRiskGateBlocked, "", "", err.Error(), eval)
		o.mu.Unlock()
		return err
	} else if soft := eval.SoftFindings(); len(soft) > 0 {
		// Advisory findings do not stop the campaign, but silence is how they
		// stop mattering. Emit them on the event channel the UIs already read.
		for _, f := range soft {
			o.emitEvent(EventRiskGateAdvisory, "", "",
				fmt.Sprintf("Advisory: %s %s", f.Gate, strings.TrimPrefix(f.Reason, "/")), f)
		}
	}

	// Normalize any dangling in-progress tasks/phases (e.g., after restart)
	o.resetInProgress()

	// Set up cancellation
	ctx, cancel := context.WithCancel(ctx)
	o.cancelFunc = cancel
	o.isRunning = true
	// Reset pause state at run-start: ensure pauseCh is closed (resumed).
	if o.isPaused {
		if o.pauseCh != nil {
			close(o.pauseCh)
		}
		o.isPaused = false
	} else if o.pauseCh == nil {
		// Defensive: ensure pauseCh exists and is closed (resumed).
		o.pauseCh = make(chan struct{})
		close(o.pauseCh)
	}
	o.updateCampaignStatus(StatusActive)
	o.mu.Unlock()

	// Record the repository root before any task runs. The completion sweep
	// compares against this, so a campaign can only ever move files it created
	// itself — never something that was already there.
	o.recordRootBaseline()

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
		o.finalizeCancellation(ctx)
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

		// Terminal-failure guard (F-STALL-1): once a phase hard-block marks the
		// campaign failed, stop immediately. Without this the loop falls through
		// to startNextPhase, which spins "no eligible phases" forever because the
		// campaign_blocked derivation is transient and no longer fires after the
		// blocking phase leaves current_phase.
		o.mu.RLock()
		terminalFailed := o.campaign != nil && o.campaign.Status == StatusFailed
		lastErr := o.lastError
		o.mu.RUnlock()
		if terminalFailed {
			logging.Campaign("Campaign in failed state; ending execution loop")
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("campaign failed")
		}

		// Check if paused — block on pauseCh instead of busy-waiting.
		o.mu.RLock()
		paused := o.isPaused
		pauseCh := o.pauseCh
		o.mu.RUnlock()
		if paused {
			logging.CampaignDebug("Campaign paused, waiting on pauseCh...")
			select {
			case <-ctx.Done():
				logging.Campaign("Campaign execution cancelled during pause")
				return ctx.Err()
			case <-pauseCh:
				// Resumed
			}
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

				// Sweep scratch the campaign left in the user's repository root.
				//
				// Done here rather than per task on purpose. Blocking the write
				// is the wrong shape: WriteSet is INFERRED by the decomposer
				// from artifact paths and description text, tasks run
				// concurrently, and a wrong block kills legitimate work. Moving
				// files mid-campaign is also wrong, because a later phase may
				// legitimately read what an earlier one produced.
				//
				// At completion both objections are gone. No further task can
				// depend on the file, and moving it into the campaign's own
				// artifacts directory preserves the content while leaving the
				// repository as the user had it.
				o.sweepUndeclaredRootWrites()

				o.mu.Lock()
				o.updateCampaignStatus(StatusCompleted)
				_ = o.saveCampaign()
				o.mu.Unlock()
				o.emitEvent(EventCampaignCompleted, "", "", "Campaign completed successfully", nil)
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
				o.emitEvent(EventContextError, currentPhase.ID, "", err.Error(), nil)
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

// finalizeCancellation persists a paused status when cancellation terminates
// execution before a terminal state is reached. It is idempotent and never
// overwrites completed or failed campaigns. It ensures the in-memory status
// and the on-disk campaign.json converge, fixing the timeout case where
// context expiration inside runPhase returned before the top-loop ctx.Done
// branch could persist the pause. It no-ops unless ctx.Err is non-nil so
// ordinary non-context errors and successful completions are not overwritten.
func (o *Orchestrator) finalizeCancellation(ctx context.Context) {
	if ctx.Err() == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.campaign == nil {
		return
	}
	if o.campaign.Status == StatusCompleted || o.campaign.Status == StatusFailed {
		return
	}
	if o.campaign.Status != StatusPaused {
		o.updateCampaignStatus(StatusPaused)
	}
	_ = o.saveCampaign()
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
				// Only use atomic transactions if the kernel supports it
				if kt, ok := o.kernel.(types.KernelTransactor); ok {
					tx := kt.Transaction()
					tx.RetractFact(core.Fact{
						Predicate: "campaign_heartbeat",
						Args:      []any{campaignID},
					})
					tx.Assert(core.Fact{
						Predicate: "campaign_heartbeat",
						Args:      []any{campaignID, time.Now().Unix()},
					})
					_ = tx.Commit()
				} else {
					_ = o.kernel.RetractFact(core.Fact{
						Predicate: "campaign_heartbeat",
						Args:      []any{campaignID},
					})
					_ = o.kernel.Assert(core.Fact{
						Predicate: "campaign_heartbeat",
						Args:      []any{campaignID, time.Now().Unix()},
					})
				}
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
