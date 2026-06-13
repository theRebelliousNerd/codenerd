package campaign

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/world"
)

// =============================================================================
// INDIVIDUAL GATHERING METHODS
// =============================================================================

func (g *IntelligenceGatherer) gatherWorldModel(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherWorldModel")
	defer timer.Stop()

	ctx, cancel := context.WithTimeout(ctx, g.config.PerSystemTimeout)
	defer cancel()

	// Determine root path for scanning
	root := "."
	if len(paths) > 0 {
		root = paths[0] // Use first path as root
	}

	facts, err := g.worldScanner.ScanWorkspaceCtx(ctx, root)
	if err != nil {
		addError(fmt.Sprintf("World model scan failed: %v", err))
		return
	}

	report.WorldFacts = facts

	// Parse facts into structured data
	for _, fact := range facts {
		switch fact.Predicate {
		case "file_topology":
			if len(fact.Args) >= 5 {
				path, _ := fact.Args[0].(string)
				hash, _ := fact.Args[1].(string)
				lang := g.parseAtom(fact.Args[2])
				modTime, _ := fact.Args[3].(int64)
				isTest := g.parseAtom(fact.Args[4]) == "/true"

				report.FileTopology[path] = FileInfo{
					Path:         path,
					Hash:         hash,
					Language:     strings.TrimPrefix(lang, "/"),
					IsTestFile:   isTest,
					LastModified: time.Unix(modTime, 0),
				}
				report.LanguageBreakdown[strings.TrimPrefix(lang, "/")]++
			}
		case "symbol_graph", "code_defines":
			if len(fact.Args) >= 4 {
				symbol := SymbolInfo{
					File:     g.parseArg(fact.Args[0]),
					Name:     g.parseArg(fact.Args[1]),
					Kind:     g.parseArg(fact.Args[2]),
					Exported: g.parseArg(fact.Args[3]) == "exported",
				}
				if len(fact.Args) >= 5 {
					if line, ok := fact.Args[4].(int); ok {
						symbol.Line = line
					}
				}
				report.SymbolGraph = append(report.SymbolGraph, symbol)
			}
		}
	}

	logging.CampaignDebug("World model gathered: %d files, %d symbols", len(report.FileTopology), len(report.SymbolGraph))
}

func (g *IntelligenceGatherer) gatherGitHistory(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherGitHistory")
	defer timer.Stop()

	ctx, cancel := context.WithTimeout(ctx, g.config.PerSystemTimeout)
	defer cancel()

	root := "."
	if len(paths) > 0 {
		root = paths[0]
	}

	facts, err := world.ScanGitHistory(ctx, root, g.config.GitHistoryDepth)
	if err != nil {
		addError(fmt.Sprintf("Git history scan failed: %v", err))
		return
	}

	// Parse git facts
	churnMap := make(map[string]int)
	for _, fact := range facts {
		switch fact.Predicate {
		case "churn_rate":
			if len(fact.Args) >= 2 {
				path, _ := fact.Args[0].(string)
				rate, _ := fact.Args[1].(int)
				churnMap[path] = rate
			}
		case "git_history":
			if len(fact.Args) >= 5 {
				commit := CommitInfo{
					Files:   []string{g.parseArg(fact.Args[0])},
					Hash:    g.parseArg(fact.Args[1]),
					Author:  g.parseArg(fact.Args[2]),
					Message: g.parseArg(fact.Args[4]),
				}
				if ts, ok := fact.Args[3].(int64); ok {
					commit.Time = time.Unix(ts, 0)
				}
				report.RecentCommits = append(report.RecentCommits, commit)
			}
		}
	}

	// Convert churn map to hotspots with Chesterton's Fence warnings
	for path, rate := range churnMap {
		hotspot := ChurnHotspot{
			Path:      path,
			ChurnRate: rate,
		}
		if rate > 10 {
			hotspot.Reason = "High churn rate"
			hotspot.Warning = fmt.Sprintf("⚠️ CHESTERTON'S FENCE: This file has been modified %d times. Understand WHY before changing it.", rate)
			report.HighChurnFiles = append(report.HighChurnFiles, path)
		} else if rate > 5 {
			hotspot.Reason = "Moderate churn rate"
			hotspot.Warning = "Consider reviewing recent changes before modification."
		}
		report.GitChurnHotspots = append(report.GitChurnHotspots, hotspot)
	}

	// Sort by churn rate descending
	sort.Slice(report.GitChurnHotspots, func(i, j int) bool {
		return report.GitChurnHotspots[i].ChurnRate > report.GitChurnHotspots[j].ChurnRate
	})

	// Limit to configured max
	if len(report.GitChurnHotspots) > g.config.MaxChurnHotspots {
		report.GitChurnHotspots = report.GitChurnHotspots[:g.config.MaxChurnHotspots]
	}

	logging.CampaignDebug("Git history gathered: %d churn hotspots, %d high-churn files",
		len(report.GitChurnHotspots), len(report.HighChurnFiles))
}

