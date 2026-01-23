package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// SESSION CONTEXT BUILDING
// =============================================================================

// buildSessionContext creates a SessionContext for shard injection (Blackboard Pattern).
// This provides shards with comprehensive session context including:
// - Compressed history, recent findings, recent actions (original)
// - World model facts (impacted files, diagnostics, symbols, dependencies)
// - User intent and focus resolutions
// - Campaign context (if active)
// - Git state for Chesterton's Fence
// - Test state for TDD loop awareness
// - Cross-shard execution history
// - Domain knowledge atoms
// - Constitutional constraints
func (m *Model) buildSessionContext(ctx context.Context) *types.SessionContext {
	sessionCtx := &types.SessionContext{
		ExtraContext: make(map[string]string),
	}

	// ==========================================================================
	// CORE CONTEXT (Original)
	// ==========================================================================

	// Get compressed history from compressor
	if m.compressor != nil {
		if ctxStr, err := m.compressor.GetContextString(ctx); err == nil {
			sessionCtx.CompressedHistory = ctxStr
		}
	}

	// Extract recent findings from shard history
	for _, sr := range m.shardResultHistory {
		if sr.ShardType == "reviewer" || sr.ShardType == "tester" {
			for _, f := range sr.Findings {
				if msg, ok := f["raw"].(string); ok {
					sessionCtx.RecentFindings = append(sessionCtx.RecentFindings, msg)
				}
			}
		}
		// Track recent actions
		sessionCtx.RecentActions = append(sessionCtx.RecentActions,
			fmt.Sprintf("[%s] %s", sr.ShardType, truncateForContext(sr.Task, 50)))
	}

	// Limit findings to last 20
	if len(sessionCtx.RecentFindings) > 20 {
		sessionCtx.RecentFindings = sessionCtx.RecentFindings[len(sessionCtx.RecentFindings)-20:]
	}

	// Limit actions to last 10
	if len(sessionCtx.RecentActions) > 10 {
		sessionCtx.RecentActions = sessionCtx.RecentActions[len(sessionCtx.RecentActions)-10:]
	}

	// ==========================================================================
	// WORLD MODEL / EDB FACTS (from kernel)
	// ==========================================================================
	if m.kernel != nil {
		// Get impacted files (transitive impact from modified files)
		sessionCtx.ImpactedFiles = m.queryKernelStrings("impacted")

		// Get current diagnostics (errors/warnings)
		sessionCtx.CurrentDiagnostics = m.queryDiagnostics()

		// Get relevant symbols in scope
		sessionCtx.SymbolContext = m.querySymbolContext()

		// Get 1-hop dependencies for active files
		if len(sessionCtx.ActiveFiles) > 0 {
			sessionCtx.DependencyContext = m.queryDependencyContext(sessionCtx.ActiveFiles)
		}

		// Get focus resolutions
		sessionCtx.FocusResolutions = m.queryFocusResolutions()
	}

	// ==========================================================================
	// CAMPAIGN CONTEXT (if active)
	// ==========================================================================
	if m.activeCampaign != nil {
		sessionCtx.CampaignActive = true
		// Get current phase from progress or derive from phases
		if m.campaignProgress != nil {
			sessionCtx.CampaignPhase = m.campaignProgress.CurrentPhase
		} else {
			sessionCtx.CampaignPhase = m.getCurrentPhaseName()
		}
		sessionCtx.CampaignGoal = m.getCampaignPhaseGoal()
		sessionCtx.TaskDependencies = m.getCampaignTaskDeps()
		sessionCtx.LinkedRequirements = m.getCampaignLinkedReqs()
	}

	// ==========================================================================
	// GIT STATE / CHESTERTON'S FENCE
	// ==========================================================================
	m.populateGitContext(sessionCtx)

	// ==========================================================================
	// TEST STATE (TDD LOOP)
	// ==========================================================================
	m.populateTestState(sessionCtx)

	// ==========================================================================
	// CROSS-SHARD EXECUTION HISTORY
	// ==========================================================================
	sessionCtx.PriorShardOutputs = m.buildPriorShardSummaries()

	// ==========================================================================
	// DOMAIN KNOWLEDGE (Type B Specialists)
	// ==========================================================================
	if m.learningStore != nil {
		sessionCtx.KnowledgeAtoms = m.queryKnowledgeAtoms()
		sessionCtx.SpecialistHints = m.querySpecialistHints()
	}

	if m.lastReflection != nil && len(m.lastReflection.ContextHits) > 0 {
		sessionCtx.ReflectionHits = append(sessionCtx.ReflectionHits, m.lastReflection.ContextHits...)
		sessionCtx.ExtraContext["reflection_hits"] = strings.Join(m.lastReflection.ContextHits, "\n")
	}

	// ==========================================================================
	// GAP-004 FIX: MULTI-TIER MEMORY RETRIEVAL (Vector, Graph, Cold)
	// ==========================================================================
	if m.localDB != nil {
		// Vector Tier: Semantic search for relevant past knowledge
		m.queryVectorMemory(ctx, sessionCtx)

		// Graph Tier: Entity relationships for active files
		m.queryGraphMemory(sessionCtx)

		// Cold Tier: Persistent learned facts
		m.queryColdMemory(sessionCtx)
	}

	// ==========================================================================
	// SEMANTIC KNOWLEDGE BRIDGE: Assert knowledge atoms to kernel
	// ==========================================================================
	// Assert strategic knowledge atoms to kernel for spreading activation.
	// This enables knowledge_atom facts to receive activation scores and
	// be injected into shards via injectable_context predicates.
	if m.kernel != nil && m.localDB != nil {
		m.assertKnowledgeAtomsToKernel()
	}

	// ==========================================================================
	// GATHERED KNOWLEDGE (LLM-First Knowledge Discovery)
	// ==========================================================================
	// Inject knowledge gathered from specialist consultations during this session.
	// This enables action shards (coder, tester) to benefit from prior research.
	if len(m.pendingKnowledge) > 0 {
		for _, kr := range m.pendingKnowledge {
			if kr.Error == nil && kr.Response != "" {
				// Truncate for context budget (full output available separately)
				summary := kr.Response
				if len(summary) > 500 {
					summary = summary[:500] + "..."
				}
				sessionCtx.GatheredKnowledge = append(sessionCtx.GatheredKnowledge, types.KnowledgeSummary{
					Specialist: kr.Specialist,
					Topic:      kr.Query,
					Summary:    summary,
					FullOutput: kr.Response,
				})
			}
		}
	}

	// ==========================================================================
	// RECENT TOOL EXECUTIONS (for LLM context awareness)
	// ==========================================================================
	if m.toolStore != nil {
		if recent, err := m.toolStore.GetRecent(5); err == nil {
			for _, exec := range recent {
				// Truncate result for context budget
				summary := exec.Result
				if len(summary) > 500 {
					summary = summary[:500] + "..."
				}
				if exec.Error != "" {
					summary = exec.Error
				}
				sessionCtx.RecentToolExecutions = append(sessionCtx.RecentToolExecutions, types.ToolExecutionSummary{
					CallID:     exec.CallID,
					ToolName:   exec.ToolName,
					Action:     exec.Action,
					Success:    exec.Success,
					ResultSize: exec.ResultSize,
					DurationMs: exec.DurationMs,
					Summary:    summary,
				})
				// Increment reference count since we're including in LLM context
				_ = m.toolStore.IncrementReference(exec.CallID)
			}
		}
	}

	// ==========================================================================
	// CONSTITUTIONAL CONSTRAINTS
	// ==========================================================================
	if m.kernel != nil {
		sessionCtx.AllowedActions = m.queryAllowedActions()
		sessionCtx.BlockedActions = m.queryBlockedActions()
		sessionCtx.SafetyWarnings = m.querySafetyWarnings()
	}

	return sessionCtx
}

