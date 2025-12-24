// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains the core review operation implementations.
package reviewer

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// =============================================================================
// REVIEW OPERATIONS
// =============================================================================

// reviewFiles performs a comprehensive review of the specified files.
func (r *ReviewerShard) reviewFiles(ctx context.Context, task *ReviewerTask) (*ReviewResult, error) {
	logging.Reviewer("Starting file review: %d files", len(task.Files))
	logging.ReviewerDebug("Files to review: %v", task.Files)
	startTime := time.Now()
	result := &ReviewResult{
		Files:    task.Files,
		Findings: make([]ReviewFinding, 0),
		Severity: ReviewSeverityClean,
	}

	// Collect file contents for specialist detection
	fileContents := make(map[string]string)
	reviewedFiles := make([]string, 0)

	// Track dependency contexts for all files
	depContexts := make(map[string]*DependencyContext)
	// Track architectural contexts for all files (Holographic View)
	archContexts := make(map[string]*ArchitectureAnalysis)

	for _, filePath := range task.Files {
		// Skip ignored patterns
		if r.shouldIgnore(filePath) {
			continue
		}

		// Read file content
		content, err := r.readFile(ctx, filePath)
		if err != nil {
			result.Findings = append(result.Findings, ReviewFinding{
				File:     filePath,
				Severity: "error",
				Category: "io",
				Message:  fmt.Sprintf("Failed to read file: %v", err),
			})
			continue
		}

		// Fetch 1-hop dependencies for context-aware review
		depCtx, _ := r.getOneHopDependencies(ctx, filePath)
		if depCtx != nil && (len(depCtx.Upstream) > 0 || len(depCtx.Downstream) > 0) {
			depContexts[filePath] = depCtx
			logging.ReviewerDebug("Found %d upstream, %d downstream deps for %s",
				len(depCtx.Upstream), len(depCtx.Downstream), filePath)
		}

		// Perform Holographic Architecture Analysis
		archCtx := r.analyzeArchitecture(ctx, filePath)
		archContexts[filePath] = archCtx

		// NEW: Get holographic package context (sibling files, signatures, etc.)
		var holoCtx *HolographicContext
		if r.holographicProvider != nil {
			holoCtx, _ = r.holographicProvider.GetContext(filePath)
			if holoCtx != nil {
				logging.ReviewerDebug("Holographic context: pkg=%s, %d siblings, %d signatures, %d types",
					holoCtx.TargetPkg, len(holoCtx.PackageSiblings), len(holoCtx.PackageSignatures), len(holoCtx.PackageTypes))
			}
		}

		// Store for specialist detection
		fileContents[filePath] = content
		reviewedFiles = append(reviewedFiles, filePath)

		// Run all checks (now with dependency, architectural, AND holographic context)
		findings, report := r.analyzeFileWithHolographic(ctx, filePath, content, depCtx, archCtx, holoCtx)
		result.Findings = append(result.Findings, findings...)
		if report != "" {
			if result.AnalysisReport != "" {
				result.AnalysisReport += "\n---\n"
			}
			result.AnalysisReport += fmt.Sprintf("# Report for %s\n%s", filePath, report)
		}

		// Check finding limit
		if len(result.Findings) >= r.reviewerConfig.MaxFindings {
			result.Findings = append(result.Findings, ReviewFinding{
				File:     "",
				Severity: "warning",
				Category: "limit",
				Message:  fmt.Sprintf("Max findings limit (%d) reached, review truncated", r.reviewerConfig.MaxFindings),
			})
			break
		}
	}

	// Calculate metrics if enabled
	if r.reviewerConfig.IncludeMetrics && len(task.Files) > 0 {
		result.Metrics = r.calculateMetrics(ctx, task.Files)
	}

	// Detect and recommend specialist shards
	result.SpecialistRecommendations = r.detectSpecialists(reviewedFiles, fileContents)

	// Determine overall severity
	result.Severity = r.calculateOverallSeverity(result.Findings)
	result.BlockCommit = r.shouldBlockCommit(result)
	result.Duration = time.Since(startTime)
	result.Summary = r.generateSummary(result)

	logging.Reviewer("Review complete: %d findings, severity=%s, duration=%v", len(result.Findings), result.Severity, result.Duration)
	if result.BlockCommit {
		logging.Reviewer("BLOCKING COMMIT: Critical issues found")
	}

	// Track patterns for Autopoiesis
	r.trackReviewPatterns(result)

	// --- NEW: Context-Aware Filtering & Persistence ---
	// 1. Assert file topology facts (e.g., is it a test file?)
	for _, f := range result.Files {
		r.assertFileFacts(f)
	}

	// 2. Filter findings using Mangle rules (suppression logic)
	activeFindings, err := r.filterFindingsWithMangle(result.Findings)
	if err == nil {
		// Log suppressed count
		if len(activeFindings) < len(result.Findings) {
			logging.ReviewerDebug("Suppressed %d findings via Mangle rules", len(result.Findings)-len(activeFindings))
		}
		result.Findings = activeFindings
	} else {
		logging.Get(logging.CategoryReviewer).Warn("Failed to filter with Mangle, using raw findings: %v", err)
	}

	// 3. Persist findings to database
	r.persistFindingsToStore(result.Findings)

	// Recalculate severity after filtering
	result.Severity = r.calculateOverallSeverity(result.Findings)
	result.Summary = r.generateSummary(result)

	return result, nil
}