func (g *IntelligenceGatherer) gatherLearningPatterns(ctx context.Context, report *IntelligenceReport, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherLearningPatterns")
	defer timer.Stop()

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		addError(fmt.Sprintf("Learning patterns cancelled: %v", err))
		return
	}

	shardTypes := []string{"coder", "tester", "reviewer", "researcher"}

	for _, shardType := range shardTypes {
		learnings, err := g.learningStore.Load(shardType)
		if err != nil {
			addError(fmt.Sprintf("Learning store load failed for %s: %v", shardType, err))
			continue
		}

		for _, learning := range learnings {
			pattern := LearningPattern{
				ShardType:   shardType,
				Predicate:   learning.FactPredicate,
				Confidence:  learning.Confidence,
				LastUsed:    time.Unix(learning.Timestamp, 0),
				Description: g.formatLearningDescription(learning),
			}
			report.HistoricalPatterns = append(report.HistoricalPatterns, pattern)
		}
	}

	// Sort by confidence descending
	sort.Slice(report.HistoricalPatterns, func(i, j int) bool {
		return report.HistoricalPatterns[i].Confidence > report.HistoricalPatterns[j].Confidence
	})

	// Limit
	if len(report.HistoricalPatterns) > g.config.MaxLearnings {
		report.HistoricalPatterns = report.HistoricalPatterns[:g.config.MaxLearnings]
	}

	logging.CampaignDebug("Learning patterns gathered: %d patterns", len(report.HistoricalPatterns))
}

func (g *IntelligenceGatherer) gatherKnowledgeGraph(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherKnowledgeGraph")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		addError(fmt.Sprintf("Knowledge graph gathering cancelled: %v", err))
		return
	}

	// Query for entities related to target paths
	for _, path := range paths {
		// Check context cancellation inside the loop to bail early on large path sets
		if err := ctx.Err(); err != nil {
			addError(fmt.Sprintf("Knowledge graph gathering cancelled mid-loop: %v", err))
			break
		}
		if strings.TrimSpace(path) == "" {
			continue // skip empty/whitespace paths
		}
		links, err := g.localStore.QueryLinks(path, "both")
		if err != nil {
			addError(fmt.Sprintf("Knowledge graph query failed for %s: %v", path, err))
			continue
		}
		report.KnowledgeLinks = append(report.KnowledgeLinks, links...)
	}

	// Cluster entities by relation
	clusterMap := make(map[string][]string)
	for _, link := range report.KnowledgeLinks {
		key := link.Relation
		clusterMap[key] = append(clusterMap[key], link.EntityB)
	}

	for relation, entities := range clusterMap {
		report.EntityClusters = append(report.EntityClusters, EntityCluster{
			ClusterID: relation,
			Entities:  entities,
			Relation:  relation,
		})
	}

	logging.CampaignDebug("Knowledge graph gathered: %d links, %d clusters",
		len(report.KnowledgeLinks), len(report.EntityClusters))
}

