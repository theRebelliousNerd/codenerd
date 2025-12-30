// Package campaign provides multi-phase goal orchestration.
// This file implements edge case detection for file action decisions.
// It determines whether files should be created, extended, modularized, or refactored first.
package campaign

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/world"
)

// =============================================================================
// EDGE CASE DETECTOR
// =============================================================================
// Analyzes files to determine the appropriate action before modification:
// - Create: File doesn't exist, needs to be created
// - Extend: File exists, add functionality
// - Modularize: File too large, split into multiple files
// - RefactorFirst: File has issues that should be addressed before adding features

// FileAction represents the recommended action for a file.
type FileAction int

const (
	// ActionCreate indicates the file should be created new.
	ActionCreate FileAction = iota
	// ActionExtend indicates the file should have functionality added.
	ActionExtend
	// ActionModularize indicates the file should be split into smaller files.
	ActionModularize
	// ActionRefactorFirst indicates the file needs cleanup before modification.
	ActionRefactorFirst
	// ActionSkip indicates the file should not be modified.
	ActionSkip
)

func (a FileAction) String() string {
	switch a {
	case ActionCreate:
		return "create"
	case ActionExtend:
		return "extend"
	case ActionModularize:
		return "modularize"
	case ActionRefactorFirst:
		return "refactor_first"
	case ActionSkip:
		return "skip"
	default:
		return "unknown"
	}
}