// =============================================================================
// KERNEL QUERY HELPERS FOR SESSION CONTEXT
// =============================================================================

// queryKernelStrings queries a predicate and returns all first-arg strings.
func (m *Model) queryKernelStrings(predicate string) []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query(predicate)
	if err != nil {
		return nil
	}
	var strs []string
	for _, fact := range results {
		if len(fact.Args) > 0 {
			if s, ok := fact.Args[0].(string); ok {
				strs = append(strs, s)
			}
		}
	}
	return strs
}

// queryDiagnostics extracts current diagnostics from the kernel.
func (m *Model) queryDiagnostics() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("diagnostic")
	if err != nil {
		return nil
	}
	var diagnostics []string
	for _, fact := range results {
		// diagnostic(Severity, FilePath, Line, ErrorCode, Message)
		if len(fact.Args) >= 5 {
			severity, _ := fact.Args[0].(string)
			file, _ := fact.Args[1].(string)
			line, _ := fact.Args[2].(int64)
			msg, _ := fact.Args[4].(string)
			diagnostics = append(diagnostics,
				fmt.Sprintf("[%s] %s:%d: %s", severity, file, line, msg))
		}
	}
	// Limit to most recent 10
	if len(diagnostics) > 10 {
		diagnostics = diagnostics[len(diagnostics)-10:]
	}
	return diagnostics
}

