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

	// Intelligence integration (Step 0): its report goes into the planner's
	// prompt.
	intelligence *IntelligenceGatherer

	// Gemini grounding (nil if the client cannot ground)
	grounding *research.GroundingHelper // Google Search / URL Context grounding

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

	// Ground planning calls in Google Search when the client can.
	if llmClient != nil {
		d.grounding = research.NewGroundingHelper(llmClient)
		if d.grounding.IsGroundingAvailable() {
			d.grounding.EnableGoogleSearch()
			logging.CampaignDebug("Gemini grounding enabled for campaign planning (Google Search active)")
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
		return response, nil
	}
	// Fall back to standard completion
	if d.llmClient == nil {
		return "", fmt.Errorf("%w: decomposer requires llm client", ErrNilDependency)
	}
	// Reply is unmarshalled directly; no Piggyback envelope schema.
	return d.llmClient.Complete(types.WithStructuredOutputOnly(ctx), prompt)
}

// DecomposeRequest represents a request to create a campaign.
type DecomposeRequest struct {
	Goal          string       // High-level goal description
	SourcePaths   []string     // Paths to spec docs, requirements, etc.
	CampaignType  CampaignType // Type of campaign
	UserHints     []string     // Optional user guidance
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

	// The advisory board reviews the plan in the risk preflight, where its
	// verdict gates the run (risk_scoring.go). A plan-time review here only
	// reached a log line, so it was removed rather than run twice.

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
	issues := append(d.validatePlan(campaignID), taskEffectIssues(campaign)...)
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
			// Refining a degraded plan does not un-degrade it. refinePlan asks
			// the model to fix validation issues in the phases it is given, so
			// polishing the generic three-task scaffold yields a polished
			// generic three-task scaffold. Carrying the flag forward is what
			// makes the CLI banner appear: without this the second
			// buildCampaign produced a Campaign with PlanDegraded false and the
			// warning vanished, even though the WARN had already fired in the
			// log.
			refinedPlan.Degraded = refinedPlan.Degraded || rawPlan.Degraded
			campaign = d.buildCampaign(campaignID, req, refinedPlan)
			campaign.SourceDocs = sourceDocs
			campaign.KnowledgeBase = kbPath
			if err := syncCampaignFacts(d.kernel, previousCampaign, campaign, "plan refinement"); err != nil {
				logging.Get(logging.CategoryCampaign).Error("decomposer: failed to commit campaign facts: %v", err)
				return nil, fmt.Errorf("failed to commit refined campaign facts: %w", err)
			}
			issues = append(d.validatePlan(campaignID), taskEffectIssues(campaign)...)
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
	// (CampaignID, ShardName, Vote, Confidence, Advice). Pre-planning advice
	// is not a ballot (the advisory board's explicit VOTE contract is), so
	// its Vote is the generic /advisory.
	//
	// ShardName and Vote are the /name slots of intelligence_shard_advice/5
	// (schemas_intelligence.mg): the advisory-board rules in
	// policy/intelligence.mg match them as name constants
	// (/coder, /tester, /approve, /reject, /conditional). Specialist arrives
	// as the bare "coder", and a bare token lands as a string constant that
	// never unifies with /coder, so it has to be promoted here. Advice stays
	// free prose, so it stays /string.
	for _, sa := range intel.ShardAdvice {
		facts = append(facts, core.Fact{
			Predicate: "intelligence_shard_advice",
			Args: []any{
				campaignID,
				types.MangleAtom(mangleNameConst(sa.Specialist)),
				types.MangleAtom("/advisory"),
				types.PercentFromRatio(sa.Confidence),
				sa.Advice,
			},
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

// mangleNameConst renders a short closed-vocabulary token (a specialist name, a
// ballot) as a Mangle name constant: lower-cased, every character outside
// [a-z0-9_] folded to "_", prefixed with "/".
//
// The sanitising is load-bearing, not cosmetic. types.Fact.ToAtom calls
// ast.Name on any MangleAtom that starts with "/", and these tokens can carry
// whatever a caller named a specialist ("Not Sure", "APPROVE — with
// caveats"). An unparseable name there fails the conversion for that fact at
// evaluation time, so the token is normalised into a legal name rather than
// trusted. A token with nothing usable in it becomes /unknown, never the
// invalid bare "/".
func mangleNameConst(s string) string {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "/")
	var b strings.Builder
	b.Grow(len(s) + 1)
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "/unknown"
	}
	return "/" + out
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