func (g *IntelligenceGatherer) gatherColdStorage(ctx context.Context, report *IntelligenceReport, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherColdStorage")
	defer timer.Stop()

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		addError(fmt.Sprintf("Cold storage gathering cancelled: %v", err))
		return
	}

	// Load campaign-relevant predicates from cold storage
	predicates := []string{
		"preference_signal",
		"style_preference",
		"avoid_pattern",
		"learned_pattern",
	}

	for _, pred := range predicates {
		facts, err := g.localStore.LoadFacts(pred)
		if err != nil {
			addError(fmt.Sprintf("Cold storage load failed for %s: %v", pred, err))
			continue
		}
		report.ColdStorageFacts = append(report.ColdStorageFacts, facts...)
	}

	logging.CampaignDebug("Cold storage gathered: %d facts", len(report.ColdStorageFacts))
}

func (g *IntelligenceGatherer) gatherSafetyWarnings(ctx context.Context, report *IntelligenceReport, goal string, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherSafetyWarnings")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		addError(fmt.Sprintf("Safety warning gathering cancelled: %v", err))
		return
	}

	// Query kernel for any pre-existing safety constraints
	facts, err := g.kernel.Query("blocked_action")
	if err != nil {
		logging.CampaignDebug("No blocked_action facts: %v", err)
		// Not an error, just means no pre-existing blocks
	}

	for _, fact := range facts {
		if len(fact.Args) >= 2 {
			report.BlockedActions = append(report.BlockedActions, g.parseArg(fact.Args[0]))
		}
	}

	// Query for safety_warning predicate
	safetyFacts, err := g.kernel.Query("safety_warning")
	if err == nil {
		for _, fact := range safetyFacts {
			if len(fact.Args) >= 4 {
				warning := SafetyWarning{
					Path:         g.parseArg(fact.Args[0]),
					Action:       g.parseArg(fact.Args[1]),
					RuleViolated: g.parseArg(fact.Args[2]),
					Severity:     g.parseArg(fact.Args[3]),
				}
				report.SafetyWarnings = append(report.SafetyWarnings, warning)
			}
		}
	}

	// Check for dangerous patterns in goal
	dangerousPatterns := []string{"rm -rf", "drop database", "delete *", "format c:"}
	goalLower := strings.ToLower(goal)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(goalLower, pattern) {
			report.SafetyWarnings = append(report.SafetyWarnings, SafetyWarning{
				Action:       pattern,
				RuleViolated: "dangerous_pattern",
				Severity:     "critical",
				Remediation:  "Review and confirm this action is intentional",
			})
		}
	}

	logging.CampaignDebug("Safety check: %d warnings, %d blocked actions",
		len(report.SafetyWarnings), len(report.BlockedActions))
}

func (g *IntelligenceGatherer) gatherMCPTools(ctx context.Context, report *IntelligenceReport, goal string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherMCPTools")
	defer timer.Stop()

	ctx, cancel := context.WithTimeout(ctx, g.config.PerSystemTimeout)
	defer cancel()

	// Get all available MCP tools
	tools, err := g.mcpStore.GetAllTools(ctx)
	if err != nil {
		addError(fmt.Sprintf("MCP tool fetch failed: %v", err))
		return
	}

	for _, tool := range tools {
		if tool == nil {
			continue
		}
		toolInfo := MCPToolInfo{
			ToolID:      tool.ToolID,
			ServerID:    tool.ServerID,
			Name:        tool.Name,
			Description: tool.Description,
			Categories:  tool.Categories,
			Affinity:    g.calculateToolAffinity(tool, goal),
		}
		report.MCPToolsAvailable = append(report.MCPToolsAvailable, toolInfo)
	}

	// Sort by affinity
	sort.Slice(report.MCPToolsAvailable, func(i, j int) bool {
		return report.MCPToolsAvailable[i].Affinity > report.MCPToolsAvailable[j].Affinity
	})

	// Limit
	if len(report.MCPToolsAvailable) > g.config.MaxMCPTools {
		report.MCPToolsAvailable = report.MCPToolsAvailable[:g.config.MaxMCPTools]
	}

	// Get server status
	servers, err := g.mcpStore.GetAllServers(ctx)
	if err == nil {
		for _, server := range servers {
			report.MCPServerStatus[server.ID] = string(server.Status)
		}
	}

	logging.CampaignDebug("MCP tools gathered: %d tools, %d servers",
		len(report.MCPToolsAvailable), len(report.MCPServerStatus))
}

