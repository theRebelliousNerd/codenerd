// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains the multi-shard review aggregator for orchestrated code reviews.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// reviewCommandOptions holds parsed flags for /review.
// It is shared between the Bubbletea command handler and the multi-shard aggregator.
type reviewCommandOptions struct {
	EnableEnhancement bool
	PassThroughFlags  []string
}

// AggregatedReview holds the combined results from all shards
type AggregatedReview struct {
	ID                 string
	Target             string
	Files              []string
	Participants       []string
	IsComplete         bool
	IncompleteReason   []string
	Summary            string
	Narrative          string
	FindingsByShard    map[string][]reviewer.ParsedFinding
	DeduplicatedList   []reviewer.ParsedFinding
	HolisticInsights   []string
	EnhancementSection string
	TotalFindings      int
	StartTime          time.Time
	Duration           time.Duration
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
func (m Model) spawnMultiShardReview(target string, opts reviewCommandOptions) tea.Cmd {
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

		// Normalize, filter, and cap files for review
		absFiles, relFiles := normalizeReviewFiles(files, m.workspace)
		if len(absFiles) == 0 {
			absFiles = files
			relFiles = files
		}

		logging.Shards("Resolved %d files for review (normalized to %d)", len(files), len(absFiles))

		// 2. Load agent registry
		registry := m.loadAgentRegistry()

		// 3. Match specialists BEFORE review
		specialists := reviewer.MatchSpecialistsForReview(ctx, absFiles, registry)
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

				logging.Get(logging.CategoryShards).Error("Shard %s failed attempt %d: %v", shardName, attempt, err)
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

		// Format base task for ReviewerShard using the resolved file list.
		// This keeps multi-shard and single-shard paths aligned.
		baseTask := fmt.Sprintf("review files:%s", strings.Join(relFiles, ","))
		if opts.EnableEnhancement {
			baseTask += " --andEnhance"
		}
		if len(opts.PassThroughFlags) > 0 {
			baseTask += " " + strings.Join(opts.PassThroughFlags, " ")
		}

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
				specTask := reviewer.BuildSpecialistTask(s, absFiles, knowledge)
				taskStr := reviewer.FormatSpecialistReviewTask(specTask)

				result := spawnWithRetry(s.AgentName, taskStr)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}(spec)
		}

		// Spawn Nemesis adversarial reviewer if enabled
		// Nemesis generates and executes attack scripts to find where code breaks
		if m.enableNemesisReview() {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Resolve target path for Nemesis
				nemesisTarget := target
				if !filepath.IsAbs(target) && target != "." && target != "codebase" {
					nemesisTarget = filepath.Join(m.workspace, target)
				} else if target == "." || target == "codebase" {
					nemesisTarget = m.workspace
				}

				// Format Nemesis task: "review:<target>"
				nemesisTask := fmt.Sprintf("review:%s", nemesisTarget)

				logging.Shards("Spawning Nemesis adversarial review: %s", nemesisTask)
				result := spawnWithRetry("nemesis", nemesisTask)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}()
		}

		// Wait for all shards
		wg.Wait()

		logging.Shards("All %d shards completed", len(results))

		// 6. Aggregate results
		agg := m.aggregateReviewResults(results, target, relFiles, startTime)

		// 7. Mark incomplete if any failures
		for _, r := range results {
			if r.Err != nil {
				agg.IsComplete = false
				agg.IncompleteReason = append(agg.IncompleteReason,
					fmt.Sprintf("%s: %v", r.Shard, r.Err))
			}
		}

		// 7.5 Generate a natural-language narrative summary for the user
		if m.client != nil {
			narrativeCtx, cancel := context.WithTimeout(ctx, 3*time.Minute) // Extended for large context
			agg.Narrative = m.generateReviewNarrative(narrativeCtx, &agg)
			cancel()
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

		// Capture enhancement suggestions from ReviewerShard output
		if result.Shard == "reviewer" && agg.EnhancementSection == "" {
			agg.EnhancementSection = extractEnhancementSection(result.Result)
		}
	}

	// Deduplicate by file:line (keep highest severity)
	agg.DeduplicatedList = deduplicateFindings(agg.FindingsByShard)

	// Generate holistic summary
	agg.Summary = generateHolisticSummary(&agg)
	agg.HolisticInsights = extractCrossShardInsights(agg.FindingsByShard)

	return agg
}

