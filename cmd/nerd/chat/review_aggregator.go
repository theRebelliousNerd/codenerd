// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains the multi-shard review aggregator for orchestrated code reviews.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/shards/reviewer"
	"codenerd/internal/store"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// MULTI-SHARD REVIEW AGGREGATOR
// =============================================================================
// Orchestrates parallel reviews from multiple specialist shards and aggregates results.

// AggregatedReview holds the combined results from all shards
type AggregatedReview struct {
	ID               string
	Target           string
	Files            []string
	Participants     []string
	IsComplete       bool
	IncompleteReason []string
	Summary          string
	FindingsByShard  map[string][]reviewer.ParsedFinding
	DeduplicatedList []reviewer.ParsedFinding
	HolisticInsights []string
	TotalFindings    int
	StartTime        time.Time
	Duration         time.Duration
}

// ShardReviewResult holds the result from a single shard
type ShardReviewResult struct {
	Shard    string
	Result   string
	Err      error
	Attempt  int
	Duration time.Duration
}

// multiShardReviewMsg is the message type for multi-shard review completion
type multiShardReviewMsg struct {
	review *AggregatedReview
	err    error
}

// spawnMultiShardReview orchestrates a parallel multi-shard review.
// It spawns ReviewerShard + matching specialists, collects results, and aggregates.
func (m Model) spawnMultiShardReview(target string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background() // No hard timeout - wait for all shards
		startTime := time.Now()

		logging.Shards("Starting multi-shard review for: %s", target)

		// 1. Resolve files to review
		files := m.resolveReviewTarget(target)
		if len(files) == 0 {
			// Fall back to single file if target is a path
			if _, err := os.Stat(target); err == nil {
				files = []string{target}
			} else if _, err := os.Stat(filepath.Join(m.workspace, target)); err == nil {
				files = []string{filepath.Join(m.workspace, target)}
			}
		}

		logging.Shards("Resolved %d files for review", len(files))

		// 2. Load agent registry
		registry := m.loadAgentRegistry()

		// 3. Match specialists BEFORE review
		specialists := reviewer.MatchSpecialistsForReview(ctx, files, registry)
		logging.Shards("Matched %d specialists for review", len(specialists))

		// 4. Track results and failures
		var mu sync.Mutex
		results := make([]ShardReviewResult, 0)

		// Spawn with retry logic (retry once on failure)
		spawnWithRetry := func(shardName, task string) ShardReviewResult {
			for attempt := 1; attempt <= 2; attempt++ {
				spawnStart := time.Now()
				result, err := m.shardMgr.Spawn(ctx, shardName, task)
				duration := time.Since(spawnStart)

				if err == nil {
					logging.Shards("Shard %s completed (attempt %d, %v)", shardName, attempt, duration)
					return ShardReviewResult{
						Shard:    shardName,
						Result:   result,
						Err:      nil,
						Attempt:  attempt,
						Duration: duration,
					}
				}

				logging.Shards("Shard %s failed attempt %d: %v", shardName, attempt, err)
				if attempt == 1 {
					time.Sleep(500 * time.Millisecond) // Brief pause before retry
				}
			}

			return ShardReviewResult{
				Shard:   shardName,
				Result:  "",
				Err:     fmt.Errorf("failed after 2 attempts"),
				Attempt: 2,
			}
		}

		// 5. Spawn all shards in parallel
		var wg sync.WaitGroup

		// Format base task for ReviewerShard
		baseTask := formatShardTask("/review", target, "", m.workspace)

		// Always spawn ReviewerShard
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := spawnWithRetry("reviewer", baseTask)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}()

		// Spawn matching specialists
		for _, spec := range specialists {
			wg.Add(1)
			go func(s reviewer.SpecialistMatch) {
				defer wg.Done()

				// Load knowledge base for this specialist
				knowledge, err := reviewer.LoadAndQueryKnowledgeBase(ctx, s.KnowledgePath, s.Files)
				if err != nil {
					logging.Shards("Warning: Failed to load KB for %s: %v", s.AgentName, err)
				}

				// Build specialist task
				specTask := reviewer.BuildSpecialistTask(s, files, knowledge)
				taskStr := reviewer.FormatSpecialistReviewTask(specTask)

				result := spawnWithRetry(s.AgentName, taskStr)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}(spec)
		}

		// Wait for all shards
		wg.Wait()

		logging.Shards("All %d shards completed", len(results))

		// 6. Aggregate results
		agg := m.aggregateReviewResults(results, target, files, startTime)

		// 7. Mark incomplete if any failures
		for _, r := range results {
			if r.Err != nil {
				agg.IsComplete = false
				agg.IncompleteReason = append(agg.IncompleteReason,
					fmt.Sprintf("%s: %v", r.Shard, r.Err))
			}
		}

		// 8. Persist to knowledge DB
		if m.localDB == nil {
			logging.Shards("Skipping persistence: localDB is nil")
		}
		if m.localDB != nil {
			persistedReview := &reviewer.PersistedReview{
				ID:               agg.ID,
				Timestamp:        agg.StartTime,
				Target:           agg.Target,
				Files:            agg.Files,
				Participants:     agg.Participants,
				IsComplete:       agg.IsComplete,
				IncompleteReason: agg.IncompleteReason,
				Summary:          agg.Summary,
				FindingsByShard:  agg.FindingsByShard,
				HolisticInsights: agg.HolisticInsights,
				TotalFindings:    agg.TotalFindings,
				Duration:         agg.Duration,
			}
			if err := reviewer.PersistReview(ctx, m.localDB, persistedReview); err != nil {
				logging.Shards("Warning: Failed to persist review: %v", err)
			}

			// Export to markdown
			reviewsDir := filepath.Join(m.workspace, ".nerd", "reviews")
			if exportPath, err := reviewer.ExportReviewToMarkdown(persistedReview, reviewsDir); err == nil {
				logging.Shards("Review exported to: %s", exportPath)
			}
		}

		// 9. Format final response
		return multiShardReviewMsg{review: &agg, err: nil}
	}
}

