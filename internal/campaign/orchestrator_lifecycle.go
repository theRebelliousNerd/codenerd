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

// SetCampaign makes a NEW campaign this orchestrator's to execute and publishes
// its first generation. It takes the campaign's cross-process ownership lock.
//
// A campaign that already has a durable journal is not new: setting it would
// overwrite the recorded generation with whatever this caller holds in memory
// before anything compared the two. Persisted campaigns resume through
// ResumeCampaign, which admits a generation first.
func (o *Orchestrator) SetCampaign(campaign *Campaign) error {
	logging.Campaign("Setting campaign: %s (title=%s)", campaign.ID, campaign.Title)

	o.mu.Lock()
	defer o.mu.Unlock()

	if err := o.acquireOwnershipLocked(campaign.ID); err != nil {
		return err
	}
	journal := o.journalPath(campaign.ID)
	if _, err := os.Stat(journal); err == nil {
		return fmt.Errorf("campaign %s already has a durable journal (%s); resume it so its recorded generation is admitted instead of overwritten", campaign.ID, journal)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("campaign %s: check for an existing journal: %w", campaign.ID, err)
	}
	o.journalSeq.Store(0)
	o.journalUncertain = false

	o.campaign = campaign

	// Load campaign facts into kernel
	facts := campaign.ToFacts()
	logging.CampaignDebug("Loading %d campaign facts into kernel", len(facts))
	if err := checkCampaignSnapshot(o.kernel, campaign, facts); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Campaign plan rejected: %v", err)
		return err
	}
	if err := o.kernel.LoadFacts(facts); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to load campaign facts: %v", err)
		return err
	}
	// Apply runtime config + budget
	o.publishPolicyParams()
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
		o.holdForStorage(checkpoint)
		return
	}
	o.releaseStorageHold()
}

// resetInProgress clears in-flight task/phase states after restarts so work can resume.
// The caller holds o.mu. Kernel facts are committed before the live campaign is
// changed, so a rejected transaction leaves the prior state available for a
// later retry and cannot let the scheduler run against mismatched facts.
func (o *Orchestrator) resetInProgress() error {
	if o.campaign == nil {
		return fmt.Errorf("restart reset: no campaign loaded")
	}
	logging.Campaign("Resetting in-progress states after restart")
	resetCount := 0

	tx := types.NewKernelTx(o.kernel)
	campaignID := o.campaign.ID
	for pi := range o.campaign.Phases {
		phase := &o.campaign.Phases[pi]
		if phase.Status == PhaseInProgress {
			logging.CampaignDebug("Resetting phase %s from in_progress to pending", phase.ID)
			tx.RetractFact(core.Fact{
				Predicate: "campaign_phase",
				Args:      []any{phase.ID},
			})
			tx.Assert(core.Fact{
				Predicate: "campaign_phase",
				Args: []any{
					phase.ID,
					campaignID,
					phase.Name,
					phase.Order,
					string(PhasePending),
					phase.ContextProfile,
				},
			})
			resetCount++
		}
		for ti := range phase.Tasks {
			task := &phase.Tasks[ti]
			if task.Status == TaskInProgress {
				logging.CampaignDebug("Resetting task %s from in_progress to pending", task.ID)
				tx.RetractFact(core.Fact{
					Predicate: "campaign_task",
					Args:      []any{task.ID},
				})
				tx.Assert(core.Fact{
					Predicate: "campaign_task",
					Args:      []any{task.ID, task.PhaseID, task.Description, string(TaskPending), string(task.Type)},
				})
				resetCount++
			}
		}
	}

	// A dropped commit leaves every row untouched. Do not update the campaign
	// or write a snapshot unless the complete phase/task reset landed.
	if resetCount > 0 {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("restart reset of %d in-progress items was rejected by the kernel: %w", resetCount, err)
		}
		for pi := range o.campaign.Phases {
			phase := &o.campaign.Phases[pi]
			if phase.Status == PhaseInProgress {
				phase.Status = PhasePending
			}
			for ti := range phase.Tasks {
				if phase.Tasks[ti].Status == TaskInProgress {
					phase.Tasks[ti].Status = TaskPending
				}
			}
		}
	}
	logging.Campaign("Reset %d in-progress items", resetCount)
	o.persistCampaign("restart reset")
	return nil
}
