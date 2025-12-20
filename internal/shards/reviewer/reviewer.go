// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// It specializes in code review, security scanning, and best practices analysis.
package reviewer

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
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

	// Neuro-symbolic pipeline configuration
	UseNeuroSymbolic bool // Enable neuro-symbolic pipeline (default: true for Go files)
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
		MaxFindings:      100,
		BlockOnCritical:  true,
		IncludeMetrics:   true,
		SeverityFilter:   "info",
		WorkingDir:       ".",
		IgnorePatterns:   []string{"vendor/", "node_modules/", ".git/", "*.min.js"},
		MaxFileSize:      1024 * 1024, // 1MB
		CustomRulesPath:  ".nerd/review-rules.json",
		UseNeuroSymbolic: true, // Enable by default for Go files
		// NeuroSymbolicConfig will be initialized with defaults in Execute if not set
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
	config types.ShardConfig
	state  types.ShardState

	// Reviewer-specific configuration
	reviewerConfig ReviewerConfig

	// Components (required)
	kernel       *core.RealKernel   // Own kernel instance for logic-driven review
	llmClient    types.LLMClient    // LLM for semantic analysis
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

	// Holographic context provider for package-aware analysis
	holographicProvider HolographicProvider

	// JIT prompt compilation support
	promptAssembler *articulation.PromptAssembler
}

// HolographicProvider interface for package-level context.
// Implemented by world.HolographicProvider.
type HolographicProvider interface {
	GetContext(filePath string) (*HolographicContext, error)
}

// HolographicContext represents rich package-level context.
// Mirrors world.HolographicContext for decoupling.
type HolographicContext struct {
	TargetFile        string
	TargetPkg         string
	PackageSiblings   []string
	PackageSignatures []SymbolSignature
	PackageTypes      []TypeDefinition
	Layer             string
	Module            string
	Role              string
	SystemPurpose     string
	HasTests          bool
}

// SymbolSignature represents a function signature from sibling files.
type SymbolSignature struct {
	Name       string
	Receiver   string
	Params     string
	Returns    string
	File       string
	Line       int
	Exported   bool
	DocComment string
}

// TypeDefinition represents a type from sibling files.
type TypeDefinition struct {
	Name     string
	Kind     string
	Fields   []string
	Methods  []string
	File     string
	Line     int
	Exported bool
}

// NewReviewerShard creates a new Reviewer shard with default configuration.
func NewReviewerShard() *ReviewerShard {
	return NewReviewerShardWithConfig(DefaultReviewerConfig())
}

