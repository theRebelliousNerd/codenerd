// Package campaign provides multi-phase goal orchestration.
// This file implements the Shard Advisory Board - domain expert consultation
// for plan review and approval before campaign execution.
package campaign

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/consultation"
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
	MinApprovalRatio        float64 // Minimum ratio of approvals needed (0.0-1.0)
	MinConfidence           float64 // Minimum confidence to count vote
	RequireUnanimous        bool    // Require all advisors to approve
	RequireCriticalApproval bool    // Require critical advisors (coder, tester) to approve

	// Enabled advisors
	EnabledAdvisors []string
}

// DefaultAdvisoryConfig returns sensible defaults.
func DefaultAdvisoryConfig() AdvisoryConfig {
	return AdvisoryConfig{
		ConsultTimeout:          2 * time.Minute,
		MinApprovalRatio:        0.5,
		MinConfidence:           0.5,
		RequireUnanimous:        false,
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
	Phases      []AdvisoryPhase `json:"phases"`
	TaskCount   int             `json:"task_count"`
	TargetPaths []string        `json:"target_paths"`

	// Manifest is the bounded task/dependency/write-set graph the advisors
	// review. It replaced RawPlan, which only ever carried the campaign title,
	// so "the board reviewed the plan" was a claim about a title.
	Manifest PlanManifest `json:"manifest"`

	// Context from intelligence gathering
	Intelligence *IntelligenceReport `json:"intelligence,omitzero"`
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

// AdvisoryResponse contains an individual advisor's response. An advisor whose
// consultation failed is still represented, with VoteUnavailable and Error set,
// so a missing review is never indistinguishable from a review never asked for.
type AdvisoryResponse struct {
	AdvisorName string        `json:"advisor_name"`
	Vote        VoteType      `json:"vote"`
	Confidence  float64       `json:"confidence"`
	Reasoning   string        `json:"reasoning"`
	Concerns    []string      `json:"concerns"`
	Suggestions []string      `json:"suggestions"`
	Caveats     []string      `json:"caveats"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitzero"`
}

// VoteType represents an advisor's vote.
type VoteType string

const (
	VoteApprove          VoteType = "approve"
	VoteApproveWithNotes VoteType = "approve_with_notes"
	VoteRequestChanges   VoteType = "request_changes"
	VoteReject           VoteType = "reject"
	VoteAbstain          VoteType = "abstain"
	// VoteUnknown: the advisor answered but named no single valid VOTE.
	VoteUnknown VoteType = "unknown"
	// VoteUnavailable: the advisor's consultation failed or was cancelled.
	VoteUnavailable VoteType = "unavailable"
)

// MissingReview is a required advisor whose vote the board does not have.
type MissingReview struct {
	Advisor string `json:"advisor"`
	Reason  string `json:"reason"`
}

// AdvisorySynthesis aggregates all advisor responses into a final decision.
type AdvisorySynthesis struct {
	Approved          bool               `json:"approved"`
	Responses         []AdvisoryResponse `json:"responses"`
	ApprovalRatio     float64            `json:"approval_ratio"`
	OverallConfidence float64            `json:"overall_confidence"`

	// Incomplete is set when any advisor's vote is missing (failed, cancelled,
	// or no valid VOTE). An incomplete panel is never approved; its successful
	// rejections are still reported as themselves.
	Incomplete bool            `json:"incomplete"`
	Missing    []MissingReview `json:"missing,omitzero"`

	// Coverage is what part of the plan manifest the panel was shown.
	Coverage AdvisoryCoverage `json:"coverage"`

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

// advisoryVoteContract is the versioned reply grammar the board parses. It is
// sent as the consultation's response contract, replacing the generic ADVICE
// format, so the specialist is not given two conflicting answer formats.
const advisoryVoteContract = `Response contract: advisory-vote/v1

Answer with exactly one VOTE line, using one value from the list:
VOTE: APPROVE | APPROVE_WITH_NOTES | REQUEST_CHANGES | REJECT | ABSTAIN
CONFIDENCE: [0-100]

REASONING:
[Your main assessment]

CONCERNS:
- [List any concerns, one per line]

SUGGESTIONS:
- [List any suggestions, one per line]

CAVEATS:
- [List any caveats or things to watch, one per line]`

// ConsultAdvisors requests review from all configured domain experts. It
// returns one AdvisoryResponse per enabled advisor: answered advisors with
// their parsed vote, failed ones as VoteUnavailable with the failure. The
// error is reserved for a request that could not be made at all.
func (b *ShardAdvisoryBoard) ConsultAdvisors(ctx context.Context, req AdvisoryRequest) ([]AdvisoryResponse, error) {
	logging.Campaign("Advisory board consultation started for campaign: %s", req.CampaignID)
	timer := logging.StartTimer(logging.CategoryCampaign, "ConsultAdvisors")
	defer timer.Stop()

	if b.consultation == nil {
		return nil, fmt.Errorf("consultation manager not configured")
	}

	// Validate critical request fields before incurring LLM cost.
	if strings.TrimSpace(req.CampaignID) == "" {
		return nil, fmt.Errorf("advisory request requires non-empty CampaignID")
	}
	if strings.TrimSpace(req.Goal) == "" {
		return nil, fmt.Errorf("advisory request requires non-empty Goal")
	}

	ctx, cancel := context.WithTimeout(ctx, b.config.ConsultTimeout*time.Duration(len(b.config.EnabledAdvisors)))
	defer cancel()

	batch := b.consultation.RequestBatchConsultation(ctx, consultation.BatchRequest{
		Topic:            "Campaign Plan Advisory Review",
		Question:         b.buildConsultationQuestion(req),
		Context:          b.buildConsultationContext(req),
		Specialists:      b.config.EnabledAdvisors,
		ResponseContract: advisoryVoteContract,
	})
	if batch.RequestErr != nil {
		return nil, fmt.Errorf("batch consultation failed: %w", batch.RequestErr)
	}

	responses := make([]AdvisoryResponse, 0, len(batch.Entries))
	for _, e := range batch.Entries {
		if e.Status == consultation.EntrySucceeded && e.Response != nil {
			responses = append(responses, b.parseAdvisoryResponse(*e.Response))
			continue
		}
		reason := string(e.Status)
		if e.Err != nil {
			reason = e.Err.Error()
		}
		responses = append(responses, AdvisoryResponse{
			AdvisorName: e.Specialist,
			Vote:        VoteUnavailable,
			Error:       reason,
		})
	}

	logging.Campaign("Advisory consultation complete: %d of %d advisors answered",
		len(batch.Succeeded()), len(batch.Entries))
	return responses, nil
}

// SynthesizeVotes aggregates advisory responses into a final decision.
func (b *ShardAdvisoryBoard) SynthesizeVotes(responses []AdvisoryResponse) AdvisorySynthesis {
	logging.Campaign("Synthesizing %d advisory votes", len(responses))

	synthesis := AdvisorySynthesis{
		Responses:        responses,
		AllConcerns:      []string{},
		AllSuggestions:   []string{},
		AllCaveats:       []string{},
		BlockingConcerns: []BlockingConcern{},
	}

	if len(responses) == 0 {
		// Fail-closed: zero advisor responses must not auto-approve, since the
		// LLM integration could have silently failed. Treat absence of votes as
		// a safety blocker and require explicit review.
		synthesis.Approved = false
		synthesis.Summary = "No advisor responses received; plan not approved (fail-closed)."
		synthesis.Recommendation = "NOT RECOMMENDED: No advisory responses received. Verify consultation pipeline before proceeding."
		return synthesis
	}

	// Count votes
	approvals := 0
	rejections := 0
	totalConfidence := 0.0
	validVotes := 0

	for _, resp := range responses {
		// A missing vote is a missing review, whatever confidence the reply
		// claimed: it is neither an approval nor an abstention.
		switch resp.Vote {
		case VoteUnavailable:
			synthesis.Missing = append(synthesis.Missing, MissingReview{Advisor: resp.AdvisorName, Reason: resp.Error})
			continue
		case VoteUnknown:
			synthesis.Missing = append(synthesis.Missing, MissingReview{Advisor: resp.AdvisorName, Reason: "no single valid VOTE in the reply"})
		}

		// Aggregate feedback
		synthesis.AllConcerns = append(synthesis.AllConcerns, resp.Concerns...)
		synthesis.AllSuggestions = append(synthesis.AllSuggestions, resp.Suggestions...)
		synthesis.AllCaveats = append(synthesis.AllCaveats, resp.Caveats...)
		if resp.Vote == VoteUnknown {
			continue
		}

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
	synthesis.Incomplete = len(synthesis.Missing) > 0

	// Calculate ratios
	if validVotes > 0 {
		synthesis.ApprovalRatio = float64(approvals) / float64(validVotes)
		synthesis.OverallConfidence = totalConfidence / float64(validVotes)
	}

	// Determine approval. An incomplete panel never approves: the missing
	// advisor might have rejected.
	synthesis.Approved = !synthesis.Incomplete && b.determineApproval(synthesis, approvals, rejections, validVotes)

	// Build summary
	synthesis.Summary = b.buildSummary(synthesis, approvals, rejections, validVotes)
	synthesis.Recommendation = b.buildRecommendation(synthesis)

	logging.Campaign("Advisory synthesis: approved=%v, incomplete=%v, ratio=%.0f%%, confidence=%.0f%%",
		synthesis.Approved, synthesis.Incomplete, synthesis.ApprovalRatio*100, synthesis.OverallConfidence*100)

	return synthesis
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
			if i >= 50 {
				sb.WriteString(fmt.Sprintf("... and %d more phases\n", len(req.Phases)-50))
				break
			}
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

	req.Manifest.render(&sb)
	return sb.String()
}

func (b *ShardAdvisoryBoard) buildConsultationQuestion(req AdvisoryRequest) string {
	return fmt.Sprintf(`As a domain expert, review this campaign plan and provide your assessment.

Consider:
1. Is the plan complete and well-structured?
2. Are the task dependencies and write sets in the manifest sound?
3. Are there any risks or edge cases not addressed?
4. From your domain expertise, what concerns do you have?
5. What suggestions would improve this plan?

Campaign Goal: %s`, req.Goal)
}

// parseAdvisoryResponse reads the advisory-vote/v1 contract from the
// specialist's raw reply. The vote comes from the VOTE field alone: scanning
// the whole text for "reject" read an approval that explained a rejected
// alternative as a rejection, and a reply with no vote used to become an
// approval on its confidence alone.
func (b *ShardAdvisoryBoard) parseAdvisoryResponse(sr consultation.Response) AdvisoryResponse {
	raw := sr.Raw
	if raw == "" {
		raw = sr.Advice
	}
	resp := AdvisoryResponse{
		AdvisorName: sr.Specialist,
		Vote:        parseAdvisoryVote(raw),
		Confidence:  clampConfidence(sr.Confidence),
		Duration:    sr.ComputeDuration,
	}

	var reasoning []string
	currentSection := ""
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if section, rest, ok := advisorySectionHeader(trimmed); ok {
			currentSection = section
			if section == "reasoning" && rest != "" {
				reasoning = append(reasoning, rest)
			}
			continue
		}

		if currentSection == "reasoning" {
			if trimmed != "" {
				reasoning = append(reasoning, trimmed)
			}
			continue
		}
		item, ok := listItem(trimmed)
		if !ok {
			continue
		}
		switch currentSection {
		case "concerns":
			resp.Concerns = append(resp.Concerns, item)
		case "suggestions":
			resp.Suggestions = append(resp.Suggestions, item)
		case "caveats":
			resp.Caveats = append(resp.Caveats, item)
		}
	}
	resp.Reasoning = strings.Join(reasoning, "\n")
	if resp.Reasoning == "" {
		resp.Reasoning = strings.TrimSpace(raw)
	}
	return resp
}

// advisorySectionHeader recognises a contract field line ("REASONING:",
// "**Concerns:**") and returns its section and any text after the colon.
// VOTE and CONFIDENCE close the current section; they are parsed elsewhere.
func advisorySectionHeader(line string) (section, rest string, ok bool) {
	t := strings.ReplaceAll(strings.TrimLeft(line, "#> "), "**", "")
	key, after, found := strings.Cut(t, ":")
	if !found {
		return "", "", false
	}
	rest = strings.TrimSpace(strings.Trim(after, "*_ "))
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "vote", "confidence":
		return "", "", true
	case "reasoning":
		return "reasoning", rest, true
	case "concern", "concerns":
		return "concerns", "", true
	case "suggestion", "suggestions":
		return "suggestions", "", true
	case "caveat", "caveats":
		return "caveats", "", true
	}
	return "", "", false
}

// listItem reads a "- x", "* x" or "1. x" line.
func listItem(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "-"), strings.HasPrefix(line, "*"):
		item := strings.TrimSpace(strings.TrimLeft(line, "-* "))
		return item, item != ""
	}
	if i := strings.IndexAny(line, ".)"); i > 0 && i <= 3 {
		digits := line[:i]
		if strings.Trim(digits, "0123456789") == "" {
			item := strings.TrimSpace(line[i+1:])
			return item, item != ""
		}
	}
	return "", false
}

