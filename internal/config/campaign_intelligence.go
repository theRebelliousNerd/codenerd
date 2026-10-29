package config

import (
	"fmt"
	"time"
)

// CampaignIntelligenceConfig is `campaign.intelligence` in .nerd/config.json:
// the pre-planning gather's knobs. Each timeout bounds one call (one system
// query, or one shard consult). Counts are how many of a system's results the
// report keeps. The switches choose which systems run.
//
// There is no gather-wide timeout. The old GatherTimeout of 5m covered every
// system at once, which is a clock on the phase; a system that was still
// inside its own call was cut off because the others had shared its budget.
// The gather ends because each call ends.
//
// Counts left at 0 and durations left empty take the default (every count
// here must be at least 1, so 0 cannot mean anything else). The switches are
// pointers because false is meaningful: every one of them defaults to on.
type CampaignIntelligenceConfig struct {
	// PerSystemTimeout bounds one system query. A workspace scan, a git log,
	// an MCP listing, a tool-gap detection, and each holographic PromptSection
	// take a context, so the deadline stops that call. Learning-store loads,
	// knowledge-graph queries, cold-storage loads, kernel queries, and
	// previous-campaign file reads do not: the deadline is checked between
	// those calls, and the call already in flight runs to completion.
	PerSystemTimeout string `json:"per_system_timeout,omitempty"`
	// ConsultTimeout bounds one shard-consult call.
	ConsultTimeout string `json:"consult_timeout,omitempty"`

	// MaxChurnHotspots is how many churn hotspots the git gather keeps.
	MaxChurnHotspots int `json:"max_churn_hotspots,omitempty"`
	// MaxLearnings is how many historical patterns the learning gather keeps.
	MaxLearnings int `json:"max_learnings,omitempty"`
	// MaxMCPTools is how many MCP tools the tool gather keeps.
	MaxMCPTools int `json:"max_mcp_tools,omitempty"`
	// MaxPreviousCampaigns is how many ended campaigns the history gather keeps.
	MaxPreviousCampaigns int `json:"max_previous_campaigns,omitempty"`
	// GitHistoryDepth is how many commits the one git log reads (`git log -n`).
	GitHistoryDepth int `json:"git_history_depth,omitempty"`

	EnableWorldModel        *bool `json:"enable_world_model,omitempty"`
	EnableGitHistory        *bool `json:"enable_git_history,omitempty"`
	EnableLearningStore     *bool `json:"enable_learning_store,omitempty"`
	EnableKnowledgeGraph    *bool `json:"enable_knowledge_graph,omitempty"`
	EnableColdStorage       *bool `json:"enable_cold_storage,omitempty"`
	EnableSafetyCheck       *bool `json:"enable_safety_check,omitempty"`
	EnableAutopoiesis       *bool `json:"enable_autopoiesis,omitempty"`
	EnableMCPTools          *bool `json:"enable_mcp_tools,omitempty"`
	EnablePreviousCampaigns *bool `json:"enable_previous_campaigns,omitempty"`
	EnableShardConsult      *bool `json:"enable_shard_consult,omitempty"`
	EnableTestCoverage      *bool `json:"enable_test_coverage,omitempty"`
	EnableCodePatterns      *bool `json:"enable_code_patterns,omitempty"`
}

// CampaignIntelligencePolicy is a CampaignIntelligenceConfig resolved for the
// gatherer: defaults filled, durations parsed, switches read.
type CampaignIntelligencePolicy struct {
	PerSystemTimeout time.Duration
	ConsultTimeout   time.Duration

	MaxChurnHotspots     int
	MaxLearnings         int
	MaxMCPTools          int
	MaxPreviousCampaigns int
	GitHistoryDepth      int

	EnableWorldModel        bool
	EnableGitHistory        bool
	EnableLearningStore     bool
	EnableKnowledgeGraph    bool
	EnableColdStorage       bool
	EnableSafetyCheck       bool
	EnableAutopoiesis       bool
	EnableMCPTools          bool
	EnablePreviousCampaigns bool
	EnableShardConsult      bool
	EnableTestCoverage      bool
	EnableCodePatterns      bool
}

func boolPtr(v bool) *bool { return &v }

// DefaultCampaignIntelligenceConfig is the intelligence block with every
// field written down. The values are the ones the gatherer carried as Go
// literals before they moved here (30s per system, 2m per consult, and the
// counts and switches below).
func DefaultCampaignIntelligenceConfig() CampaignIntelligenceConfig {
	return CampaignIntelligenceConfig{
		PerSystemTimeout:        "30s",
		ConsultTimeout:          "2m",
		MaxChurnHotspots:        50,
		MaxLearnings:            100,
		MaxMCPTools:             30,
		MaxPreviousCampaigns:    10,
		GitHistoryDepth:         100,
		EnableWorldModel:        boolPtr(true),
		EnableGitHistory:        boolPtr(true),
		EnableLearningStore:     boolPtr(true),
		EnableKnowledgeGraph:    boolPtr(true),
		EnableColdStorage:       boolPtr(true),
		EnableSafetyCheck:       boolPtr(true),
		EnableAutopoiesis:       boolPtr(true),
		EnableMCPTools:          boolPtr(true),
		EnablePreviousCampaigns: boolPtr(true),
		EnableShardConsult:      boolPtr(true),
		EnableTestCoverage:      boolPtr(true),
		EnableCodePatterns:      boolPtr(true),
	}
}