// querySymbolContext gets relevant symbols from symbol_graph.
func (m *Model) querySymbolContext() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("symbol_graph")
	if err != nil {
		return nil
	}
	var symbols []string
	for _, fact := range results {
		// symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature)
		if len(fact.Args) >= 5 {
			symbolID, _ := fact.Args[0].(string)
			symType, _ := fact.Args[1].(string)
			visibility, _ := fact.Args[2].(string)
			signature, _ := fact.Args[4].(string)
			if visibility == "/public" || visibility == "/exported" {
				symbols = append(symbols,
					fmt.Sprintf("%s %s: %s", symType, symbolID, truncateForContext(signature, 60)))
			}
		}
	}
	// Limit to 15 most relevant
	if len(symbols) > 15 {
		symbols = symbols[:15]
	}
	return symbols
}

// queryDependencyContext gets 1-hop dependencies for target files.
func (m *Model) queryDependencyContext(files []string) []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("dependency_link")
	if err != nil {
		return nil
	}
	var deps []string
	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}
	for _, fact := range results {
		// dependency_link(CallerID, CalleeID, ImportPath)
		if len(fact.Args) >= 3 {
			caller, _ := fact.Args[0].(string)
			callee, _ := fact.Args[1].(string)
			importPath, _ := fact.Args[2].(string)
			// Check if caller or callee is in our active files
			if fileSet[caller] {
				deps = append(deps, fmt.Sprintf("%s imports %s", caller, importPath))
			}
			if fileSet[callee] {
				deps = append(deps, fmt.Sprintf("%s imported by %s", callee, caller))
			}
		}
	}
	// Limit to 10
	if len(deps) > 10 {
		deps = deps[:10]
	}
	return deps
}

// queryFocusResolutions gets resolved paths from fuzzy references.
func (m *Model) queryFocusResolutions() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("focus_resolution")
	if err != nil {
		return nil
	}
	var resolutions []string
	for _, fact := range results {
		// focus_resolution(RawReference, ResolvedPath, SymbolName, Confidence)
		if len(fact.Args) >= 4 {
			rawRef, _ := fact.Args[0].(string)
			resolved, _ := fact.Args[1].(string)
			confidence, _ := fact.Args[3].(float64)
			resolutions = append(resolutions,
				fmt.Sprintf("'%s' -> %s (%.0f%%)", rawRef, resolved, confidence*100))
		}
	}
	return resolutions
}

// getCurrentPhaseName derives the current phase name from campaign phases.
func (m *Model) getCurrentPhaseName() string {
	if m.activeCampaign == nil {
		return ""
	}
	// Find phase with /in_progress status
	for _, phase := range m.activeCampaign.Phases {
		if phase.Status == campaign.PhaseInProgress {
			return phase.Name
		}
	}
	// Fallback: find first pending phase
	for _, phase := range m.activeCampaign.Phases {
		if phase.Status == campaign.PhasePending {
			return phase.Name
		}
	}
	// Fallback: return first phase name
	if len(m.activeCampaign.Phases) > 0 {
		return m.activeCampaign.Phases[0].Name
	}
	return ""
}

