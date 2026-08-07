package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/tools/research"
	"codenerd/internal/types"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ShardLister provides shard discovery for campaign planning.
type ShardLister interface {
	ListAvailableShards() []types.ShardInfo
}

// Decomposer creates campaign plans through LLM + Mangle collaboration.
// It parses messy specifications and user goals into structured, validated plans.
type Decomposer struct {
	kernel         core.Kernel
	llmClient      perception.LLMClient
	workspace      string
	promptProvider PromptProvider // Optional JIT prompt provider
	shardLister    ShardLister    // Optional shard discovery for shard-aware planning

	// Intelligence integration (Step 0)
	intelligence     *IntelligenceGatherer // Pre-planning intelligence from 12 systems
	advisoryBoard    *ShardAdvisoryBoard   // Domain expert consultation (Step 4b)
	edgeCaseDetector *EdgeCaseDetector     // File action decisions
	toolPregenerator *ToolPregenerator     // Tool pre-generation (Step 9)

	// Gemini advanced features (nil if not Gemini or features unavailable)
	grounding *research.GroundingHelper // Google Search / URL Context grounding
	thinking  *research.ThinkingHelper  // Thinking mode metadata capture

	// Cached intelligence report for current decomposition
	lastIntelligence *IntelligenceReport
}

// NewDecomposer creates a new decomposer.
func NewDecomposer(kernel core.Kernel, llmClient perception.LLMClient, workspace string) *Decomposer {
	logging.CampaignDebug("Creating new Decomposer for workspace: %s", workspace)

	d := &Decomposer{
		kernel:         kernel,
		llmClient:      llmClient,
		workspace:      workspace,
		promptProvider: NewStaticPromptProvider(), // Default to static prompts
	}

	// Initialize Gemini advanced features helpers
	if llmClient != nil {
		d.grounding = research.NewGroundingHelper(llmClient)
		d.thinking = research.NewThinkingHelper(llmClient)

		// Enable Google Search grounding for research-intensive planning
		if d.grounding.IsGroundingAvailable() {
			d.grounding.EnableGoogleSearch()
			logging.CampaignDebug("Gemini grounding enabled for campaign planning (Google Search active)")
		}
		if d.thinking.IsThinkingAvailable() {
			logging.CampaignDebug("Gemini thinking mode active for campaign planning (level=%s)", d.thinking.GetThinkingLevel())
		}
	}

	return d
}

// SetPromptProvider sets the PromptProvider for JIT-compiled prompts.
// This allows using JIT-compiled prompts from the articulation package.
// If not set, static prompts will be used.
func (d *Decomposer) SetPromptProvider(provider PromptProvider) {
	if provider == nil {
		d.promptProvider = NewStaticPromptProvider()
		return
	}

	d.promptProvider = provider
	logging.CampaignDebug("Decomposer configured with custom prompt provider")
}

// SetShardLister sets the shard discovery interface for shard-aware planning.
// When set, the decomposer can inform the LLM about available shards.
func (d *Decomposer) SetShardLister(lister ShardLister) {
	d.shardLister = lister
	if lister != nil {
		logging.CampaignDebug("Decomposer configured with shard discovery")
	}
}

// SetIntelligenceGatherer sets the intelligence gatherer for pre-planning intelligence.
// When set, the decomposer will gather intelligence from 12 systems before planning.
func (d *Decomposer) SetIntelligenceGatherer(gatherer *IntelligenceGatherer) {
	d.intelligence = gatherer
	if gatherer != nil {
		logging.CampaignDebug("Decomposer configured with intelligence gathering")
	}
}

// SetAdvisoryBoard sets the shard advisory board for plan review.
// When set, domain experts will review plans before execution.
func (d *Decomposer) SetAdvisoryBoard(board *ShardAdvisoryBoard) {
	d.advisoryBoard = board
	if board != nil {
		logging.CampaignDebug("Decomposer configured with advisory board")
	}
}

// SetEdgeCaseDetector sets the edge case detector for file action decisions.
// When set, the decomposer will analyze files to determine create/extend/modularize actions.
func (d *Decomposer) SetEdgeCaseDetector(detector *EdgeCaseDetector) {
	d.edgeCaseDetector = detector
	if detector != nil {
		logging.CampaignDebug("Decomposer configured with edge case detection")
	}
}

// SetToolPregenerator sets the tool pregenerator for pre-execution tool generation.
// When set, the decomposer will generate missing tools before campaign execution.
func (d *Decomposer) SetToolPregenerator(pregenerator *ToolPregenerator) {
	d.toolPregenerator = pregenerator
	if pregenerator != nil {
		logging.CampaignDebug("Decomposer configured with tool pre-generation")
	}
}

// GetLastIntelligence returns the intelligence report from the last decomposition.
func (d *Decomposer) GetLastIntelligence() *IntelligenceReport {
	return d.lastIntelligence
}

// =============================================================================
// GEMINI ADVANCED FEATURES
// =============================================================================