func (m Model) generateReviewNarrative(ctx context.Context, agg *AggregatedReview) string {
	if agg == nil || m.client == nil {
		return ""
	}

	promptInput := fmt.Sprintf(
		"Summarize the multi-shard code review of %s. Identify the biggest flaw if any. Provide next steps.",
		agg.Target,
	)
	reviewContext := formatReviewNarrativeContext(agg)

	narrative, err := m.interpretShardOutput(ctx, promptInput, "reviewer", "multi-shard review", reviewContext)
	if err != nil {
		logging.Get(logging.CategoryShards).Error("Narrative summary failed: %v", err)
		return ""
	}
	return narrative
}

func formatReviewNarrativeContext(agg *AggregatedReview) string {
	if agg == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Target: %s\n", agg.Target))
	if len(agg.Participants) > 0 {
		sb.WriteString(fmt.Sprintf("Participants: %s\n", strings.Join(agg.Participants, ", ")))
	}
	sb.WriteString(fmt.Sprintf("Files Reviewed: %d\n", len(agg.Files)))
	sb.WriteString(fmt.Sprintf("Total Findings: %d\n", agg.TotalFindings))
	sb.WriteString(fmt.Sprintf("Duration: %s\n", agg.Duration.Round(time.Second)))

	if len(agg.IncompleteReason) > 0 {
		sb.WriteString("Incomplete Reasons:\n")
		for _, reason := range agg.IncompleteReason {
			sb.WriteString(fmt.Sprintf("- %s\n", reason))
		}
	}

	if summary := strings.TrimSpace(agg.Summary); summary != "" {
		sb.WriteString("\nSummary:\n")
		sb.WriteString(summary)
		sb.WriteString("\n")
	}

	sb.WriteString("\nTop Findings (deduplicated):\n")
	sb.WriteString(formatNarrativeFindings(agg.DeduplicatedList, 12))

	if enhancement := strings.TrimSpace(agg.EnhancementSection); enhancement != "" {
		sb.WriteString("\nEnhancement Suggestions (excerpt):\n")
		sb.WriteString(trimPromptSection(enhancement, 800))
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatNarrativeFindings(findings []reviewer.ParsedFinding, limit int) string {
	if len(findings) == 0 {
		return "None\n"
	}

	ordered := make([]reviewer.ParsedFinding, 0, len(findings))
	ordered = append(ordered, findings...)

	severityRank := map[string]int{
		"critical": 4,
		"error":    3,
		"warning":  2,
		"info":     1,
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		left := strings.ToLower(ordered[i].Severity)
		right := strings.ToLower(ordered[j].Severity)
		if severityRank[left] != severityRank[right] {
			return severityRank[left] > severityRank[right]
		}
		if ordered[i].File != ordered[j].File {
			return ordered[i].File < ordered[j].File
		}
		return ordered[i].Line < ordered[j].Line
	})

	if limit <= 0 || limit > len(ordered) {
		limit = len(ordered)
	}

	var sb strings.Builder
	for i := 0; i < limit; i++ {
		f := ordered[i]
		line := ""
		if f.Line > 0 {
			line = fmt.Sprintf(":%d", f.Line)
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s%s - %s\n",
			strings.ToUpper(strings.TrimSpace(f.Severity)),
			strings.TrimSpace(f.File),
			line,
			strings.TrimSpace(f.Message),
		))
	}
	return sb.String()
}

func trimPromptSection(value string, maxLen int) string {
	trimmed := strings.TrimSpace(value)
	if maxLen <= 0 || len(trimmed) <= maxLen {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:maxLen]) + "..."
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

// normalizeReviewFiles converts mixed absolute/relative paths into parallel absolute and
// workspace-relative lists, filters to reviewable extensions, skips hidden/vendor dirs,
// and caps the result to 50 files. If filtering removes everything, it falls back to
// allowing all files so explicit non-code targets still get reviewed.
func normalizeReviewFiles(files []string, workspace string) ([]string, []string) {
	abs, rel := normalizeReviewFilesInternal(files, workspace, true)
	if len(abs) == 0 {
		return normalizeReviewFilesInternal(files, workspace, false)
	}
	return abs, rel
}