// getCampaignPhaseGoal returns the current phase's goal description.
func (m *Model) getCampaignPhaseGoal() string {
	if m.activeCampaign == nil {
		return ""
	}
	currentPhaseName := m.getCurrentPhaseName()
	for _, phase := range m.activeCampaign.Phases {
		if phase.Name == currentPhaseName {
			// Use first objective's description if available
			if len(phase.Objectives) > 0 {
				return phase.Objectives[0].Description
			}
			return phase.Name
		}
	}
	return currentPhaseName
}

// getCampaignTaskDeps returns dependencies for the current task.
func (m *Model) getCampaignTaskDeps() []string {
	if m.activeCampaign == nil || m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("has_blocking_task_dep")
	if err != nil {
		return nil
	}
	var deps []string
	for _, fact := range results {
		if len(fact.Args) >= 1 {
			if dep, ok := fact.Args[0].(string); ok {
				deps = append(deps, dep)
			}
		}
	}
	return deps
}

// getCampaignLinkedReqs returns requirements linked to current task.
func (m *Model) getCampaignLinkedReqs() []string {
	if m.activeCampaign == nil || m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("requirement_task_link")
	if err != nil {
		return nil
	}
	var reqs []string
	for _, fact := range results {
		// requirement_task_link(RequirementID, TaskID, Strength)
		if len(fact.Args) >= 2 {
			if req, ok := fact.Args[0].(string); ok {
				reqs = append(reqs, req)
			}
		}
	}
	return reqs
}

// populateGitContext fills in Git state for Chesterton's Fence awareness.
func (m *Model) populateGitContext(sessionCtx *types.SessionContext) {
	if m.kernel == nil {
		return
	}
	// Query git_state facts from kernel
	results, err := m.kernel.Query("git_state")
	if err != nil {
		return
	}
	for _, fact := range results {
		// git_state(Attribute, Value)
		if len(fact.Args) >= 2 {
			attr, _ := fact.Args[0].(string)
			val := fmt.Sprintf("%v", fact.Args[1])
			switch attr {
			case "branch":
				sessionCtx.GitBranch = val
				sessionCtx.ExtraContext["git_branch"] = val
			case "modified_files":
				sessionCtx.GitModifiedFiles = splitContextList(val)
				sessionCtx.ExtraContext["git_modified"] = val
			case "recent_commits":
				sessionCtx.GitRecentCommits = splitContextList(val)
				sessionCtx.ExtraContext["git_commits"] = val
			case "unstaged_count":
				if count, convErr := strconv.Atoi(strings.TrimSpace(val)); convErr == nil {
					sessionCtx.GitUnstagedCount = count
				}
			}
		}
	}
}

func splitContextList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []string
	for _, part := range strings.Split(raw, "\n") {
		for _, entry := range strings.Split(part, ",") {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				items = append(items, entry)
			}
		}
	}
	return items
}

// populateTestState fills in test execution state for TDD loop awareness.
func (m *Model) populateTestState(sessionCtx *types.SessionContext) {
	if m.kernel == nil {
		return
	}
	// Query test_result facts from kernel
	results, err := m.kernel.Query("test_result")
	if err != nil {
		return
	}
	var testSummary strings.Builder
	passCount := 0
	failCount := 0
	for _, fact := range results {
		// test_result(TestID, Status, Message)
		if len(fact.Args) >= 2 {
			status, _ := fact.Args[1].(string)
			switch status {
			case "/pass":
				passCount++
			case "/fail":
				failCount++
			}
		}
	}
	if passCount+failCount > 0 {
		testSummary.WriteString(fmt.Sprintf("Tests: %d pass, %d fail", passCount, failCount))
		sessionCtx.ExtraContext["test_state"] = testSummary.String()
	}
}

// buildPriorShardSummaries extracts summaries from recent shard executions.
func (m *Model) buildPriorShardSummaries() []types.ShardSummary {
	var summaries []types.ShardSummary
	for _, sr := range m.shardResultHistory {
		summaries = append(summaries, types.ShardSummary{
			ShardType: sr.ShardType,
			Task:      truncateForContext(sr.Task, 100),
			Summary:   extractShardSummary(sr),
			Timestamp: sr.Timestamp,
			Success:   true, // Default to success since we're showing completed shards
		})
	}
	return summaries
}

