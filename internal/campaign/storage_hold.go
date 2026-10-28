package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// storageUncertainPredicate is the observation the policy holds dispatch on
// (campaign_storage_uncertain in schemas_campaign.mg).
const storageUncertainPredicate = "campaign_storage_uncertain"

// holdForStorage tells the kernel the last snapshot write failed. Whether that
// stops new work is the policy's decision (eligible_task); Go only reports the
// storage. The caller holds o.mu.
func (o *Orchestrator) holdForStorage(checkpoint string) {
	if o.kernel == nil || o.campaign == nil {
		return
	}
	id := o.campaign.ID
	if err := o.retractCampaignScopedFacts(storageUncertainPredicate, id); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("storage hold for %s: clearing the previous observation: %v", id, err)
	}
	fact := core.Fact{Predicate: storageUncertainPredicate, Args: []any{id, checkpoint}}
	o.storageHeld = true
	if err := o.kernel.Assert(fact); err != nil {
		logging.Get(logging.CategoryCampaign).Error(
			"campaign %s: storage is uncertain but the kernel could not be told (%v); new tasks are NOT held", id, err)
	}
}

// releaseStorageHold retracts the observation after a write that landed. It is
// a no-op when none stands. The caller holds o.mu.
func (o *Orchestrator) releaseStorageHold() {
	if !o.storageHeld || o.kernel == nil || o.campaign == nil {
		return
	}
	if err := o.retractCampaignScopedFacts(storageUncertainPredicate, o.campaign.ID); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("storage hold for %s: release failed (dispatch stays held): %v", o.campaign.ID, err)
		return
	}
	o.storageHeld = false
}