// WithDefaults fills every absent field from DefaultCampaignIntelligenceConfig.
// A nil switch is absent; false is kept.
func (c CampaignIntelligenceConfig) WithDefaults() CampaignIntelligenceConfig {
	d := DefaultCampaignIntelligenceConfig()
	if c.PerSystemTimeout == "" {
		c.PerSystemTimeout = d.PerSystemTimeout
	}
	if c.ConsultTimeout == "" {
		c.ConsultTimeout = d.ConsultTimeout
	}
	if c.MaxChurnHotspots == 0 {
		c.MaxChurnHotspots = d.MaxChurnHotspots
	}
	if c.MaxLearnings == 0 {
		c.MaxLearnings = d.MaxLearnings
	}
	if c.MaxMCPTools == 0 {
		c.MaxMCPTools = d.MaxMCPTools
	}
	if c.MaxPreviousCampaigns == 0 {
		c.MaxPreviousCampaigns = d.MaxPreviousCampaigns
	}
	if c.GitHistoryDepth == 0 {
		c.GitHistoryDepth = d.GitHistoryDepth
	}
	if c.EnableWorldModel == nil {
		c.EnableWorldModel = boolPtr(*d.EnableWorldModel)
	}
	if c.EnableGitHistory == nil {
		c.EnableGitHistory = boolPtr(*d.EnableGitHistory)
	}
	if c.EnableLearningStore == nil {
		c.EnableLearningStore = boolPtr(*d.EnableLearningStore)
	}
	if c.EnableKnowledgeGraph == nil {
		c.EnableKnowledgeGraph = boolPtr(*d.EnableKnowledgeGraph)
	}
	if c.EnableColdStorage == nil {
		c.EnableColdStorage = boolPtr(*d.EnableColdStorage)
	}
	if c.EnableSafetyCheck == nil {
		c.EnableSafetyCheck = boolPtr(*d.EnableSafetyCheck)
	}
	if c.EnableAutopoiesis == nil {
		c.EnableAutopoiesis = boolPtr(*d.EnableAutopoiesis)
	}
	if c.EnableMCPTools == nil {
		c.EnableMCPTools = boolPtr(*d.EnableMCPTools)
	}
	if c.EnablePreviousCampaigns == nil {
		c.EnablePreviousCampaigns = boolPtr(*d.EnablePreviousCampaigns)
	}
	if c.EnableShardConsult == nil {
		c.EnableShardConsult = boolPtr(*d.EnableShardConsult)
	}
	if c.EnableTestCoverage == nil {
		c.EnableTestCoverage = boolPtr(*d.EnableTestCoverage)
	}
	if c.EnableCodePatterns == nil {
		c.EnableCodePatterns = boolPtr(*d.EnableCodePatterns)
	}
	return c
}

// Resolve defaults, checks and parses the block.
func (c CampaignIntelligenceConfig) Resolve() (CampaignIntelligencePolicy, error) {
	c = c.WithDefaults()
	if problems := c.Check("campaign.intelligence"); len(problems) > 0 {
		msg := fmt.Sprintf("the campaign intelligence config has %d error(s):", len(problems))
		for _, p := range problems {
			msg += "\n  " + p.String()
		}
		return CampaignIntelligencePolicy{}, fmt.Errorf("%s", msg)
	}
	d := func(s string) time.Duration {
		v, _ := time.ParseDuration(s) // Check parsed every one of these
		return v
	}
	return CampaignIntelligencePolicy{
		PerSystemTimeout:        d(c.PerSystemTimeout),
		ConsultTimeout:          d(c.ConsultTimeout),
		MaxChurnHotspots:        c.MaxChurnHotspots,
		MaxLearnings:            c.MaxLearnings,
		MaxMCPTools:             c.MaxMCPTools,
		MaxPreviousCampaigns:    c.MaxPreviousCampaigns,
		GitHistoryDepth:         c.GitHistoryDepth,
		EnableWorldModel:        *c.EnableWorldModel,
		EnableGitHistory:        *c.EnableGitHistory,
		EnableLearningStore:     *c.EnableLearningStore,
		EnableKnowledgeGraph:    *c.EnableKnowledgeGraph,
		EnableColdStorage:       *c.EnableColdStorage,
		EnableSafetyCheck:       *c.EnableSafetyCheck,
		EnableAutopoiesis:       *c.EnableAutopoiesis,
		EnableMCPTools:          *c.EnableMCPTools,
		EnablePreviousCampaigns: *c.EnablePreviousCampaigns,
		EnableShardConsult:      *c.EnableShardConsult,
		EnableTestCoverage:      *c.EnableTestCoverage,
		EnableCodePatterns:      *c.EnableCodePatterns,
	}, nil
}

// Check reports the contradictions in an intelligence block, addressed under
// prefix ("campaign.intelligence"). Absent fields are defaulted first, so
// only what the file says can be wrong.
func (c CampaignIntelligenceConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	add := func(field, msg, fix string) {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + "." + field, Message: msg, Fix: fix})
	}
	for _, f := range []struct {
		name string
		v    int
	}{
		{"max_churn_hotspots", c.MaxChurnHotspots},
		{"max_learnings", c.MaxLearnings},
		{"max_mcp_tools", c.MaxMCPTools},
		{"max_previous_campaigns", c.MaxPreviousCampaigns},
		{"git_history_depth", c.GitHistoryDepth},
	} {
		if f.v < 1 {
			add(f.name, fmt.Sprintf("%d is below 1", f.v), "a count of at least 1, or remove the key for the default")
		}
	}
	for _, f := range []struct {
		name string
		v    string
	}{
		{"per_system_timeout", c.PerSystemTimeout},
		{"consult_timeout", c.ConsultTimeout},
	} {
		d, err := time.ParseDuration(f.v)
		switch {
		case err != nil:
			add(f.name, fmt.Sprintf("%q is not a duration: %v", f.v, err), `a Go duration such as "30s" or "2m"`)
		case d <= 0:
			add(f.name, fmt.Sprintf("%q is not positive", f.v), "a positive duration, or remove the key for the default")
		}
	}
	return out
}
