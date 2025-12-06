// Package shards implements specialized ShardAgent types for the Cortex 1.5.0 architecture.
// This file implements the Reviewer ShardAgent per §7.0 Sharding.
package shards

import (
	"codenerd/internal/core"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// CONFIGURATION
// =============================================================================

// ReviewerConfig holds configuration for the reviewer shard.
type ReviewerConfig struct {
	StyleGuide      string   // Path to style guide or preset name
	SecurityRules   []string // Security patterns to check (OWASP categories)
	MaxFindings     int      // Max findings before abort (default: 100)
	BlockOnCritical bool     // Block commit if critical issues found (default: true)
	IncludeMetrics  bool     // Include complexity metrics (default: true)
	SeverityFilter  string   // Minimum severity to report: "info", "warning", "error", "critical"
	WorkingDir      string   // Workspace directory
	IgnorePatterns  []string // File patterns to ignore
	MaxFileSize     int64    // Max file size to review in bytes (default: 1MB)
}

// DefaultReviewerConfig returns sensible defaults for code review.
func DefaultReviewerConfig() ReviewerConfig {
	return ReviewerConfig{
		StyleGuide: "default",
		SecurityRules: []string{
			"sql_injection",
			"xss",
			"command_injection",
			"path_traversal",
			"hardcoded_secrets",
			"insecure_crypto",
			"unsafe_deserialization",
		},
		MaxFindings:     100,
		BlockOnCritical: true,
		IncludeMetrics:  true,
		SeverityFilter:  "info",
		WorkingDir:      ".",
		IgnorePatterns:  []string{"vendor/", "node_modules/", ".git/", "*.min.js"},
		MaxFileSize:     1024 * 1024, // 1MB
	}
}

// =============================================================================
// REVIEW RESULT TYPES
// =============================================================================

// ReviewSeverity represents the overall severity level of a review.
type ReviewSeverity string

const (
	ReviewSeverityClean    ReviewSeverity = "clean"
	ReviewSeverityInfo     ReviewSeverity = "info"
	ReviewSeverityWarning  ReviewSeverity = "warning"
	ReviewSeverityError    ReviewSeverity = "error"
	ReviewSeverityCritical ReviewSeverity = "critical"
)

// ReviewFinding represents a single issue found during review.
type ReviewFinding struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
	Severity    string `json:"severity"` // "critical", "error", "warning", "info", "suggestion"
	Category    string `json:"category"` // "security", "style", "performance", "maintainability", "bug"
	RuleID      string `json:"rule_id"`
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion,omitempty"`
	CodeSnippet string `json:"code_snippet,omitempty"`
}

// SpecialistRecommendation suggests a specialist shard for working on this code.
type SpecialistRecommendation struct {
	ShardName  string   `json:"shard_name"`  // e.g., "rod", "golang", "react"
	Reason     string   `json:"reason"`      // Why this specialist is recommended
	Confidence float64  `json:"confidence"`  // 0.0-1.0
	ForFiles   []string `json:"for_files"`   // Which files this applies to
	TaskHints  []string `json:"task_hints"`  // Suggested tasks for the specialist
}

// ReviewResult represents the outcome of a code review.
type ReviewResult struct {
	Files       []string        `json:"files"`
	Findings    []ReviewFinding `json:"findings"`
	Severity    ReviewSeverity  `json:"severity"`
	Summary     string          `json:"summary"`
	Duration    time.Duration   `json:"duration"`
	BlockCommit bool            `json:"block_commit"`
	Metrics     *CodeMetrics    `json:"metrics,omitempty"`

	// Specialist recommendations based on detected technologies
	SpecialistRecommendations []SpecialistRecommendation `json:"specialist_recommendations,omitempty"`
}

// CodeMetrics holds code complexity metrics.
type CodeMetrics struct {
	TotalLines      int     `json:"total_lines"`
	CodeLines       int     `json:"code_lines"`
	CommentLines    int     `json:"comment_lines"`
	BlankLines      int     `json:"blank_lines"`
	CyclomaticAvg   float64 `json:"cyclomatic_avg"`
	CyclomaticMax   int     `json:"cyclomatic_max"`
	MaxNesting      int     `json:"max_nesting"`
	FunctionCount   int     `json:"function_count"`
	LongFunctions   int     `json:"long_functions"` // Functions > 50 lines
	DuplicateBlocks int     `json:"duplicate_blocks"`
}

// =============================================================================
// REVIEWER SHARD
// =============================================================================

// ReviewerShard is specialized for code review, security scanning, and best practices.
// Per Cortex 1.5.0 §7.0, this shard acts as a quality gate for code changes.
type ReviewerShard struct {
	mu sync.RWMutex

	// Identity
	id     string
	config core.ShardConfig
	state  core.ShardState

	// Reviewer-specific configuration
	reviewerConfig ReviewerConfig

	// Components (required)
	kernel       *core.RealKernel     // Own kernel instance for logic-driven review
	llmClient    core.LLMClient       // LLM for semantic analysis
	virtualStore *core.VirtualStore   // Action routing

	// State tracking
	startTime time.Time
	findings  []ReviewFinding
	severity  ReviewSeverity

	// Autopoiesis tracking (§8.3)
	approvedPatterns    map[string]int    // Patterns that pass review
	flaggedPatterns     map[string]int    // Patterns that get flagged repeatedly
	learnedAntiPatterns map[string]string // Anti-patterns learned from rejections
}

// NewReviewerShard creates a new Reviewer shard with default configuration.
func NewReviewerShard() *ReviewerShard {
	return NewReviewerShardWithConfig(DefaultReviewerConfig())
}

// NewReviewerShardWithConfig creates a reviewer shard with custom configuration.
func NewReviewerShardWithConfig(reviewerConfig ReviewerConfig) *ReviewerShard {
	return &ReviewerShard{
		config:              core.DefaultSpecialistConfig("reviewer", ""),
		state:               core.ShardStateIdle,
		reviewerConfig:      reviewerConfig,
		findings:            make([]ReviewFinding, 0),
		severity:            ReviewSeverityClean,
		approvedPatterns:    make(map[string]int),
		flaggedPatterns:     make(map[string]int),
		learnedAntiPatterns: make(map[string]string),
	}
}

// =============================================================================
// DEPENDENCY INJECTION
// =============================================================================