// FileDecision contains the analysis and recommendation for a file.
type FileDecision struct {
	Path              string     `json:"path"`
	RecommendedAction FileAction `json:"recommended_action"`
	Reasoning         string     `json:"reasoning"`
	Confidence        float64    `json:"confidence"` // 0.0-1.0

	// Analysis data
	Exists        bool   `json:"exists"`
	Language      string `json:"language"`
	LineCount     int    `json:"line_count"`
	ChurnRate     int    `json:"churn_rate"`
	Complexity    float64 `json:"complexity"` // Cyclomatic complexity estimate
	TODOCount     int    `json:"todo_count"`
	HasTests      bool   `json:"has_tests"`

	// Dependencies
	Dependencies    []string `json:"dependencies"`
	Dependents      []string `json:"dependents"`
	ImpactScore     int      `json:"impact_score"`

	// Suggestions
	SuggestedSplits  []SplitSuggestion `json:"suggested_splits,omitempty"`
	RefactorReasons  []string          `json:"refactor_reasons,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
}

// SplitSuggestion suggests how to split a large file.
type SplitSuggestion struct {
	NewFileName string   `json:"new_file_name"`
	Functions   []string `json:"functions"`
	Reason      string   `json:"reason"`
}

// EdgeCaseDetector analyzes files to determine appropriate actions.
type EdgeCaseDetector struct {
	kernel  *core.RealKernel
	scanner *world.Scanner

	// Configuration
	config EdgeCaseConfig
}

// EdgeCaseConfig configures detection thresholds.
type EdgeCaseConfig struct {
	// Size thresholds
	LargeFileLines       int // Lines to consider file "large" (default: 500)
	VeryLargeFileLines   int // Lines requiring modularization (default: 1000)
	MaxFunctionsPerFile  int // Max functions before suggesting split (default: 20)

	// Complexity thresholds
	HighComplexity   float64 // Complexity requiring refactor (default: 10.0)
	ModerateComplexity float64 // Complexity worth noting (default: 5.0)

	// Churn thresholds
	HighChurnRate    int // Commits suggesting caution (default: 10)
	VeryHighChurnRate int // Commits requiring discussion (default: 20)

	// Quality thresholds
	MaxTODOsBeforeRefactor int // TODOs requiring cleanup (default: 5)

	// Timeout
	AnalysisTimeout time.Duration
}

// DefaultEdgeCaseConfig returns sensible defaults.
func DefaultEdgeCaseConfig() EdgeCaseConfig {
	return EdgeCaseConfig{
		LargeFileLines:        500,
		VeryLargeFileLines:    1000,
		MaxFunctionsPerFile:   20,
		HighComplexity:        10.0,
		ModerateComplexity:    5.0,
		HighChurnRate:         10,
		VeryHighChurnRate:     20,
		MaxTODOsBeforeRefactor: 5,
		AnalysisTimeout:       30 * time.Second,
	}
}

// NewEdgeCaseDetector creates a new edge case detector.
func NewEdgeCaseDetector(kernel *core.RealKernel, scanner *world.Scanner) *EdgeCaseDetector {
	return &EdgeCaseDetector{
		kernel:  kernel,
		scanner: scanner,
		config:  DefaultEdgeCaseConfig(),
	}
}

// WithConfig sets custom configuration.
func (d *EdgeCaseDetector) WithConfig(config EdgeCaseConfig) *EdgeCaseDetector {
	d.config = config
	return d
}

// AnalyzeFiles analyzes multiple files and returns decisions for each.
func (d *EdgeCaseDetector) AnalyzeFiles(ctx context.Context, paths []string, intelligence *IntelligenceReport) ([]FileDecision, error) {
	logging.Campaign("Edge case analysis started for %d files", len(paths))
	timer := logging.StartTimer(logging.CategoryCampaign, "AnalyzeFiles")
	defer timer.Stop()

	ctx, cancel := context.WithTimeout(ctx, d.config.AnalysisTimeout*time.Duration(len(paths)))
	defer cancel()

	decisions := make([]FileDecision, 0, len(paths))

	for _, path := range paths {
		select {
		case <-ctx.Done():
			return decisions, ctx.Err()
		default:
		}

		decision := d.analyzeFile(ctx, path, intelligence)
		decisions = append(decisions, decision)
	}

	// Sort by priority: refactor_first > modularize > create > extend > skip
	sort.Slice(decisions, func(i, j int) bool {
		return d.actionPriority(decisions[i].RecommendedAction) > d.actionPriority(decisions[j].RecommendedAction)
	})

	logging.Campaign("Edge case analysis complete: %d decisions made", len(decisions))
	return decisions, nil
}

// analyzeFile performs analysis on a single file.
func (d *EdgeCaseDetector) analyzeFile(ctx context.Context, path string, intel *IntelligenceReport) FileDecision {
	decision := FileDecision{
		Path:         path,
		Language:     d.detectLanguage(path),
		Confidence:   0.8, // Default confidence
		Dependencies: []string{},
		Dependents:   []string{},
		Warnings:     []string{},
	}

	// Check if file exists (from intelligence report)
	if intel != nil && len(intel.FileTopology) > 0 {
		if fileInfo, exists := intel.FileTopology[path]; exists {
			decision.Exists = true
			decision.Language = fileInfo.Language
		}
	}

	// If file doesn't exist, recommend creation
	if !decision.Exists {
		decision.RecommendedAction = ActionCreate
		decision.Reasoning = "File does not exist - creation required"
		return decision
	}

	// Gather metrics from intelligence report
	d.gatherMetrics(&decision, path, intel)

	// Apply decision logic
	decision.RecommendedAction, decision.Reasoning = d.determineAction(decision)

	// Add warnings for edge cases
	d.addWarnings(&decision, intel)

	return decision
}

// gatherMetrics populates decision metrics from intelligence data.
func (d *EdgeCaseDetector) gatherMetrics(decision *FileDecision, path string, intel *IntelligenceReport) {
	if intel == nil {
		return
	}

	// Get churn rate from git history
	for _, hotspot := range intel.GitChurnHotspots {
		if hotspot.Path == path || strings.HasSuffix(hotspot.Path, filepath.Base(path)) {
			decision.ChurnRate = hotspot.ChurnRate
			break
		}
	}

	// Count symbols in file
	symbolCount := 0
	for _, symbol := range intel.SymbolGraph {
		if symbol.File == path || strings.HasSuffix(symbol.File, filepath.Base(path)) {
			symbolCount++
		}
	}
	// Estimate line count from symbol density
	decision.LineCount = symbolCount * 25 // Rough estimate

	// Check for test file
	if strings.HasSuffix(path, "_test.go") {
		decision.HasTests = true
	} else {
		testPath := strings.TrimSuffix(path, filepath.Ext(path)) + "_test" + filepath.Ext(path)
		_, decision.HasTests = intel.FileTopology[testPath]
	}

	// Query kernel for dependencies
	if d.kernel != nil {
		d.queryDependencies(decision, path)
		d.queryComplexity(decision, path)
	}
}

// queryDependencies gets file dependencies from the kernel.
func (d *EdgeCaseDetector) queryDependencies(decision *FileDecision, path string) {
	// Query code_imports for dependencies
	facts, err := d.kernel.Query("code_imports")
	if err != nil {
		return
	}

	for _, fact := range facts {
		if len(fact.Args) >= 2 {
			file := d.parseArg(fact.Args[0])
			imported := d.parseArg(fact.Args[1])

			if strings.HasSuffix(file, filepath.Base(path)) {
				decision.Dependencies = append(decision.Dependencies, imported)
			}
			if strings.HasSuffix(imported, filepath.Base(path)) {
				decision.Dependents = append(decision.Dependents, file)
			}
		}
	}

	// Calculate impact score based on dependents
	decision.ImpactScore = len(decision.Dependents)
}

// queryComplexity estimates complexity from kernel facts.
func (d *EdgeCaseDetector) queryComplexity(decision *FileDecision, path string) {
	// Query for complexity-related facts
	facts, err := d.kernel.Query("code_complexity")
	if err != nil {
		return
	}

	for _, fact := range facts {
		if len(fact.Args) >= 2 {
			file := d.parseArg(fact.Args[0])
			if strings.HasSuffix(file, filepath.Base(path)) {
				if complexity, ok := fact.Args[1].(float64); ok {
					decision.Complexity = complexity
				}
			}
		}
	}

	// If no complexity data, estimate from line count
	if decision.Complexity == 0 && decision.LineCount > 0 {
		// Rough heuristic: 1 complexity point per 50 lines
		decision.Complexity = float64(decision.LineCount) / 50.0
	}
}

// determineAction decides what action to take based on gathered metrics.
func (d *EdgeCaseDetector) determineAction(decision FileDecision) (FileAction, string) {
	var reasons []string

	// Check for modularization need
	if decision.LineCount >= d.config.VeryLargeFileLines {
		reasons = append(reasons, fmt.Sprintf("File has %d lines (threshold: %d)", decision.LineCount, d.config.VeryLargeFileLines))
		decision.SuggestedSplits = d.suggestSplits(decision)
		return ActionModularize, strings.Join(reasons, "; ")
	}

	// Check for refactor-first scenarios
	refactorReasons := d.checkRefactorReasons(decision)
	if len(refactorReasons) >= 2 {
		decision.RefactorReasons = refactorReasons
		return ActionRefactorFirst, "Multiple quality issues: " + strings.Join(refactorReasons, "; ")
	}

	// Check for complexity issues
	if decision.Complexity >= d.config.HighComplexity {
		return ActionRefactorFirst, fmt.Sprintf("High complexity (%.1f) - refactor before adding features", decision.Complexity)
	}

	// Check for high churn (Chesterton's Fence)
	if decision.ChurnRate >= d.config.VeryHighChurnRate {
		return ActionRefactorFirst, fmt.Sprintf("Very high churn rate (%d commits) - understand before modifying", decision.ChurnRate)
	}

	// Default to extend for existing files
	if decision.LineCount >= d.config.LargeFileLines {
		return ActionExtend, fmt.Sprintf("Large file (%d lines) - extend carefully", decision.LineCount)
	}

	return ActionExtend, "File exists and is suitable for extension"
}

// checkRefactorReasons identifies reasons for refactoring.
func (d *EdgeCaseDetector) checkRefactorReasons(decision FileDecision) []string {
	var reasons []string

	if decision.Complexity >= d.config.ModerateComplexity {
		reasons = append(reasons, fmt.Sprintf("Moderate complexity (%.1f)", decision.Complexity))
	}

	if decision.ChurnRate >= d.config.HighChurnRate {
		reasons = append(reasons, fmt.Sprintf("High churn rate (%d)", decision.ChurnRate))
	}

	if decision.TODOCount >= d.config.MaxTODOsBeforeRefactor {
		reasons = append(reasons, fmt.Sprintf("Many TODOs (%d)", decision.TODOCount))
	}

	if !decision.HasTests && decision.ImpactScore > 3 {
		reasons = append(reasons, "No tests for high-impact file")
	}

	if decision.LineCount >= d.config.LargeFileLines {
		reasons = append(reasons, fmt.Sprintf("Large file (%d lines)", decision.LineCount))
	}

	return reasons
}

// suggestSplits suggests how to split a large file.
func (d *EdgeCaseDetector) suggestSplits(decision FileDecision) []SplitSuggestion {
	var suggestions []SplitSuggestion

	baseName := strings.TrimSuffix(filepath.Base(decision.Path), filepath.Ext(decision.Path))
	ext := filepath.Ext(decision.Path)
	dir := filepath.Dir(decision.Path)

	// Generic split suggestions based on common patterns
	patterns := []struct {
		suffix string
		desc   string
	}{
		{"_types", "Type definitions and interfaces"},
		{"_helpers", "Helper functions and utilities"},
		{"_handlers", "Request/response handlers"},
		{"_validation", "Validation logic"},
		{"_persistence", "Database/storage operations"},
	}

	// Suggest at most 3 splits
	for i, p := range patterns {
		if i >= 3 {
			break
		}
		suggestions = append(suggestions, SplitSuggestion{
			NewFileName: filepath.Join(dir, baseName+p.suffix+ext),
			Reason:      p.desc,
		})
	}

	return suggestions
}

// addWarnings adds contextual warnings to the decision.
func (d *EdgeCaseDetector) addWarnings(decision *FileDecision, intel *IntelligenceReport) {
	// Chesterton's Fence warning
	if decision.ChurnRate >= d.config.HighChurnRate {
		decision.Warnings = append(decision.Warnings,
			fmt.Sprintf("⚠️ CHESTERTON'S FENCE: This file has changed %d times. Understand WHY before modifying.", decision.ChurnRate))
	}

	// No tests warning
	if !decision.HasTests && decision.ImpactScore > 0 {
		decision.Warnings = append(decision.Warnings,
			"⚠️ No test file exists for this code. Consider adding tests first.")
	}

	// High impact warning
	if decision.ImpactScore > 5 {
		decision.Warnings = append(decision.Warnings,
			fmt.Sprintf("⚠️ HIGH IMPACT: %d files depend on this code.", decision.ImpactScore))
	}

	// Check for safety warnings from intelligence
	if intel != nil {
		for _, warning := range intel.SafetyWarnings {
			if warning.Path == decision.Path {
				decision.Warnings = append(decision.Warnings,
					fmt.Sprintf("⚠️ SAFETY: %s - %s", warning.Action, warning.RuleViolated))
			}
		}
	}
}

// actionPriority returns the priority of an action (higher = more urgent).
func (d *EdgeCaseDetector) actionPriority(action FileAction) int {
	switch action {
	case ActionRefactorFirst:
		return 4
	case ActionModularize:
		return 3
	case ActionCreate:
		return 2
	case ActionExtend:
		return 1
	case ActionSkip:
		return 0
	default:
		return 0
	}
}

// detectLanguage determines the language from file extension.
func (d *EdgeCaseDetector) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	langMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".rs":   "rust",
		".java": "java",
		".mg":   "mangle",
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "unknown"
}

// parseArg safely extracts a string from an interface{}.
func (d *EdgeCaseDetector) parseArg(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case core.MangleAtom:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// =============================================================================
// BATCH ANALYSIS
// =============================================================================

// AnalyzeForCampaign performs comprehensive analysis for a campaign.
func (d *EdgeCaseDetector) AnalyzeForCampaign(ctx context.Context, targetPaths []string, intel *IntelligenceReport) (*EdgeCaseAnalysis, error) {
	decisions, err := d.AnalyzeFiles(ctx, targetPaths, intel)
	if err != nil {
		return nil, err
	}

	analysis := &EdgeCaseAnalysis{
		Decisions:     decisions,
		ActionCounts:  make(map[FileAction]int),
		ModularizeFiles: []string{},
		RefactorFiles:  []string{},
		CreateFiles:    []string{},
		ExtendFiles:    []string{},
		HighImpactFiles: []string{},
		NoTestFiles:    []string{},
	}

	// Categorize decisions
	for _, decision := range decisions {
		analysis.ActionCounts[decision.RecommendedAction]++

		switch decision.RecommendedAction {
		case ActionModularize:
			analysis.ModularizeFiles = append(analysis.ModularizeFiles, decision.Path)
		case ActionRefactorFirst:
			analysis.RefactorFiles = append(analysis.RefactorFiles, decision.Path)
		case ActionCreate:
			analysis.CreateFiles = append(analysis.CreateFiles, decision.Path)
		case ActionExtend:
			analysis.ExtendFiles = append(analysis.ExtendFiles, decision.Path)
		}

		if decision.ImpactScore > 5 {
			analysis.HighImpactFiles = append(analysis.HighImpactFiles, decision.Path)
		}

		if !decision.HasTests && decision.Exists {
			analysis.NoTestFiles = append(analysis.NoTestFiles, decision.Path)
		}
	}

	// Calculate summary statistics
	analysis.TotalFiles = len(decisions)
	analysis.RequiresPrework = len(analysis.ModularizeFiles) + len(analysis.RefactorFiles)

	return analysis, nil
}

// EdgeCaseAnalysis contains the full analysis results.
type EdgeCaseAnalysis struct {
	Decisions       []FileDecision      `json:"decisions"`
	ActionCounts    map[FileAction]int  `json:"action_counts"`
	TotalFiles      int                 `json:"total_files"`
	RequiresPrework int                 `json:"requires_prework"`

	// Categorized file lists
	ModularizeFiles []string `json:"modularize_files"`
	RefactorFiles   []string `json:"refactor_files"`
	CreateFiles     []string `json:"create_files"`
	ExtendFiles     []string `json:"extend_files"`
	HighImpactFiles []string `json:"high_impact_files"`
	NoTestFiles     []string `json:"no_test_files"`
}

// FormatForContext formats the analysis for LLM context injection.
func (a *EdgeCaseAnalysis) FormatForContext() string {
	var sb strings.Builder

	sb.WriteString("# EDGE CASE ANALYSIS\n\n")
	sb.WriteString(fmt.Sprintf("**Total Files:** %d\n", a.TotalFiles))
	sb.WriteString(fmt.Sprintf("**Requires Pre-work:** %d files\n\n", a.RequiresPrework))

	// Action breakdown
	sb.WriteString("## Action Breakdown\n")
	for action, count := range a.ActionCounts {
		sb.WriteString(fmt.Sprintf("- %s: %d files\n", action.String(), count))
	}
	sb.WriteString("\n")

	// Files requiring modularization
	if len(a.ModularizeFiles) > 0 {
		sb.WriteString("## ⚠️ Files Requiring Modularization\n")
		sb.WriteString("These files are too large and should be split:\n")
		for _, f := range a.ModularizeFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	// Files requiring refactoring
	if len(a.RefactorFiles) > 0 {
		sb.WriteString("## ⚠️ Files Requiring Refactoring First\n")
		sb.WriteString("These files have quality issues to address before modification:\n")
		for _, f := range a.RefactorFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	// High impact files
	if len(a.HighImpactFiles) > 0 {
		sb.WriteString("## 🔴 High Impact Files\n")
		sb.WriteString("Changes to these files affect many dependents:\n")
		for _, f := range a.HighImpactFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	// Files needing tests
	if len(a.NoTestFiles) > 0 {
		sb.WriteString("## 📋 Files Without Tests\n")
		sb.WriteString("Consider adding tests for:\n")
		for i, f := range a.NoTestFiles {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(a.NoTestFiles)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// HasBlockingIssues returns true if there are issues that should be resolved before proceeding.
func (a *EdgeCaseAnalysis) HasBlockingIssues() bool {
	return len(a.ModularizeFiles) > 0 || len(a.RefactorFiles) > 3
}

// GetPreworkTasks returns tasks that should be done before the main campaign.
func (a *EdgeCaseAnalysis) GetPreworkTasks() []string {
	var tasks []string

	for _, file := range a.ModularizeFiles {
		tasks = append(tasks, fmt.Sprintf("Modularize %s into smaller files", file))
	}

	for _, file := range a.RefactorFiles {
		tasks = append(tasks, fmt.Sprintf("Refactor %s to address quality issues", file))
	}

	return tasks
}