// parseAdvisoryVote returns the one vote the reply's VOTE field names. Every
// VOTE line must agree on one exact enum value; none, an invalid value, or two
// different values is VoteUnknown, which never counts as approval.
func parseAdvisoryVote(raw string) VoteType {
	var found VoteType
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		t = strings.TrimLeft(t, "-*>#• ")
		t = strings.ReplaceAll(t, "**", "")
		t = strings.ReplaceAll(t, "__", "")
		key, value, ok := strings.Cut(t, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "vote") {
			continue
		}
		vote := voteFromToken(value)
		if vote == VoteUnknown {
			return VoteUnknown
		}
		if found != "" && found != vote {
			return VoteUnknown
		}
		found = vote
	}
	if found == "" {
		return VoteUnknown
	}
	return found
}

func voteFromToken(value string) VoteType {
	v := strings.ToUpper(strings.TrimSpace(value))
	v = strings.Trim(v, "[]`*_.!\"' ")
	v = strings.Join(strings.Fields(v), "_")
	switch v {
	case "APPROVE":
		return VoteApprove
	case "APPROVE_WITH_NOTES":
		return VoteApproveWithNotes
	case "REQUEST_CHANGES":
		return VoteRequestChanges
	case "REJECT":
		return VoteReject
	case "ABSTAIN":
		return VoteAbstain
	}
	return VoteUnknown
}

