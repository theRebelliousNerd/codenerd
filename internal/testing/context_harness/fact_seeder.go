package context_harness

import (
	"fmt"

	"codenerd/internal/core"
)

// FactSeeder seeds facts into a kernel based on scenario configuration.
// Used to set up initial state for integration scenarios.
type FactSeeder struct {
	kernel *core.RealKernel
}

// NewFactSeeder creates a new fact seeder.
func NewFactSeeder(kernel *core.RealKernel) *FactSeeder {
	return &FactSeeder{kernel: kernel}
}

// SeedScenario seeds all initial facts from a scenario.
func (fs *FactSeeder) SeedScenario(scenario *Scenario) error {
	if len(scenario.InitialFacts) == 0 {
		return nil
	}

	facts := make([]core.Fact, 0, len(scenario.InitialFacts))
	for _, factStr := range scenario.InitialFacts {
		fact, err := parseMangleFact(factStr)
		if err != nil {
			return fmt.Errorf("failed to parse fact %q: %w", factStr, err)
		}
		facts = append(facts, fact)
	}

	return fs.kernel.LoadFacts(facts)
}

// SeedCampaignContext seeds campaign-related facts.
func (fs *FactSeeder) SeedCampaignContext(campaignID, currentPhase string, phaseNum int, goals []string) error {
	facts := []core.Fact{
		{
			Predicate: "current_campaign",
			Args:      []interface{}{campaignID},
		},
		{
			Predicate: "campaign_phase",
			Args:      []interface{}{campaignID, currentPhase, phaseNum},
		},
	}

	for _, goal := range goals {
		facts = append(facts, core.Fact{
			Predicate: "phase_objective",
			Args:      []interface{}{currentPhase, goal},
		})
	}

	return fs.kernel.LoadFacts(facts)
}

// SeedIssueContext seeds issue-related facts for SWE-bench style scenarios.
func (fs *FactSeeder) SeedIssueContext(issueID, issueText string, mentionedFiles []string, errorTypes []string) error {
	facts := []core.Fact{
		{
			Predicate: "active_issue",
			Args:      []interface{}{issueID},
		},
		{
			Predicate: "issue_description",
			Args:      []interface{}{issueID, issueText},
		},
	}

	// Add mentioned files as tier 1
	for _, file := range mentionedFiles {
		facts = append(facts, core.Fact{
			Predicate: "issue_mentioned_file",
			Args:      []interface{}{issueID, file, 1}, // tier 1
		})
	}

	// Add error types
	for _, errType := range errorTypes {
		facts = append(facts, core.Fact{
			Predicate: "issue_error_type",
			Args:      []interface{}{issueID, errType},
		})
	}

	return fs.kernel.LoadFacts(facts)
}

// SeedSymbolGraph seeds symbol relationships for dependency spreading tests.
func (fs *FactSeeder) SeedSymbolGraph(edges map[string][]string) error {
	var facts []core.Fact

	for caller, callees := range edges {
		for _, callee := range callees {
			facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args:      []interface{}{caller, callee, "calls"},
			})
		}
	}

	return fs.kernel.LoadFacts(facts)
}

// SeedDependencyLinks seeds file-level dependencies.
func (fs *FactSeeder) SeedDependencyLinks(edges map[string][]string) error {
	var facts []core.Fact

	for from, tos := range edges {
		for _, to := range tos {
			facts = append(facts, core.Fact{
				Predicate: "dependency_link",
				Args:      []interface{}{from, to, "imports"},
			})
		}
	}

	return fs.kernel.LoadFacts(facts)
}

// SeedProjectPatterns seeds learned project patterns.
func (fs *FactSeeder) SeedProjectPatterns(patterns map[string]string) error {
	var facts []core.Fact

	for pattern, value := range patterns {
		facts = append(facts, core.Fact{
			Predicate: "project_pattern",
			Args:      []interface{}{pattern, value},
		})
	}

	return fs.kernel.LoadFacts(facts)
}

// SeedFileTopology seeds file structure facts.
func (fs *FactSeeder) SeedFileTopology(files []string) error {
	var facts []core.Fact

	for _, file := range files {
		facts = append(facts, core.Fact{
			Predicate: "file_topology",
			Args:      []interface{}{file, "exists", true},
		})
	}

	return fs.kernel.LoadFacts(facts)
}

// Clear removes all seeded facts (for test isolation).
func (fs *FactSeeder) Clear() error {
	// In production, this would retract all facts
	// For now, scenarios should use fresh kernels
	return nil
}