func (g *IntelligenceGatherer) gatherPreviousCampaigns(ctx context.Context, report *IntelligenceReport, goal string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherPreviousCampaigns")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		addError(fmt.Sprintf("Previous campaign gathering cancelled: %v", err))
		return
	}
	// Logging usage of goal to silence unused warning
	logging.CampaignDebug("Gathering previous campaigns for goal: %s", goal)

	// Query kernel for campaign artifacts
	facts, err := g.kernel.Query("campaign_completed")
	if err != nil {
		logging.CampaignDebug("No previous campaigns: %v", err)
		return
	}

	for _, fact := range facts {
		if len(fact.Args) >= 4 {
			artifact := CampaignArtifact{
				CampaignID:  g.parseArg(fact.Args[0]),
				Goal:        g.parseArg(fact.Args[1]),
				TaskCount:   g.parseIntArg(fact.Args[2]),
				SuccessRate: g.parseFloatArg(fact.Args[3]),
			}
			report.PreviousCampaigns = append(report.PreviousCampaigns, artifact)
		}
	}

	// Limit to most recent
	if len(report.PreviousCampaigns) > g.config.MaxPreviousCampaigns {
		report.PreviousCampaigns = report.PreviousCampaigns[:g.config.MaxPreviousCampaigns]
	}

	logging.CampaignDebug("Previous campaigns gathered: %d campaigns", len(report.PreviousCampaigns))
}

func (g *IntelligenceGatherer) gatherTestCoverage(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherTestCoverage")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		addError(fmt.Sprintf("Test coverage gathering cancelled: %v", err))
		return
	}

	// Query kernel for test_coverage facts
	facts, err := g.kernel.Query("test_coverage")
	if err != nil {
		logging.CampaignDebug("No test coverage facts: %v", err)
		return
	}

	for _, fact := range facts {
		if len(fact.Args) >= 2 {
			path := g.parseArg(fact.Args[0])
			coverage := g.parseFloatArg(fact.Args[1])
			report.TestCoverage[path] = coverage
			if coverage < 0.5 {
				report.UncoveredPaths = append(report.UncoveredPaths, path)
			}
		}
	}

	// Also check for test files corresponding to target paths
	for _, path := range paths {
		testPath := strings.TrimSuffix(path, ".go") + "_test.go"
		if _, ok := report.FileTopology[testPath]; !ok {
			report.UncoveredPaths = append(report.UncoveredPaths, path+" (no test file)")
		}
	}

	logging.CampaignDebug("Test coverage gathered: %d entries, %d uncovered",
		len(report.TestCoverage), len(report.UncoveredPaths))
}

