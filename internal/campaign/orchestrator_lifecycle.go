package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadCampaign loads an existing campaign from disk.
func (o *Orchestrator) LoadCampaign(campaignID string) error {
	timer := logging.StartTimer(logging.CategoryCampaign, "LoadCampaign")
	defer timer.Stop()

	logging.Campaign("Loading campaign: %s", campaignID)

	o.mu.Lock()
	defer o.mu.Unlock()

	campaignPath := filepath.Join(o.nerdDir, "campaigns", campaignID+".json")
	logging.CampaignDebug("Reading campaign from: %s", campaignPath)

	data, err := os.ReadFile(campaignPath)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to read campaign file: %v", err)
		return fmt.Errorf("failed to load campaign: %w", err)
	}

	var campaign Campaign
	if err := json.Unmarshal(data, &campaign); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to parse campaign JSON: %v", err)
		return fmt.Errorf("failed to parse campaign: %w", err)
	}

	o.campaign = &campaign

	logging.Campaign("Campaign loaded: %s (title=%s, phases=%d, tasks=%d)",
		campaign.ID, campaign.Title, len(campaign.Phases), campaign.TotalTasks)

	// Recover monotonic journal sequence, ignoring corrupt tail records.
	o.recoverJournalSequence(campaign.ID)

	// Load campaign facts into kernel
	facts := campaign.ToFacts()
	logging.CampaignDebug("Loading %d facts into kernel", len(facts))
	if err := o.kernel.LoadFacts(facts); err != nil {
		return err
	}
	// Apply runtime config + budget
	o.assertCampaignConfigFacts()
	if o.contextPager != nil && o.campaign.ContextBudget > 0 {
		o.contextPager.SetBudget(o.campaign.ContextBudget)
	}
	return nil
}

// SetCampaign sets the campaign to execute.
func (o *Orchestrator) SetCampaign(campaign *Campaign) error {
	logging.Campaign("Setting campaign: %s (title=%s)", campaign.ID, campaign.Title)

	o.mu.Lock()
	defer o.mu.Unlock()

	o.campaign = campaign

	// Resume journal sequence for existing campaign IDs to avoid sequence reuse.
	o.recoverJournalSequence(campaign.ID)

	// Load campaign facts into kernel
	facts := campaign.ToFacts()
	logging.CampaignDebug("Loading %d campaign facts into kernel", len(facts))
	if err := o.kernel.LoadFacts(facts); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to load campaign facts: %v", err)
		return err
	}
	// Apply runtime config + budget
	o.assertCampaignConfigFacts()
	if o.contextPager != nil && campaign.ContextBudget > 0 {
		o.contextPager.SetBudget(campaign.ContextBudget)
	}

	// Save campaign to disk
	logging.CampaignDebug("Persisting campaign to disk")
	return o.saveCampaign()
}

// saveCampaign persists the campaign to disk.
func (o *Orchestrator) saveCampaign() error {
	if o.campaign == nil {
		return fmt.Errorf("no campaign loaded")
	}
	logging.CampaignDebug("Saving campaign to disk: %s", o.campaign.ID)
	campaignsDir := filepath.Join(o.nerdDir, "campaigns")
	if err := os.MkdirAll(campaignsDir, 0755); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to create campaigns directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(o.campaign, "", "  ")
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to marshal campaign JSON: %v", err)
		return err
	}
	snapshotChecksum := checksumBytes(data)

	// Event-before-ack: append journal entry first.
	if err := o.appendJournalEventLocked(
		"snapshot_write_requested",
		map[string]any{
			"status":          o.campaign.Status,
			"completed_tasks": o.campaign.CompletedTasks,
			"total_tasks":     o.campaign.TotalTasks,
		},
		snapshotChecksum,
	); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to append journal event: %v", err)
		return err
	}

	campaignPath := filepath.Join(campaignsDir, o.campaign.ID+".json")
	if err := o.writeCampaignSnapshotAtomic(campaignPath, data); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to write campaign file: %v", err)
		return err
	}

	if err := o.appendJournalEventLocked(
		"snapshot_write_committed",
		map[string]any{"path": campaignPath},
		snapshotChecksum,
	); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to append commit journal event: %v", err)
		return err
	}
	logging.CampaignDebug("Campaign saved successfully: %s (%d bytes)", campaignPath, len(data))
	return nil
}

// persistCampaign writes the campaign snapshot and makes a failed write visible.
//
// Nine call sites used to spell this `_ = o.saveCampaign()`. saveCampaign logs
// its own cause, but the caller dropped the outcome entirely, so nothing
// recorded WHICH checkpoint was lost and no operator-facing surface heard about
// it at all: a campaign kept running with completed phases, replans and
// autosaves that existed only in memory, and a crash then rolled it back to the
// last snapshot that happened to succeed. Persistence failure is not
// recoverable in place — the in-memory campaign is still correct — so this
// logs at Error and emits EventSnapshotWriteFailed, which is the only way the
// CLI and TUI can tell the operator their progress is not on disk.
//
// Callers hold o.mu, as saveCampaign requires; emitEvent takes no lock.
func (o *Orchestrator) persistCampaign(checkpoint string) {
	if err := o.saveCampaign(); err != nil {
		campaignID := ""
		if o.campaign != nil {
			campaignID = o.campaign.ID
		}
		logging.Get(logging.CategoryCampaign).Error(
			"Campaign snapshot at %q failed; campaign %s is running from memory only: %v",
			checkpoint, campaignID, err)
		o.emitEvent(EventSnapshotWriteFailed, "", "", checkpoint+": "+err.Error(), nil)
	}
}

// resetInProgress clears in-flight task/phase states after restarts so work can resume.
func (o *Orchestrator) resetInProgress() {
	logging.Campaign("Resetting in-progress states after restart")
	resetCount := 0

	tx := types.NewKernelTx(o.kernel)
	for pi := range o.campaign.Phases {
		phase := &o.campaign.Phases[pi]
		if phase.Status == PhaseInProgress {
			logging.CampaignDebug("Resetting phase %s from in_progress to pending", phase.ID)
			phase.Status = PhasePending
			resetCount++
		}
		for ti := range phase.Tasks {
			task := &phase.Tasks[ti]
			if task.Status == TaskInProgress {
				logging.CampaignDebug("Resetting task %s from in_progress to pending", task.ID)
				task.Status = TaskPending
				resetCount++
				// Update kernel fact for the task
				tx.RetractFact(core.Fact{
					Predicate: "campaign_task",
					Args:      []any{task.ID},
				})
				tx.Assert(core.Fact{
					Predicate: "campaign_task",
					Args:      []any{task.ID, task.PhaseID, task.Description, string(TaskPending), string(task.Type)},
				})
			}
		}
	}

	// The batch holds every campaign_task row this restart moved back to
	// pending. A dropped commit discards all of them at once, leaving the
	// kernel believing those tasks are still in flight while the in-memory
	// campaign has them pending: nothing schedules them and nothing completes
	// them, and the campaign stalls with no recorded cause.
	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategoryCampaign).Error(
			"Restart reset %d in-progress items in memory but the kernel batch was rejected; those tasks are still /in_progress to the logic layer: %v",
			resetCount, err)
	}
	logging.Campaign("Reset %d in-progress items", resetCount)
	o.persistCampaign("restart reset")
}