// aggregateReviewResults combines results from all shards
func (m Model) aggregateReviewResults(results []ShardReviewResult, target string, files []string, startTime time.Time) AggregatedReview {
	agg := AggregatedReview{
		ID:              fmt.Sprintf("review-%d", time.Now().UnixNano()),
		Target:          target,
		Files:           files,
		StartTime:       startTime,
		Duration:        time.Since(startTime),
		IsComplete:      true,
		FindingsByShard: make(map[string][]reviewer.ParsedFinding),
		Participants:    make([]string, 0),
	}

	// Parse findings from each shard
	for _, result := range results {
		agg.Participants = append(agg.Participants, result.Shard)

		if result.Err != nil {
			continue
		}

		// Parse the shard's output
		findings := reviewer.ParseShardOutput(result.Result, result.Shard)
		agg.FindingsByShard[result.Shard] = findings
		agg.TotalFindings += len(findings)
	}

	// Deduplicate by file:line (keep highest severity)
	agg.DeduplicatedList = deduplicateFindings(agg.FindingsByShard)

	// Generate holistic summary
	agg.Summary = generateHolisticSummary(&agg)
	agg.HolisticInsights = extractCrossShardInsights(agg.FindingsByShard)

	return agg
}

// deduplicateFindings removes duplicate findings, keeping highest severity
func deduplicateFindings(findingsByShard map[string][]reviewer.ParsedFinding) []reviewer.ParsedFinding {
	// Key: file:line
	seen := make(map[string]reviewer.ParsedFinding)

	severityRank := map[string]int{
		"critical": 4,
		"error":    3,
		"warning":  2,
		"info":     1,
	}

	for _, findings := range findingsByShard {
		for _, f := range findings {
			key := fmt.Sprintf("%s:%d", f.File, f.Line)

			existing, exists := seen[key]
			if !exists {
				seen[key] = f
				continue
			}

			// Keep higher severity
			existingRank := severityRank[strings.ToLower(existing.Severity)]
			newRank := severityRank[strings.ToLower(f.Severity)]
			if newRank > existingRank {
				seen[key] = f
			}
		}
	}

	// Convert to slice
	result := make([]reviewer.ParsedFinding, 0, len(seen))
	for _, f := range seen {
		result = append(result, f)
	}

	return result
}

// generateHolisticSummary creates a summary of the review
func generateHolisticSummary(agg *AggregatedReview) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Multi-shard review of %s completed.\n\n", agg.Target))

	// Count by severity
	severityCounts := make(map[string]int)
	for _, findings := range agg.FindingsByShard {
		for _, f := range findings {
			sev := strings.ToLower(f.Severity)
			if sev == "" {
				sev = "info"
			}
			severityCounts[sev]++
		}
	}

	sb.WriteString("**Findings Summary**:\n")
	if count := severityCounts["critical"]; count > 0 {
		sb.WriteString(fmt.Sprintf("- Critical: %d\n", count))
	}
	if count := severityCounts["error"]; count > 0 {
		sb.WriteString(fmt.Sprintf("- Error: %d\n", count))
	}
	if count := severityCounts["warning"]; count > 0 {
		sb.WriteString(fmt.Sprintf("- Warning: %d\n", count))
	}
	if count := severityCounts["info"]; count > 0 {
		sb.WriteString(fmt.Sprintf("- Info: %d\n", count))
	}

	sb.WriteString(fmt.Sprintf("\n**Participants**: %s\n", strings.Join(agg.Participants, ", ")))
	sb.WriteString(fmt.Sprintf("**Files Reviewed**: %d\n", len(agg.Files)))
	sb.WriteString(fmt.Sprintf("**Duration**: %s\n", agg.Duration.Round(time.Second)))

	return sb.String()
}

