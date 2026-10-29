package config

import "testing"

// The campaign section's contradictions refuse the file like every other
// section's: UserConfig.Check reaches CampaignConfig.Check.
func TestCheck_ACampaignContradictionRefusesTheFile(t *testing.T) {
	cfg := &UserConfig{Campaign: &CampaignConfig{MaxTaskAttempts: -1}}
	for _, p := range cfg.Check(nil) {
		if p.Path == "campaign.max_task_attempts" && p.Severity == SeverityError {
			return
		}
	}
	t.Fatalf("campaign.max_task_attempts = -1 was not reported as an error: %+v", cfg.Check(nil))
}

// The defaults nerd config full prints include the campaign section, so the
// user sees every knob the campaign executive reads.
func TestDefaultUserConfig_ShowsTheCampaignSection(t *testing.T) {
	if DefaultUserConfig().Campaign == nil {
		t.Fatal("DefaultUserConfig has no campaign section")
	}
}
