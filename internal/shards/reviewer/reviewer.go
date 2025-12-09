// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// It specializes in code review, security scanning, and best practices analysis.
package reviewer

import (
	"codenerd/internal/core"
	"codenerd/internal/store"
	"context"
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
	CustomRulesPath string   // Path to custom rules JSON file (default: .nerd/review-rules.json)
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
		CustomRulesPath: ".nerd/review-rules.json",
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

// ReviewResult represents the outcome of a code review.
type ReviewResult struct {
	Files       []string        `json:"files"`
	Findings    []ReviewFinding `json:"findings"`
	Severity    ReviewSeverity  `json:"severity"`
	Summary     string          `json:"summary"`
	Duration    time.Duration   `json:"duration"`
	BlockCommit bool            `json:"block_commit"`
	Metrics     *CodeMetrics    `json:"metrics,omitempty"`

	// Textual analysis report from LLM (Markdown)
	AnalysisReport string `json:"analysis_report,omitempty"`

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
	kernel       *core.RealKernel   // Own kernel instance for logic-driven review
	llmClient    core.LLMClient     // LLM for semantic analysis
	virtualStore *core.VirtualStore // Action routing

	// State tracking
	startTime time.Time
	findings  []ReviewFinding
	severity  ReviewSeverity

	// Custom rules
	customRules []CustomRule // User-defined custom review rules

	// Autopoiesis tracking (§8.3) - in-memory, synced to LearningStore
	approvedPatterns    map[string]int     // Patterns that pass review
	flaggedPatterns     map[string]int     // Patterns that get flagged repeatedly
	learnedAntiPatterns map[string]string  // Anti-patterns learned from rejections
	learningStore       core.LearningStore // Persistent learning storage

	// Policy loading guard (prevents duplicate Decl errors)
	policyLoaded bool
}

// NewReviewerShard creates a new Reviewer shard with default configuration.
func NewReviewerShard() *ReviewerShard {
	return NewReviewerShardWithConfig(DefaultReviewerConfig())
}

// NewReviewerShardWithConfig creates a reviewer shard with custom configuration.
func NewReviewerShardWithConfig(reviewerConfig ReviewerConfig) *ReviewerShard {
	shard := &ReviewerShard{
		config:              core.DefaultSpecialistConfig("reviewer", ""),
		state:               core.ShardStateIdle,
		reviewerConfig:      reviewerConfig,
		findings:            make([]ReviewFinding, 0),
		severity:            ReviewSeverityClean,
		customRules:         make([]CustomRule, 0),
		approvedPatterns:    make(map[string]int),
		flaggedPatterns:     make(map[string]int),
		learnedAntiPatterns: make(map[string]string),
	}

	// Attempt to load custom rules if path is configured
	if reviewerConfig.CustomRulesPath != "" {
		_ = shard.LoadCustomRules(reviewerConfig.CustomRulesPath)
	}

	return shard
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

// SetSessionContext sets the session context (for dream mode, etc.).
func (r *ReviewerShard) SetSessionContext(ctx *core.SessionContext) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.SessionContext = ctx
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

// SetLearningStore sets the learning store for persistent autopoiesis.
func (r *ReviewerShard) SetLearningStore(ls core.LearningStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.learningStore = ls
	// Load existing patterns from store
	r.loadLearnedPatterns()
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
// DREAM MODE (Simulation/Learning)
// =============================================================================

// describeDreamPlan returns a description of what the reviewer would do WITHOUT executing.
func (r *ReviewerShard) describeDreamPlan(ctx context.Context, task string) (string, error) {
	fmt.Printf("[ReviewerShard] DREAM MODE - describing plan without execution\n")

	if r.llmClient == nil {
		return "ReviewerShard would analyze code for issues, but no LLM client available for dream description.", nil
	}

	prompt := fmt.Sprintf(`You are a code reviewer agent in DREAM MODE. Describe what you WOULD do for this task WITHOUT actually doing it.

Task: %s

Provide a structured analysis:
1. **Understanding**: What kind of review is being asked?
2. **Files to Review**: What files would I examine?
3. **Review Approach**: What checks would I perform? (style, security, complexity, etc.)
4. **Tools Needed**: What analysis tools would I use?
5. **Potential Findings**: What types of issues might I look for?
6. **Questions**: What would I need clarified?

Remember: This is a simulation. Describe the plan, don't execute it.`, task)

	response, err := r.llmClient.Complete(ctx, prompt)
	if err != nil {
		return fmt.Sprintf("ReviewerShard dream analysis failed: %v", err), nil
	}

	return response, nil
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

	// DREAM MODE: Only describe what we would do, don't execute
	if r.config.SessionContext != nil && r.config.SessionContext.DreamMode {
		return r.describeDreamPlan(ctx, task)
	}

	fmt.Printf("[ReviewerShard:%s] Starting task: %s\n", r.id, task)

	// Initialize kernel if not set
	if r.kernel == nil {
		r.kernel = core.NewRealKernel()
	}
	// Load reviewer-specific policy (only once to avoid duplicate Decl errors)
	if !r.policyLoaded {
		_ = r.kernel.LoadPolicyFile("reviewer.mg")
		r.policyLoaded = true
	}

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
			fmt.Printf("[ReviewerShard:%s] Found %d upstream, %d downstream deps for %s\n",
				r.id, len(depCtx.Upstream), len(depCtx.Downstream), filePath)
		}

		// Perform Holographic Architecture Analysis
		archCtx := r.analyzeArchitecture(ctx, filePath)
		archContexts[filePath] = archCtx

		// Store for specialist detection
		fileContents[filePath] = content
		reviewedFiles = append(reviewedFiles, filePath)

		// Run all checks (now with dependency AND architectural context)
		findings, report := r.analyzeFileWithDeps(ctx, filePath, content, depCtx, archCtx)
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
			fmt.Printf("[ReviewerShard:%s] Suppressed %d findings via Mangle rules\n", r.id, len(result.Findings)-len(activeFindings))
		}
		result.Findings = activeFindings
	} else {
		fmt.Printf("[ReviewerShard:%s] Failed to filter with Mangle, using raw findings: %v\n", r.id, err)
	}

	// 3. Persist findings to database
	r.persistFindings(result.Findings)

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

// =============================================================================
// FILE ANALYSIS
// =============================================================================

// analyzeFile runs all analysis checks on a file (no dependency context).
func (r *ReviewerShard) analyzeFile(ctx context.Context, filePath, content string) ([]ReviewFinding, string) {
	return r.analyzeFileWithDeps(ctx, filePath, content, nil, nil)
}

// ArchitectureAnalysis holds the "Holographic" view of the code.
type ArchitectureAnalysis struct {
	Module      string   `json:"module"`       // The module this file belongs to
	Layer       string   `json:"layer"`        // e.g., "core", "api", "data"
	Related     []string `json:"related"`      // Semantically related entities
	Role        string   `json:"role"`         // Deduced role (adapter, service, model)
	SystemValue string   `json:"system_value"` // High-level system purpose
}

// analyzeArchitecture performs a "Holographic" analysis using the knowledge graph.
func (r *ReviewerShard) analyzeArchitecture(ctx context.Context, filePath string) *ArchitectureAnalysis {
	analysis := &ArchitectureAnalysis{
		Module: "unknown",
		Layer:  "unknown",
		Role:   "unknown",
	}

	if r.virtualStore == nil {
		return analysis
	}

	localDB := r.virtualStore.GetLocalDB()
	if localDB == nil {
		return analysis
	}

	// 1. Determine Module/Layer from path
	// Simple heuristic fallback if graph is empty
	parts := strings.Split(filePath, "/")
	if len(parts) > 1 {
		for i, part := range parts {
			if part == "internal" || part == "pkg" || part == "cmd" {
				if i+1 < len(parts) {
					analysis.Module = parts[i+1]
				}
				analysis.Layer = part
				break
			}
		}
	}

	// 2. Query Knowledge Graph for relationships
	// Using "contains" or "defines" relations
	links, err := localDB.QueryLinks(filePath, "incoming")
	if err == nil {
		for _, link := range links {
			if link.Relation == "defines" || link.Relation == "contains" {
				// The container (directory/package) is the entity A
				analysis.Module = link.EntityA
			}
		}
	}

	// 3. Find related entities (semantic neighbors)
	// Using "imports" or "calls"
	outgoing, err := localDB.QueryLinks(filePath, "outgoing")
	if err == nil {
		for _, link := range outgoing {
			if link.Relation == "imports" || link.Relation == "depends_on" {
				analysis.Related = append(analysis.Related, link.EntityB)
			}
		}
	}

	return analysis
}

// analyzeFileWithDeps runs all analysis checks on a file with optional dependency and architectural context.
// Returns findings and the LLM analysis report (if any).
func (r *ReviewerShard) analyzeFileWithDeps(ctx context.Context, filePath, content string, depCtx *DependencyContext, archCtx *ArchitectureAnalysis) ([]ReviewFinding, string) {
	findings := make([]ReviewFinding, 0)
	var report string

	// Code DOM safety checks (check kernel facts first)
	findings = append(findings, r.checkCodeDOMSafety(filePath)...)

	// Security checks
	findings = append(findings, r.checkSecurity(filePath, content)...)

	// Style checks
	findings = append(findings, r.checkStyle(filePath, content)...)

	// Bug pattern checks
	findings = append(findings, r.checkBugPatterns(filePath, content)...)

	// Custom rules checks (user-defined patterns)
	findings = append(findings, r.checkCustomRules(filePath, content)...)

	// LLM-powered semantic analysis (if available) - now with dependency and architectural context
	if r.llmClient != nil {
		var err error
		var llmFindings []ReviewFinding
		llmFindings, report, err = r.llmAnalysisWithDeps(ctx, filePath, content, depCtx, archCtx)
		if err == nil {
			findings = append(findings, llmFindings...)
		} else {
			// Log LLM failure but continue with regex-based checks
			fmt.Printf("[ReviewerShard:%s] LLM analysis failed for %s, continuing with regex checks: %v\n", r.id, filePath, err)
		}
	}

	// Check against learned anti-patterns
	findings = append(findings, r.checkLearnedPatterns(filePath, content)...)

	return findings, report
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
// PERSISTENCE & FILTERING HELPERS
// =============================================================================

// assertFileFacts asserts file topology facts to the kernel (e.g., for test file detection).
func (r *ReviewerShard) assertFileFacts(filePath string) {
	if r.kernel == nil {
		return
	}

	isTest := strings.HasSuffix(filePath, "_test.go") || strings.Contains(filePath, "test/")
	testAtom := core.MangleAtom("/false")
	if isTest {
		testAtom = core.MangleAtom("/true")
	}

	// file_topology(Path, Hash, Language, LastModified, IsTestFile)
	// Using placeholders for Hash/Time as they aren't critical for suppression rules yet
	fact := core.Fact{
		Predicate: "file_topology",
		Args:      []interface{}{filePath, "unknown_hash", r.detectLanguage(filePath), "unknown_time", testAtom},
	}
	_ = r.kernel.Assert(fact)
}

// filterFindingsWithMangle asserts findings to Mangle and queries back only the active ones.
func (r *ReviewerShard) filterFindingsWithMangle(findings []ReviewFinding) ([]ReviewFinding, error) {
	if r.kernel == nil {
		return findings, nil
	}

	// 1. Assert raw findings
	for _, f := range findings {
		fact := core.Fact{
			Predicate: "raw_finding",
			Args:      []interface{}{f.File, f.Line, f.Severity, f.Category, f.RuleID, f.Message},
		}
		_ = r.kernel.Assert(fact)
	}

	// 2. Query active findings
	results, err := r.kernel.Query("active_finding")
	if err != nil {
		return nil, err
	}

	// 3. Reconstruct list
	var active []ReviewFinding
	for _, res := range results {
		if len(res.Args) < 6 {
			continue
		}
		f := ReviewFinding{
			File:     res.Args[0].(string),
			Line:     toStartInt(res.Args[1]),
			Severity: res.Args[2].(string),
			Category: res.Args[3].(string),
			RuleID:   res.Args[4].(string),
			Message:  res.Args[5].(string),
		}
		active = append(active, f)
	}

	return active, nil
}

// persistFindings stores findings in the LocalStore.
func (r *ReviewerShard) persistFindings(findings []ReviewFinding) {
	if r.virtualStore == nil || r.virtualStore.GetLocalDB() == nil {
		return
	}
	localDB := r.virtualStore.GetLocalDB()
	root := r.reviewerConfig.WorkingDir

	for _, f := range findings {
		// Use the DTO defined in store package
		sf := store.StoredReviewFinding{
			FilePath:    f.File,
			Line:        f.Line,
			Severity:    f.Severity,
			Category:    f.Category,
			RuleID:      f.RuleID,
			Message:     f.Message,
			ProjectRoot: root,
		}
		_ = localDB.StoreReviewFinding(sf)
	}
}

// Helper to safely convert interface{} to int
func toStartInt(v interface{}) int {
	if i, ok := v.(int); ok {
		return i
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}
