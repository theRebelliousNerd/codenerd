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
	RoleAssault   CampaignRole = "assault"
)

// PromptProvider is an interface for generating prompts for campaign roles.
// This abstraction allows the campaign package to use JIT-compiled prompts
// without directly depending on the articulation package (avoiding circular imports).
type PromptProvider interface {
	// GetPrompt returns a prompt for the specified campaign role.
	// Returns the prompt string and an error if prompt generation fails.
	GetPrompt(ctx context.Context, role CampaignRole, campaignID string) (string, error)
}

// CampaignRoleAtomFamily maps a campaign role to the prompt-atom family that
// owns its content under internal/prompt/atoms/campaign/.
//
// Every role's real prompt lives in that corpus, where it is versioned,
// budgeted and JIT-selected against the campaign's actual facts. prompts.go
// holds a frozen copy for the case where no JIT compiler could be built at all.
// The map is what keeps "which atoms serve this role" answerable from code
// rather than by grepping YAML, and it is what
// TestCampaignRoles_HaveAtomCoverage checks the corpus against.
func CampaignRoleAtomFamily(role CampaignRole) string {
	switch role {
	case RoleLibrarian:
		return "campaign/librarian"
	case RoleExtractor:
		return "campaign/extractor"
	case RoleTaxonomy:
		return "campaign/taxonomy"
	case RolePlanner:
		return "campaign/planner"
	case RoleReplanner:
		return "campaign/replanning"
	case RoleAnalysis:
		return "campaign/analysis"
	case RoleAssault:
		return "campaign/assault"
	default:
		return ""
	}
}

// AllCampaignRoles returns every role the provider serves.
func AllCampaignRoles() []CampaignRole {
	return []CampaignRole{
		RoleLibrarian, RoleExtractor, RoleTaxonomy,
		RolePlanner, RoleReplanner, RoleAnalysis, RoleAssault,
	}
}

// StaticPromptProvider is the LAST-RESORT fallback, not a peer of the JIT path.
//
// It returns a frozen ~1000-line prompt from prompts.go with no awareness of
// the campaign it is serving: no phase, no goal, no kernel facts, no token
// budget. Production callers wire CampaignJITProvider, which assembles the
// atoms under internal/prompt/atoms/campaign/ against the live campaign. This
// exists so a boot that cannot build a JIT compiler still plans something
// instead of failing — and it says so, because a campaign silently planned from
// a frozen prompt looks identical to one planned properly and is measurably
// worse.
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
	if family := CampaignRoleAtomFamily(role); family != "" {
		logging.Get(logging.CategoryCampaign).Warn(
			"Campaign role %s served from the STATIC fallback prompt; the JIT provider was not wired, "+
				"so the %s atoms were not assembled and this plan sees none of the campaign's facts",
			role, family)
	}
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
	case RoleAssault:
		return AssaultLogic
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
	case RoleAssault:
		return "/assault"
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
	case RoleAssault:
		return "analyzer"
	case RolePlanner, RoleTaxonomy, RoleReplanner:
		return "planner"
	default:
		return "planner"
	}
}
