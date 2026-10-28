package campaign

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// =============================================================================
// CAMPAIGN PROMPT PROVIDER
// =============================================================================
// Every model call the campaign package makes is a campaign role. Its system
// prompt is compiled by the PromptProvider (jit_prompt_provider.go) from the
// role's atoms under internal/prompt/atoms/campaign/; the user message carries
// only data (the goal, the documents, the plan, the tasks). There is no
// embedded fallback text: without a provider, or when the compile fails, the
// call reports types.ErrRolePromptUnavailable.

// CampaignRole names one model call the campaign package makes.
type CampaignRole string

const (
	// RoleLibrarian classifies a source document into an architectural layer.
	RoleLibrarian CampaignRole = "librarian"
	// RoleExtractor extracts the goal's requirements from retrieved snippets.
	RoleExtractor CampaignRole = "extractor"
	// RolePlanner decomposes the goal into the campaign plan.
	RolePlanner CampaignRole = "planner"
	// RolePlanRetry re-asks for the plan after a reply that was not one.
	RolePlanRetry CampaignRole = "plan_retry"
	// RolePlanRevision fixes a plan's validation issues.
	RolePlanRevision CampaignRole = "plan_revision"
	// RoleReplanner refines the next phase after a phase completes.
	RoleReplanner CampaignRole = "replanner"
	// RoleRequirementReplan folds a requirement added mid-campaign into the plan.
	RoleRequirementReplan CampaignRole = "requirement_replan"
	// RoleFailureReplan proposes fixes for failed and blocked tasks.
	RoleFailureReplan CampaignRole = "failure_replan"
	// RoleAssault turns assault results into a remediation plan.
	RoleAssault CampaignRole = "assault"
	// RolePhaseSummary compresses a completed phase into a short summary.
	RolePhaseSummary CampaignRole = "phase_summary"
	// RoleDocumentWriter writes a deliverable document directly.
	RoleDocumentWriter CampaignRole = "document_writer"
)

// campaignRoleSpec is how one role compiles: the shard type and campaign phase
// the compile carries, and the atom family that owns the role's content.
//
// Every shard type here is one prompt.IsStructuredOutputOnly names, because
// every role's reply is parsed (or written) by Go as it comes back: a
// Piggyback envelope around it is a second, competing output contract. The
// phase is a regime dimension (jit_compiler.mg), so each role's atoms declare
// campaign_phases with exactly this value and are live only in its compile.
type campaignRoleSpec struct {
	shardType string
	phase     string
	family    string
}

var campaignRoleSpecs = map[CampaignRole]campaignRoleSpec{
	RoleLibrarian:         {shardType: "librarian", phase: "/doc_classification", family: "campaign/librarian"},
	RoleExtractor:         {shardType: "extractor", phase: "/requirement_extraction", family: "campaign/extractor"},
	RolePlanner:           {shardType: "planner", phase: "/planning", family: "campaign/planner"},
	RolePlanRetry:         {shardType: "planner", phase: "/plan_retry", family: "campaign/plan_retry"},
	RolePlanRevision:      {shardType: "planner", phase: "/plan_revision", family: "campaign/plan_revision"},
	RoleReplanner:         {shardType: "planner", phase: "/replanning", family: "campaign/replanning"},
	RoleRequirementReplan: {shardType: "planner", phase: "/requirement_replan", family: "campaign/requirement_replan"},
	RoleFailureReplan:     {shardType: "planner", phase: "/failure_replan", family: "campaign/failure_replan"},
	RoleAssault:           {shardType: "analyzer", phase: "/assault", family: "campaign/assault"},
	RolePhaseSummary:      {shardType: "analyzer", phase: "/phase_summary", family: "campaign/phase_summary"},
	RoleDocumentWriter:    {shardType: "analyzer", phase: "/document_writing", family: "campaign/document_writer"},
}

// campaignPromptIntent is the intent verb every campaign role compiles under.
// No atom declares it, so the worker intent atoms (intent/*/core, the
// refactoring discipline) that a compile with no intent admits stay out of a
// campaign role's prompt.
const campaignPromptIntent = "/campaign"

// CampaignRoles lists every campaign role, sorted.
func CampaignRoles() []CampaignRole {
	roles := make([]CampaignRole, 0, len(campaignRoleSpecs))
	for role := range campaignRoleSpecs {
		roles = append(roles, role)
	}
	slices.Sort(roles)
	return roles
}

// CampaignRoleAtomFamily is the prompt-atom family that owns role's content
// under internal/prompt/atoms/campaign/, or "" for an unknown role.
func CampaignRoleAtomFamily(role CampaignRole) string {
	return campaignRoleSpecs[role].family
}

// GetCampaignPhaseForRole is the campaign phase role's prompt compiles under,
// or "" for an unknown role.
func GetCampaignPhaseForRole(role CampaignRole) string {
	return campaignRoleSpecs[role].phase
}

// GetShardTypeForRole is the shard type role's prompt compiles under, or ""
// for an unknown role.
func GetShardTypeForRole(role CampaignRole) string {
	return campaignRoleSpecs[role].shardType
}

// PromptProvider compiles the system prompts of the campaign roles.
type PromptProvider interface {
	// GetPrompt returns the prompt for role, compiled for servedBy: the client
	// that will consume it, whose provider and model select the model-pinned
	// prompt atoms. Every caller passes the client it is about to send the
	// prompt to, so the pins match the model that actually runs them.
	GetPrompt(ctx context.Context, role CampaignRole, campaignID string, servedBy perception.LLMClient) (string, error)
}

// rolePrompt is role's compiled system prompt. No provider, a failed compile
// and an empty prompt are all types.ErrRolePromptUnavailable: the caller
// reports it, because there is no other text to send.
func rolePrompt(ctx context.Context, provider PromptProvider, role CampaignRole, campaignID string, servedBy perception.LLMClient) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("%w: campaign role %s: no prompt provider is wired", types.ErrRolePromptUnavailable, role)
	}
	prompt, err := provider.GetPrompt(ctx, role, campaignID, servedBy)
	if err != nil {
		return "", fmt.Errorf("%w: campaign role %s: %w", types.ErrRolePromptUnavailable, role, err)
	}
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("%w: campaign role %s compiled empty", types.ErrRolePromptUnavailable, role)
	}
	return prompt, nil
}
