package campaign

import (
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

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

// policyKernel is the shipped kernel holding the campaign policy's thresholds
// as NewOrchestrator publishes them, for tests of a decision the kernel
// derives without a whole orchestrator around it.
func policyKernel(t *testing.T, edit func(*config.CampaignConfig)) core.Kernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	if err := k.LoadFacts(config.ParamFacts(testPolicy(edit).Params())); err != nil {
		t.Fatalf("publish the campaign policy: %v", err)
	}
	return k
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