// extractShardSummary extracts a brief summary from shard result.
func extractShardSummary(sr *ShardResult) string {
	// For reviewer: count findings
	if sr.ShardType == "reviewer" && len(sr.Findings) > 0 {
		return fmt.Sprintf("%d findings", len(sr.Findings))
	}
	// For tester: show pass/fail counts
	if sr.ShardType == "tester" && sr.Metrics != nil {
		pass, _ := sr.Metrics["pass"].(int)
		fail, _ := sr.Metrics["fail"].(int)
		return fmt.Sprintf("%d pass, %d fail", pass, fail)
	}
	// Generic: truncate output
	return truncateForContext(sr.RawOutput, 100)
}

// assertKnowledgeAtomsToKernel asserts strategic knowledge atoms to the kernel
// for spreading activation scoring. This enables knowledge_atom facts to
// participate in context selection and be injected via injectable_context.
func (m *Model) assertKnowledgeAtomsToKernel() {
	if m.kernel == nil || m.localDB == nil {
		return
	}

	// Query strategic knowledge atoms (highest value for architectural decisions)
	strategicAtoms, err := m.localDB.GetKnowledgeAtomsByPrefix("strategic/")
	if err != nil {
		logging.Get(logging.CategoryContext).Debug("Failed to query strategic atoms: %v", err)
		return
	}

	asserted := 0
	for _, atom := range strategicAtoms {
		// Only assert high-confidence atoms to avoid noise
		if atom.Confidence < 0.7 {
			continue
		}

		fact := core.Fact{
			Predicate: "knowledge_atom",
			Args:      []interface{}{atom.Concept, atom.Content, atom.Confidence},
		}
		if err := m.kernel.Assert(fact); err == nil {
			asserted++
		}
	}

	// Also assert doc-level atoms with architecture/pattern tags
	docAtoms, err := m.localDB.GetKnowledgeAtomsByPrefix("doc/")
	if err == nil {
		for _, atom := range docAtoms {
			// Only high-confidence architecture/pattern docs
			if atom.Confidence < 0.85 {
				continue
			}
			// Filter for architecture and pattern categories
			if !strings.Contains(atom.Concept, "/architecture/") &&
				!strings.Contains(atom.Concept, "/pattern/") &&
				!strings.Contains(atom.Concept, "/philosophy/") {
				continue
			}

			fact := core.Fact{
				Predicate: "knowledge_atom",
				Args:      []interface{}{atom.Concept, atom.Content, atom.Confidence},
			}
			if err := m.kernel.Assert(fact); err == nil {
				asserted++
			}
		}
	}

	if asserted > 0 {
		logging.Get(logging.CategoryContext).Debug("Asserted %d knowledge atoms to kernel", asserted)
	}
}

// queryKnowledgeAtoms retrieves relevant knowledge from learning store.
// Returns high-confidence learnings formatted as readable knowledge atoms.
func (m *Model) queryKnowledgeAtoms() []string {
	if m.learningStore == nil {
		return nil
	}

	var atoms []string
	shardTypes := []string{"coder", "reviewer", "tester", "researcher"}

	for _, shardType := range shardTypes {
		learnings, err := m.learningStore.Load(shardType)
		if err != nil {
			continue
		}
		for _, l := range learnings {
			if l.Confidence < 0.5 {
				continue // Only include high-confidence learnings
			}
			atom := formatLearningAsAtom(shardType, l)
			if atom != "" {
				atoms = append(atoms, atom)
			}
		}
	}
	return atoms
}