// IsGroundingAvailable returns true if Gemini grounding features are available.
func (d *Decomposer) IsGroundingAvailable() bool {
	return d.grounding != nil && d.grounding.IsGroundingAvailable()
}

// IsThinkingAvailable returns true if Gemini thinking mode is available.
func (d *Decomposer) IsThinkingAvailable() bool {
	return d.thinking != nil && d.thinking.IsThinkingAvailable()
}

// EnableURLContext enables URL Context grounding with documentation URLs.
// Useful for campaign planning that references specific documentation.
func (d *Decomposer) EnableURLContext(urls []string) {
	if d.grounding != nil && d.grounding.IsGroundingAvailable() {
		d.grounding.EnableURLContext(urls)
		logging.CampaignDebug("URL Context enabled for decomposer with %d URLs", len(urls))
	}
}

// DisableURLContext disables URL Context grounding.
func (d *Decomposer) DisableURLContext() {
	if d.grounding != nil {
		d.grounding.DisableURLContext()
	}
}

// GetGroundingStats returns statistics about grounding usage during decomposition.
func (d *Decomposer) GetGroundingStats() *research.GroundingStats {
	if d.grounding == nil {
		return nil
	}
	stats := d.grounding.GetStats()
	return &stats
}

// GetThinkingStats returns statistics about thinking mode usage during decomposition.
func (d *Decomposer) GetThinkingStats() *research.ThinkingStats {
	if d.thinking == nil {
		return nil
	}
	stats := d.thinking.GetStats()
	return &stats
}

// completeWithGrounding performs an LLM completion with grounding if available.
// Falls back to standard Complete if grounding is not available.
func (d *Decomposer) completeWithGrounding(ctx context.Context, prompt string) (string, error) {
	if d.grounding != nil && d.grounding.IsGroundingAvailable() {
		response, sources, err := d.grounding.CompleteWithGrounding(ctx, prompt)
		if err != nil {
			return "", err
		}
		if len(sources) > 0 {
			logging.CampaignDebug("Decomposer LLM call grounded with %d sources", len(sources))
		}
		// Capture thinking metadata after grounded call
		if d.thinking != nil {
			d.thinking.CaptureThinkingMetadata()
		}
		return response, nil
	}
	// Fall back to standard completion
	if d.llmClient == nil {
		return "", fmt.Errorf("%w: decomposer requires llm client", ErrNilDependency)
	}
	return d.llmClient.Complete(ctx, prompt)
}

// DecomposeRequest represents a request to create a campaign.
type DecomposeRequest struct {
	Goal          string       // High-level goal description
	SourcePaths   []string     // Paths to spec docs, requirements, etc.
	CampaignType  CampaignType // Type of campaign
	UserHints     []string     // Optional user guidance
	MaxPhases     int          // Max phases (0 = unlimited)
	ContextBudget int          // Token budget (0 = default 100k)
}

// DecomposeResult represents the result of decomposition.
type DecomposeResult struct {
	Campaign     *Campaign
	ValidationOK bool
	Issues       []PlanValidationIssue
	SourceDocs   []SourceDocument
	Requirements []Requirement
}