func normalizeReviewFilesInternal(files []string, workspace string, filterExt bool) ([]string, []string) {
	allowedExts := map[string]bool{
		".go":   true,
		".py":   true,
		".js":   true,
		".jsx":  true,
		".ts":   true,
		".tsx":  true,
		".rs":   true,
		".java": true,
		".c":    true,
		".cpp":  true,
		".h":    true,
		".mg":   true,
		".gl":   true,
	}

	skipDirs := []string{"vendor", "node_modules", ".git", ".nerd", "dist", "build"}
	seen := make(map[string]bool)
	var absOut, relOut []string

	for _, f := range files {
		if f == "" {
			continue
		}
		absPath := f
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(workspace, absPath)
		}
		absPath = filepath.Clean(absPath)

		// Skip hidden paths
		if strings.Contains(absPath, string(filepath.Separator)+".") {
			continue
		}

		// Skip common vendor/build dirs
		skip := false
		for _, dir := range skipDirs {
			if strings.Contains(absPath, string(filepath.Separator)+dir+string(filepath.Separator)) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Filter by extension if requested
		if filterExt {
			ext := strings.ToLower(filepath.Ext(absPath))
			if ext != "" && !allowedExts[ext] {
				continue
			}
		}

		relPath, err := filepath.Rel(workspace, absPath)
		if err != nil {
			relPath = absPath
		}
		if seen[relPath] {
			continue
		}
		seen[relPath] = true

		absOut = append(absOut, absPath)
		relOut = append(relOut, relPath)

		if len(absOut) >= 50 {
			break
		}
	}

	return absOut, relOut
}

// extractEnhancementSection pulls the "Enhancement Suggestions" markdown block
// from a ReviewerShard output so it can be displayed in aggregated reviews.
func extractEnhancementSection(output string) string {
	if output == "" {
		return ""
	}
	lower := strings.ToLower(output)
	marker := "## enhancement suggestions"
	idx := strings.Index(lower, marker)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(output[idx:])
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
		logging.Get(logging.CategoryShards).Error("Failed to parse agent registry: %v", err)
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

	// Narrative summary (LLM-interpreted)
	if strings.TrimSpace(review.Narrative) != "" {
		sb.WriteString("## Interpretation\n\n")
		sb.WriteString(strings.TrimSpace(review.Narrative))
		sb.WriteString("\n\n")
	}

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

	// Enhancement suggestions (from ReviewerShard) if present
	if strings.TrimSpace(review.EnhancementSection) != "" {
		sb.WriteString("\n---\n\n")
		sb.WriteString(strings.TrimSpace(review.EnhancementSection))
		sb.WriteString("\n")
	}

	return sb.String()
}

// Helper to get localDB from model (needs to be added to Model struct)
// For now, we'll use a method that accesses the workspace store
func (m Model) getLocalDB() *store.LocalStore {
	return m.localDB
}

// enableNemesisReview checks if Nemesis adversarial review should run.
// Nemesis runs attack scripts against the code to find vulnerabilities.
// It can be enabled via:
// 1. NEMESIS_REVIEW=1 environment variable
// 2. .nerd/config.json with "nemesis_review": true
// 3. Default: enabled for Go projects (detected by presence of go.mod)
func (m Model) enableNemesisReview() bool {
	// Check environment variable override
	if env := os.Getenv("NEMESIS_REVIEW"); env != "" {
		return env == "1" || env == "true" || env == "yes"
	}

	// Check if disabled via environment
	if env := os.Getenv("NEMESIS_REVIEW"); env == "0" || env == "false" || env == "no" {
		return false
	}

	// Check if nemesis shard is registered in shard manager
	if m.shardMgr != nil {
		// Try to spawn - if it fails, shard isn't available
		// We'll check registration differently
	}

	// Check for go.mod (Nemesis currently works best with Go code)
	goModPath := filepath.Join(m.workspace, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		// Go project detected - enable Nemesis by default
		logging.Shards("Nemesis review enabled (Go project detected)")
		return true
	}

	// Default: disabled for non-Go projects
	return false
}