// NewReviewerShardWithConfig creates a reviewer shard with custom configuration.
func NewReviewerShardWithConfig(reviewerConfig ReviewerConfig) *ReviewerShard {
	shard := &ReviewerShard{
		config:              coreshards.DefaultSpecialistConfig("reviewer", ""),
		state:               types.ShardStateIdle,
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
func (r *ReviewerShard) SetLLMClient(client types.LLMClient) {
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
func (r *ReviewerShard) SetParentKernel(k types.Kernel) {
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

// SetHolographicProvider sets the holographic context provider for package-aware analysis.
func (r *ReviewerShard) SetHolographicProvider(hp HolographicProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.holographicProvider = hp
}

// SetPromptAssembler sets the JIT prompt assembler for dynamic prompt compilation.
func (r *ReviewerShard) SetPromptAssembler(pa *articulation.PromptAssembler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.promptAssembler = pa
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
func (r *ReviewerShard) GetState() types.ShardState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// GetConfig returns the shard configuration.
func (r *ReviewerShard) GetConfig() types.ShardConfig {
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
	r.state = types.ShardStateCompleted
	return nil
}

// =============================================================================
// DREAM MODE (Simulation/Learning)
// =============================================================================

// describeDreamPlan returns a description of what the reviewer would do WITHOUT executing.
func (r *ReviewerShard) describeDreamPlan(ctx context.Context, task string) (string, error) {
	logging.ReviewerDebug("DREAM MODE - describing plan without execution")

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
	r.state = types.ShardStateRunning
	r.startTime = time.Now()
	r.id = fmt.Sprintf("reviewer-%d", time.Now().UnixNano())
	r.findings = make([]ReviewFinding, 0)
	r.severity = ReviewSeverityClean
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.state = types.ShardStateCompleted
		r.mu.Unlock()
	}()

	// DREAM MODE: Only describe what we would do, don't execute
	if r.config.SessionContext != nil && r.config.SessionContext.DreamMode {
		logging.ReviewerDebug("Dream mode enabled, describing plan only")
		return r.describeDreamPlan(ctx, task)
	}

	logging.Reviewer("Starting review task: %s", task)
	logging.ReviewerDebug("Reviewer ID: %s", r.id)

	// Initialize kernel if not set
	if r.kernel == nil {
		kernel, err := core.NewRealKernel()
		if err != nil {
			return "", fmt.Errorf("failed to create kernel: %w", err)
		}
		r.kernel = kernel
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
	r.expandTaskFiles(parsedTask)

	// Assert initial facts to kernel
	r.assertInitialFacts(parsedTask)

	// Route to appropriate handler
	var result *ReviewResult
	var neuroResult *NeuroSymbolicResult

	// Determine if we should use the neuro-symbolic pipeline
	useNeuroSymbolic := r.shouldUseNeuroSymbolic(parsedTask)

	// Build config with enhancement flag from parsed task
	config := DefaultNeuroSymbolicConfig()
	if parsedTask.EnableEnhancement {
		config.EnableCreativeEnhancement = true
		config.EnableSelfInterrogation = true
		logging.Reviewer("Creative enhancement enabled (Steps 8-12)")
	}

	switch parsedTask.Action {
	case "review":
		if useNeuroSymbolic {
			neuroResult, err = r.executeNeuroSymbolicReview(ctx, parsedTask, config)
			if neuroResult != nil {
				result = neuroResult.ReviewResult
			}
		} else {
			result, err = r.reviewFiles(ctx, parsedTask)
		}
	case "security_scan":
		result, err = r.securityScan(ctx, parsedTask)
	case "style_check":
		result, err = r.styleCheck(ctx, parsedTask)
	case "complexity":
		result, err = r.complexityAnalysis(ctx, parsedTask)
	case "diff":
		if useNeuroSymbolic {
			neuroResult, err = r.executeNeuroSymbolicReview(ctx, parsedTask, config)
			if neuroResult != nil {
				result = neuroResult.ReviewResult
			}
		} else {
			result, err = r.reviewDiff(ctx, parsedTask)
		}
	default:
		if useNeuroSymbolic {
			neuroResult, err = r.executeNeuroSymbolicReview(ctx, parsedTask, config)
			if neuroResult != nil {
				result = neuroResult.ReviewResult
			}
		} else {
			result, err = r.reviewFiles(ctx, parsedTask)
		}
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

	// Format output - use neuro-symbolic format if available
	if neuroResult != nil {
		return r.formatNeuroSymbolicResult(neuroResult), nil
	}
	return r.formatResult(result), nil
}

// shouldUseNeuroSymbolic determines if the neuro-symbolic pipeline should be used.
// The pipeline is enabled when:
// 1. UseNeuroSymbolic is true in config
// 2. At least some files are Go files (pipeline is optimized for Go)
// 3. Kernel is available for Mangle hypothesis generation
func (r *ReviewerShard) shouldUseNeuroSymbolic(task *ReviewerTask) bool {
	// Check config flag
	if !r.reviewerConfig.UseNeuroSymbolic {
		return false
	}

	// Check if kernel is available (required for hypothesis generation)
	if r.kernel == nil {
		logging.ReviewerDebug("Neuro-symbolic disabled: no kernel available")
		return false
	}

	// Check if any Go files are in the task
	hasGoFiles := false
	for _, f := range task.Files {
		if strings.HasSuffix(f, ".go") {
			hasGoFiles = true
			break
		}
	}

	// For diff reviews, we'll check files after parsing
	if task.Action == "diff" {
		// Enable for diffs - they'll be parsed for Go files later
		return true
	}

	if !hasGoFiles {
		logging.ReviewerDebug("Neuro-symbolic disabled: no Go files in task")
		return false
	}

	return true
}

// =============================================================================
// TASK PARSING
// =============================================================================

// ReviewerTask represents a parsed review task.
type ReviewerTask struct {
	Action            string   // "review", "security_scan", "style_check", "complexity", "diff"
	Files             []string // Files to review
	DiffRef           string   // Git diff reference (e.g., "HEAD~1")
	Options           map[string]string
	EnableEnhancement bool // --andEnhance flag enables creative suggestions (Steps 8-12)
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
		} else if strings.HasPrefix(part, "--") {
			// Handle double-dash flags
			flag := strings.TrimPrefix(part, "--")
			switch strings.ToLower(flag) {
			case "andenhance", "enhance":
				parsed.EnableEnhancement = true
				logging.ReviewerDebug("Enhancement mode enabled via --%s flag", flag)
			}
		} else if !strings.HasPrefix(part, "-") {
			// Bare argument - treat as file
			parsed.Files = append(parsed.Files, part)
		}
	}

	return parsed, nil
}

func (r *ReviewerShard) expandTaskFiles(task *ReviewerTask) {
	if task == nil || task.Action == "diff" {
		return
	}

	explicit := make([]string, 0, len(task.Files))
	broadRequested := false
	for _, file := range task.Files {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			continue
		}
		if isBroadTargetToken(trimmed) {
			broadRequested = true
			continue
		}
		explicit = append(explicit, trimmed)
	}

	if len(explicit) > 0 && !broadRequested {
		task.Files = explicit
		return
	}
	task.Files = explicit

	if r.kernel == nil {
		if len(task.Files) == 0 {
			logging.ReviewerDebug("Skipping file expansion: kernel unavailable")
		}
		return
	}

	facts, err := r.kernel.Query("file_topology")
	if err != nil {
		logging.ReviewerDebug("file_topology query failed: %v", err)
		return
	}

	expanded := make([]string, 0, len(facts))
	for _, fact := range facts {
		if len(fact.Args) < 5 {
			continue
		}
		path := fmt.Sprintf("%v", fact.Args[0])
		isTest := fmt.Sprintf("%v", fact.Args[4])
		if isTest == "/true" {
			continue
		}
		if !isCodeFile(path) {
			continue
		}
		expanded = append(expanded, path)
	}

	if len(expanded) == 0 {
		return
	}

	task.Files = dedupeFiles(append(task.Files, expanded...))
	if len(task.Files) > 50 {
		task.Files = task.Files[:50]
	}
}

func isBroadTargetToken(token string) bool {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "all", "codebase", "*", ".", "repo", "project", "workspace":
		return true
	default:
		return false
	}
}

func isCodeFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rs", ".java", ".c", ".cpp", ".h", ".cs", ".rb", ".php":
		return true
	default:
		return false
	}
}