// clampConfidence constrains a confidence score to the valid [0.0, 1.0] range.
// NaN values are normalized to 0.0 (treated as no confidence).
func clampConfidence(c float64) float64 {
	if c != c { // NaN check
		return 0.0
	}
	if c < 0.0 {
		return 0.0
	}
	if c > 1.0 {
		return 1.0
	}
	return c
}

func (b *ShardAdvisoryBoard) isCriticalAdvisor(name string) bool {
	criticalAdvisors := map[string]bool{
		"coder":  true,
		"tester": true,
	}
	return criticalAdvisors[strings.ToLower(name)]
}

func (b *ShardAdvisoryBoard) determineApproval(synthesis AdvisorySynthesis, approvals, rejections, validVotes int) bool {
	// Log vote details for transparency
	logging.CampaignDebug("Advisory votes: %d approve, %d reject, %d valid", approvals, rejections, validVotes)

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
		// Fail-closed: if no votes met the confidence threshold, do not approve.
		// A silent LLM failure or unanimously low-confidence panel must not
		// pass the plan through.
		return false
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
	if len(synthesis.Missing) > 0 {
		sb.WriteString("\n**Missing Reviews (panel incomplete):**\n")
		for _, m := range synthesis.Missing {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", m.Advisor, m.Reason))
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
	if synthesis.Incomplete {
		return "NOT RECOMMENDED: The advisory panel is incomplete; missing reviews must be obtained before approval."
	}

	return "NOT RECOMMENDED: Plan did not receive sufficient approval. Consider revision."
}
