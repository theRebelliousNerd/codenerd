package campaign

import "codenerd/internal/config"

// IntelligenceConfigFromPolicy maps a resolved campaign section onto the
// gatherer. The literals live in config.DefaultCampaignIntelligenceConfig;
// this is a copy, not a second set of defaults.
func IntelligenceConfigFromPolicy(p config.CampaignPolicy) IntelligenceConfig {
	i := p.Intelligence
	return IntelligenceConfig{
		PerSystemTimeout:        i.PerSystemTimeout,
		ConsultTimeout:          i.ConsultTimeout,
		MaxChurnHotspots:        i.MaxChurnHotspots,
		MaxLearnings:            i.MaxLearnings,
		MaxMCPTools:             i.MaxMCPTools,
		MaxPreviousCampaigns:    i.MaxPreviousCampaigns,
		GitHistoryDepth:         i.GitHistoryDepth,
		EnableWorldModel:        i.EnableWorldModel,
		EnableGitHistory:        i.EnableGitHistory,
		EnableLearningStore:     i.EnableLearningStore,
		EnableKnowledgeGraph:    i.EnableKnowledgeGraph,
		EnableColdStorage:       i.EnableColdStorage,
		EnableSafetyCheck:       i.EnableSafetyCheck,
		EnableAutopoiesis:       i.EnableAutopoiesis,
		EnableMCPTools:          i.EnableMCPTools,
		EnablePreviousCampaigns: i.EnablePreviousCampaigns,
		EnableShardConsult:      i.EnableShardConsult,
		EnableTestCoverage:      i.EnableTestCoverage,
		EnableCodePatterns:      i.EnableCodePatterns,
	}
}

// IntelligenceConfigFrom resolves a campaign section and maps it. Resolve
// failing is a programmer error at the construction sites: they pass a
// section LoadUserConfig already accepted, or the defaults. NewOrchestrator
// stamps the same mapping from the policy it has already resolved, and
// returns that failure instead of panicking.
func IntelligenceConfigFrom(section config.CampaignConfig) IntelligenceConfig {
	policy, err := section.Resolve()
	if err != nil {
		panic("campaign: intelligence config: " + err.Error())
	}
	return IntelligenceConfigFromPolicy(policy)
}

// DefaultIntelligenceConfig is the gatherer config for a campaign section
// the user did not write. It is the mapping of the config package's
// defaults, not a second list of literals.
func DefaultIntelligenceConfig() IntelligenceConfig {
	return IntelligenceConfigFrom(config.DefaultCampaignConfig())
}