func dedupeFiles(files []string) []string {
	seen := make(map[string]struct{}, len(files))
	result := make([]string, 0, len(files))
	for _, file := range files {
		if file == "" {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		result = append(result, file)
	}
	return result
}

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
			logging.Get(logging.CategoryReviewer).Warn("LLM analysis failed for %s, continuing with regex checks: %v", filePath, err)
		}
	}

	// Check against learned anti-patterns
	findings = append(findings, r.checkLearnedPatterns(filePath, content)...)

	return findings, report
}

// analyzeFileWithHolographic runs all analysis checks with full holographic context.
// This is the enhanced version that includes package sibling awareness.
func (r *ReviewerShard) analyzeFileWithHolographic(ctx context.Context, filePath, content string, depCtx *DependencyContext, archCtx *ArchitectureAnalysis, holoCtx *HolographicContext) ([]ReviewFinding, string) {
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

	// LLM-powered semantic analysis with FULL holographic context
	if r.llmClient != nil {
		var err error
		var llmFindings []ReviewFinding
		llmFindings, report, err = r.llmAnalysisWithHolographic(ctx, filePath, content, depCtx, archCtx, holoCtx)
		if err == nil {
			findings = append(findings, llmFindings...)
		} else {
			// Log LLM failure but continue with regex-based checks
			logging.Get(logging.CategoryReviewer).Warn("LLM analysis failed for %s, continuing with regex checks: %v", filePath, err)
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

// filterFindingsWithMangle uses Mangle rules to determine which findings should be suppressed.
// Instead of reconstructing findings from query results (which loses metadata like line numbers),
// we query for suppression decisions and filter the original findings list.
func (r *ReviewerShard) filterFindingsWithMangle(findings []ReviewFinding) ([]ReviewFinding, error) {
	if r.kernel == nil {
		return findings, nil
	}

	// 1. Assert raw findings to kernel for rule evaluation
	for _, f := range findings {
		fact := core.Fact{
			Predicate: "raw_finding",
			Args:      []interface{}{f.File, f.Line, f.Severity, f.Category, f.RuleID, f.Message},
		}
		_ = r.kernel.Assert(fact)
	}

	// 2. Query for suppressed findings (File, Line, RuleID, Reason)
	suppressedResults, err := r.kernel.Query("suppressed_finding")
	if err != nil {
		logging.ReviewerDebug("Mangle suppression query failed: %v", err)
		return findings, nil // Return original findings on error
	}

	// 3. Build suppression index: key = "file:line:ruleID"
	suppressed := make(map[string]string) // key -> reason
	for _, res := range suppressedResults {
		if len(res.Args) < 3 {
			continue
		}
		file, _ := res.Args[0].(string)
		line := toStartInt(res.Args[1])
		ruleID, _ := res.Args[2].(string)
		reason := ""
		if len(res.Args) >= 4 {
			reason, _ = res.Args[3].(string)
		}
		key := fmt.Sprintf("%s:%d:%s", file, line, ruleID)
		suppressed[key] = reason
	}

	// 4. Filter original findings, keeping all metadata intact
	active := make([]ReviewFinding, 0, len(findings))
	for _, f := range findings {
		key := fmt.Sprintf("%s:%d:%s", f.File, f.Line, f.RuleID)
		if reason, isSuppressed := suppressed[key]; isSuppressed {
			logging.ReviewerDebug("Suppressed finding [%s] at %s:%d - reason: %s",
				f.RuleID, f.File, f.Line, reason)
			continue
		}
		active = append(active, f)
	}

	if len(active) < len(findings) {
		logging.ReviewerDebug("Mangle filtering: %d -> %d findings (%d suppressed)",
			len(findings), len(active), len(findings)-len(active))
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

// =============================================================================
// NEURO-SYMBOLIC VERIFICATION PIPELINE
// =============================================================================
// Implements the 7-step beyond-SOTA review pipeline:
// 1. Pre-flight check (go build, go vet)
// 2. World update (parse diff, assert facts)
// 3. Hypothesis generation (Mangle rules)
// 4. Impact-aware context (bounded 3-hop)
// 5. Neuro-symbolic verification (LLM verifies hypotheses)
// 6. Autopoiesis (learn from dismissals)
// 7. Findings output (persist and format)

// NeuroSymbolicConfig holds configuration for the neuro-symbolic pipeline.
type NeuroSymbolicConfig struct {
	EnablePreFlight     bool    // Run go build/vet before review (default: true)
	EnableImpactContext bool    // Build impact-aware context (default: true)
	MaxHypotheses       int     // Maximum hypotheses to verify (default: 50)
	MinConfidence       float64 // Minimum hypothesis confidence to consider (default: 0.3)
	ImpactDepthLimit    int     // Maximum hops for impact analysis (default: 3)
	BatchSize           int     // Hypotheses per LLM call (default: 10)

	// Creative Enhancement (--andEnhance flag)
	EnableCreativeEnhancement bool // Run creative enhancement pipeline (Steps 8-12)
	MaxSuggestionsPerLevel    int  // Max suggestions per level (file/module/system/feature) (default: 5)
	EnableSelfInterrogation   bool // Enable self-Q&A refinement (default: true)
	VectorSearchLimit         int  // Max past suggestions to retrieve (default: 10)
}

// DefaultNeuroSymbolicConfig returns sensible defaults for the pipeline.
func DefaultNeuroSymbolicConfig() NeuroSymbolicConfig {
	return NeuroSymbolicConfig{
		EnablePreFlight:     true,
		EnableImpactContext: true,
		MaxHypotheses:       50,
		MinConfidence:       0.3,
		ImpactDepthLimit:    3,
		BatchSize:           10,

		// Creative enhancement disabled by default (opt-in via --andEnhance)
		EnableCreativeEnhancement: false,
		MaxSuggestionsPerLevel:    5,
		EnableSelfInterrogation:   true,
		VectorSearchLimit:         10,
	}
}

// NeuroSymbolicResult extends ReviewResult with pipeline-specific data.
type NeuroSymbolicResult struct {
	*ReviewResult

	// Pipeline metadata
	PreFlightPassed     bool               `json:"preflight_passed"`
	PreFlightDiags      []Diagnostic       `json:"preflight_diagnostics,omitempty"`
	PreFlightSolutions  []string           `json:"preflight_solutions,omitempty"` // Proposed solutions for preflight issues
	HypothesesTotal     int                `json:"hypotheses_total"`
	HypothesesVerified  int                `json:"hypotheses_verified"`
	HypothesesDismissed int                `json:"hypotheses_dismissed"`
	ImpactContext       *ImpactContext     `json:"impact_context,omitempty"`
	VerificationStats   *VerificationStats `json:"verification_stats,omitempty"`

	// Creative Enhancement (Steps 8-12)
	Enhancement *EnhancementResult `json:"enhancement,omitempty"`
}

// executeNeuroSymbolicReview implements the full 7-step neuro-symbolic verification pipeline.
// This is the beyond-SOTA review flow that combines deterministic Mangle analysis with
// LLM semantic verification.
func (r *ReviewerShard) executeNeuroSymbolicReview(ctx context.Context, task *ReviewerTask, config NeuroSymbolicConfig) (*NeuroSymbolicResult, error) {
	logging.Reviewer("Starting neuro-symbolic review pipeline for %d files", len(task.Files))
	startTime := time.Now()

	result := &NeuroSymbolicResult{
		ReviewResult: &ReviewResult{
			Files:    task.Files,
			Findings: make([]ReviewFinding, 0),
			Severity: ReviewSeverityClean,
		},
		PreFlightPassed: true,
	}

	// Collect files for review (expand diff if needed)
	filesToReview := task.Files
	var diffContent string
	var modifiedFuncs []ModifiedFunction

	// If this is a diff review, get the diff content and extract modified functions
	if task.Action == "diff" && task.DiffRef != "" {
		var err error
		diffContent, filesToReview, modifiedFuncs, err = r.extractDiffInfo(ctx, task.DiffRef)
		if err != nil {
			return nil, fmt.Errorf("failed to extract diff info: %w", err)
		}
		result.Files = filesToReview
		logging.ReviewerDebug("Diff analysis: %d files, %d modified functions", len(filesToReview), len(modifiedFuncs))
	}

	// ==========================================================================
	// STEP 1: PRE-FLIGHT CHECK (Non-blocking - surfaces issues as findings)
	// ==========================================================================
	if config.EnablePreFlight && len(filesToReview) > 0 {
		logging.Reviewer("Step 1/7: Running pre-flight checks")

		diagnostics, proceed := r.PreFlightCheck(ctx, filesToReview)
		if !proceed {
			// Pre-flight failed - surface as findings but CONTINUE with review
			result.PreFlightPassed = false
			result.PreFlightDiags = diagnostics

			// Convert diagnostics to findings with proposed solutions
			for _, diag := range diagnostics {
				finding := ReviewFinding{
					File:     diag.File,
					Line:     diag.Line,
					Severity: strings.ToLower(diag.Severity),
					Category: "build",
					RuleID:   "preflight-" + strings.ToLower(diag.Severity),
					Message:  diag.Message,
				}

				// Generate proposed solution based on the error
				solution := r.proposeSolutionForDiagnostic(diag)
				if solution != "" {
					finding.Suggestion = solution
					result.PreFlightSolutions = append(result.PreFlightSolutions, solution)
				}

				result.Findings = append(result.Findings, finding)
			}

			logging.Reviewer("Pre-flight issues found (%d diagnostics) - continuing with review", len(diagnostics))
		} else {
			// Pre-flight passed, inject vet warnings as hypotheses
			if len(diagnostics) > 0 {
				r.injectVetDiagnostics(diagnostics)
				logging.Reviewer("Pre-flight passed with %d warnings, injected as hypotheses", len(diagnostics))
			} else {
				logging.ReviewerDebug("Pre-flight passed with no issues")
			}
		}
	}

	// ==========================================================================
	// STEP 2: WORLD UPDATE - Parse diff and assert facts
	// ==========================================================================
	logging.Reviewer("Step 2/7: Updating world model with diff facts")

	// Read file contents for all files
	fileContents := make(map[string]string)
	for _, filePath := range filesToReview {
		if r.shouldIgnore(filePath) {
			continue
		}
		content, err := r.readFile(ctx, filePath)
		if err != nil {
			logging.ReviewerDebug("Could not read %s: %v", filePath, err)
			continue
		}
		fileContents[filePath] = content

		// Assert file facts for Mangle rules
		r.assertFileFacts(filePath)
	}

	// Parse modified functions from diff if available
	if diffContent != "" && len(modifiedFuncs) == 0 {
		modifiedFuncs = ParseModifiedFunctionsFromDiff(diffContent)
	}

	// Assert modified function facts for impact analysis
	if err := r.assertModifiedFunctionFacts(modifiedFuncs, fileContents); err != nil {
		logging.ReviewerDebug("Failed to assert modified function facts: %v", err)
	}

	// ==========================================================================
	// STEP 3: HYPOTHESIS GENERATION - Query Mangle for potential issues
	// ==========================================================================
	logging.Reviewer("Step 3/7: Generating hypotheses from Mangle rules")

	// Load suppressions from learned.mg equivalent (LearningStore)
	if err := r.LoadSuppressions(); err != nil {
		logging.ReviewerDebug("Failed to load suppressions: %v", err)
	}

	// Generate hypotheses from Mangle
	hypotheses, err := r.GenerateHypotheses(ctx)
	if err != nil {
		logging.Get(logging.CategoryReviewer).Warn("Hypothesis generation failed: %v", err)
		hypotheses = []Hypothesis{} // Continue with empty hypotheses
	}

	result.HypothesesTotal = len(hypotheses)
	logging.ReviewerDebug("Generated %d hypotheses", len(hypotheses))

	// Filter by minimum confidence and limit count
	hypotheses = FilterByMinConfidence(hypotheses, config.MinConfidence)
	if len(hypotheses) > config.MaxHypotheses {
		hypotheses = TopN(hypotheses, config.MaxHypotheses)
	}

	// ==========================================================================
	// STEP 4: IMPACT-AWARE CONTEXT - Build bounded 3-hop context
	// ==========================================================================
	if config.EnableImpactContext && len(modifiedFuncs) > 0 {
		logging.Reviewer("Step 4/7: Building impact-aware context (max %d hops)", config.ImpactDepthLimit)

		impactCtx, err := r.BuildImpactContext(ctx, modifiedFuncs)
		if err != nil {
			logging.ReviewerDebug("Impact context build failed: %v", err)
		} else {
			result.ImpactContext = impactCtx
			logging.ReviewerDebug("Impact context: %s", impactCtx.FormatCompact())

			// Add impacted caller bodies to file contents for verification
			for _, caller := range impactCtx.ImpactedCallers {
				if caller.File != "" && caller.Body != "" {
					// Append caller context to file contents if not already present
					if _, exists := fileContents[caller.File]; !exists {
						fileContents[caller.File] = caller.Body
					}
				}
			}
		}
	} else {
		logging.Reviewer("Step 4/7: Skipping impact context (no modified functions)")
	}

	// ==========================================================================
	// STEP 5: NEURO-SYMBOLIC VERIFICATION - LLM verifies hypotheses
	// ==========================================================================
	if len(hypotheses) > 0 && r.llmClient != nil {
		logging.Reviewer("Step 5/7: Verifying %d hypotheses through LLM", len(hypotheses))

		verifiedFindings, err := r.VerifyHypothesesBatch(ctx, hypotheses, fileContents, config.BatchSize)
		if err != nil {
			logging.Get(logging.CategoryReviewer).Warn("Hypothesis verification failed: %v", err)
		} else {
			// Convert verified findings to standard findings
			for _, vf := range verifiedFindings {
				result.Findings = append(result.Findings, vf.ReviewFinding)
			}
			result.HypothesesVerified = len(verifiedFindings)
			result.HypothesesDismissed = len(hypotheses) - len(verifiedFindings)
			logging.ReviewerDebug("Verification complete: %d confirmed, %d dismissed",
				result.HypothesesVerified, result.HypothesesDismissed)
		}

		// Get verification stats
		stats := r.GetVerificationStats()
		result.VerificationStats = &stats
	} else {
		logging.Reviewer("Step 5/7: Skipping verification (no hypotheses or no LLM)")
	}

	// ==========================================================================
	// STEP 6: AUTOPOIESIS - Learning happens in VerifyHypotheses via LearnFromDismissal
	// ==========================================================================
	logging.Reviewer("Step 6/7: Autopoiesis learning (integrated in verification)")
	// Note: Learning from dismissals is already integrated into VerifyHypotheses
	// The LearnFromDismissal method:
	// - Writes suppression facts to LearningStore (equivalent to learned.mg)
	// - Updates confidence scores with sigmoid growth
	// - Promotes high-confidence patterns to global rules

	// ==========================================================================
	// STEP 7: FINDINGS OUTPUT - Persist and format results
	// ==========================================================================
	logging.Reviewer("Step 7/7: Finalizing findings and generating report")

	// Persist findings to database
	r.persistFindings(result.Findings)

	// ==========================================================================
	// STEPS 8-12: CREATIVE ENHANCEMENT (if --andEnhance flag)
	// ==========================================================================
	if config.EnableCreativeEnhancement {
		logging.Reviewer("Steps 8-12: Running creative enhancement pipeline")

		// Build holographic context for first file (representative context)
		var holoCtx *HolographicContext
		if r.holographicProvider != nil && len(filesToReview) > 0 {
			holoCtx, _ = r.holographicProvider.GetContext(filesToReview[0])
		}

		enhancement, err := r.ExecuteCreativeEnhancement(ctx, fileContents, holoCtx, result.Findings)
		if err != nil {
			logging.Get(logging.CategoryReviewer).Warn("Enhancement failed: %v", err)
		} else {
			result.Enhancement = enhancement

			// Persist enhancements for future vector search
			if err := r.PersistEnhancements(ctx, enhancement, r.id); err != nil {
				logging.ReviewerDebug("Enhancement persistence failed: %v", err)
			}

			logging.Reviewer("Enhancement complete: %d suggestions generated", enhancement.TotalSuggestions())
		}
	}

	// Assert findings to kernel for downstream rules
	for _, finding := range result.Findings {
		fact := core.Fact{
			Predicate: "review_finding",
			Args: []interface{}{
				finding.File,
				finding.Line,
				finding.Severity,
				finding.Category,
				finding.RuleID,
				finding.Message,
			},
		}
		if r.kernel != nil {
			_ = r.kernel.Assert(fact)
		}
	}

	// Calculate metrics if enabled
	if r.reviewerConfig.IncludeMetrics && len(filesToReview) > 0 {
		result.Metrics = r.calculateMetrics(ctx, filesToReview)
	}

	// Generate analysis report with impact context
	if result.ImpactContext != nil {
		result.AnalysisReport = result.ImpactContext.FormatForPrompt()
	}

	// Calculate final severity and summary
	result.Severity = r.calculateOverallSeverity(result.Findings)
	result.BlockCommit = r.shouldBlockCommit(result.ReviewResult)
	result.Duration = time.Since(startTime)
	result.Summary = r.generateNeuroSymbolicSummary(result)

	logging.Reviewer("Neuro-symbolic review complete: %d findings, severity=%s, duration=%v",
		len(result.Findings), result.Severity, result.Duration)

	return result, nil
}

// extractDiffInfo extracts diff content, affected files, and modified functions from a diff reference.
func (r *ReviewerShard) extractDiffInfo(ctx context.Context, diffRef string) (string, []string, []ModifiedFunction, error) {
	if r.virtualStore == nil {
		return "", nil, nil, fmt.Errorf("virtualStore required for diff operations")
	}

	// Get diff via VirtualStore
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
		return "", nil, nil, fmt.Errorf("failed to get diff: %w", err)
	}

	// Parse files from diff
	files := r.parseDiffFiles(diffOutput)

	// Parse modified functions from diff
	modifiedFuncs := ParseModifiedFunctionsFromDiff(diffOutput)

	return diffOutput, files, modifiedFuncs, nil
}

// assertModifiedFunctionFacts asserts modified_function facts to the kernel for impact analysis.
func (r *ReviewerShard) assertModifiedFunctionFacts(modifiedFuncs []ModifiedFunction, fileContents map[string]string) error {
	if r.kernel == nil {
		return nil
	}

	for _, mf := range modifiedFuncs {
		// Assert modified_function(Name, File, StartLine, EndLine)
		fact := core.Fact{
			Predicate: "modified_function",
			Args:      []interface{}{mf.Name, mf.File, mf.StartLine, mf.EndLine},
		}
		if err := r.kernel.Assert(fact); err != nil {
			logging.ReviewerDebug("Failed to assert modified_function: %v", err)
		}

		// If we have the file content, extract data flow facts
		if content, ok := fileContents[mf.File]; ok {
			r.extractAndAssertDataFlowFacts(mf, content)
		}
	}

	return nil
}

// extractAndAssertDataFlowFacts extracts variable assignments, guards, and uses from a function body.
func (r *ReviewerShard) extractAndAssertDataFlowFacts(mf ModifiedFunction, fileContent string) {
	if r.kernel == nil {
		return
	}

	// This is a simplified extraction - in production, use full AST analysis
	// The key facts we want to assert:
	// - assigns(Var, Type) - variable assignments
	// - guards(Var, Kind) - nil checks, error checks
	// - uses(Var) - variable usages

	// Extract function body
	body, err := r.extractGoFunctionBody(fileContent, mf.Name)
	if err != nil {
		return
	}

	// Simple pattern matching for common Go patterns
	// Production implementation would use go/ast for accurate analysis

	// Track nil checks (guards)
	if strings.Contains(body, "!= nil") || strings.Contains(body, "== nil") {
		fact := core.Fact{
			Predicate: "has_nil_check",
			Args:      []interface{}{mf.File, mf.Name},
		}
		_ = r.kernel.Assert(fact)
	}

	// Track error handling
	if strings.Contains(body, "if err != nil") {
		fact := core.Fact{
			Predicate: "has_error_handling",
			Args:      []interface{}{mf.File, mf.Name},
		}
		_ = r.kernel.Assert(fact)
	}

	// Track mutex usage (concurrency safety)
	if strings.Contains(body, ".Lock()") || strings.Contains(body, ".RLock()") {
		fact := core.Fact{
			Predicate: "has_mutex_protection",
			Args:      []interface{}{mf.File, mf.Name},
		}
		_ = r.kernel.Assert(fact)
	}

	// Track defer patterns
	if strings.Contains(body, "defer ") {
		fact := core.Fact{
			Predicate: "has_defer",
			Args:      []interface{}{mf.File, mf.Name},
		}
		_ = r.kernel.Assert(fact)
	}

	// Track context usage
	if strings.Contains(body, "ctx.Done()") || strings.Contains(body, "context.WithCancel") {
		fact := core.Fact{
			Predicate: "respects_context",
			Args:      []interface{}{mf.File, mf.Name},
		}
		_ = r.kernel.Assert(fact)
	}
}

// generateNeuroSymbolicSummary creates a summary for the neuro-symbolic review.
func (r *ReviewerShard) generateNeuroSymbolicSummary(result *NeuroSymbolicResult) string {
	var sb strings.Builder

	sb.WriteString("Neuro-symbolic review complete: ")

	if !result.PreFlightPassed {
		sb.WriteString("PRE-FLIGHT FAILED (code does not compile)")
		return sb.String()
	}

	// Count by severity
	criticalCount := 0
	errorCount := 0
	warningCount := 0
	for _, f := range result.Findings {
		switch f.Severity {
		case "critical":
			criticalCount++
		case "error":
			errorCount++
		case "warning":
			warningCount++
		}
	}

	sb.WriteString(fmt.Sprintf("%d critical, %d errors, %d warnings", criticalCount, errorCount, warningCount))

	if result.HypothesesTotal > 0 {
		precision := float64(result.HypothesesVerified) / float64(result.HypothesesTotal) * 100
		sb.WriteString(fmt.Sprintf(" | Verification: %d/%d hypotheses confirmed (%.0f%% precision)",
			result.HypothesesVerified, result.HypothesesTotal, precision))
	}

	if result.ImpactContext != nil && len(result.ImpactContext.ImpactedCallers) > 0 {
		sb.WriteString(fmt.Sprintf(" | Impact: %d callers analyzed", len(result.ImpactContext.ImpactedCallers)))
	}

	return sb.String()
}

// formatNeuroSymbolicResult formats the neuro-symbolic result for output.
func (r *ReviewerShard) formatNeuroSymbolicResult(result *NeuroSymbolicResult) string {
	var sb strings.Builder

	sb.WriteString("# Code Review Report (Neuro-Symbolic Pipeline)\n\n")

	// Summary section
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Files Reviewed:** %d\n", len(result.Files)))
	preflightStatus := boolToStatus(result.PreFlightPassed)
	if !result.PreFlightPassed {
		preflightStatus = "ISSUES FOUND (review continued)"
	}
	sb.WriteString(fmt.Sprintf("- **Pre-flight:** %s\n", preflightStatus))
	sb.WriteString(fmt.Sprintf("- **Overall Severity:** %s\n", result.Severity))
	sb.WriteString(fmt.Sprintf("- **Duration:** %v\n", result.Duration))
	sb.WriteString(fmt.Sprintf("- **Block Commit:** %v\n\n", result.BlockCommit))

	// Pre-flight issues section (if any)
	if !result.PreFlightPassed && len(result.PreFlightDiags) > 0 {
		sb.WriteString("## Build/Environment Issues\n\n")
		sb.WriteString("*The following issues were detected during pre-flight checks. The review continued with static analysis.*\n\n")

		for i, diag := range result.PreFlightDiags {
			severity := strings.ToLower(diag.Severity)
			location := ""
			if diag.File != "" {
				if diag.Line > 0 {
					location = fmt.Sprintf("`%s:%d` ", diag.File, diag.Line)
				} else {
					location = fmt.Sprintf("`%s` ", diag.File)
				}
			}
			sb.WriteString(fmt.Sprintf("%d. **[%s]** %s%s\n", i+1, severity, location, diag.Message))

			// Show proposed solution if available
			if i < len(result.PreFlightSolutions) && result.PreFlightSolutions[i] != "" {
				sb.WriteString(fmt.Sprintf("   - **Proposed fix:** %s\n", result.PreFlightSolutions[i]))
			}

			if diag.Detail != "" && len(diag.Detail) < 200 {
				sb.WriteString(fmt.Sprintf("   - Detail: `%s`\n", diag.Detail))
			}
		}
		sb.WriteString("\n")
	}

	// Verification stats
	if result.HypothesesTotal > 0 {
		sb.WriteString("### Verification Statistics\n\n")
		sb.WriteString(fmt.Sprintf("- **Hypotheses Generated:** %d\n", result.HypothesesTotal))
		sb.WriteString(fmt.Sprintf("- **Confirmed (True Positives):** %d\n", result.HypothesesVerified))
		sb.WriteString(fmt.Sprintf("- **Dismissed (False Positives):** %d\n", result.HypothesesDismissed))
		if result.HypothesesTotal > 0 {
			precision := float64(result.HypothesesVerified) / float64(result.HypothesesTotal) * 100
			sb.WriteString(fmt.Sprintf("- **Precision:** %.1f%%\n\n", precision))
		}
	}

	// Impact context section
	if result.ImpactContext != nil && len(result.ImpactContext.ImpactedCallers) > 0 {
		sb.WriteString("### Impact Analysis\n\n")
		sb.WriteString(fmt.Sprintf("- **Modified Functions:** %d\n", len(result.ImpactContext.ModifiedFunctions)))
		sb.WriteString(fmt.Sprintf("- **Impacted Callers:** %d\n", len(result.ImpactContext.ImpactedCallers)))
		sb.WriteString(fmt.Sprintf("- **Affected Files:** %d\n\n", len(result.ImpactContext.AffectedFiles)))
	}

	// Findings section (reuse existing format)
	sb.WriteString(r.formatResult(result.ReviewResult))

	// Enhancement Suggestions section (separate from findings)
	if result.Enhancement != nil && result.Enhancement.HasSuggestions() {
		sb.WriteString("\n---\n\n")
		sb.WriteString("## Enhancement Suggestions\n\n")
		sb.WriteString("*Creative analysis powered by two-pass self-consultation*\n\n")

		// Pipeline stats
		if result.Enhancement.FirstPassCount > 0 {
			sb.WriteString(fmt.Sprintf("*First pass: %d suggestions | Second pass: %d suggestions (%.1fx enhancement)*\n\n",
				result.Enhancement.FirstPassCount,
				result.Enhancement.SecondPassCount,
				result.Enhancement.EnhancementRatio))
		}

		// File-level suggestions
		if len(result.Enhancement.FileSuggestions) > 0 {
			sb.WriteString("### File-Level Improvements\n\n")
			for _, fs := range result.Enhancement.FileSuggestions {
				sb.WriteString(fmt.Sprintf("**[%s]** `%s` - %s\n", fs.Category, fs.File, fs.Title))
				sb.WriteString(fmt.Sprintf("  %s\n", fs.Description))
				if fs.CodeExample != "" {
					sb.WriteString(fmt.Sprintf("  ```go\n  %s\n  ```\n", fs.CodeExample))
				}
				sb.WriteString(fmt.Sprintf("  *Effort: %s*\n\n", fs.Effort))
			}
		}

		// Module-level suggestions
		if len(result.Enhancement.ModuleSuggestions) > 0 {
			sb.WriteString("### Module-Level Improvements\n\n")
			for _, ms := range result.Enhancement.ModuleSuggestions {
				sb.WriteString(fmt.Sprintf("**[%s]** `%s` - %s\n", ms.Category, ms.Package, ms.Title))
				sb.WriteString(fmt.Sprintf("  %s\n", ms.Description))
				if len(ms.AffectedFiles) > 0 {
					sb.WriteString(fmt.Sprintf("  *Affected files: %s*\n", strings.Join(ms.AffectedFiles, ", ")))
				}
				sb.WriteString(fmt.Sprintf("  *Effort: %s*\n\n", ms.Effort))
			}
		}

		// System-level insights
		if len(result.Enhancement.SystemInsights) > 0 {
			sb.WriteString("### System-Level Insights\n\n")
			for _, si := range result.Enhancement.SystemInsights {
				sb.WriteString(fmt.Sprintf("**[%s]** %s\n", si.Category, si.Title))
				sb.WriteString(fmt.Sprintf("  %s\n", si.Description))
				if len(si.RelatedModules) > 0 {
					sb.WriteString(fmt.Sprintf("  *Related modules: %s*\n", strings.Join(si.RelatedModules, ", ")))
				}
				sb.WriteString(fmt.Sprintf("  *Impact: %s*\n\n", si.Impact))
			}
		}

		// Feature ideas
		if len(result.Enhancement.FeatureIdeas) > 0 {
			sb.WriteString("### Feature Ideas\n\n")
			for _, fi := range result.Enhancement.FeatureIdeas {
				sb.WriteString(fmt.Sprintf("**%s**\n", fi.Title))
				sb.WriteString(fmt.Sprintf("  %s\n", fi.Description))
				if fi.Rationale != "" {
					sb.WriteString(fmt.Sprintf("  *Rationale: %s*\n", fi.Rationale))
				}
				if len(fi.Prerequisites) > 0 {
					sb.WriteString(fmt.Sprintf("  *Prerequisites: %s*\n", strings.Join(fi.Prerequisites, ", ")))
				}
				sb.WriteString(fmt.Sprintf("  *Complexity: %s*\n\n", fi.Complexity))
			}
		}

		// Self-interrogation insights (if any)
		if len(result.Enhancement.SelfQA) > 0 {
			sb.WriteString("### Self-Consultation Insights\n\n")
			for _, qa := range result.Enhancement.SelfQA {
				sb.WriteString(fmt.Sprintf("**Q:** %s\n", qa.Question))
				sb.WriteString(fmt.Sprintf("**A:** %s\n", qa.Answer))
				if qa.Insight != "" {
					sb.WriteString(fmt.Sprintf("*Insight: %s*\n", qa.Insight))
				}
				sb.WriteString("\n")
			}
		}

		// Vector inspiration (if any)
		if len(result.Enhancement.VectorInspiration) > 0 {
			sb.WriteString("### Historical Inspiration\n\n")
			sb.WriteString("*Suggestions informed by similar past reviews:*\n\n")
			for _, ps := range result.Enhancement.VectorInspiration {
				status := "not implemented"
				if ps.WasImplemented {
					status = "implemented"
				}
				sb.WriteString(fmt.Sprintf("- (%.0f%% similar, %s) %s\n", ps.Similarity*100, status, ps.Summary))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// boolToStatus converts a boolean to a status string.
func boolToStatus(b bool) string {
	if b {
		return "PASSED"
	}
	return "FAILED"
}
