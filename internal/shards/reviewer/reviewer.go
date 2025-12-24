// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// It specializes in code review, security scanning, and best practices analysis.
//
// File Index (Modularized Structure):
//
// Configuration:
//   - config.go         - ReviewerConfig, DefaultReviewerConfig()
//
// Types:
//   - types.go          - ReviewSeverity, ReviewFinding, ReviewResult, CodeMetrics
//
// Initialization:
//   - init.go           - NewReviewerShard(), NewReviewerShardWithConfig()
//   - setters.go        - SetLLMClient(), SetParentKernel(), etc.
//
// Core Execution:
//   - reviewer.go       - ReviewerShard struct, Execute(), neuro-symbolic pipeline (this file)
//   - operations.go     - reviewFiles(), securityScan(), styleCheck(), complexityAnalysis(), reviewDiff()
//   - analysis.go       - analyzeFile(), analyzeFileWithDeps(), analyzeFileWithHolographic()
//   - task.go           - ReviewerTask, parseTask(), expandTaskFiles()
//
// Checks:
//   - checks.go         - checkSecurity(), checkStyle(), checkBugPatterns()
//   - custom_rules.go   - Custom rule loading and checking
//
// Neuro-Symbolic:
//   - hypotheses.go     - Hypothesis generation from Mangle
//   - verification.go   - LLM verification of hypotheses
//   - impact.go         - Impact context building
//   - preflight.go      - Pre-flight checks (go build, go vet)
//
// Support:
//   - helpers.go        - Utility functions (readFile, shouldIgnore, etc.)
//   - filtering.go      - Mangle-based filtering, assertFileFacts()
//   - format.go         - Output formatting
//   - persistence.go    - Review persistence and export
//   - dependencies.go   - DependencyContext, 1-hop dependency fetching
//   - llm.go            - LLM analysis functions
//   - specialists.go    - Specialist shard detection
//   - autopoiesis.go    - Learning from review patterns
//   - dream.go          - Dream mode simulation
//   - creative.go       - Creative enhancement pipeline
//   - enhancement.go    - Enhancement result types
//   - metrics.go        - Code metrics calculation
//   - facts.go          - Fact generation and assertion
//   - feedback.go       - Feedback loop integration
//   - knowledge.go      - Knowledge base integration
package reviewer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

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
	r.persistFindingsToStore(result.Findings)

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