// SetLLMClient sets the LLM client for semantic analysis.
func (r *ReviewerShard) SetLLMClient(client core.LLMClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.llmClient = client
}

// SetParentKernel sets the Mangle kernel for logic-driven review.
func (r *ReviewerShard) SetParentKernel(k core.Kernel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rk, ok := k.(*core.RealKernel); ok {
		r.kernel = rk
	} else {
		panic("ReviewerShard requires *core.RealKernel")
	}
}

// SetVirtualStore sets the virtual store for action routing.
func (r *ReviewerShard) SetVirtualStore(vs *core.VirtualStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.virtualStore = vs
}

// =============================================================================
// SHARD INTERFACE IMPLEMENTATION
// =============================================================================

// GetID returns the shard ID.
func (r *ReviewerShard) GetID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.id
}

// GetState returns the current state.
func (r *ReviewerShard) GetState() core.ShardState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// GetConfig returns the shard configuration.
func (r *ReviewerShard) GetConfig() core.ShardConfig {
	return r.config
}

// GetKernel returns the kernel (for fact propagation).
func (r *ReviewerShard) GetKernel() *core.RealKernel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.kernel
}

// Stop stops the shard.
func (r *ReviewerShard) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = core.ShardStateCompleted
	return nil
}

// =============================================================================
// MAIN EXECUTION
// =============================================================================

// Execute performs the review task.
// Task formats:
//   - "review file:PATH"
//   - "review diff:HEAD~1" (git diff review)
//   - "review pr:files:file1.go,file2.go"
//   - "security_scan file:PATH"
//   - "style_check file:PATH"
//   - "complexity file:PATH"
func (r *ReviewerShard) Execute(ctx context.Context, task string) (string, error) {
	r.mu.Lock()
	r.state = core.ShardStateRunning
	r.startTime = time.Now()
	r.id = fmt.Sprintf("reviewer-%d", time.Now().UnixNano())
	r.findings = make([]ReviewFinding, 0)
	r.severity = ReviewSeverityClean
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.state = core.ShardStateCompleted
		r.mu.Unlock()
	}()

	fmt.Printf("[ReviewerShard:%s] Starting task: %s\n", r.id, task)

	// Initialize kernel if not set
	if r.kernel == nil {
		r.kernel = core.NewRealKernel()
	}
	// Load reviewer-specific policy
	_ = r.kernel.LoadPolicyFile("reviewer.gl")

	// Parse the task
	parsedTask, err := r.parseTask(task)
	if err != nil {
		return "", fmt.Errorf("failed to parse task: %w", err)
	}

	// Assert initial facts to kernel
	r.assertInitialFacts(parsedTask)

	// Route to appropriate handler
	var result *ReviewResult
	switch parsedTask.Action {
	case "review":
		result, err = r.reviewFiles(ctx, parsedTask)
	case "security_scan":
		result, err = r.securityScan(ctx, parsedTask)
	case "style_check":
		result, err = r.styleCheck(ctx, parsedTask)
	case "complexity":
		result, err = r.complexityAnalysis(ctx, parsedTask)
	case "diff":
		result, err = r.reviewDiff(ctx, parsedTask)
	default:
		result, err = r.reviewFiles(ctx, parsedTask)
	}

	if err != nil {
		return "", err
	}

	// Generate facts for propagation
	facts := r.generateFacts(result)
	for _, fact := range facts {
		if r.kernel != nil {
			_ = r.kernel.Assert(fact)
		}
	}

	// Format output
	return r.formatResult(result), nil
}

// =============================================================================
// TASK PARSING
// =============================================================================

// ReviewerTask represents a parsed review task.
type ReviewerTask struct {
	Action  string   // "review", "security_scan", "style_check", "complexity", "diff"
	Files   []string // Files to review
	DiffRef string   // Git diff reference (e.g., "HEAD~1")
	Options map[string]string
}

// parseTask extracts action and parameters from task string.
func (r *ReviewerShard) parseTask(task string) (*ReviewerTask, error) {
	parsed := &ReviewerTask{
		Action:  "review",
		Files:   make([]string, 0),
		Options: make(map[string]string),
	}

	parts := strings.Fields(task)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty task")
	}

	// First token is the action
	action := strings.ToLower(parts[0])
	switch action {
	case "review", "check":
		parsed.Action = "review"
	case "security_scan", "security", "scan":
		parsed.Action = "security_scan"
	case "style_check", "style", "lint":
		parsed.Action = "style_check"
	case "complexity", "metrics":
		parsed.Action = "complexity"
	case "diff":
		parsed.Action = "diff"
	default:
		// Assume review if action is a file path
		if strings.Contains(action, ".") || strings.Contains(action, "/") {
			parsed.Action = "review"
			parsed.Files = append(parsed.Files, action)
		}
	}

	// Parse key:value pairs
	for _, part := range parts[1:] {
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			key := strings.ToLower(kv[0])
			value := kv[1]

			switch key {
			case "file":
				parsed.Files = append(parsed.Files, value)
			case "files":
				// Comma-separated list
				for _, f := range strings.Split(value, ",") {
					if f = strings.TrimSpace(f); f != "" {
						parsed.Files = append(parsed.Files, f)
					}
				}
			case "diff":
				parsed.DiffRef = value
				parsed.Action = "diff"
			case "pr":
				// PR files format: pr:files:a.go,b.go
				if strings.HasPrefix(value, "files:") {
					files := strings.TrimPrefix(value, "files:")
					for _, f := range strings.Split(files, ",") {
						if f = strings.TrimSpace(f); f != "" {
							parsed.Files = append(parsed.Files, f)
						}
					}
				}
			default:
				parsed.Options[key] = value
			}
		} else if !strings.HasPrefix(part, "-") {
			// Bare argument - treat as file
			parsed.Files = append(parsed.Files, part)
		}
	}

	return parsed, nil
}

// =============================================================================
// REVIEW OPERATIONS
// =============================================================================

