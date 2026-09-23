package campaign

import "codenerd/internal/config"

// testCampaignConfig is the config's default campaign section with edit
// applied: tests state the knobs they depend on, and everything else is what a
// user who wrote nothing would run.
func testCampaignConfig(edit func(*config.CampaignConfig)) config.CampaignConfig {
	c := config.DefaultCampaignConfig()
	if edit != nil {
		edit(&c)
	}
	return c
}

// testPolicy resolves testCampaignConfig for orchestrators tests build as
// literals, which never pass through NewOrchestrator.
func testPolicy(edit func(*config.CampaignConfig)) config.CampaignPolicy {
	p, err := testCampaignConfig(edit).Resolve()
	if err != nil {
		panic(err)
	}
	return p
}

// fastRetries is a campaign whose task has attempts failed attempts before it
// fails and whose retries wait a millisecond.
func fastRetries(attempts int) func(*config.CampaignConfig) {
	return func(c *config.CampaignConfig) {
		c.MaxTaskAttempts = attempts
		c.RetryBackoffBase = "1ms"
		c.RetryBackoffMax = "1ms"
		c.RetryWithReasonBackoffMax = "1ms"
	}
}
