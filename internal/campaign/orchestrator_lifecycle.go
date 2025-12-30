package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
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

	campaignPath := filepath.Join(campaignsDir, o.campaign.ID+".json")
	if err := os.WriteFile(campaignPath, data, 0644); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to write campaign file: %v", err)
		return err
	}
	logging.CampaignDebug("Campaign saved successfully: %s (%d bytes)", campaignPath, len(data))
	return nil
}

// resetInProgress clears in-flight task/phase states after restarts so work can resume.
func (o *Orchestrator) resetInProgress() {
	logging.Campaign("Resetting in-progress states after restart")
	resetCount := 0

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
				_ = o.kernel.RetractFact(core.Fact{
					Predicate: "campaign_task",
					Args:      []interface{}{task.ID},
				})
				_ = o.kernel.Assert(core.Fact{
					Predicate: "campaign_task",
					Args:      []interface{}{task.ID, task.PhaseID, task.Description, string(TaskPending), string(task.Type)},
				})
			}
		}
	}

	logging.Campaign("Reset %d in-progress items", resetCount)
	_ = o.saveCampaign()
}