// reviewFiles performs a comprehensive review of the specified files.
func (r *ReviewerShard) reviewFiles(ctx context.Context, task *ReviewerTask) (*ReviewResult, error) {
	startTime := time.Now()
	result := &ReviewResult{
		Files:    task.Files,
		Findings: make([]ReviewFinding, 0),
		Severity: ReviewSeverityClean,
	}

	// Collect file contents for specialist detection
	fileContents := make(map[string]string)
	reviewedFiles := make([]string, 0)

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

		// Store for specialist detection
		fileContents[filePath] = content
		reviewedFiles = append(reviewedFiles, filePath)

		// Run all checks
		findings := r.analyzeFile(ctx, filePath, content)
		result.Findings = append(result.Findings, findings...)

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

	// Track patterns for Autopoiesis
	r.trackReviewPatterns(result)

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
		Args:      []interface{}{"/git_diff", diffRef},
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

		findings := r.analyzeFile(ctx, filePath, content)
		result.Findings = append(result.Findings, findings...)
	}

	result.Severity = r.calculateOverallSeverity(result.Findings)
	result.BlockCommit = r.shouldBlockCommit(result)
	result.Duration = time.Since(startTime)
	result.Summary = fmt.Sprintf("Diff review complete: %d files, %d issues", len(changedFiles), len(result.Findings))

	return result, nil
}

// =============================================================================
// FILE ANALYSIS
// =============================================================================

// analyzeFile runs all analysis checks on a file.
func (r *ReviewerShard) analyzeFile(ctx context.Context, filePath, content string) []ReviewFinding {
	findings := make([]ReviewFinding, 0)

	// Code DOM safety checks (check kernel facts first)
	findings = append(findings, r.checkCodeDOMSafety(filePath)...)

	// Security checks
	findings = append(findings, r.checkSecurity(filePath, content)...)

	// Style checks
	findings = append(findings, r.checkStyle(filePath, content)...)

	// Bug pattern checks
	findings = append(findings, r.checkBugPatterns(filePath, content)...)

	// LLM-powered semantic analysis (if available)
	if r.llmClient != nil {
		llmFindings, err := r.llmAnalysis(ctx, filePath, content)
		if err == nil {
			findings = append(findings, llmFindings...)
		}
	}

	// Check against learned anti-patterns
	findings = append(findings, r.checkLearnedPatterns(filePath, content)...)

	return findings
}

// checkCodeDOMSafety checks Code DOM predicates for safety concerns.
func (r *ReviewerShard) checkCodeDOMSafety(filePath string) []ReviewFinding {
	findings := make([]ReviewFinding, 0)

	if r.kernel == nil {
		return findings
	}

	// Check if file is generated code
	generatedResults, _ := r.kernel.Query("generated_code")
	for _, fact := range generatedResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[0].(string); ok && file == filePath {
				generator := "unknown"
				marker := ""
				if g, ok := fact.Args[1].(string); ok {
					generator = g
				}
				if m, ok := fact.Args[2].(string); ok {
					marker = m
				}
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       1,
					Severity:   "warning",
					Category:   "generated",
					RuleID:     "CDOM001",
					Message:    fmt.Sprintf("Generated code (%s) - changes will be lost on regeneration", generator),
					Suggestion: fmt.Sprintf("Modify the generator source instead. Marker: %s", marker),
				})
			}
		}
	}

	// Check for breaking change risk
	breakingResults, _ := r.kernel.Query("breaking_change_risk")
	for _, fact := range breakingResults {
		if len(fact.Args) >= 3 {
			ref, _ := fact.Args[0].(string)
			level, _ := fact.Args[1].(string)
			reason, _ := fact.Args[2].(string)

			if strings.Contains(ref, filePath) {
				severity := "info"
				if level == "/critical" {
					severity = "critical"
				} else if level == "/high" {
					severity = "error"
				} else if level == "/medium" {
					severity = "warning"
				}

				findings = append(findings, ReviewFinding{
					File:       filePath,
					Severity:   severity,
					Category:   "breaking_change",
					RuleID:     "CDOM002",
					Message:    fmt.Sprintf("Breaking change risk: %s", reason),
					Suggestion: "Review downstream consumers and update tests",
				})
			}
		}
	}

	// Check for API client functions that need integration tests
	apiClientResults, _ := r.kernel.Query("api_client_function")
	for _, fact := range apiClientResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[1].(string); ok && file == filePath {
				funcName := "unknown"
				pattern := ""
				if ref, ok := fact.Args[0].(string); ok {
					funcName = ref
				}
				if p, ok := fact.Args[2].(string); ok {
					pattern = p
				}
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Severity:   "info",
					Category:   "api",
					RuleID:     "CDOM003",
					Message:    fmt.Sprintf("API client function detected (%s): %s", pattern, funcName),
					Suggestion: "Ensure proper error handling, timeouts, and consider integration tests",
				})
			}
		}
	}

	// Check for API handler functions
	apiHandlerResults, _ := r.kernel.Query("api_handler_function")
	for _, fact := range apiHandlerResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[1].(string); ok && file == filePath {
				funcName := "unknown"
				framework := ""
				if ref, ok := fact.Args[0].(string); ok {
					funcName = ref
				}
				if f, ok := fact.Args[2].(string); ok {
					framework = f
				}
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Severity:   "info",
					Category:   "api",
					RuleID:     "CDOM004",
					Message:    fmt.Sprintf("API handler detected (%s framework): %s", framework, funcName),
					Suggestion: "Validate inputs, handle errors appropriately, check authentication",
				})
			}
		}
	}

	// Check for mock update suggestions
	mockResults, _ := r.kernel.Query("suggest_update_mocks")
	for _, fact := range mockResults {
		if len(fact.Args) >= 1 {
			if file, ok := fact.Args[0].(string); ok && file == filePath {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Severity:   "warning",
					Category:   "testing",
					RuleID:     "CDOM005",
					Message:    "Signature change detected - mock files may need updating",
					Suggestion: "Run 'mockgen' or update mock implementations",
				})
			}
		}
	}

	// Check for CGo code
	cgoResults, _ := r.kernel.Query("cgo_code")
	for _, fact := range cgoResults {
		if len(fact.Args) >= 1 {
			if file, ok := fact.Args[0].(string); ok && file == filePath {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       1,
					Severity:   "warning",
					Category:   "cgo",
					RuleID:     "CDOM006",
					Message:    "CGo code detected - requires careful memory management review",
					Suggestion: "Verify proper memory allocation/deallocation and type conversions",
				})
			}
		}
	}

	return findings
}