// DocClassification holds the LLM's judgement of a file.
type DocClassification struct {
	Layer      string  `json:"layer"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

const (
	maxCampaignClassificationBytes  = 1 << 20
	maxCampaignKnowledgeIngestBytes = 5 << 20
)

// Decompose creates a campaign plan through LLM + Mangle collaboration.
func (d *Decomposer) Decompose(ctx context.Context, req DecomposeRequest) (*DecomposeResult, error) {
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Goal == "" {
		return nil, ErrEmptyGoal
	}
	if d.kernel == nil {
		return nil, ErrNilKernel
	}
	if d.llmClient == nil {
		return nil, fmt.Errorf("%w: decomposer requires llm client", ErrNilDependency)
	}
	if req.ContextBudget < 0 {
		return nil, fmt.Errorf("%w: context budget must be non-negative", ErrInvalidConfig)
	}
	// Validate SourcePaths: reject empty/whitespace-only entries to prevent
	// accidentally ingesting the workspace root or current working directory.
	for i, p := range req.SourcePaths {
		if strings.TrimSpace(p) == "" {
			return nil, fmt.Errorf("%w: SourcePaths[%d] is empty or whitespace-only", ErrInvalidConfig, i)
		}
	}

	timer := logging.StartTimer(logging.CategoryCampaign, "Decompose")
	defer timer.StopWithInfo()

	logging.Campaign("=== Starting campaign decomposition ===")
	logging.Campaign("Goal: %s", req.Goal[:min(200, len(req.Goal))])
	logging.CampaignDebug("Campaign type: %s, source paths: %d, context budget: %d",
		req.CampaignType, len(req.SourcePaths), req.ContextBudget)

	// Generate campaign ID
	campaignID := fmt.Sprintf("/campaign_%s", uuid.New().String()[:8])
	safeCampaignID := sanitizeCampaignID(campaignID)
	logging.Campaign("Generated campaign ID: %s", campaignID)

	// Set defaults - if not provided, caller should pass from config.ContextWindow.MaxTokens
	// 200k is a reasonable default if config wasn't passed through
	if req.ContextBudget == 0 {
		req.ContextBudget = 200000 // 200k tokens default - caller should override from config
	}

	kbPath := filepath.Join(d.workspace, ".nerd", "campaigns", safeCampaignID, "knowledge.db")

	// Step 0: Intelligence Gathering (NEW - from 12 systems)
	if d.intelligence != nil {
		logging.Campaign("Step 0: Gathering intelligence from all systems")
		intelTimer := logging.StartTimer(logging.CategoryCampaign, "gatherIntelligence")
		intel, intelErr := d.intelligence.Gather(ctx, req.Goal, req.SourcePaths)
		intelTimer.Stop()
		if intelErr != nil {
			logging.Get(logging.CategoryCampaign).Warn("Intelligence gathering failed (non-fatal): %v", intelErr)
		} else {
			d.lastIntelligence = intel
			logging.Campaign("Intelligence gathered: %d world facts, %d churn hotspots, %d learnings, %d MCP tools",
				len(intel.WorldFacts), len(intel.GitChurnHotspots),
				len(intel.HistoricalPatterns), len(intel.MCPToolsAvailable))

			// Seed intelligence facts into kernel
			d.seedIntelligenceFacts(campaignID, intel)
		}
	}

	// Step 1: Ingest source documents
	logging.Campaign("Step 1: Ingesting source documents")
	ingestTimer := logging.StartTimer(logging.CategoryCampaign, "ingestSourceDocuments")
	sourceDocs, fileMeta, err := d.ingestSourceDocuments(ctx, campaignID, req.SourcePaths)
	ingestTimer.Stop()
	if err != nil {
		// Anchor errors against campaign + source-path list so triage can
		// locate the offending document without scanning the whole run log.
		logging.Get(logging.CategoryCampaign).Error(
			"Source document ingestion failed campaign=%s paths=%v: %v",
			campaignID, req.SourcePaths, err)
		return nil, fmt.Errorf("failed to ingest source documents: %w", err)
	}
	logging.Campaign("Ingested %d source documents, %d file metadata entries", len(sourceDocs), len(fileMeta))

	// Seed metadata + goal signals for Mangle-driven selection
	logging.CampaignDebug("Seeding document facts into kernel")
	d.seedDocFacts(campaignID, req.Goal, fileMeta)

	// Step 1b: Ingest into campaign knowledge store (vectors + graph) for retrieval
	logging.Campaign("Step 1b: Ingesting into knowledge store")
	if err := d.ingestIntoKnowledgeStore(ctx, campaignID, kbPath, fileMeta); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Knowledge ingestion failed (non-fatal): %v", err)
	}

	// Step 2: Extract requirements from source documents
	logging.Campaign("Step 2: Extracting requirements (RAG-based)")
	reqTimer := logging.StartTimer(logging.CategoryCampaign, "extractRequirementsSmart")
	requirements, err := d.extractRequirementsSmart(ctx, campaignID, req.Goal, kbPath, fileMeta)
	reqTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error(
			"Requirement extraction failed campaign=%s goal=%q file_count=%d: %v",
			campaignID, req.Goal, len(fileMeta), err)
		return nil, fmt.Errorf("failed to extract requirements: %w", err)
	}
	logging.Campaign("Extracted %d requirements", len(requirements))

	// Step 3: LLM proposes phases and tasks
	logging.Campaign("Step 3: LLM proposing plan structure")
	planTimer := logging.StartTimer(logging.CategoryCampaign, "llmProposePlan")
	rawPlan, err := d.llmProposePlan(ctx, campaignID, req, kbPath, fileMeta, requirements)
	planTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error(
			"LLM plan proposal failed campaign=%s goal=%q requirements=%d: %v",
			campaignID, req.Goal, len(requirements), err)
		return nil, fmt.Errorf("failed to propose plan: %w", err)
	}
	logging.Campaign("LLM proposed plan: %s (confidence=%.2f, phases=%d)",
		rawPlan.Title, rawPlan.Confidence, len(rawPlan.Phases))

	// Step 4: Convert to Campaign structure
	logging.Campaign("Step 4: Building campaign structure")
	campaign := d.buildCampaign(campaignID, req, rawPlan)
	campaign.SourceDocs = sourceDocs
	campaign.KnowledgeBase = kbPath
	logging.CampaignDebug("Campaign built: phases=%d, totalTasks=%d", len(campaign.Phases), campaign.TotalTasks)

	// Step 4b: Shard Advisory Board Review (NEW)
	if d.advisoryBoard != nil {
		logging.Campaign("Step 4b: Consulting advisory board")
		advisoryTimer := logging.StartTimer(logging.CategoryCampaign, "advisoryBoardReview")

		// Build advisory request
		advisoryPhases := make([]AdvisoryPhase, len(campaign.Phases))
		for i, phase := range campaign.Phases {
			advisoryPhases[i] = AdvisoryPhase{
				ID:          phase.ID,
				Name:        phase.Name,
				Description: phase.Objectives[0].Description,
				TaskCount:   len(phase.Tasks),
			}
		}

		advisoryReq := AdvisoryRequest{
			CampaignID:   campaignID,
			Goal:         req.Goal,
			RawPlan:      rawPlan.Title,
			Phases:       advisoryPhases,
			TaskCount:    campaign.TotalTasks,
			TargetPaths:  req.SourcePaths,
			Intelligence: d.lastIntelligence,
		}

		responses, advErr := d.advisoryBoard.ConsultAdvisors(ctx, advisoryReq)
		advisoryTimer.Stop()

		if advErr != nil {
			logging.Get(logging.CategoryCampaign).Warn("Advisory board consultation failed (non-fatal): %v", advErr)
		} else {
			synthesis := d.advisoryBoard.SynthesizeVotes(responses)
			logging.Campaign("Advisory board: approved=%v, confidence=%.2f, votes=%d",
				synthesis.Approved, synthesis.OverallConfidence, len(responses))

			// Log blocking concerns if any
			if len(synthesis.BlockingConcerns) > 0 {
				logging.Campaign("Advisory board has %d blocking concerns:", len(synthesis.BlockingConcerns))
				for _, bc := range synthesis.BlockingConcerns {
					logging.CampaignDebug("  - [%s] %s: %s", bc.Severity, bc.Advisor, bc.Concern)
				}
			}

			// Log suggestions for user awareness
			if len(synthesis.AllSuggestions) > 0 {
				logging.Campaign("Advisory suggestions: %d total", len(synthesis.AllSuggestions))
				for i, suggestion := range synthesis.AllSuggestions {
					if i >= 5 {
						logging.CampaignDebug("  ... and %d more suggestions", len(synthesis.AllSuggestions)-5)
						break
					}
					logging.CampaignDebug("  - %s", suggestion)
				}
			}

			// Log synthesis summary for later reference
			// Note: AdvisorySummary and AdvisoryApproved fields to be added to Campaign type
			logging.Campaign("Advisory synthesis: %s", synthesis.Summary)
		}
	}

	// Step 5: Load into Mangle for validation
	logging.Campaign("Step 5: Loading campaign facts into Mangle kernel")
	facts := campaign.ToFacts()
	logging.CampaignDebug("Loading %d facts", len(facts))
	if err := d.kernel.LoadFacts(facts); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to load campaign facts: %v", err)
		return nil, fmt.Errorf("failed to load campaign facts: %w", err)
	}

	// Step 6: Mangle validates (circular deps, unreachable tasks, etc.)
	logging.Campaign("Step 6: Mangle validation")
	issues := d.validatePlan(campaignID)
	if len(issues) > 0 {
		logging.Get(logging.CategoryCampaign).Warn("Validation found %d issues", len(issues))
		for i, issue := range issues {
			logging.CampaignDebug("Issue %d: [%s] %s", i+1, issue.IssueType, issue.Description)
		}
	} else {
		logging.Campaign("Validation passed with no issues")
	}

	// Step 7: If issues, attempt LLM refinement
	if len(issues) > 0 {
		logging.Campaign("Step 7: Attempting LLM refinement to fix %d issues", len(issues))
		refineTimer := logging.StartTimer(logging.CategoryCampaign, "refinePlan")
		refinedPlan, err := d.refinePlan(ctx, rawPlan, issues)
		refineTimer.Stop()
		if err == nil && refinedPlan != nil {
			logging.Campaign("Refinement successful, rebuilding campaign")
			previousCampaign := campaign
			campaign = d.buildCampaign(campaignID, req, refinedPlan)
			campaign.SourceDocs = sourceDocs
			campaign.KnowledgeBase = kbPath
			if err := syncCampaignFacts(d.kernel, previousCampaign, campaign, "plan refinement"); err != nil {
				logging.Get(logging.CategoryCampaign).Error("decomposer: failed to commit campaign facts: %v", err)
				return nil, fmt.Errorf("failed to commit refined campaign facts: %w", err)
			}
			issues = d.validatePlan(campaignID)
			logging.Campaign("After refinement: %d issues remaining", len(issues))
		} else if err != nil {
			logging.Get(logging.CategoryCampaign).Warn("Refinement failed: %v", err)
		}
	}

	// Step 8: Link requirements to tasks
	logging.Campaign("Step 8: Linking requirements to tasks")
	d.linkRequirementsToTasks(requirements, campaign)
	coveredCount := 0
	for _, req := range requirements {
		if len(req.CoveredBy) > 0 {
			coveredCount++
		}
	}
	logging.Campaign("Requirement coverage: %d/%d requirements linked to tasks", coveredCount, len(requirements))

	// Step 9: Tool Pre-Generation (NEW - Ouroboros integration)
	if d.toolPregenerator != nil {
		logging.Campaign("Step 9: Pre-generating tools for campaign")
		toolTimer := logging.StartTimer(logging.CategoryCampaign, "toolPregeneration")

		// Extract task info for gap analysis
		taskInfos := d.extractTaskInfos(campaign)

		// Detect tool gaps
		gaps, gapErr := d.toolPregenerator.DetectGaps(ctx, req.Goal, taskInfos, d.lastIntelligence)
		if gapErr != nil {
			logging.Get(logging.CategoryCampaign).Warn("Tool gap detection failed (non-fatal): %v", gapErr)
		} else if len(gaps) > 0 {
			logging.Campaign("Detected %d tool gaps, attempting pre-generation", len(gaps))

			// Pre-generate tools
			result, genErr := d.toolPregenerator.PregenerateTools(ctx, gaps)
			if genErr != nil {
				logging.Get(logging.CategoryCampaign).Warn("Tool pre-generation failed (non-fatal): %v", genErr)
			} else if result != nil {
				logging.Campaign("Tool pre-generation: %d generated, %d failed, %d unresolved",
					len(result.ToolsGenerated), result.FailedTools, len(result.UnresolvedGaps))

				// Log generated tools
				for _, tool := range result.ToolsGenerated {
					logging.CampaignDebug("  Generated: %s - %s", tool.Name, tool.Purpose)
				}
			}
		} else {
			logging.Campaign("No tool gaps detected")
		}
		toolTimer.Stop()
	}

	logging.Campaign("=== Decomposition complete: %s ===", campaign.Title)
	logging.Campaign("Final plan: phases=%d, tasks=%d, validation=%v",
		campaign.TotalPhases, campaign.TotalTasks, len(issues) == 0)

	return &DecomposeResult{
		Campaign:     campaign,
		ValidationOK: len(issues) == 0,
		Issues:       issues,
		SourceDocs:   sourceDocs,
		Requirements: requirements,
	}, nil
}

// seedIntelligenceFacts loads intelligence facts into the Mangle kernel for logical reasoning.
func (d *Decomposer) seedIntelligenceFacts(campaignID string, intel *IntelligenceReport) {
	if d.kernel == nil || intel == nil {
		return
	}

	facts := make([]core.Fact, 0, 100)

	// World facts
	for _, wf := range intel.WorldFacts {
		facts = append(facts, core.Fact{
			Predicate: "intelligence_world_fact",
			Args:      []any{campaignID, wf.Predicate, wf.Args},
		})
	}

	// Churn hotspots (Chesterton's Fence warnings)
	for _, ch := range intel.GitChurnHotspots {
		facts = append(facts, core.Fact{
			Predicate: "intelligence_churn_hotspot",
			Args:      []any{ch.Path, ch.ChurnRate, ch.Reason},
		})
	}

	// Historical patterns from learning store. Schema wants
	// (ShardType, Predicate, Confidence) — earlier this passed Description
	// in the Predicate slot, which still type-checked as /string but caused
	// downstream rules that filtered on predicate-name patterns to miss.
	for _, lp := range intel.HistoricalPatterns {
		facts = append(facts, core.Fact{
			Predicate: "intelligence_learning_pattern",
			Args:      []any{lp.ShardType, lp.Predicate, types.PercentFromRatio(lp.Confidence)},
		})
	}

	// Safety warnings from constitutional gate. Schema is 5-tuple
	// (CampaignID, Path, Action, RuleViolated, Severity) — earlier the
	// emission was 4-tuple and Mangle silently dropped the row with an
	// arity error in the kernel log.
	for _, sw := range intel.SafetyWarnings {
		facts = append(facts, core.Fact{
			Predicate: "intelligence_safety_warning",
			Args:      []any{campaignID, sw.Path, sw.Action, sw.RuleViolated, sw.Severity},
		})
	}

	// Tool gaps. Schema is 5-tuple
	// (CampaignID, Capability, RequiredBy, Priority, Confidence) bound
	// [/string, /string, /string, /number, /number]. autopoiesis.ToolNeed
	// has Name+Purpose+Priority+Confidence; map Name→Capability and use
	// Purpose as a single-string RequiredBy summary (the schema is a
	// string, not a list, so we collapse the requirements down).
	for _, tg := range intel.ToolGaps {
		facts = append(facts, core.Fact{
			Predicate: "intelligence_tool_gap",
			Args:      []any{campaignID, tg.Name, tg.Purpose, tg.Priority, types.PercentFromRatio(tg.Confidence)},
		})
	}

	// MCP tools available. Schema is 4-tuple (ToolID, ServerID, Name, Affinity).
	for _, mt := range intel.MCPToolsAvailable {
		facts = append(facts, core.Fact{
			Predicate: "intelligence_mcp_tool",
			Args:      []any{mt.ToolID, mt.ServerID, mt.Name, types.PercentFromRatio(mt.Affinity)},
		})
	}

	// Shard advice. Schema is 5-tuple
	// (CampaignID, ShardName, Vote, Confidence, Advice). Vote is sourced
	// from the metadata["vote"] hint if present, otherwise marked as the
	// generic "advisory" tag so the row still satisfies /string binding.
	for _, sa := range intel.ShardAdvice {
		vote := "advisory"
		if v, ok := sa.Metadata["vote"]; ok && v != "" {
			vote = v
		}
		facts = append(facts, core.Fact{
			Predicate: "intelligence_shard_advice",
			Args:      []any{campaignID, sa.FromSpec, vote, types.PercentFromRatio(sa.Confidence), sa.Advice},
		})
	}

	// Test coverage data. Every numeric intelligence_* slot is declared /number
	// (schemas_intelligence.mg) and the policy thresholds are integer percents
	// (Coverage < 30). Passing the raw 0..1 float would reach the kernel as an
	// ast.Float64, which this Mangle fork's comparison builtins reject — taking
	// down the whole fixpoint, not just the rules that read this predicate.
	for path, coverage := range intel.TestCoverage {
		facts = append(facts, core.Fact{
			Predicate: "intelligence_test_coverage",
			Args:      []any{path, types.PercentFromRatio(coverage)},
		})
	}

	// Code patterns detected. Schema is 4-tuple
	// (Name, Type, File, Confidence) — earlier the Type slot was dropped,
	// so the policy rule that branches on "anti-pattern" vs "design" never
	// fired.
	for _, cp := range intel.CodePatterns {
		files := ""
		if len(cp.Files) > 0 {
			files = strings.Join(cp.Files, ",")
		}
		facts = append(facts, core.Fact{
			Predicate: "intelligence_code_pattern",
			Args:      []any{cp.Name, cp.Type, files, types.PercentFromRatio(cp.Confidence)},
		})
	}

	// Previous campaign artifacts. Schema is 4-tuple
	// (CampaignID, Goal, TaskCount, SuccessRate) bound
	// [/string, /string, /number, /number]. Earlier the third arg was a
	// derived bool which violated the /number binding and the row was
	// rejected by the kernel.
	for _, ca := range intel.PreviousCampaigns {
		facts = append(facts, core.Fact{
			Predicate: "intelligence_previous_campaign",
			Args:      []any{ca.CampaignID, ca.Goal, ca.TaskCount, types.PercentFromRatio(ca.SuccessRate)},
		})
	}

	if err := d.kernel.AssertBatch(facts); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to seed intelligence facts: %v", err)
	} else {
		logging.CampaignDebug("Seeded %d intelligence facts into kernel", len(facts))
	}
}

// formatIntelligenceContext builds LLM context from intelligence report.
func (d *Decomposer) formatIntelligenceContext(intel *IntelligenceReport) string {
	if intel == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("INTELLIGENCE REPORT (from 12 systems):\n\n")

	// Churn hotspots (Chesterton's Fence)
	if len(intel.GitChurnHotspots) > 0 {
		sb.WriteString("## HIGH-CHURN FILES (Chesterton's Fence - understand before modifying)\n")
		shown := 0
		for _, ch := range intel.GitChurnHotspots {
			if shown >= 10 {
				sb.WriteString(fmt.Sprintf("... and %d more high-churn files\n", len(intel.GitChurnHotspots)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s: %d changes (%s)\n", ch.Path, ch.ChurnRate, ch.Reason))
			shown++
		}
		sb.WriteString("\n")
	}

	// Historical patterns
	if len(intel.HistoricalPatterns) > 0 {
		sb.WriteString("## LEARNED PATTERNS (from previous sessions)\n")
		for _, lp := range intel.HistoricalPatterns {
			if lp.Confidence >= 0.5 {
				sb.WriteString(fmt.Sprintf("- [%s] %s (%.0f%% confidence)\n", lp.ShardType, lp.Description, lp.Confidence*100))
			}
		}
		sb.WriteString("\n")
	}

	// Safety warnings
	if len(intel.SafetyWarnings) > 0 {
		sb.WriteString("## SAFETY WARNINGS (constitutional pre-check)\n")
		for _, sw := range intel.SafetyWarnings {
			sb.WriteString(fmt.Sprintf("- %s on %s: blocked by rule '%s'\n", sw.Action, sw.Path, sw.RuleViolated))
		}
		sb.WriteString("\n")
	}

	// Available MCP tools
	if len(intel.MCPToolsAvailable) > 0 {
		sb.WriteString("## AVAILABLE TOOLS (from MCP servers)\n")
		shown := 0
		for _, mt := range intel.MCPToolsAvailable {
			if shown >= 15 {
				sb.WriteString(fmt.Sprintf("... and %d more MCP tools available\n", len(intel.MCPToolsAvailable)-15))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s: %s\n", mt.Name, mt.Description))
			shown++
		}
		sb.WriteString("\n")
	}

	// Tool gaps detected
	if len(intel.ToolGaps) > 0 {
		sb.WriteString("## TOOL GAPS (capabilities needed but not available)\n")
		for _, tg := range intel.ToolGaps {
			sb.WriteString(fmt.Sprintf("- %s: %s (confidence: %.0f%%)\n", tg.Name, tg.Purpose, tg.Confidence*100))
		}
		sb.WriteString("\n")
	}

	// Expert recommendations from shards
	if len(intel.ShardAdvice) > 0 {
		sb.WriteString("## EXPERT RECOMMENDATIONS\n")
		for _, sa := range intel.ShardAdvice {
			if sa.Confidence >= 0.6 {
				sb.WriteString(fmt.Sprintf("### %s (%.0f%% confidence)\n%s\n\n", sa.FromSpec, sa.Confidence*100, sa.Advice))
			}
		}
	}

	// Test coverage summary
	if len(intel.TestCoverage) > 0 {
		sb.WriteString("## TEST COVERAGE (by path)\n")
		lowCoverage := make([]string, 0)
		for path, cov := range intel.TestCoverage {
			if cov < 0.5 {
				lowCoverage = append(lowCoverage, fmt.Sprintf("- %s: %.0f%%", path, cov*100))
			}
		}
		if len(lowCoverage) > 0 {
			sb.WriteString("Low coverage areas:\n")
			for _, lc := range lowCoverage {
				sb.WriteString(lc + "\n")
			}
		} else {
			sb.WriteString("All areas have adequate test coverage.\n")
		}
		sb.WriteString("\n")
	}

	// Code patterns
	if len(intel.CodePatterns) > 0 {
		sb.WriteString("## DETECTED CODE PATTERNS\n")
		for _, cp := range intel.CodePatterns {
			files := ""
			if len(cp.Files) > 0 {
				files = strings.Join(cp.Files, ", ")
			}
			sb.WriteString(fmt.Sprintf("- %s in %s\n", cp.Name, files))
		}
		sb.WriteString("\n")
	}

	// Previous campaign references
	if len(intel.PreviousCampaigns) > 0 {
		sb.WriteString("## RELEVANT PREVIOUS CAMPAIGNS\n")
		for _, ca := range intel.PreviousCampaigns {
			status := "failed"
			if ca.SuccessRate > 0.5 {
				status = fmt.Sprintf("succeeded (%.0f%%)", ca.SuccessRate*100)
			} else {
				status = fmt.Sprintf("failed (%.0f%%)", ca.SuccessRate*100)
			}
			goalSummary := ca.Goal
			if len(goalSummary) > 50 {
				goalSummary = goalSummary[:50] + "..."
			}
			sb.WriteString(fmt.Sprintf("- %s: %s - %s\n", ca.CampaignID, goalSummary, status))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// extractTaskInfos extracts task information for tool gap analysis.
func (d *Decomposer) extractTaskInfos(campaign *Campaign) []TaskInfo {
	taskInfos := make([]TaskInfo, 0)

	for _, phase := range campaign.Phases {
		for _, task := range phase.Tasks {
			// Extract file paths from artifacts
			filePaths := make([]string, 0, len(task.Artifacts))
			for _, artifact := range task.Artifacts {
				filePaths = append(filePaths, artifact.Path)
			}

			// Extract actions from task description (simple heuristic)
			actions := extractActionsFromDescription(task.Description)

			taskInfos = append(taskInfos, TaskInfo{
				ID:          task.ID,
				Description: task.Description,
				Type:        string(task.Type),
				Actions:     actions,
				FilePaths:   filePaths,
			})
		}
	}

	return taskInfos
}

// extractActionsFromDescription extracts action keywords from a task description.
func extractActionsFromDescription(description string) []string {
	actions := []string{}
	lower := strings.ToLower(description)

	// Common action keywords
	actionKeywords := map[string]string{
		"parse":    "parse",
		"validate": "validate",
		"generate": "generate",
		"create":   "create",
		"update":   "update",
		"delete":   "delete",
		"read":     "read",
		"write":    "write",
		"test":     "test",
		"build":    "build",
		"deploy":   "deploy",
		"analyze":  "analyze",
		"refactor": "refactor",
		"optimize": "optimize",
	}

	for keyword, action := range actionKeywords {
		if strings.Contains(lower, keyword) {
			actions = append(actions, action)
		}
	}

	return actions
}

// linkRequirementsToTasks links extracted requirements to tasks.
func (d *Decomposer) linkRequirementsToTasks(requirements []Requirement, campaign *Campaign) {
	logging.CampaignDebug("Linking %d requirements to campaign tasks", len(requirements))
	linkedCount := 0

	for i := range requirements {
		// Simple heuristic: match by keyword overlap
		reqWords := strings.Fields(strings.ToLower(requirements[i].Description))

		for _, phase := range campaign.Phases {
			for _, task := range phase.Tasks {
				taskWords := strings.Fields(strings.ToLower(task.Description))

				// Count matching words
				matches := 0
				for _, rw := range reqWords {
					for _, tw := range taskWords {
						if rw == tw && len(rw) > 3 { // Ignore short words
							matches++
						}
					}
				}

				// If significant overlap, link
				if matches >= 2 {
					requirements[i].CoveredBy = append(requirements[i].CoveredBy, task.ID)
					linkedCount++
					logging.CampaignDebug("Linked requirement %s to task %s (matches=%d)",
						requirements[i].ID, task.ID, matches)
				}
			}
		}
	}

	// Load requirement coverage facts
	var coverageFacts []core.Fact
	for _, req := range requirements {
		for _, taskID := range req.CoveredBy {
			coverageFacts = append(coverageFacts, core.Fact{
				Predicate: "requirement_coverage",
				Args:      []any{req.ID, taskID},
			})
		}
	}
	if len(coverageFacts) > 0 {
		if err := d.kernel.AssertBatch(coverageFacts); err != nil {
			logging.CampaignDebug("Error asserting requirement_coverage batch: %v", err)
		}
	}

	logging.CampaignDebug("Requirement linking complete: %d links created", linkedCount)
}

func limitString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// cleanJSONResponse extracts JSON from LLM response that may contain reasoning traces.
// It looks for JSON objects/arrays in the response, handling markdown fences and preamble text.
func cleanJSONResponse(resp string) string {
	resp = strings.TrimSpace(resp)

	// First try: Look for ```json block
	if idx := strings.Index(resp, "```json"); idx != -1 {
		start := idx + 7 // len("```json")
		// Skip any whitespace after ```json
		for start < len(resp) && (resp[start] == '\n' || resp[start] == '\r' || resp[start] == ' ') {
			start++
		}
		end := strings.Index(resp[start:], "```")
		if end != -1 {
			return strings.TrimSpace(resp[start : start+end])
		}
	}

	// Second try: Look for first { and match to closing }
	if braceStart := strings.Index(resp, "{"); braceStart != -1 {
		// Find matching closing brace by counting
		depth := 0
		inString := false
		escape := false
		for i := braceStart; i < len(resp); i++ {
			c := resp[i]
			if escape {
				escape = false
				continue
			}
			if c == '\\' && inString {
				escape = true
				continue
			}
			if c == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
				if depth == 0 {
					return strings.TrimSpace(resp[braceStart : i+1])
				}
			}
		}
	}

	// Third try: Look for [ for arrays
	if bracketStart := strings.Index(resp, "["); bracketStart != -1 {
		depth := 0
		inString := false
		escape := false
		for i := bracketStart; i < len(resp); i++ {
			c := resp[i]
			if escape {
				escape = false
				continue
			}
			if c == '\\' && inString {
				escape = true
				continue
			}
			if c == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if c == '[' {
				depth++
			} else if c == ']' {
				depth--
				if depth == 0 {
					return strings.TrimSpace(resp[bracketStart : i+1])
				}
			}
		}
	}

	// Fallback: strip markdown fences and return
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	return strings.TrimSpace(resp)
}

