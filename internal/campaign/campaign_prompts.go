package campaign

import (
	"context"

	"codenerd/internal/logging"
)

// =============================================================================
// CAMPAIGN PROMPT PROVIDER INTERFACE
// =============================================================================
// This file defines interfaces for prompt generation to avoid circular dependencies.
// The actual JIT integration will be provided by the articulation package.
//
// This allows campaign roles to use either JIT compilation (if available) or
// fall back to the static prompts defined in prompts.go.

// CampaignRole represents the different specialized roles in campaign orchestration.
type CampaignRole string

const (
	RoleLibrarian CampaignRole = "librarian"
	RoleExtractor CampaignRole = "extractor"
	RoleTaxonomy  CampaignRole = "taxonomy"
	RolePlanner   CampaignRole = "planner"
	RoleReplanner CampaignRole = "replanner"
	RoleAnalysis  CampaignRole = "analysis"
)

// PromptProvider is an interface for generating prompts for campaign roles.
// This abstraction allows the campaign package to use JIT-compiled prompts
// without directly depending on the articulation package (avoiding circular imports).
type PromptProvider interface {
	// GetPrompt returns a prompt for the specified campaign role.
	// Returns the prompt string and an error if prompt generation fails.
	GetPrompt(ctx context.Context, role CampaignRole, campaignID string) (string, error)
}

// StaticPromptProvider provides static fallback prompts.
type StaticPromptProvider struct{}

// NewStaticPromptProvider creates a provider that uses only static prompts.
func NewStaticPromptProvider() *StaticPromptProvider {
	return &StaticPromptProvider{}
}

// GetPrompt returns the static prompt for the specified campaign role.
func (spp *StaticPromptProvider) GetPrompt(
	ctx context.Context,
	role CampaignRole,
	campaignID string,
) (string, error) {
	logging.CampaignDebug("Using static prompt for role: %s", role)
	return getStaticPrompt(role), nil
}

// getStaticPrompt returns the static fallback prompt for a role.
func getStaticPrompt(role CampaignRole) string {
	switch role {
	case RoleLibrarian:
		return LibrarianLogic
	case RoleExtractor:
		return ExtractorLogic
	case RoleTaxonomy:
		return TaxonomyLogic
	case RolePlanner:
		return PlannerLogic
	case RoleReplanner:
		return ReplannerLogic
	case RoleAnalysis:
		return AnalysisLogic
	default:
		logging.Get(logging.CategoryCampaign).Warn("Unknown campaign role: %s, using generic prompt", role)
		return "You are a specialized campaign agent. Execute your task precisely and efficiently."
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// GetCampaignPhaseForRole returns the campaign phase string for a given role.
func GetCampaignPhaseForRole(role CampaignRole) string {
	switch role {
	case RoleLibrarian:
		return "/doc_classification"
	case RoleExtractor:
		return "/requirement_extraction"
	case RoleTaxonomy:
		return "/taxonomy_classification"
	case RolePlanner:
		return "/planning"
	case RoleReplanner:
		return "/replanning"
	case RoleAnalysis:
		return "/analysis"
	default:
		return "/active"
	}
}

// GetShardTypeForRole returns the shard type to use for JIT prompt compilation
// for a given campaign role. Some roles share the /planner or /analyzer shards.
func GetShardTypeForRole(role CampaignRole) string {
	switch role {
	case RoleLibrarian:
		return "librarian"
	case RoleExtractor:
		return "extractor"
	case RoleAnalysis:
		return "analyzer"
	case RolePlanner, RoleTaxonomy, RoleReplanner:
		return "planner"
	default:
		return "planner"
	}
}