func (g *IntelligenceGatherer) gatherCodePatterns(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherCodePatterns")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		addError(fmt.Sprintf("Code pattern gathering cancelled: %v", err))
		return
	}
	logging.CampaignDebug("Gathering code patterns for %d paths", len(paths))

	// Query kernel for detected patterns
	patternPredicates := []string{"design_pattern", "anti_pattern", "architecture_pattern"}
	for _, pred := range patternPredicates {
		facts, err := g.kernel.Query(pred)
		if err != nil {
			continue
		}
		for _, fact := range facts {
			if len(fact.Args) >= 3 {
				pattern := CodePattern{
					Name:        g.parseArg(fact.Args[0]),
					Type:        pred,
					Description: g.parseArg(fact.Args[2]),
				}
				if len(fact.Args) >= 4 {
					pattern.Confidence = g.parseFloatArg(fact.Args[3])
				}
				report.CodePatterns = append(report.CodePatterns, pattern)
			}
		}
	}

	// Detect architecture hints from file structure
	if len(report.FileTopology) > 0 {
		hints := g.detectArchitectureHints(report.FileTopology)
		report.ArchitectureHints = hints
	}

	logging.CampaignDebug("Code patterns gathered: %d patterns, %d architecture hints",
		len(report.CodePatterns), len(report.ArchitectureHints))
}

func (g *IntelligenceGatherer) gatherToolGaps(ctx context.Context, report *IntelligenceReport, goal string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherToolGaps")
	defer timer.Stop()

	if g.toolGenerator == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, g.config.PerSystemTimeout)
	defer cancel()

	// Detect tool needs based on goal
	need, err := g.toolGenerator.DetectToolNeed(ctx, goal, "")
	if err != nil {
		addError(fmt.Sprintf("Tool gap detection failed: %v", err))
		return
	}

	if need != nil {
		report.ToolGaps = append(report.ToolGaps, *need)
		report.MissingCapabilities = append(report.MissingCapabilities, need.Purpose)
	}

	logging.CampaignDebug("Tool gaps gathered: %d gaps", len(report.ToolGaps))
}

func (g *IntelligenceGatherer) gatherShardAdvice(ctx context.Context, report *IntelligenceReport, goal string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherShardAdvice")
	defer timer.Stop()

	if g.consultation == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, g.config.ConsultTimeout)
	defer cancel()

	// Build context from already-gathered intelligence
	contextBuilder := strings.Builder{}
	contextBuilder.WriteString(fmt.Sprintf("Campaign Goal: %s\n\n", goal))

	if len(report.HighChurnFiles) > 0 {
		contextBuilder.WriteString("High Churn Files:\n")
		for _, f := range report.HighChurnFiles[:min(5, len(report.HighChurnFiles))] {
			contextBuilder.WriteString(fmt.Sprintf("- %s\n", f))
		}
		contextBuilder.WriteString("\n")
	}

	if len(report.SafetyWarnings) > 0 {
		contextBuilder.WriteString("Safety Warnings:\n")
		for _, w := range report.SafetyWarnings[:min(3, len(report.SafetyWarnings))] {
			contextBuilder.WriteString(fmt.Sprintf("- %s: %s\n", w.Action, w.RuleViolated))
		}
		contextBuilder.WriteString("\n")
	}

	// Consult domain experts
	request := BatchConsultRequest{
		Topic:      "Campaign Planning Review",
		Question:   fmt.Sprintf("Review this campaign plan and provide your expert advice. What should we be careful about? What patterns should we follow? What are the risks?\n\nGoal: %s", goal),
		Context:    contextBuilder.String(),
		TargetSpec: []string{"coder", "tester", "reviewer", "researcher"},
	}

	responses, err := g.consultation.RequestBatchConsultation(ctx, request)
	if err != nil {
		addError(fmt.Sprintf("Shard consultation failed: %v", err))
		return
	}

	report.ShardAdvice = responses

	// Synthesize advisory summary
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString("## Advisory Summary\n\n")
	for _, resp := range responses {
		if resp.Confidence > 0.5 {
			summaryBuilder.WriteString(fmt.Sprintf("**%s** (%.0f%% confidence): %s\n\n",
				resp.FromSpec, resp.Confidence*100, g.truncateAdvice(resp.Advice, 200)))
		}
	}
	report.AdvisorySummary = summaryBuilder.String()

	logging.CampaignDebug("Shard advice gathered: %d responses", len(report.ShardAdvice))
}
