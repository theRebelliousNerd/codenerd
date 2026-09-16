package campaign

import "testing"

// Control-plane calls must be safe before any campaign is loaded: chat
// keypresses reach Pause/Resume directly, and a nil campaign must be a
// no-op, not a crash.
func TestOrchestratorControl_WithoutCampaign(t *testing.T) {
	// A bare orchestrator has no campaign, no channels, no cancel func:
	// exactly the state a stray control call must survive.
	o := &Orchestrator{}
	o.Pause()
	o.Resume()
	o.Stop()
	if got := o.GetProgress(); got.CampaignID != "" {
		t.Errorf("progress without campaign = %+v, want zero", got)
	}
}