// checkSecurity performs security vulnerability checks.
func (r *ReviewerShard) checkSecurity(filePath, content string) []ReviewFinding {
	findings := make([]ReviewFinding, 0)
	lines := strings.Split(content, "\n")
	lang := r.detectLanguage(filePath)

	// Security patterns to check
	securityPatterns := []struct {
		Pattern    *regexp.Regexp
		RuleID     string
		Severity   string
		Message    string
		Suggestion string
		Languages  []string // Empty means all languages
	}{
		// SQL Injection
		{
			Pattern:    regexp.MustCompile(`(?i)(execute|query|raw)\s*\(\s*["'].*\+.*["']|fmt\.Sprintf\s*\(\s*["'][^"']*%[sv].*["'].*\)\s*\)`),
			RuleID:     "SEC001",
			Severity:   "critical",
			Message:    "Potential SQL injection: string concatenation in query",
			Suggestion: "Use parameterized queries instead",
			Languages:  []string{"go", "python", "java", "javascript"},
		},
		// Command Injection
		{
			Pattern:    regexp.MustCompile(`(?i)(exec\.Command|os\.system|subprocess\.|child_process\.exec)\s*\([^)]*\+`),
			RuleID:     "SEC002",
			Severity:   "critical",
			Message:    "Potential command injection: user input in command execution",
			Suggestion: "Sanitize inputs or use safer alternatives",
		},
		// Hardcoded Secrets
		{
			Pattern:    regexp.MustCompile(`(?i)(password|secret|api_key|apikey|token|credential)\s*[:=]\s*["'][^"']{8,}["']`),
			RuleID:     "SEC003",
			Severity:   "critical",
			Message:    "Hardcoded secret detected",
			Suggestion: "Use environment variables or secret management",
		},
		// XSS (JavaScript/TypeScript)
		{
			Pattern:    regexp.MustCompile(`(?i)(innerHTML|outerHTML|document\.write)\s*=`),
			RuleID:     "SEC004",
			Severity:   "error",
			Message:    "Potential XSS: unsafe DOM manipulation",
			Suggestion: "Use textContent or sanitize HTML input",
			Languages:  []string{"javascript", "typescript"},
		},
		// Path Traversal
		{
			Pattern:    regexp.MustCompile(`(?i)(filepath\.Join|os\.path\.join|path\.join)\s*\([^)]*\+`),
			RuleID:     "SEC005",
			Severity:   "error",
			Message:    "Potential path traversal: unchecked path construction",
			Suggestion: "Validate and sanitize file paths",
		},
		// Insecure Crypto - require word boundary to avoid false positives
		// Matches: md5.New(), sha1.Sum(), crypto/des, rc4.NewCipher
		// Avoids: nodes.map(), somesha1value, etc.
		{
			Pattern:    regexp.MustCompile(`(?i)\b(md5|sha1|des|rc4)\b\s*[\.(]|\bcrypto/(md5|sha1|des|rc4)\b`),
			RuleID:     "SEC006",
			Severity:   "warning",
			Message:    "Weak cryptographic algorithm detected",
			Suggestion: "Use SHA-256 or stronger algorithms",
		},
		// Unsafe Deserialization
		{
			Pattern:    regexp.MustCompile(`(?i)(pickle\.loads|yaml\.load\(|unserialize\(|eval\()`),
			RuleID:     "SEC007",
			Severity:   "critical",
			Message:    "Unsafe deserialization detected",
			Suggestion: "Use safe_load or validate input before deserialization",
		},
		// Debug/Development Code
		{
			Pattern:    regexp.MustCompile(`(?i)(console\.log|print\(|fmt\.Print|debug\s*=\s*true)`),
			RuleID:     "SEC008",
			Severity:   "info",
			Message:    "Debug/logging code detected",
			Suggestion: "Remove or disable in production",
		},
	}

	for lineNum, line := range lines {
		for _, sp := range securityPatterns {
			// Check language filter
			if len(sp.Languages) > 0 && !contains(sp.Languages, lang) {
				continue
			}

			if sp.Pattern.MatchString(line) {
				findings = append(findings, ReviewFinding{
					File:        filePath,
					Line:        lineNum + 1,
					Severity:    sp.Severity,
					Category:    "security",
					RuleID:      sp.RuleID,
					Message:     sp.Message,
					Suggestion:  sp.Suggestion,
					CodeSnippet: strings.TrimSpace(line),
				})
			}
		}
	}

	return findings
}