// formatShardList formats available shards for injection into the planner prompt.
func formatShardList(shards []types.ShardInfo) string {
	if len(shards) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("AVAILABLE SHARDS (you can specify any of these for tasks):\n")

	// Group by type for clarity
	groups := make(map[types.ShardType][]types.ShardInfo)
	for _, s := range shards {
		groups[s.Type] = append(groups[s.Type], s)
	}

	typeOrder := []types.ShardType{types.ShardTypeEphemeral, types.ShardTypePersistent, types.ShardTypeUser}
	for _, shardType := range typeOrder {
		if list, ok := groups[shardType]; ok && len(list) > 0 {
			typeLabel := "Ephemeral"
			switch shardType {
			case types.ShardTypePersistent:
				typeLabel = "Specialist"
			case types.ShardTypeUser:
				typeLabel = "User-defined"
			}
			sb.WriteString(fmt.Sprintf("\n[%s shards]\n", typeLabel))
			for _, s := range list {
				desc := s.Description
				if desc == "" {
					desc = "General purpose"
				}
				sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, desc))
			}
		}
	}

	sb.WriteString(`
SHARD ROUTING INSTRUCTIONS:
- For each task, you MAY specify "shard" to route to a specific shard
- Use "shard_input" to provide the exact input for the shard
- Use "context_from" to inject results from previous tasks (array of task indices)
- If shard is not specified, the system infers based on task type

IMPORTANT: For documentation tasks needing directory content awareness:
1. First use "researcher" shard to enumerate/read directory contents
2. Then use "coder" with context_from referencing the research task

Example task with explicit shard routing:
{
  "description": "Read contents of internal/core directory",
  "type": "/research",
  "shard": "researcher",
  "shard_input": "List all files in internal/core and summarize their purpose",
  "order": 0
}

Example task with context injection:
{
  "description": "Create documentation for internal/core",
  "type": "/file_create",
  "shard": "coder",
  "context_from": [0],
  "order": 1
}
`)
	return sb.String()
}