// securityScan performs security-focused analysis.
func (r *ReviewerShard) securityScan(ctx context.Context, task *ReviewerTask) (*ReviewResult, error) {
	startTime := time.Now()
	result := &ReviewResult{
		Files:    task.Files,
		Findings: make([]ReviewFinding, 0),
		Severity: ReviewSeverityClean,
	}

	for _, filePath := range task.Files {
		if r.shouldIgnore(filePath) {
			continue
		}

		content, err := r.readFile(ctx, filePath)
		if err != nil {
			continue
		}

		// Security-specific checks
		findings := r.checkSecurity(filePath, content)
		result.Findings = append(result.Findings, findings...)
	}

	result.Severity = r.calculateOverallSeverity(result.Findings)
	result.BlockCommit = r.shouldBlockCommit(result)
	result.Duration = time.Since(startTime)
	result.Summary = fmt.Sprintf("Security scan complete: %d issues found", len(result.Findings))

	return result, nil
}

// styleCheck performs style and formatting analysis.
func (r *ReviewerShard) styleCheck(ctx context.Context, task *ReviewerTask) (*ReviewResult, error) {
	startTime := time.Now()
	result := &ReviewResult{
		Files:    task.Files,
		Findings: make([]ReviewFinding, 0),
		Severity: ReviewSeverityClean,
	}

	for _, filePath := range task.Files {
		if r.shouldIgnore(filePath) {
			continue
		}

		content, err := r.readFile(ctx, filePath)
		if err != nil {
			continue
		}

		// Style-specific checks
		findings := r.checkStyle(filePath, content)
		result.Findings = append(result.Findings, findings...)
	}

	result.Severity = r.calculateOverallSeverity(result.Findings)
	result.Duration = time.Since(startTime)
	result.Summary = fmt.Sprintf("Style check complete: %d issues found", len(result.Findings))

	return result, nil
}

// complexityAnalysis performs complexity metrics analysis.
func (r *ReviewerShard) complexityAnalysis(ctx context.Context, task *ReviewerTask) (*ReviewResult, error) {
	startTime := time.Now()
	result := &ReviewResult{
		Files:    task.Files,
		Findings: make([]ReviewFinding, 0),
		Severity: ReviewSeverityClean,
	}

	result.Metrics = r.calculateMetrics(ctx, task.Files)

	// Generate findings from metrics
	if result.Metrics != nil {
		if result.Metrics.CyclomaticMax > 15 {
			result.Findings = append(result.Findings, ReviewFinding{
				Severity:   "warning",
				Category:   "maintainability",
				Message:    fmt.Sprintf("High cyclomatic complexity detected (max: %d)", result.Metrics.CyclomaticMax),
				Suggestion: "Consider breaking down complex functions",
			})
		}
		if result.Metrics.MaxNesting > 5 {
			result.Findings = append(result.Findings, ReviewFinding{
				Severity:   "warning",
				Category:   "maintainability",
				Message:    fmt.Sprintf("Deep nesting detected (max: %d levels)", result.Metrics.MaxNesting),
				Suggestion: "Consider using early returns or extracting functions",
			})
		}
		if result.Metrics.LongFunctions > 0 {
			result.Findings = append(result.Findings, ReviewFinding{
				Severity:   "info",
				Category:   "maintainability",
				Message:    fmt.Sprintf("%d functions exceed 50 lines", result.Metrics.LongFunctions),
				Suggestion: "Consider splitting long functions",
			})
		}
	}

	result.Severity = r.calculateOverallSeverity(result.Findings)
	result.Duration = time.Since(startTime)
	result.Summary = "Complexity analysis complete"

	return result, nil
}

// reviewDiff reviews changes from a git diff.
func (r *ReviewerShard) reviewDiff(ctx context.Context, task *ReviewerTask) (*ReviewResult, error) {
	startTime := time.Now()
	result := &ReviewResult{
		Files:    make([]string, 0),
		Findings: make([]ReviewFinding, 0),
		Severity: ReviewSeverityClean,
	}

	// Get diff via VirtualStore
	if r.virtualStore == nil {
		return nil, fmt.Errorf("virtualStore required for diff review")
	}

	diffRef := task.DiffRef
	if diffRef == "" {
		diffRef = "HEAD~1"
	}

	action := core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"/git_operation",
			"diff",
			map[string]interface{}{"args": []interface{}{diffRef}},
		},
	}
	diffOutput, err := r.virtualStore.RouteAction(ctx, action)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	// Parse diff to extract changed files and hunks
	changedFiles := r.parseDiffFiles(diffOutput)
	result.Files = changedFiles

	// Review each changed file
	for _, filePath := range changedFiles {
		content, err := r.readFile(ctx, filePath)
		if err != nil {
			continue
		}

		findings, report := r.analyzeFile(ctx, filePath, content)
		result.Findings = append(result.Findings, findings...)
		if report != "" {
			if result.AnalysisReport != "" {
				result.AnalysisReport += "\n---\n"
			}
			result.AnalysisReport += fmt.Sprintf("# Report for %s\n%s", filePath, report)
		}
	}

	result.Severity = r.calculateOverallSeverity(result.Findings)
	result.BlockCommit = r.shouldBlockCommit(result)
	result.Duration = time.Since(startTime)
	result.Summary = fmt.Sprintf("Diff review complete: %d files, %d issues", len(changedFiles), len(result.Findings))

	return result, nil
}
