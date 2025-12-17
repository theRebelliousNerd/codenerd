package campaign

import (
	"codenerd/internal/logging"
)

// Pause pauses campaign execution.
func (o *Orchestrator) Pause() {
	o.mu.Lock()
	defer o.mu.Unlock()
	logging.Campaign("Pausing campaign: %s", o.campaign.ID)
	o.isPaused = true
	o.updateCampaignStatus(StatusPaused)
	_ = o.saveCampaign()
}

// Resume resumes paused campaign execution.
func (o *Orchestrator) Resume() {
	o.mu.Lock()
	defer o.mu.Unlock()
	logging.Campaign("Resuming campaign: %s", o.campaign.ID)
	o.isPaused = false
	o.updateCampaignStatus(StatusActive)
}

// Stop stops campaign execution.
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	logging.Campaign("Stopping campaign: %s", o.campaign.ID)
	if o.cancelFunc != nil {
		o.cancelFunc()
	}
	o.updateCampaignStatus(StatusPaused)
	_ = o.saveCampaign()

	// Close channels to signal consumers
	if o.progressChan != nil {
		close(o.progressChan)
		o.progressChan = nil
	}
	if o.eventChan != nil {
		close(o.eventChan)
		o.eventChan = nil
	}
}

// GetProgress returns current campaign progress.
func (o *Orchestrator) GetProgress() Progress {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.campaign == nil {
		return Progress{}
	}

	currentPhase := o.getCurrentPhase()
	currentPhaseName := ""
	currentPhaseIdx := 0
	if currentPhase != nil {
		currentPhaseName = currentPhase.Name
		currentPhaseIdx = currentPhase.Order
	}

	currentTask := ""
	nextTask := o.getNextTask(currentPhase)
	if nextTask != nil {
		currentTask = nextTask.Description
	}

	// Calculate progress
	phaseProgress := 0.0
	if currentPhase != nil && len(currentPhase.Tasks) > 0 {
		completed := 0
		for _, t := range currentPhase.Tasks {
			if t.Status == TaskCompleted || t.Status == TaskSkipped {
				completed++
			}
		}
		phaseProgress = float64(completed) / float64(len(currentPhase.Tasks))
	}

	overallProgress := 0.0
	if o.campaign.TotalTasks > 0 {
		overallProgress = float64(o.campaign.CompletedTasks) / float64(o.campaign.TotalTasks)
	}

	contextUsage := 0.0
	if o.campaign.ContextBudget > 0 {
		contextUsage = float64(o.campaign.ContextUsed) / float64(o.campaign.ContextBudget)
	}

	return Progress{
		CampaignID:      o.campaign.ID,
		CampaignTitle:   o.campaign.Title,
		CampaignStatus:  string(o.campaign.Status),
		CurrentPhase:    currentPhaseName,
		CurrentPhaseIdx: currentPhaseIdx,
		TotalPhases:     o.campaign.TotalPhases,
		PhaseProgress:   phaseProgress,
		OverallProgress: overallProgress,
		CurrentTask:     currentTask,
		CompletedTasks:  o.campaign.CompletedTasks,
		TotalTasks:      o.campaign.TotalTasks,
		ActiveShards:    o.getActiveShardNames(),
		ContextUsage:    contextUsage,
		Learnings:       len(o.campaign.Learnings),
		Replans:         o.campaign.RevisionNumber,
	}
}

// getActiveShardNames returns the names of currently active shards.
func (o *Orchestrator) getActiveShardNames() []string {
	if o.shardMgr == nil {
		return []string{}
	}
	activeShards := o.shardMgr.GetActiveShards()
	names := make([]string, 0, len(activeShards))
	for _, shard := range activeShards {
		names = append(names, shard.GetConfig().Name)
	}
	return names
}
