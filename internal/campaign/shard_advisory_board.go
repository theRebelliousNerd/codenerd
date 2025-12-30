// Package campaign provides multi-phase goal orchestration.
// This file implements the Shard Advisory Board - domain expert consultation
// for plan review and approval before campaign execution.
package campaign

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// SHARD ADVISORY BOARD
// =============================================================================
// The Shard Advisory Board allows domain experts (Coder, Tester, Reviewer, Researcher)
// to review and vote on proposed campaign plans before execution.
// This implements "preparation before execution" by getting expert sign-off.

// ShardAdvisoryBoard coordinates expert review of campaign plans.
type ShardAdvisoryBoard struct {
	consultation ConsultationProvider

	// Configuration
	config AdvisoryConfig
}

// AdvisoryConfig configures the advisory board behavior.
type AdvisoryConfig struct {
	// Timeout for individual consultations
	ConsultTimeout time.Duration

	// Approval thresholds
	MinApprovalRatio   float64 // Minimum ratio of approvals needed (0.0-1.0)
	MinConfidence      float64 // Minimum confidence to count vote
	RequireUnanimous   bool    // Require all advisors to approve
	RequireCriticalApproval bool // Require critical advisors (coder, tester) to approve

	// Enabled advisors
	EnabledAdvisors []string
}

// DefaultAdvisoryConfig returns sensible defaults.
func DefaultAdvisoryConfig() AdvisoryConfig {
	return AdvisoryConfig{
		ConsultTimeout:         2 * time.Minute,
		MinApprovalRatio:       0.5,
		MinConfidence:          0.5,
		RequireUnanimous:       false,
		RequireCriticalApproval: true,
		EnabledAdvisors: []string{
			"coder",
			"tester",
			"reviewer",
			"researcher",
		},
	}
}

// AdvisoryRequest contains the plan to be reviewed.
type AdvisoryRequest struct {
	CampaignID  string          `json:"campaign_id"`
	Goal        string          `json:"goal"`
	RawPlan     string          `json:"raw_plan"`
	Phases      []AdvisoryPhase `json:"phases"`
	TaskCount   int             `json:"task_count"`
	TargetPaths []string        `json:"target_paths"`

	// Context from intelligence gathering
	Intelligence *IntelligenceReport `json:"intelligence,omitempty"`
}

// AdvisoryPhase represents a simplified campaign phase for advisory review.
// This is separate from campaign.Phase to avoid circular dependencies and
// to provide only the information needed for advisory decisions.
type AdvisoryPhase struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TaskCount   int    `json:"task_count"`
}

// AdvisoryResponse contains an individual advisor's response.
type AdvisoryResponse struct {
	AdvisorName string    `json:"advisor_name"`
	Vote        VoteType  `json:"vote"`
	Confidence  float64   `json:"confidence"`
	Reasoning   string    `json:"reasoning"`
	Concerns    []string  `json:"concerns"`
	Suggestions []string  `json:"suggestions"`
	Caveats     []string  `json:"caveats"`
	Duration    time.Duration `json:"duration"`
}

// VoteType represents an advisor's vote.
type VoteType string

const (
	VoteApprove          VoteType = "approve"
	VoteApproveWithNotes VoteType = "approve_with_notes"
	VoteRequestChanges   VoteType = "request_changes"
	VoteReject           VoteType = "reject"
	VoteAbstain          VoteType = "abstain"
)

// AdvisorySynthesis aggregates all advisor responses into a final decision.
type AdvisorySynthesis struct {
	Approved       bool              `json:"approved"`
	Responses      []AdvisoryResponse `json:"responses"`
	ApprovalRatio  float64           `json:"approval_ratio"`
	OverallConfidence float64        `json:"overall_confidence"`

	// Aggregated feedback
	AllConcerns    []string `json:"all_concerns"`
	AllSuggestions []string `json:"all_suggestions"`
	AllCaveats     []string `json:"all_caveats"`

	// Blocking issues
	BlockingConcerns []BlockingConcern `json:"blocking_concerns"`

	// Summary for human review
	Summary        string `json:"summary"`
	Recommendation string `json:"recommendation"`
}

// BlockingConcern represents a concern that blocks approval.
type BlockingConcern struct {
	Advisor  string `json:"advisor"`
	Concern  string `json:"concern"`
	Severity string `json:"severity"`
}