// querySpecialistHints retrieves specialist-specific hints.
// Returns patterns that suggest specific tools or approaches.
// Extended with Semantic Knowledge Bridge to include strategic knowledge atoms.
func (m *Model) querySpecialistHints() []string {
	var hints []string

	// Query LearningStore for behavioral learnings (existing)
	if m.learningStore != nil {
		specialistPredicates := []string{"domain_expertise", "tool_preference", "style_preference", "preferred_pattern"}

		for _, pred := range specialistPredicates {
			for _, shardType := range []string{"coder", "reviewer", "tester"} {
				learnings, err := m.learningStore.LoadByPredicate(shardType, pred)
				if err != nil {
					continue
				}
				for _, l := range learnings {
					if l.Confidence >= 0.6 {
						hint := formatLearningAsHint(shardType, l)
						if hint != "" {
							hints = append(hints, hint)
						}
					}
				}
			}
		}
	}

	// Semantic Knowledge Bridge: Also query knowledge atoms for domain expertise
	if m.localDB != nil {
		// Get strategic capability knowledge
		capabilityAtoms, err := m.localDB.GetKnowledgeAtomsByPrefix("strategic/capability")
		if err == nil {
			for _, atom := range capabilityAtoms {
				if atom.Confidence >= 0.8 {
					hints = append(hints, fmt.Sprintf("[STRATEGIC] %s", atom.Content))
				}
			}
		}

		// Get strategic pattern knowledge
		patternAtoms, err := m.localDB.GetKnowledgeAtomsByPrefix("strategic/pattern")
		if err == nil {
			for _, atom := range patternAtoms {
				if atom.Confidence >= 0.85 {
					hints = append(hints, fmt.Sprintf("[PATTERN] %s", atom.Content))
				}
			}
		}

		// Get high-confidence doc architecture atoms
		archAtoms, err := m.localDB.GetKnowledgeAtomsByPrefix("doc/")
		if err == nil {
			archCount := 0
			for _, atom := range archAtoms {
				if archCount >= 5 { // Limit to avoid context bloat
					break
				}
				if atom.Confidence >= 0.9 && strings.Contains(atom.Concept, "/architecture/") {
					hints = append(hints, fmt.Sprintf("[ARCHITECTURE] %s", atom.Content))
					archCount++
				}
			}
		}
	}

	return hints
}

// formatLearningAsAtom converts a ShardLearning to a readable knowledge atom.
func formatLearningAsAtom(shardType string, l types.ShardLearning) string {
	switch l.FactPredicate {
	case "avoid_pattern":
		if len(l.FactArgs) >= 2 {
			return fmt.Sprintf("[%s] Avoid: %v - %v", shardType, l.FactArgs[0], l.FactArgs[1])
		} else if len(l.FactArgs) >= 1 {
			return fmt.Sprintf("[%s] Avoid: %v", shardType, l.FactArgs[0])
		}
	case "preferred_pattern":
		if len(l.FactArgs) >= 1 {
			return fmt.Sprintf("[%s] Prefer: %v", shardType, l.FactArgs[0])
		}
	case "style_preference":
		if len(l.FactArgs) >= 1 {
			return fmt.Sprintf("[%s] Style: %v", shardType, l.FactArgs[0])
		}
	case "domain_expertise":
		if len(l.FactArgs) >= 1 {
			return fmt.Sprintf("[%s] Expertise: %v", shardType, l.FactArgs[0])
		}
	}
	return ""
}

// formatLearningAsHint converts a ShardLearning to a specialist hint.
func formatLearningAsHint(shardType string, l types.ShardLearning) string {
	prefix := fmt.Sprintf("[%s]", shardType)
	switch l.FactPredicate {
	case "tool_preference":
		if len(l.FactArgs) >= 2 {
			return fmt.Sprintf("%s For %v, use %v", prefix, l.FactArgs[0], l.FactArgs[1])
		}
	case "style_preference":
		if len(l.FactArgs) >= 1 {
			return fmt.Sprintf("%s Style preference: %v", prefix, l.FactArgs[0])
		}
	case "domain_expertise":
		if len(l.FactArgs) >= 1 {
			return fmt.Sprintf("%s Domain focus: %v", prefix, l.FactArgs[0])
		}
	case "preferred_pattern":
		if len(l.FactArgs) >= 1 {
			return fmt.Sprintf("%s Preferred approach: %v", prefix, l.FactArgs[0])
		}
	}
	return ""
}

// queryAllowedActions returns constitutionally permitted actions.
func (m *Model) queryAllowedActions() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("permitted")
	if err != nil {
		return nil
	}
	var actions []string
	for _, fact := range results {
		if len(fact.Args) > 0 {
			if action, ok := fact.Args[0].(string); ok {
				actions = append(actions, action)
			}
		}
	}
	return actions
}