// extractCrossShardInsights finds patterns across multiple shards
func extractCrossShardInsights(findingsByShard map[string][]reviewer.ParsedFinding) []string {
	var insights []string

	// Count files with findings from multiple shards
	fileShards := make(map[string][]string)
	for shard, findings := range findingsByShard {
		for _, f := range findings {
			if f.File != "" {
				fileShards[f.File] = append(fileShards[f.File], shard)
			}
		}
	}

	// Identify hot spots (files with findings from 2+ shards)
	for file, shards := range fileShards {
		unique := make(map[string]bool)
		for _, s := range shards {
			unique[s] = true
		}
		if len(unique) >= 2 {
			shardNames := make([]string, 0, len(unique))
			for s := range unique {
				shardNames = append(shardNames, s)
			}
			insights = append(insights,
				fmt.Sprintf("Hot spot: %s flagged by multiple specialists (%s)",
					file, strings.Join(shardNames, ", ")))
		}
	}

	// Count severity across all shards
	totalCritical := 0
	totalError := 0
	for _, findings := range findingsByShard {
		for _, f := range findings {
			switch strings.ToLower(f.Severity) {
			case "critical":
				totalCritical++
			case "error":
				totalError++
			}
		}
	}

	if totalCritical > 0 {
		insights = append(insights,
			fmt.Sprintf("Attention: %d critical issues require immediate attention", totalCritical))
	}
	if totalError > 3 {
		insights = append(insights,
			fmt.Sprintf("Pattern: Multiple error-level issues (%d) suggest systemic problems", totalError))
	}

	// Specialist-specific insights
	if len(findingsByShard) > 2 {
		insights = append(insights,
			fmt.Sprintf("Cross-domain review: %d specialists provided independent analysis", len(findingsByShard)))
	}

	return insights
}

// resolveReviewTarget converts a review target to a list of files
func (m Model) resolveReviewTarget(target string) []string {
	var files []string

	// Handle "." or "codebase" or empty
	if target == "." || target == "codebase" || target == "" {
		return discoverFiles(m.workspace, "")
	}

	// Handle explicit file path
	fullPath := target
	if !filepath.IsAbs(target) {
		fullPath = filepath.Join(m.workspace, target)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		// Try relative to workspace
		return nil
	}

	if info.IsDir() {
		// Walk directory
		filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			// Skip hidden and vendor
			if strings.Contains(path, "/.") || strings.Contains(path, "\\.") ||
				strings.Contains(path, "vendor") || strings.Contains(path, "node_modules") {
				return nil
			}
			files = append(files, path)
			return nil
		})
	} else {
		files = []string{fullPath}
	}

	return files
}

// loadAgentRegistry loads the agent registry from .nerd/agents.json
func (m Model) loadAgentRegistry() *reviewer.AgentRegistry {
	registryPath := filepath.Join(m.workspace, ".nerd", "agents.json")

	data, err := os.ReadFile(registryPath)
	if err != nil {
		logging.Shards("No agent registry found at %s", registryPath)
		return nil
	}

	var registry reviewer.AgentRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		logging.Shards("Failed to parse agent registry: %v", err)
		return nil
	}

	logging.Shards("Loaded agent registry with %d agents", len(registry.Agents))
	return &registry
}

// formatMultiShardResponse formats the aggregated review for display
func formatMultiShardResponse(review *AggregatedReview) string {
	var sb strings.Builder

	// Header
	sb.WriteString(reviewer.FormatMultiShardReviewHeader(review.Target, review.Participants, review.IsComplete))

	// Summary
	sb.WriteString("## Summary\n\n")
	sb.WriteString(review.Summary)
	sb.WriteString("\n")

	// Holistic insights
	if len(review.HolisticInsights) > 0 {
		sb.WriteString("## Cross-Shard Insights\n\n")
		for _, insight := range review.HolisticInsights {
			sb.WriteString(fmt.Sprintf("- %s\n", insight))
		}
		sb.WriteString("\n")
	}

	// Incomplete reasons
	if !review.IsComplete && len(review.IncompleteReason) > 0 {
		sb.WriteString("## Incomplete Reasons\n\n")
		for _, reason := range review.IncompleteReason {
			sb.WriteString(fmt.Sprintf("- %s\n", reason))
		}
		sb.WriteString("\n")
	}

	// Findings by shard
	sb.WriteString("## Findings by Specialist\n\n")
	for shard, findings := range review.FindingsByShard {
		sb.WriteString(reviewer.FormatShardSection(shard, findings))
	}

	return sb.String()
}

// Helper to get localDB from model (needs to be added to Model struct)
// For now, we'll use a method that accesses the workspace store
func (m Model) getLocalDB() *store.LocalStore {
	return m.localDB
}