// NewShardAdvisoryBoard creates a new advisory board.
func NewShardAdvisoryBoard(consultation ConsultationProvider) *ShardAdvisoryBoard {
	return &ShardAdvisoryBoard{
		consultation: consultation,
		config:       DefaultAdvisoryConfig(),
	}
}

// WithConfig sets custom configuration.
func (b *ShardAdvisoryBoard) WithConfig(config AdvisoryConfig) *ShardAdvisoryBoard {
	b.config = config
	return b
}

// ConsultAdvisors requests review from all configured domain experts.
func (b *ShardAdvisoryBoard) ConsultAdvisors(ctx context.Context, req AdvisoryRequest) ([]AdvisoryResponse, error) {
	logging.Campaign("Advisory board consultation started for campaign: %s", req.CampaignID)
	timer := logging.StartTimer(logging.CategoryCampaign, "ConsultAdvisors")
	defer timer.Stop()

	if b.consultation == nil {
		return nil, fmt.Errorf("consultation manager not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, b.config.ConsultTimeout*time.Duration(len(b.config.EnabledAdvisors)))
	defer cancel()

	// Build context for advisors
	consultContext := b.buildConsultationContext(req)
	question := b.buildConsultationQuestion(req)

	// Request batch consultation
	shardResponses, err := b.consultation.RequestBatchConsultation(
		ctx,
		BatchConsultRequest{
			Topic:      "Campaign Plan Advisory Review",
			Question:   question,
			Context:    consultContext,
			TargetSpec: b.config.EnabledAdvisors,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("batch consultation failed: %w", err)
	}

	// Parse responses into advisory format
	responses := make([]AdvisoryResponse, 0, len(shardResponses))
	for _, sr := range shardResponses {
		response := b.parseAdvisoryResponse(sr)
		responses = append(responses, response)
	}

	logging.Campaign("Advisory consultation complete: %d responses received", len(responses))
	return responses, nil
}

// SynthesizeVotes aggregates advisory responses into a final decision.
func (b *ShardAdvisoryBoard) SynthesizeVotes(responses []AdvisoryResponse) AdvisorySynthesis {
	logging.Campaign("Synthesizing %d advisory votes", len(responses))

	synthesis := AdvisorySynthesis{
		Responses:      responses,
		AllConcerns:    []string{},
		AllSuggestions: []string{},
		AllCaveats:     []string{},
		BlockingConcerns: []BlockingConcern{},
	}

	if len(responses) == 0 {
		synthesis.Approved = true // No advisors = auto-approve
		synthesis.Summary = "No advisors configured; plan auto-approved."
		synthesis.Recommendation = "Proceed with execution."
		return synthesis
	}

	// Count votes
	approvals := 0
	rejections := 0
	totalConfidence := 0.0
	validVotes := 0

	for _, resp := range responses {
		// Aggregate feedback
		synthesis.AllConcerns = append(synthesis.AllConcerns, resp.Concerns...)
		synthesis.AllSuggestions = append(synthesis.AllSuggestions, resp.Suggestions...)
		synthesis.AllCaveats = append(synthesis.AllCaveats, resp.Caveats...)

		// Only count votes with sufficient confidence
		if resp.Confidence < b.config.MinConfidence {
			logging.CampaignDebug("Skipping low-confidence vote from %s (%.0f%%)",
				resp.AdvisorName, resp.Confidence*100)
			continue
		}

		validVotes++
		totalConfidence += resp.Confidence

		switch resp.Vote {
		case VoteApprove, VoteApproveWithNotes:
			approvals++
		case VoteReject:
			rejections++
			// Rejections from critical advisors are blocking
			if b.isCriticalAdvisor(resp.AdvisorName) {
				synthesis.BlockingConcerns = append(synthesis.BlockingConcerns, BlockingConcern{
					Advisor:  resp.AdvisorName,
					Concern:  resp.Reasoning,
					Severity: "blocking",
				})
			}
		case VoteRequestChanges:
			// Request for changes from critical advisors is also blocking
			if b.isCriticalAdvisor(resp.AdvisorName) {
				synthesis.BlockingConcerns = append(synthesis.BlockingConcerns, BlockingConcern{
					Advisor:  resp.AdvisorName,
					Concern:  resp.Reasoning,
					Severity: "requires_changes",
				})
			}
		case VoteAbstain:
			// Don't count abstentions
			validVotes--
			totalConfidence -= resp.Confidence
		}
	}

	// Calculate ratios
	if validVotes > 0 {
		synthesis.ApprovalRatio = float64(approvals) / float64(validVotes)
		synthesis.OverallConfidence = totalConfidence / float64(validVotes)
	}

	// Determine approval
	synthesis.Approved = b.determineApproval(synthesis, approvals, rejections, validVotes)

	// Build summary
	synthesis.Summary = b.buildSummary(synthesis, approvals, rejections, validVotes)
	synthesis.Recommendation = b.buildRecommendation(synthesis)

	logging.Campaign("Advisory synthesis: approved=%v, ratio=%.0f%%, confidence=%.0f%%",
		synthesis.Approved, synthesis.ApprovalRatio*100, synthesis.OverallConfidence*100)

	return synthesis
}

// IncorporateFeedback modifies a plan based on advisory feedback.
func (b *ShardAdvisoryBoard) IncorporateFeedback(originalPlan string, synthesis AdvisorySynthesis) string {
	if len(synthesis.AllSuggestions) == 0 && len(synthesis.AllConcerns) == 0 {
		return originalPlan
	}

	var sb strings.Builder
	sb.WriteString(originalPlan)
	sb.WriteString("\n\n## Advisory Feedback Integration\n\n")

	if len(synthesis.AllConcerns) > 0 {
		sb.WriteString("### Addressed Concerns\n")
		for i, concern := range synthesis.AllConcerns {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("... and %d more concerns\n", len(synthesis.AllConcerns)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("- [ADDRESSED] %s\n", concern))
		}
		sb.WriteString("\n")
	}

	if len(synthesis.AllSuggestions) > 0 {
		sb.WriteString("### Incorporated Suggestions\n")
		for i, suggestion := range synthesis.AllSuggestions {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("... and %d more suggestions\n", len(synthesis.AllSuggestions)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("- [INCORPORATED] %s\n", suggestion))
		}
		sb.WriteString("\n")
	}

	if len(synthesis.AllCaveats) > 0 {
		sb.WriteString("### Caveats to Monitor\n")
		for _, caveat := range synthesis.AllCaveats {
			sb.WriteString(fmt.Sprintf("- ⚠️ %s\n", caveat))
		}
	}

	return sb.String()
}

// =============================================================================
// HELPER METHODS
// =============================================================================

func (b *ShardAdvisoryBoard) buildConsultationContext(req AdvisoryRequest) string {
	var sb strings.Builder

	sb.WriteString("## Campaign Plan Review Request\n\n")
	sb.WriteString(fmt.Sprintf("**Campaign ID:** %s\n", req.CampaignID))
	sb.WriteString(fmt.Sprintf("**Goal:** %s\n", req.Goal))
	sb.WriteString(fmt.Sprintf("**Total Tasks:** %d\n", req.TaskCount))
	sb.WriteString(fmt.Sprintf("**Phases:** %d\n\n", len(req.Phases)))

	if len(req.Phases) > 0 {
		sb.WriteString("### Proposed Phases\n")
		for i, phase := range req.Phases {
			sb.WriteString(fmt.Sprintf("%d. **%s**: %s (%d tasks)\n",
				i+1, phase.Name, phase.Description, phase.TaskCount))
		}
		sb.WriteString("\n")
	}

	if len(req.TargetPaths) > 0 {
		sb.WriteString("### Target Files\n")
		for i, path := range req.TargetPaths {
			if i >= 20 {
				sb.WriteString(fmt.Sprintf("... and %d more files\n", len(req.TargetPaths)-20))
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s`\n", path))
		}
		sb.WriteString("\n")
	}

	if req.Intelligence != nil {
		sb.WriteString("### Intelligence Summary\n")
		if len(req.Intelligence.HighChurnFiles) > 0 {
			sb.WriteString(fmt.Sprintf("- **High Churn Files:** %d (Chesterton's Fence applies)\n",
				len(req.Intelligence.HighChurnFiles)))
		}
		if len(req.Intelligence.SafetyWarnings) > 0 {
			sb.WriteString(fmt.Sprintf("- **Safety Warnings:** %d\n", len(req.Intelligence.SafetyWarnings)))
		}
		if len(req.Intelligence.ToolGaps) > 0 {
			sb.WriteString(fmt.Sprintf("- **Tool Gaps Detected:** %d\n", len(req.Intelligence.ToolGaps)))
		}
		sb.WriteString("\n")
	}

	if req.RawPlan != "" {
		sb.WriteString("### Raw Plan\n```\n")
		// Truncate if too long
		plan := req.RawPlan
		if len(plan) > 3000 {
			plan = plan[:3000] + "\n... (truncated)"
		}
		sb.WriteString(plan)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}

func (b *ShardAdvisoryBoard) buildConsultationQuestion(req AdvisoryRequest) string {
	return fmt.Sprintf(`As a domain expert, review this campaign plan and provide your assessment.

Consider:
1. Is the plan complete and well-structured?
2. Are there any risks or edge cases not addressed?
3. From your domain expertise, what concerns do you have?
4. What suggestions would improve this plan?

Respond with:
VOTE: [APPROVE / APPROVE_WITH_NOTES / REQUEST_CHANGES / REJECT / ABSTAIN]
CONFIDENCE: [0-100]

REASONING:
[Your main assessment]

CONCERNS:
- [List any concerns, one per line]

SUGGESTIONS:
- [List any suggestions, one per line]

CAVEATS:
- [List any caveats or things to watch, one per line]

Campaign Goal: %s`, req.Goal)
}

func (b *ShardAdvisoryBoard) parseAdvisoryResponse(sr ConsultationResponse) AdvisoryResponse {
	resp := AdvisoryResponse{
		AdvisorName: sr.FromSpec,
		Confidence:  sr.Confidence,
		Reasoning:   sr.Advice,
		Caveats:     sr.Caveats,
		Duration:    sr.Duration,
	}

	// Parse vote from advice text
	adviceLower := strings.ToLower(sr.Advice)
	switch {
	case strings.Contains(adviceLower, "reject"):
		resp.Vote = VoteReject
	case strings.Contains(adviceLower, "request_changes") || strings.Contains(adviceLower, "request changes"):
		resp.Vote = VoteRequestChanges
	case strings.Contains(adviceLower, "approve_with_notes") || strings.Contains(adviceLower, "approve with notes"):
		resp.Vote = VoteApproveWithNotes
	case strings.Contains(adviceLower, "abstain"):
		resp.Vote = VoteAbstain
	case strings.Contains(adviceLower, "approve"):
		resp.Vote = VoteApprove
	default:
		// Default to approve with notes if confidence is high
		if sr.Confidence > 0.7 {
			resp.Vote = VoteApprove
		} else if sr.Confidence > 0.4 {
			resp.Vote = VoteApproveWithNotes
		} else {
			resp.Vote = VoteAbstain
		}
	}

	// Parse structured sections
	lines := strings.Split(sr.Advice, "\n")
	currentSection := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lowerLine := strings.ToLower(trimmed)

		switch {
		case strings.HasPrefix(lowerLine, "concern"):
			currentSection = "concerns"
			continue
		case strings.HasPrefix(lowerLine, "suggestion"):
			currentSection = "suggestions"
			continue
		case strings.HasPrefix(lowerLine, "caveat"):
			currentSection = "caveats"
			continue
		case strings.HasPrefix(lowerLine, "reasoning"):
			currentSection = "reasoning"
			continue
		}

		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
			item := strings.TrimPrefix(strings.TrimPrefix(trimmed, "-"), "*")
			item = strings.TrimSpace(item)
			if item != "" {
				switch currentSection {
				case "concerns":
					resp.Concerns = append(resp.Concerns, item)
				case "suggestions":
					resp.Suggestions = append(resp.Suggestions, item)
				case "caveats":
					resp.Caveats = append(resp.Caveats, item)
				}
			}
		}
	}

	return resp
}

func (b *ShardAdvisoryBoard) isCriticalAdvisor(name string) bool {
	criticalAdvisors := map[string]bool{
		"coder":  true,
		"tester": true,
	}
	return criticalAdvisors[strings.ToLower(name)]
}

func (b *ShardAdvisoryBoard) determineApproval(synthesis AdvisorySynthesis, approvals, rejections, validVotes int) bool {
	// Check for blocking concerns first
	if len(synthesis.BlockingConcerns) > 0 && b.config.RequireCriticalApproval {
		return false
	}

	// Unanimous approval required?
	if b.config.RequireUnanimous && rejections > 0 {
		return false
	}

	// Check approval ratio
	if validVotes == 0 {
		return true // No valid votes = auto-approve
	}

	return synthesis.ApprovalRatio >= b.config.MinApprovalRatio
}

func (b *ShardAdvisoryBoard) buildSummary(synthesis AdvisorySynthesis, approvals, rejections, validVotes int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**Votes:** %d approve, %d reject, %d valid votes total\n",
		approvals, rejections, validVotes))
	sb.WriteString(fmt.Sprintf("**Approval Ratio:** %.0f%%\n", synthesis.ApprovalRatio*100))
	sb.WriteString(fmt.Sprintf("**Overall Confidence:** %.0f%%\n", synthesis.OverallConfidence*100))

	if len(synthesis.BlockingConcerns) > 0 {
		sb.WriteString("\n**Blocking Concerns:**\n")
		for _, bc := range synthesis.BlockingConcerns {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", bc.Severity, bc.Advisor, bc.Concern))
		}
	}

	return sb.String()
}

func (b *ShardAdvisoryBoard) buildRecommendation(synthesis AdvisorySynthesis) string {
	if synthesis.Approved {
		if len(synthesis.AllCaveats) > 0 {
			return "PROCEED WITH CAUTION: Plan approved with caveats. Monitor flagged concerns during execution."
		}
		if len(synthesis.AllSuggestions) > 0 {
			return "PROCEED: Plan approved. Consider incorporating suggestions for improved execution."
		}
		return "PROCEED: Plan approved by advisory board."
	}

	if len(synthesis.BlockingConcerns) > 0 {
		return "REVISE REQUIRED: Critical concerns must be addressed before proceeding."
	}

	return "NOT RECOMMENDED: Plan did not receive sufficient approval. Consider revision."
}

// =============================================================================
// FORMATTING FOR LLM CONTEXT
// =============================================================================

// FormatForContext formats the synthesis for LLM context injection.
func (s *AdvisorySynthesis) FormatForContext() string {
	var sb strings.Builder

	sb.WriteString("# ADVISORY BOARD REVIEW\n\n")
	sb.WriteString(s.Summary)
	sb.WriteString("\n")

	if s.Approved {
		sb.WriteString("✅ **APPROVED**\n\n")
	} else {
		sb.WriteString("❌ **NOT APPROVED**\n\n")
	}

	sb.WriteString(fmt.Sprintf("**Recommendation:** %s\n\n", s.Recommendation))

	// Individual responses
	if len(s.Responses) > 0 {
		sb.WriteString("## Advisor Responses\n\n")
		for _, resp := range s.Responses {
			sb.WriteString(fmt.Sprintf("### %s\n", resp.AdvisorName))
			sb.WriteString(fmt.Sprintf("- **Vote:** %s\n", resp.Vote))
			sb.WriteString(fmt.Sprintf("- **Confidence:** %.0f%%\n", resp.Confidence*100))
			if resp.Reasoning != "" {
				reasoning := resp.Reasoning
				if len(reasoning) > 300 {
					reasoning = reasoning[:300] + "..."
				}
				sb.WriteString(fmt.Sprintf("- **Reasoning:** %s\n", reasoning))
			}
			sb.WriteString("\n")
		}
	}

	// Aggregated concerns
	if len(s.AllConcerns) > 0 {
		sb.WriteString("## All Concerns\n")
		seen := make(map[string]bool)
		for _, c := range s.AllConcerns {
			if !seen[c] {
				sb.WriteString(fmt.Sprintf("- %s\n", c))
				seen[c] = true
			}
		}
		sb.WriteString("\n")
	}

	// Aggregated suggestions
	if len(s.AllSuggestions) > 0 {
		sb.WriteString("## Suggestions\n")
		seen := make(map[string]bool)
		for _, s := range s.AllSuggestions {
			if !seen[s] {
				sb.WriteString(fmt.Sprintf("- %s\n", s))
				seen[s] = true
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatVoteSummary returns a compact vote summary.
func (s *AdvisorySynthesis) FormatVoteSummary() string {
	votes := make(map[VoteType]int)
	for _, resp := range s.Responses {
		votes[resp.Vote]++
	}

	parts := []string{}
	for vote, count := range votes {
		parts = append(parts, fmt.Sprintf("%s=%d", vote, count))
	}

	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