// queryBlockedActions returns constitutionally forbidden actions.
func (m *Model) queryBlockedActions() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("forbidden")
	if err != nil {
		return nil
	}
	var actions []string
	for _, fact := range results {
		if len(fact.Args) > 0 {
			if action, ok := fact.Args[0].(string); ok {
				actions = append(actions, action)
			}
		}
	}
	return actions
}

// querySafetyWarnings returns active safety warnings from the kernel.
func (m *Model) querySafetyWarnings() []string {
	if m.kernel == nil {
		return nil
	}
	results, err := m.kernel.Query("safety_warning")
	if err != nil {
		return nil
	}
	var warnings []string
	for _, fact := range results {
		if len(fact.Args) > 0 {
			if warning, ok := fact.Args[0].(string); ok {
				warnings = append(warnings, warning)
			}
		}
	}
	return warnings
}

// =============================================================================
// GAP-004 FIX: MEMORY TIER QUERY HELPERS
// =============================================================================

// queryVectorMemory performs semantic search in the vector tier.
// Appends relevant past knowledge to KnowledgeAtoms.
func (m *Model) queryVectorMemory(ctx context.Context, sessionCtx *types.SessionContext) {
	// Use last user message as query for semantic search
	query := ""
	for i := len(m.history) - 1; i >= 0; i-- {
		if m.history[i].Role == "user" {
			query = m.history[i].Content
			break
		}
	}
	if query == "" || len(query) < 10 {
		return
	}

	// Limit query length for embedding
	if len(query) > 500 {
		query = query[:500]
	}

	// Semantic recall from vector store
	entries, err := m.localDB.VectorRecallSemantic(ctx, query, 5)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.Content != "" {
			// Format as knowledge atom: "[vector] content"
			atom := fmt.Sprintf("[memory] %s", truncateForContext(entry.Content, 200))
			sessionCtx.KnowledgeAtoms = append(sessionCtx.KnowledgeAtoms, atom)
		}
	}
}

// queryGraphMemory retrieves entity relationships for active files.
// Appends relationships to DependencyContext.
func (m *Model) queryGraphMemory(sessionCtx *types.SessionContext) {
	// Query graph for entities related to active files
	for _, file := range sessionCtx.ActiveFiles {
		if file == "" {
			continue
		}
		// Query outgoing relationships
		links, err := m.localDB.QueryLinks(file, "outgoing")
		if err != nil {
			continue
		}
		for _, link := range links {
			// Format as dependency: "file -> target (relation)"
			dep := fmt.Sprintf("%s -> %s (%s)", link.EntityA, link.EntityB, link.Relation)
			sessionCtx.DependencyContext = append(sessionCtx.DependencyContext, dep)
		}
		// Limit to 10 relationships per file
		if len(links) >= 10 {
			break
		}
	}

	// Limit total dependencies
	if len(sessionCtx.DependencyContext) > 30 {
		sessionCtx.DependencyContext = sessionCtx.DependencyContext[:30]
	}
}

// queryColdMemory retrieves persistent learned facts from cold storage.
// Appends high-priority facts to KnowledgeAtoms.
func (m *Model) queryColdMemory(sessionCtx *types.SessionContext) {
	// Query for high-priority persistent facts
	predicates := []string{"learned_preference", "project_pattern", "avoid_pattern"}

	for _, pred := range predicates {
		facts, err := m.localDB.LoadFacts(pred)
		if err != nil {
			continue
		}
		for _, fact := range facts {
			if fact.Priority >= 5 { // Only high-priority facts
				// Format as knowledge atom
				argsStr := ""
				for i, arg := range fact.Args {
					if i > 0 {
						argsStr += ", "
					}
					argsStr += fmt.Sprintf("%v", arg)
				}
				atom := fmt.Sprintf("[cold:%s] %s", fact.Predicate, argsStr)
				sessionCtx.KnowledgeAtoms = append(sessionCtx.KnowledgeAtoms, atom)
			}
		}
	}

	// Limit total knowledge atoms
	if len(sessionCtx.KnowledgeAtoms) > 50 {
		sessionCtx.KnowledgeAtoms = sessionCtx.KnowledgeAtoms[:50]
	}
}
