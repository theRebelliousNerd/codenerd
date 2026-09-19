package campaign

import (
	"testing"
	"time"
)

// A campaign has no clock of its own. Until 2026-09-19 a zero CampaignTimeout
// became 4 hours and a zero TaskTimeout 30 minutes unless the caller set
// DisableTimeouts, which every long-horizon caller had to remember to do. The
// limits are now a caller's own, and zero means none.
func TestOrchestratorDefaults_NoTimeoutUnlessTheCallerSetsOne(t *testing.T) {
	var unset OrchestratorConfig
	applyOrchestratorDefaults(&unset)
	if unset.CampaignTimeout != 0 || unset.TaskTimeout != 0 {
		t.Fatalf("defaults gave the campaign %s and each task %s; a campaign runs while it makes progress",
			unset.CampaignTimeout, unset.TaskTimeout)
	}

	set := OrchestratorConfig{CampaignTimeout: 6 * time.Hour, TaskTimeout: 90 * time.Minute}
	applyOrchestratorDefaults(&set)
	if set.CampaignTimeout != 6*time.Hour || set.TaskTimeout != 90*time.Minute {
		t.Fatalf("a caller's own limits were changed: campaign %s, task %s", set.CampaignTimeout, set.TaskTimeout)
	}
}