// checkStyle performs style and formatting checks.
func (r *ReviewerShard) checkStyle(filePath, content string) []ReviewFinding {
	findings := make([]ReviewFinding, 0)
	lines := strings.Split(content, "\n")
	lang := r.detectLanguage(filePath)

	// Style patterns
	stylePatterns := []struct {
		Pattern    *regexp.Regexp
		RuleID     string
		Severity   string
		Message    string
		Suggestion string
		Languages  []string
	}{
		// Long lines
		{
			Pattern:    regexp.MustCompile(`.{121,}`),
			RuleID:     "STY001",
			Severity:   "info",
			Message:    "Line exceeds 120 characters",
			Suggestion: "Break into multiple lines",
		},
		// Trailing whitespace
		{
			Pattern:    regexp.MustCompile(`\s+$`),
			RuleID:     "STY002",
			Severity:   "info",
			Message:    "Trailing whitespace",
			Suggestion: "Remove trailing whitespace",
		},
		// TODO without issue reference
		{
			Pattern:    regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX)`),
			RuleID:     "STY003",
			Severity:   "info",
			Message:    "TODO/FIXME without issue reference",
			Suggestion: "Link to an issue tracker",
		},
		// Magic numbers
		{
			Pattern:    regexp.MustCompile(`[^0-9a-zA-Z_]\d{3,}[^0-9a-zA-Z_]`),
			RuleID:     "STY004",
			Severity:   "info",
			Message:    "Magic number detected",
			Suggestion: "Extract to a named constant",
		},
		// Deep nesting (6+ levels) - more lenient to reduce false positives
		// Matches: 6+ tabs OR 24+ leading spaces (6 levels of 4-space indent)
		// This is high enough to catch genuinely problematic nesting
		{
			Pattern:    regexp.MustCompile(`^\t{6,}|^(    ){6,}|^ {24,}`),
			RuleID:     "STY005",
			Severity:   "warning",
			Message:    "Deep nesting detected (6+ levels)",
			Suggestion: "Consider refactoring to reduce nesting - extract helper functions or use early returns",
		},
		// Go: naked returns in long functions
		{
			Pattern:    regexp.MustCompile(`^\s*return\s*$`),
			RuleID:     "STY006",
			Severity:   "info",
			Message:    "Naked return statement",
			Suggestion: "Consider explicit returns for clarity",
			Languages:  []string{"go"},
		},
	}

	for lineNum, line := range lines {
		for _, sp := range stylePatterns {
			if len(sp.Languages) > 0 && !contains(sp.Languages, lang) {
				continue
			}

			if sp.Pattern.MatchString(line) {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       lineNum + 1,
					Severity:   sp.Severity,
					Category:   "style",
					RuleID:     sp.RuleID,
					Message:    sp.Message,
					Suggestion: sp.Suggestion,
				})
			}
		}
	}

	return findings
}

// checkBugPatterns checks for common bug patterns.
func (r *ReviewerShard) checkBugPatterns(filePath, content string) []ReviewFinding {
	findings := make([]ReviewFinding, 0)
	lines := strings.Split(content, "\n")
	lang := r.detectLanguage(filePath)

	bugPatterns := []struct {
		Pattern    *regexp.Regexp
		RuleID     string
		Severity   string
		Message    string
		Suggestion string
		Languages  []string
	}{
		// Go: ignoring errors
		{
			Pattern:    regexp.MustCompile(`\s+_\s*=\s*\w+\(`),
			RuleID:     "BUG001",
			Severity:   "warning",
			Message:    "Error potentially ignored",
			Suggestion: "Handle or explicitly log the error",
			Languages:  []string{"go"},
		},
		// Null/nil comparisons
		{
			Pattern:    regexp.MustCompile(`(?i)(==\s*nil|==\s*null|===\s*null)\s*\)`),
			RuleID:     "BUG002",
			Severity:   "info",
			Message:    "Explicit null/nil check",
			Suggestion: "Consider using optional chaining or guard clauses",
		},
		// Empty catch blocks
		{
			Pattern:    regexp.MustCompile(`catch\s*\([^)]*\)\s*\{\s*\}`),
			RuleID:     "BUG003",
			Severity:   "error",
			Message:    "Empty catch block - errors silently swallowed",
			Suggestion: "Log or handle the error",
			Languages:  []string{"javascript", "typescript", "java"},
		},
		// Go: defer in loop
		{
			Pattern:    regexp.MustCompile(`for\s.*\{[^}]*defer\s`),
			RuleID:     "BUG004",
			Severity:   "warning",
			Message:    "Defer inside loop - may cause resource leak",
			Suggestion: "Move defer outside loop or use explicit cleanup",
			Languages:  []string{"go"},
		},
		// Rust: unwrap in production code
		{
			Pattern:    regexp.MustCompile(`\.unwrap\(\)|\.expect\(`),
			RuleID:     "BUG005",
			Severity:   "warning",
			Message:    "Panic-inducing unwrap/expect in code",
			Suggestion: "Use proper error handling with ? or match",
			Languages:  []string{"rust"},
		},
	}

	for lineNum, line := range lines {
		for _, bp := range bugPatterns {
			if len(bp.Languages) > 0 && !contains(bp.Languages, lang) {
				continue
			}

			if bp.Pattern.MatchString(line) {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       lineNum + 1,
					Severity:   bp.Severity,
					Category:   "bug",
					RuleID:     bp.RuleID,
					Message:    bp.Message,
					Suggestion: bp.Suggestion,
				})
			}
		}
	}

	return findings
}

// checkLearnedPatterns checks against patterns learned through Autopoiesis.
func (r *ReviewerShard) checkLearnedPatterns(filePath, content string) []ReviewFinding {
	findings := make([]ReviewFinding, 0)
	lines := strings.Split(content, "\n")

	r.mu.RLock()
	antiPatterns := r.learnedAntiPatterns
	r.mu.RUnlock()

	for pattern, reason := range antiPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		for lineNum, line := range lines {
			if re.MatchString(line) {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       lineNum + 1,
					Severity:   "warning",
					Category:   "learned",
					RuleID:     "LEARN001",
					Message:    fmt.Sprintf("Learned anti-pattern: %s", reason),
					Suggestion: "This pattern was flagged in previous reviews",
				})
			}
		}
	}

	return findings
}

// llmAnalysis uses LLM for semantic code analysis.
func (r *ReviewerShard) llmAnalysis(ctx context.Context, filePath, content string) ([]ReviewFinding, error) {
	findings := make([]ReviewFinding, 0)

	// Truncate very long files for LLM
	if len(content) > 10000 {
		content = content[:10000] + "\n... (truncated)"
	}

	systemPrompt := `You are a senior code reviewer. Analyze the code for:
1. Security vulnerabilities (SQL injection, XSS, command injection, etc.)
2. Logic errors and potential bugs
3. Code smells and maintainability issues
4. Performance issues

Return findings as JSON array:
[{"line": N, "severity": "critical|error|warning|info", "category": "security|bug|performance|maintainability", "message": "...", "suggestion": "..."}]

Only report significant issues. Return empty array [] if code is clean.`

	userPrompt := fmt.Sprintf("Review this %s file (%s):\n\n```\n%s\n```",
		r.detectLanguage(filePath), filePath, content)

	response, err := r.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return findings, err
	}

	// Parse JSON response
	var llmFindings []struct {
		Line       int    `json:"line"`
		Severity   string `json:"severity"`
		Category   string `json:"category"`
		Message    string `json:"message"`
		Suggestion string `json:"suggestion"`
	}

	// Extract JSON from response
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart != -1 && jsonEnd > jsonStart {
		jsonStr := response[jsonStart : jsonEnd+1]
		if err := json.Unmarshal([]byte(jsonStr), &llmFindings); err == nil {
			for _, f := range llmFindings {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       f.Line,
					Severity:   f.Severity,
					Category:   f.Category,
					RuleID:     "LLM001",
					Message:    f.Message,
					Suggestion: f.Suggestion,
				})
			}
		}
	}

	return findings, nil
}

// =============================================================================
// METRICS CALCULATION
// =============================================================================

// calculateMetrics calculates code complexity metrics.
func (r *ReviewerShard) calculateMetrics(ctx context.Context, files []string) *CodeMetrics {
	metrics := &CodeMetrics{}

	for _, filePath := range files {
		content, err := r.readFile(ctx, filePath)
		if err != nil {
			continue
		}

		lines := strings.Split(content, "\n")
		metrics.TotalLines += len(lines)

		// Count line types
		inMultiLineComment := false
		currentNesting := 0
		maxNestingInFile := 0
		currentFunctionLines := 0
		inFunction := false

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Blank lines
			if trimmed == "" {
				metrics.BlankLines++
				continue
			}

			// Multi-line comments
			if strings.Contains(line, "/*") {
				inMultiLineComment = true
			}
			if strings.Contains(line, "*/") {
				inMultiLineComment = false
				metrics.CommentLines++
				continue
			}
			if inMultiLineComment {
				metrics.CommentLines++
				continue
			}

			// Single-line comments
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
				metrics.CommentLines++
				continue
			}

			metrics.CodeLines++

			// Track nesting (rough estimate)
			currentNesting += strings.Count(line, "{") - strings.Count(line, "}")
			if currentNesting > maxNestingInFile {
				maxNestingInFile = currentNesting
			}

			// Track function boundaries (Go/C-style)
			if strings.Contains(line, "func ") || strings.Contains(line, "function ") ||
				strings.Contains(line, "def ") || strings.Contains(line, "fn ") {
				if inFunction && currentFunctionLines > 50 {
					metrics.LongFunctions++
				}
				metrics.FunctionCount++
				inFunction = true
				currentFunctionLines = 0
			}

			if inFunction {
				currentFunctionLines++
			}

			// Cyclomatic complexity indicators
			if strings.Contains(line, "if ") || strings.Contains(line, "else ") ||
				strings.Contains(line, "for ") || strings.Contains(line, "while ") ||
				strings.Contains(line, "case ") || strings.Contains(line, "catch ") {
				metrics.CyclomaticMax++ // Simplified - just counting decision points
			}
		}

		if maxNestingInFile > metrics.MaxNesting {
			metrics.MaxNesting = maxNestingInFile
		}

		// Check last function
		if inFunction && currentFunctionLines > 50 {
			metrics.LongFunctions++
		}
	}

	// Calculate average cyclomatic complexity
	if metrics.FunctionCount > 0 {
		metrics.CyclomaticAvg = float64(metrics.CyclomaticMax) / float64(metrics.FunctionCount)
	}

	return metrics
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// readFile reads a file via VirtualStore or directly.
func (r *ReviewerShard) readFile(ctx context.Context, filePath string) (string, error) {
	if r.virtualStore != nil {
		action := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", filePath},
		}
		return r.virtualStore.RouteAction(ctx, action)
	}
	return "", fmt.Errorf("virtualStore required for file operations")
}

// shouldIgnore checks if a file should be ignored.
func (r *ReviewerShard) shouldIgnore(filePath string) bool {
	for _, pattern := range r.reviewerConfig.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return true
		}
		// Also check if pattern is contained in path
		if strings.Contains(filePath, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	return false
}

// detectLanguage detects programming language from file extension.
func (r *ReviewerShard) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".hpp":
		return "cpp"
	default:
		return "unknown"
	}
}

// parseDiffFiles extracts file names from git diff output.
func (r *ReviewerShard) parseDiffFiles(diffOutput string) []string {
	files := make([]string, 0)
	lines := strings.Split(diffOutput, "\n")

	diffFileRegex := regexp.MustCompile(`^diff --git a/(.+) b/`)
	for _, line := range lines {
		if matches := diffFileRegex.FindStringSubmatch(line); len(matches) > 1 {
			files = append(files, matches[1])
		}
	}

	return files
}

// calculateOverallSeverity determines the highest severity from findings.
func (r *ReviewerShard) calculateOverallSeverity(findings []ReviewFinding) ReviewSeverity {
	if len(findings) == 0 {
		return ReviewSeverityClean
	}

	hasCritical := false
	hasError := false
	hasWarning := false

	for _, f := range findings {
		switch f.Severity {
		case "critical":
			hasCritical = true
		case "error":
			hasError = true
		case "warning":
			hasWarning = true
		}
	}

	if hasCritical {
		return ReviewSeverityCritical
	}
	if hasError {
		return ReviewSeverityError
	}
	if hasWarning {
		return ReviewSeverityWarning
	}
	return ReviewSeverityInfo
}

// shouldBlockCommit determines if the review should block commits.
func (r *ReviewerShard) shouldBlockCommit(result *ReviewResult) bool {
	if !r.reviewerConfig.BlockOnCritical {
		return false
	}
	return result.Severity == ReviewSeverityCritical
}

// generateSummary creates a human-readable summary.
func (r *ReviewerShard) generateSummary(result *ReviewResult) string {
	criticalCount := 0
	errorCount := 0
	warningCount := 0
	infoCount := 0

	for _, f := range result.Findings {
		switch f.Severity {
		case "critical":
			criticalCount++
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}

	return fmt.Sprintf("Review complete: %d critical, %d errors, %d warnings, %d info",
		criticalCount, errorCount, warningCount, infoCount)
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// =============================================================================
// FACT GENERATION
// =============================================================================

// assertInitialFacts asserts initial facts to the kernel.
func (r *ReviewerShard) assertInitialFacts(task *ReviewerTask) {
	if r.kernel == nil {
		return
	}

	_ = r.kernel.Assert(core.Fact{
		Predicate: "reviewer_task",
		Args:      []interface{}{r.id, "/" + task.Action, strings.Join(task.Files, ","), time.Now().Unix()},
	})
}

// generateFacts generates facts from review results for propagation.
func (r *ReviewerShard) generateFacts(result *ReviewResult) []core.Fact {
	facts := make([]core.Fact, 0)

	// Review status
	facts = append(facts, core.Fact{
		Predicate: "review_complete",
		Args:      []interface{}{strings.Join(result.Files, ","), "/" + string(result.Severity)},
	})

	// Block commit fact
	if result.BlockCommit {
		facts = append(facts, core.Fact{
			Predicate: "block_commit",
			Args:      []interface{}{"critical_review_findings"},
		})
	}

	// Individual findings
	for _, finding := range result.Findings {
		facts = append(facts, core.Fact{
			Predicate: "review_finding",
			Args: []interface{}{
				finding.File,
				int64(finding.Line),
				"/" + finding.Severity,
				finding.Category,
				finding.Message,
			},
		})

		// Security-specific facts
		if finding.Category == "security" && (finding.Severity == "critical" || finding.Severity == "error") {
			facts = append(facts, core.Fact{
				Predicate: "security_issue",
				Args:      []interface{}{finding.File, int64(finding.Line), finding.RuleID, finding.Message},
			})
		}
	}

	// Metrics facts
	if result.Metrics != nil {
		facts = append(facts, core.Fact{
			Predicate: "code_metrics",
			Args: []interface{}{
				int64(result.Metrics.TotalLines),
				int64(result.Metrics.CodeLines),
				result.Metrics.CyclomaticAvg,
				int64(result.Metrics.FunctionCount),
			},
		})
	}

	// Autopoiesis facts
	r.mu.RLock()
	for pattern, count := range r.flaggedPatterns {
		if count >= 3 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"anti_pattern", pattern},
			})
		}
	}
	for pattern, count := range r.approvedPatterns {
		if count >= 5 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"approved_style", pattern},
			})
		}
	}
	r.mu.RUnlock()

	// Specialist recommendation facts
	// These enable the main agent and kernel to suggest specialists for follow-up tasks
	for _, rec := range result.SpecialistRecommendations {
		// suggest_use_specialist(TaskType, SpecialistName)
		facts = append(facts, core.Fact{
			Predicate: "suggest_use_specialist",
			Args:      []interface{}{"/" + rec.ShardName, rec.Reason},
		})

		// shard_can_handle(ShardType, TaskType) for each task hint
		for _, hint := range rec.TaskHints {
			facts = append(facts, core.Fact{
				Predicate: "shard_can_handle",
				Args:      []interface{}{"/" + rec.ShardName, hint},
			})
		}

		// specialist_recommended(ShardName, FilePath, Confidence)
		for _, file := range rec.ForFiles {
			facts = append(facts, core.Fact{
				Predicate: "specialist_recommended",
				Args:      []interface{}{"/" + rec.ShardName, file, rec.Confidence},
			})
		}
	}

	return facts
}

// =============================================================================
// AUTOPOIESIS (SELF-IMPROVEMENT)
// =============================================================================

// trackReviewPatterns tracks patterns for Autopoiesis (§8.3).
func (r *ReviewerShard) trackReviewPatterns(result *ReviewResult) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, finding := range result.Findings {
		// Track flagged patterns
		if finding.Severity == "critical" || finding.Severity == "error" {
			pattern := normalizeReviewPattern(finding.Message)
			r.flaggedPatterns[pattern]++
		}
	}
}

// LearnAntiPattern adds a new anti-pattern to watch for.
func (r *ReviewerShard) LearnAntiPattern(pattern, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.learnedAntiPatterns[pattern] = reason
}

// normalizeReviewPattern normalizes a finding message into a pattern key.
func normalizeReviewPattern(s string) string {
	// Remove specific values, keep structure
	re := regexp.MustCompile(`\d+`)
	normalized := re.ReplaceAllString(s, "N")
	if len(normalized) > 100 {
		normalized = normalized[:100]
	}
	return strings.ToLower(normalized)
}

// =============================================================================
// OUTPUT FORMATTING
// =============================================================================

// formatResult formats a ReviewResult for human-readable output.
func (r *ReviewerShard) formatResult(result *ReviewResult) string {
	var sb strings.Builder

	// Header
	status := "✓ PASSED"
	if result.BlockCommit {
		status = "✗ BLOCKED"
	} else if result.Severity == ReviewSeverityError || result.Severity == ReviewSeverityCritical {
		status = "⚠ ISSUES FOUND"
	}

	sb.WriteString(fmt.Sprintf("%s - %s (%s)\n", status, result.Summary, result.Duration))
	sb.WriteString(fmt.Sprintf("Files reviewed: %d\n", len(result.Files)))

	// Group findings by severity
	if len(result.Findings) > 0 {
		sb.WriteString("\nFindings:\n")

		// Critical first
		for _, f := range result.Findings {
			if f.Severity == "critical" {
				sb.WriteString(fmt.Sprintf("  [CRITICAL] %s:%d - %s\n", f.File, f.Line, f.Message))
				if f.Suggestion != "" {
					sb.WriteString(fmt.Sprintf("    → %s\n", f.Suggestion))
				}
			}
		}

		// Then errors
		for _, f := range result.Findings {
			if f.Severity == "error" {
				sb.WriteString(fmt.Sprintf("  [ERROR] %s:%d - %s\n", f.File, f.Line, f.Message))
				if f.Suggestion != "" {
					sb.WriteString(fmt.Sprintf("    → %s\n", f.Suggestion))
				}
			}
		}

		// Then warnings (limited)
		warningCount := 0
		for _, f := range result.Findings {
			if f.Severity == "warning" {
				warningCount++
				if warningCount <= 5 {
					sb.WriteString(fmt.Sprintf("  [WARN] %s:%d - %s\n", f.File, f.Line, f.Message))
				}
			}
		}
		if warningCount > 5 {
			sb.WriteString(fmt.Sprintf("  ... and %d more warnings\n", warningCount-5))
		}

		// Info count only
		infoCount := 0
		for _, f := range result.Findings {
			if f.Severity == "info" {
				infoCount++
			}
		}
		if infoCount > 0 {
			sb.WriteString(fmt.Sprintf("  (%d info-level suggestions)\n", infoCount))
		}
	}

	// Metrics summary
	if result.Metrics != nil {
		sb.WriteString(fmt.Sprintf("\nMetrics: %d lines (%d code, %d comments), %d functions\n",
			result.Metrics.TotalLines, result.Metrics.CodeLines,
			result.Metrics.CommentLines, result.Metrics.FunctionCount))
		if result.Metrics.CyclomaticMax > 10 {
			sb.WriteString(fmt.Sprintf("  ⚠ Max cyclomatic complexity: %d\n", result.Metrics.CyclomaticMax))
		}
		if result.Metrics.MaxNesting > 4 {
			sb.WriteString(fmt.Sprintf("  ⚠ Max nesting depth: %d\n", result.Metrics.MaxNesting))
		}
	}

	// Specialist recommendations
	if len(result.SpecialistRecommendations) > 0 {
		sb.WriteString("\n## Specialist Recommendations\n")
		for _, rec := range result.SpecialistRecommendations {
			sb.WriteString(fmt.Sprintf("  → **%s** (%.0f%% confidence): %s\n",
				rec.ShardName, rec.Confidence*100, rec.Reason))
			if len(rec.TaskHints) > 0 {
				sb.WriteString(fmt.Sprintf("    Suggested tasks: %s\n", strings.Join(rec.TaskHints, ", ")))
			}
		}
	}

	return sb.String()
}

// =============================================================================
// SPECIALIST RECOMMENDATION SYSTEM
// =============================================================================
// Detects technologies in reviewed code and recommends specialist shards.

// TechnologyPattern maps file patterns and imports to specialist shards.
type TechnologyPattern struct {
	ShardName    string            // Specialist shard name
	FilePatterns []string          // File path patterns (glob-like)
	ImportHints  []string          // Import statements to look for
	ContentHints []string          // Code patterns to detect
	TaskHints    []string          // Suggested tasks for this specialist
	Description  string            // Human-readable description
}

// knownTechnologies defines the mapping from code patterns to specialist shards.
var knownTechnologies = []TechnologyPattern{
	{
		ShardName:    "rod",
		FilePatterns: []string{"browser", "scraper", "crawler", "selenium", "playwright"},
		ImportHints:  []string{"github.com/go-rod/rod", "chromedp", "selenium", "playwright"},
		ContentHints: []string{"Browser()", "MustPage", "MustElement", "CDP", "DevTools"},
		TaskHints:    []string{"implement browser automation", "add page scraping", "fix element selection"},
		Description:  "Browser automation with Rod/CDP",
	},
	{
		ShardName:    "golang",
		FilePatterns: []string{".go"},
		ImportHints:  []string{}, // Any Go file qualifies
		ContentHints: []string{"func ", "type ", "interface ", "struct "},
		TaskHints:    []string{"refactor code", "add error handling", "improve concurrency"},
		Description:  "Go language patterns and idioms",
	},
	{
		ShardName:    "react",
		FilePatterns: []string{".tsx", ".jsx", "component"},
		ImportHints:  []string{"react", "useState", "useEffect", "next/"},
		ContentHints: []string{"<", "/>", "useState", "useEffect", "className"},
		TaskHints:    []string{"add component", "fix state management", "improve rendering"},
		Description:  "React/Next.js frontend development",
	},
	{
		ShardName:    "mangle",
		FilePatterns: []string{".gl", "mangle", "policy", "schema"},
		ImportHints:  []string{},
		ContentHints: []string{"Decl ", ":-", "fn:", "let "},
		TaskHints:    []string{"add policy rule", "define predicate", "fix constraint"},
		Description:  "Mangle/Datalog logic programming",
	},
	{
		ShardName:    "sql",
		FilePatterns: []string{"database", "store", "repository", "dao"},
		ImportHints:  []string{"database/sql", "sqlx", "gorm", "pgx", "sqlite"},
		ContentHints: []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE TABLE"},
		TaskHints:    []string{"optimize query", "add migration", "fix schema"},
		Description:  "SQL database operations",
	},
	{
		ShardName:    "api",
		FilePatterns: []string{"handler", "controller", "endpoint", "route", "api"},
		ImportHints:  []string{"net/http", "gin", "echo", "fiber", "chi"},
		ContentHints: []string{"http.Handler", "c.JSON", "r.GET", "r.POST"},
		TaskHints:    []string{"add endpoint", "fix authentication", "improve validation"},
		Description:  "REST API development",
	},
	{
		ShardName:    "testing",
		FilePatterns: []string{"_test.go", "test_", "spec."},
		ImportHints:  []string{"testing", "testify", "gomock", "jest", "pytest"},
		ContentHints: []string{"func Test", "t.Run", "assert.", "expect("},
		TaskHints:    []string{"add test cases", "improve coverage", "fix flaky test"},
		Description:  "Test writing and coverage",
	},
}

// detectSpecialists analyzes file content and returns specialist recommendations.
func (r *ReviewerShard) detectSpecialists(files []string, contents map[string]string) []SpecialistRecommendation {
	var recommendations []SpecialistRecommendation
	shardScores := make(map[string]*SpecialistRecommendation)

	for _, file := range files {
		content := contents[file]
		lowerFile := strings.ToLower(file)
		lowerContent := strings.ToLower(content)

		for _, tech := range knownTechnologies {
			score := 0.0
			matches := false

			// Check file patterns
			for _, pattern := range tech.FilePatterns {
				if strings.Contains(lowerFile, strings.ToLower(pattern)) {
					score += 0.3
					matches = true
					break
				}
			}

			// Check import hints
			for _, imp := range tech.ImportHints {
				if strings.Contains(content, imp) {
					score += 0.4
					matches = true
					break
				}
			}

			// Check content hints
			hintMatches := 0
			for _, hint := range tech.ContentHints {
				if strings.Contains(lowerContent, strings.ToLower(hint)) {
					hintMatches++
				}
			}
			if len(tech.ContentHints) > 0 && hintMatches > 0 {
				score += float64(hintMatches) / float64(len(tech.ContentHints)) * 0.3
				matches = true
			}

			// Only add if we found matches
			if matches && score > 0.2 {
				if existing, ok := shardScores[tech.ShardName]; ok {
					// Update existing
					if score > existing.Confidence {
						existing.Confidence = score
					}
					existing.ForFiles = append(existing.ForFiles, file)
				} else {
					// Create new
					shardScores[tech.ShardName] = &SpecialistRecommendation{
						ShardName:  tech.ShardName,
						Reason:     tech.Description,
						Confidence: score,
						ForFiles:   []string{file},
						TaskHints:  tech.TaskHints,
					}
				}
			}
		}
	}

	// Convert to slice and sort by confidence
	for _, rec := range shardScores {
		// Cap confidence at 1.0
		if rec.Confidence > 1.0 {
			rec.Confidence = 1.0
		}
		// Only include high-confidence recommendations
		if rec.Confidence >= 0.3 {
			recommendations = append(recommendations, *rec)
		}
	}

	// Sort by confidence (descending)
	for i := 0; i < len(recommendations); i++ {
		for j := i + 1; j < len(recommendations); j++ {
			if recommendations[j].Confidence > recommendations[i].Confidence {
				recommendations[i], recommendations[j] = recommendations[j], recommendations[i]
			}
		}
	}

	// Limit to top 3 recommendations
	if len(recommendations) > 3 {
		recommendations = recommendations[:3]
	}

	return recommendations
}
