package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
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
func (m *Model) buildSessionContext(ctx context.Context) *core.SessionContext {
	sessionCtx := &core.SessionContext{
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
func (m *Model) populateGitContext(sessionCtx *core.SessionContext) {
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
func (m *Model) populateTestState(sessionCtx *core.SessionContext) {
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
			if status == "/pass" {
				passCount++
			} else if status == "/fail" {
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
func (m *Model) buildPriorShardSummaries() []core.ShardSummary {
	var summaries []core.ShardSummary
	for _, sr := range m.shardResultHistory {
		summaries = append(summaries, core.ShardSummary{
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

// queryKnowledgeAtoms retrieves relevant knowledge from learning store.
func (m *Model) queryKnowledgeAtoms() []string {
	// TODO: Implement query to learning store for domain knowledge
	// For now, return empty
	return nil
}

// querySpecialistHints retrieves specialist-specific hints.
func (m *Model) querySpecialistHints() []string {
	// TODO: Implement query to learning store for specialist hints
	// For now, return empty
	return nil
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

