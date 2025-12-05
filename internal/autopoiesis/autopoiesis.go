// Package autopoiesis implements self-modification capabilities for codeNERD.
// Autopoiesis (from Greek: self-creation) enables the system to:
// 1. Detect when tasks require campaign orchestration (complex multi-phase work)
// 2. Generate new tools when existing capabilities are insufficient
// 3. Create persistent agents when ongoing monitoring/learning is needed
package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// =============================================================================
// KERNEL INTERFACE - Bridge to Mangle Logic Core
// =============================================================================

// KernelFact represents a fact that can be asserted to the kernel.
// This mirrors core.Fact but avoids import cycles.
type KernelFact struct {
	Predicate string
	Args      []interface{}
}

// KernelInterface defines the interface for interacting with the Mangle kernel.
// This allows autopoiesis to assert facts and query for derived actions.
type KernelInterface interface {
	// AssertFact adds a fact to the kernel's EDB
	AssertFact(fact KernelFact) error
	// QueryPredicate queries for facts matching a predicate
	QueryPredicate(predicate string) ([]KernelFact, error)
	// QueryBool returns true if any facts match the predicate
	QueryBool(predicate string) bool
}

// =============================================================================
// AUTOPOIESIS ORCHESTRATOR
// =============================================================================
// The main coordinator for all self-modification capabilities.

// Config holds configuration for the autopoiesis system
type Config struct {
	ToolsDir      string // Directory for generated tools
	AgentsDir     string // Directory for agent definitions
	MinConfidence float64 // Minimum confidence to trigger autopoiesis
	EnableLLM     bool   // Whether to use LLM for analysis
}

// DefaultConfig returns default configuration
func DefaultConfig(workspaceRoot string) Config {
	return Config{
		ToolsDir:      filepath.Join(workspaceRoot, ".nerd", "tools"),
		AgentsDir:     filepath.Join(workspaceRoot, ".nerd", "agents"),
		MinConfidence: 0.6,
		EnableLLM:     true,
	}
}

// Orchestrator coordinates all autopoiesis capabilities
type Orchestrator struct {
	mu          sync.RWMutex
	config      Config
	complexity  *ComplexityAnalyzer
	toolGen     *ToolGenerator
	persistence *PersistenceAnalyzer
	agentCreate *AgentCreator
	ouroboros   *OuroborosLoop  // The Ouroboros Loop for tool self-generation
	client      LLMClient

	// Kernel Integration - Bridge to Mangle Logic Core
	kernel      KernelInterface     // The Mangle kernel for fact assertion/query

	// Feedback and Learning System
	evaluator   *QualityEvaluator   // Assess tool output quality
	patterns    *PatternDetector    // Detect recurring issues
	refiner     *ToolRefiner        // Improve suboptimal tools
	learnings   *LearningStore      // Persist learnings
	profiles    *ProfileStore       // Tool-specific quality profiles

	// Reasoning Trace and Logging System
	traces      *TraceCollector     // Capture reasoning during generation
	logInjector *LogInjector        // Inject mandatory logging into tools
}

// NewOrchestrator creates a new autopoiesis orchestrator
func NewOrchestrator(client LLMClient, config Config) *Orchestrator {
	// Create Ouroboros config from autopoiesis config
	ouroborosConfig := OuroborosConfig{
		ToolsDir:        config.ToolsDir,
		CompiledDir:     filepath.Join(config.ToolsDir, ".compiled"),
		MaxToolSize:     100 * 1024, // 100KB
		CompileTimeout:  30 * time.Second,
		ExecuteTimeout:  60 * time.Second,
		AllowNetworking: false,
		AllowFileSystem: true,
		AllowExec:       false,
	}

	toolGen := NewToolGenerator(client, config.ToolsDir)
	learningsDir := filepath.Join(config.ToolsDir, ".learnings")
	tracesDir := filepath.Join(config.ToolsDir, ".traces")
	profilesDir := filepath.Join(config.ToolsDir, ".profiles")
	profileStore := NewProfileStore(profilesDir)

	return &Orchestrator{
		config:      config,
		complexity:  NewComplexityAnalyzer(client),
		toolGen:     toolGen,
		persistence: NewPersistenceAnalyzer(client),
		agentCreate: NewAgentCreator(client, config.AgentsDir),
		ouroboros:   NewOuroborosLoop(client, ouroborosConfig),
		client:      client,

		// Initialize feedback and learning system
		evaluator:   NewQualityEvaluator(client, profileStore),
		patterns:    NewPatternDetector(),
		refiner:     NewToolRefiner(client, toolGen),
		learnings:   NewLearningStore(learningsDir),
		profiles:    profileStore,

		// Initialize reasoning trace and logging system
		traces:      NewTraceCollector(tracesDir, client),
		logInjector: NewLogInjector(DefaultLoggingRequirements()),
	}
}

// SetKernel attaches a Mangle kernel for fact assertion and query.
// This enables the full neuro-symbolic loop where autopoiesis
// events are reflected as Mangle facts for logic-driven orchestration.
func (o *Orchestrator) SetKernel(kernel KernelInterface) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.kernel = kernel
}

// GetKernel returns the attached kernel (may be nil).
func (o *Orchestrator) GetKernel() KernelInterface {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.kernel
}

// =============================================================================
// KERNEL FACT ASSERTION - Wire Autopoiesis Events to Mangle
// =============================================================================

// assertToKernel safely asserts a fact to the kernel if attached.
func (o *Orchestrator) assertToKernel(predicate string, args ...interface{}) error {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return nil // No kernel attached, silently skip
	}

	return kernel.AssertFact(KernelFact{
		Predicate: predicate,
		Args:      args,
	})
}

// assertToolRegistered asserts tool_registered and related facts to kernel.
// Called when a tool is successfully generated and registered.
func (o *Orchestrator) assertToolRegistered(tool *RuntimeTool) {
	if tool == nil {
		return
	}

	timestamp := tool.RegisteredAt.Format("2006-01-02T15:04:05Z07:00")

	// tool_registered(ToolName, RegisteredAt)
	_ = o.assertToKernel("tool_registered", tool.Name, timestamp)

	// tool_hash(ToolName, Hash)
	_ = o.assertToKernel("tool_hash", tool.Name, tool.Hash)

	// has_capability(ToolName)
	_ = o.assertToKernel("has_capability", tool.Name)
}

// assertMissingTool asserts missing_tool_for fact to kernel.
// Called when a capability gap is detected.
func (o *Orchestrator) assertMissingTool(intentID, capability string) {
	// missing_tool_for(Intent, Capability)
	_ = o.assertToKernel("missing_tool_for", intentID, capability)
}

// assertToolLearning asserts tool_learning fact to kernel.
// Called when execution feedback is recorded.
func (o *Orchestrator) assertToolLearning(toolName string, executions int, successRate, avgQuality float64) {
	// tool_learning(ToolName, Executions, SuccessRate, AvgQuality)
	_ = o.assertToKernel("tool_learning", toolName, executions, successRate, avgQuality)
}

// assertToolKnownIssue asserts tool_known_issue fact to kernel.
func (o *Orchestrator) assertToolKnownIssue(toolName string, issueType string) {
	// tool_known_issue(ToolName, IssueType) - use name constant format
	_ = o.assertToKernel("tool_known_issue", toolName, "/"+issueType)
}

// SyncLearningsToKernel pushes all current learnings to the kernel.
// Call this periodically or after significant learning updates.
func (o *Orchestrator) SyncLearningsToKernel() {
	learnings := o.learnings.GetAllLearnings()
	for _, learning := range learnings {
		o.assertToolLearning(
			learning.ToolName,
			learning.TotalExecutions,
			learning.SuccessRate,
			learning.AverageQuality,
		)
		for _, issue := range learning.KnownIssues {
			o.assertToolKnownIssue(learning.ToolName, string(issue))
		}
	}
}

// =============================================================================
// CODE DOM INTEGRATION
// =============================================================================
// Methods for querying code elements from the kernel.
// Code DOM facts are asserted to the kernel by VirtualStore when files are opened.

// QueryCodeElements queries the kernel for code_element facts.
// Returns the number of elements in scope.
func (o *Orchestrator) QueryCodeElementCount() int {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return 0
	}

	facts, err := kernel.QueryPredicate("code_element")
	if err != nil {
		return 0
	}
	return len(facts)
}

// QueryElementsByType returns count of elements matching a type.
func (o *Orchestrator) QueryElementsByType(elemType string) int {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return 0
	}

	facts, err := kernel.QueryPredicate("code_element")
	if err != nil {
		return 0
	}

	count := 0
	for _, fact := range facts {
		if len(fact.Args) >= 2 {
			if t, ok := fact.Args[1].(string); ok {
				if t == "/"+elemType || t == elemType {
					count++
				}
			}
		}
	}
	return count
}

// QueryActiveFile returns the currently active file from the kernel.
func (o *Orchestrator) QueryActiveFile() string {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return ""
	}

	facts, err := kernel.QueryPredicate("active_file")
	if err != nil || len(facts) == 0 {
		return ""
	}

	if len(facts[0].Args) >= 1 {
		if path, ok := facts[0].Args[0].(string); ok {
			return path
		}
	}
	return ""
}

// QueryFilesInScope returns the count of files in the current scope.
func (o *Orchestrator) QueryFilesInScope() int {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return 0
	}

	facts, err := kernel.QueryPredicate("file_in_scope")
	if err != nil {
		return 0
	}
	return len(facts)
}

// RecordCodeEditOutcome records the outcome of a code edit for learning.
func (o *Orchestrator) RecordCodeEditOutcome(elementRef string, editType string, success bool) {
	successStr := "/false"
	if success {
		successStr = "/true"
	}
	_ = o.assertToKernel("code_edit_outcome", elementRef, "/"+editType, successStr)
}

// QueryNextAction queries the kernel for derived next_action facts.
// Returns the action type if the kernel derives one, empty string otherwise.
func (o *Orchestrator) QueryNextAction() string {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return ""
	}

	// Check for various autopoiesis-related next actions
	actions := []string{
		"next_action(/generate_tool)",
		"next_action(/refine_tool)",
	}

	for _, action := range actions {
		if kernel.QueryBool(action) {
			// Extract the action name from the query
			return action
		}
	}

	return ""
}

// ShouldGenerateTool queries the kernel to check if tool generation is needed.
// This provides logic-driven triggering instead of just heuristics.
func (o *Orchestrator) ShouldGenerateTool() bool {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return false
	}

	return kernel.QueryBool("next_action(/generate_tool)")
}

// ShouldRefineTool queries the kernel to check if a tool needs refinement.
func (o *Orchestrator) ShouldRefineToolByKernel(toolName string) bool {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return false
	}

	// Query for tool_needs_refinement(toolName)
	return kernel.QueryBool(fmt.Sprintf("tool_needs_refinement(%q)", toolName))
}

// AnalysisResult contains the complete autopoiesis analysis
type AnalysisResult struct {
	// Complexity analysis
	Complexity     ComplexityResult
	NeedsCampaign  bool
	SuggestedPhases []string

	// Tool generation
	ToolNeeds []ToolNeed

	// Persistence analysis
	Persistence    PersistenceResult
	NeedsPersistent bool
	SuggestedAgents []AgentSpec

	// Actions to take
	Actions []AutopoiesisAction

	// Metadata
	AnalyzedAt time.Time
	InputHash  string
}

// AutopoiesisAction represents an action the system should take
type AutopoiesisAction struct {
	Type        ActionType
	Priority    float64
	Description string
	Payload     any // Type-specific payload
}

// ActionType defines types of autopoiesis actions
type ActionType int

const (
	ActionNone ActionType = iota
	ActionStartCampaign
	ActionGenerateTool
	ActionCreateAgent
	ActionDelegateToShard
)

// String returns the string representation of an action type
func (at ActionType) String() string {
	switch at {
	case ActionStartCampaign:
		return "start_campaign"
	case ActionGenerateTool:
		return "generate_tool"
	case ActionCreateAgent:
		return "create_agent"
	case ActionDelegateToShard:
		return "delegate_to_shard"
	default:
		return "none"
	}
}

// Analyze performs complete autopoiesis analysis on user input
func (o *Orchestrator) Analyze(ctx context.Context, input string, target string) (*AnalysisResult, error) {
	result := &AnalysisResult{
		AnalyzedAt: time.Now(),
		InputHash:  hashString(input),
		Actions:    []AutopoiesisAction{},
	}

	// 1. Analyze complexity
	if o.config.EnableLLM {
		complexity, err := o.complexity.AnalyzeWithLLM(ctx, input)
		if err != nil {
			// Fall back to heuristic
			complexity = o.complexity.Analyze(ctx, input, target)
		}
		result.Complexity = complexity
	} else {
		result.Complexity = o.complexity.Analyze(ctx, input, target)
	}

	result.NeedsCampaign = result.Complexity.NeedsCampaign
	result.SuggestedPhases = result.Complexity.SuggestedPhases

	// Add campaign action if needed
	if result.NeedsCampaign {
		result.Actions = append(result.Actions, AutopoiesisAction{
			Type:        ActionStartCampaign,
			Priority:    result.Complexity.Score,
			Description: fmt.Sprintf("Start campaign with %d phases", len(result.Complexity.SuggestedPhases)),
			Payload: CampaignPayload{
				Phases:         result.Complexity.SuggestedPhases,
				EstimatedFiles: result.Complexity.EstimatedFiles,
				Reasons:        result.Complexity.Reasons,
			},
		})
	}

	// 2. Analyze persistence needs
	if o.config.EnableLLM {
		persistence, err := o.persistence.AnalyzeWithLLM(ctx, input)
		if err != nil {
			persistence = o.persistence.Analyze(ctx, input)
		}
		result.Persistence = persistence
	} else {
		result.Persistence = o.persistence.Analyze(ctx, input)
	}

	result.NeedsPersistent = result.Persistence.NeedsPersistent

	// Create agent specs for persistence needs
	for _, need := range result.Persistence.Needs {
		if need.Confidence >= o.config.MinConfidence {
			spec, err := o.agentCreate.CreateFromNeed(ctx, need)
			if err != nil {
				continue
			}
			result.SuggestedAgents = append(result.SuggestedAgents, *spec)

			result.Actions = append(result.Actions, AutopoiesisAction{
				Type:        ActionCreateAgent,
				Priority:    need.Confidence,
				Description: fmt.Sprintf("Create persistent %s agent", need.AgentType),
				Payload:     spec,
			})
		}
	}

	// 3. Tool need detection (only if task seems to need new capability)
	if shouldCheckToolNeed(input) {
		toolNeed, err := o.toolGen.DetectToolNeed(ctx, input, "")
		if err == nil && toolNeed != nil && toolNeed.Confidence >= o.config.MinConfidence {
			result.ToolNeeds = append(result.ToolNeeds, *toolNeed)

			result.Actions = append(result.Actions, AutopoiesisAction{
				Type:        ActionGenerateTool,
				Priority:    toolNeed.Priority,
				Description: fmt.Sprintf("Generate tool: %s", toolNeed.Name),
				Payload:     toolNeed,
			})
		}
	}

	// Sort actions by priority
	sortActionsByPriority(result.Actions)

	return result, nil
}

// CampaignPayload contains data for starting a campaign
type CampaignPayload struct {
	Phases         []string
	EstimatedFiles int
	Reasons        []string
}

// ExecuteAction executes a single autopoiesis action
func (o *Orchestrator) ExecuteAction(ctx context.Context, action AutopoiesisAction) error {
	switch action.Type {
	case ActionGenerateTool:
		return o.executeToolGeneration(ctx, action)
	case ActionCreateAgent:
		return o.executeAgentCreation(ctx, action)
	case ActionStartCampaign:
		// Campaign starting is handled by the campaign orchestrator
		return nil
	default:
		return fmt.Errorf("unknown action type: %v", action.Type)
	}
}

// executeToolGeneration generates and registers a new tool
func (o *Orchestrator) executeToolGeneration(ctx context.Context, action AutopoiesisAction) error {
	need, ok := action.Payload.(*ToolNeed)
	if !ok {
		return fmt.Errorf("invalid payload for tool generation")
	}

	// Generate the tool
	tool, err := o.toolGen.GenerateTool(ctx, need)
	if err != nil {
		return fmt.Errorf("failed to generate tool: %w", err)
	}

	// Write to disk
	if err := o.toolGen.WriteTool(tool); err != nil {
		return fmt.Errorf("failed to write tool: %w", err)
	}

	// Register in memory
	if err := o.toolGen.RegisterTool(tool); err != nil {
		return fmt.Errorf("failed to register tool: %w", err)
	}

	return nil
}

// executeAgentCreation creates a new persistent agent
func (o *Orchestrator) executeAgentCreation(ctx context.Context, action AutopoiesisAction) error {
	spec, ok := action.Payload.(*AgentSpec)
	if !ok {
		return fmt.Errorf("invalid payload for agent creation")
	}

	// Agent creation is currently a stub - the actual implementation
	// would write the agent spec to disk and register with the shard manager
	_ = spec
	return nil
}

// =============================================================================
// QUICK ANALYSIS (for real-time use in processInput)
// =============================================================================

// QuickResult is a lightweight analysis result for real-time decisions
type QuickResult struct {
	NeedsCampaign   bool
	NeedsPersistent bool
	NeedsTool       bool
	ComplexityLevel ComplexityLevel
	TopAction       *AutopoiesisAction
}

// QuickAnalyze performs fast analysis without LLM calls
func (o *Orchestrator) QuickAnalyze(ctx context.Context, input string, target string) QuickResult {
	result := QuickResult{}

	// Quick complexity check (heuristic only)
	complexity := o.complexity.Analyze(ctx, input, target)
	result.ComplexityLevel = complexity.Level
	result.NeedsCampaign = complexity.NeedsCampaign

	// Enhance with code element awareness from kernel
	elementCount := o.QueryCodeElementCount()
	filesInScope := o.QueryFilesInScope()

	// If many elements are in scope, the task might be more complex
	if elementCount > 20 && result.ComplexityLevel < ComplexityComplex {
		result.ComplexityLevel = ComplexityComplex
		result.NeedsCampaign = true
	}

	// If many files in scope, consider complexity
	if filesInScope > 5 && result.ComplexityLevel < ComplexityModerate {
		result.ComplexityLevel = ComplexityModerate
	}

	// Quick persistence check (heuristic only)
	persistence := o.persistence.Analyze(ctx, input)
	result.NeedsPersistent = persistence.NeedsPersistent

	// Determine top action
	if result.NeedsCampaign {
		result.TopAction = &AutopoiesisAction{
			Type:        ActionStartCampaign,
			Priority:    complexity.Score,
			Description: "Complex task - recommend campaign",
		}
	} else if result.NeedsPersistent && len(persistence.Needs) > 0 {
		result.TopAction = &AutopoiesisAction{
			Type:        ActionCreateAgent,
			Priority:    persistence.Needs[0].Confidence,
			Description: "Persistent agent recommended",
		}
	}

	return result
}

// ShouldTriggerCampaign is a quick check for campaign needs
func (o *Orchestrator) ShouldTriggerCampaign(ctx context.Context, input string, target string) (bool, string) {
	complexity := o.complexity.Analyze(ctx, input, target)

	if !complexity.NeedsCampaign {
		return false, ""
	}

	// Build reason string
	reason := fmt.Sprintf("Complexity: %s (score: %.2f). ", complexityLevelString(complexity.Level), complexity.Score)
	if len(complexity.SuggestedPhases) > 0 {
		reason += fmt.Sprintf("Suggested phases: %v. ", complexity.SuggestedPhases)
	}
	if len(complexity.Reasons) > 0 {
		reason += fmt.Sprintf("Reasons: %v", complexity.Reasons)
	}

	return true, reason
}

// ShouldCreatePersistentAgent is a quick check for persistence needs
func (o *Orchestrator) ShouldCreatePersistentAgent(ctx context.Context, input string) (bool, *PersistenceNeed) {
	persistence := o.persistence.Analyze(ctx, input)

	if !persistence.NeedsPersistent || len(persistence.Needs) == 0 {
		return false, nil
	}

	// Return highest confidence need
	var best *PersistenceNeed
	for i := range persistence.Needs {
		if best == nil || persistence.Needs[i].Confidence > best.Confidence {
			best = &persistence.Needs[i]
		}
	}

	return true, best
}

// =============================================================================
// TOOL GENERATION WRAPPERS
// =============================================================================
// These methods expose the internal ToolGenerator for direct use from chat.go

// DetectToolNeed analyzes input to determine if a new tool is needed.
// If a need is detected, it asserts missing_tool_for to the kernel.
func (o *Orchestrator) DetectToolNeed(ctx context.Context, input string) (*ToolNeed, error) {
	need, err := o.toolGen.DetectToolNeed(ctx, input, "")
	if err != nil {
		return nil, err
	}

	// Wire to kernel: Assert missing_tool_for fact if capability gap detected
	if need != nil {
		intentID := hashString(input) // Use input hash as intent ID
		o.assertMissingTool(intentID, need.Name)
	}

	return need, nil
}

// GenerateTool creates a new tool based on the detected need
func (o *Orchestrator) GenerateTool(ctx context.Context, need *ToolNeed) (*GeneratedTool, error) {
	return o.toolGen.GenerateTool(ctx, need)
}

// WriteAndRegisterTool writes the generated tool to disk and registers it
func (o *Orchestrator) WriteAndRegisterTool(tool *GeneratedTool) error {
	if err := o.toolGen.WriteTool(tool); err != nil {
		return err
	}
	return o.toolGen.RegisterTool(tool)
}

// =============================================================================
// OUROBOROS LOOP WRAPPERS
// =============================================================================
// These methods expose the Ouroboros Loop for full tool self-generation.

// ExecuteOuroborosLoop runs the complete tool self-generation cycle.
// On success, it asserts tool_registered facts to the kernel.
func (o *Orchestrator) ExecuteOuroborosLoop(ctx context.Context, need *ToolNeed) *LoopResult {
	result := o.ouroboros.Execute(ctx, need)

	// Wire to kernel: Assert tool registration facts on success
	if result.Success && result.ToolHandle != nil {
		o.assertToolRegistered(result.ToolHandle)
	}

	return result
}

// ExecuteGeneratedTool runs a previously generated and compiled tool
func (o *Orchestrator) ExecuteGeneratedTool(ctx context.Context, toolName string, input string) (string, error) {
	return o.ouroboros.ExecuteTool(ctx, toolName, input)
}

// GetOuroborosStats returns statistics about tool generation
func (o *Orchestrator) GetOuroborosStats() OuroborosStats {
	return o.ouroboros.GetStats()
}

// ListGeneratedTools returns all registered generated tools
func (o *Orchestrator) ListGeneratedTools() []*RuntimeTool {
	return o.ouroboros.registry.List()
}

// HasGeneratedTool checks if a tool exists in the registry
func (o *Orchestrator) HasGeneratedTool(name string) bool {
	_, exists := o.ouroboros.registry.Get(name)
	return exists
}

// CheckToolSafety validates tool code without compiling
func (o *Orchestrator) CheckToolSafety(code string) *SafetyReport {
	return o.ouroboros.safetyChecker.Check(code)
}

// =============================================================================
// FEEDBACK AND LEARNING WRAPPERS
// =============================================================================
// These methods expose the feedback loop for tool execution evaluation and improvement.

// RecordExecution records a tool execution and evaluates its quality.
// It also syncs learning facts to the kernel for logic-driven refinement triggers.
func (o *Orchestrator) RecordExecution(ctx context.Context, feedback *ExecutionFeedback) {
	// Evaluate quality
	if feedback.Quality == nil {
		feedback.Quality = o.evaluator.Evaluate(ctx, feedback)
	}

	// Record in pattern detector
	o.patterns.RecordExecution(*feedback)

	// Get patterns for this tool
	patterns := o.patterns.GetToolPatterns(feedback.ToolName)

	// Update learning store
	o.learnings.RecordLearning(feedback.ToolName, feedback, patterns)

	// Wire to kernel: Assert learning facts for logic-driven refinement
	learning := o.learnings.GetLearning(feedback.ToolName)
	if learning != nil {
		o.assertToolLearning(
			learning.ToolName,
			learning.TotalExecutions,
			learning.SuccessRate,
			learning.AverageQuality,
		)

		// Assert known issues for pattern detection
		for _, issue := range learning.KnownIssues {
			o.assertToolKnownIssue(learning.ToolName, string(issue))
		}
	}
}

// EvaluateToolQuality assesses the quality of a tool execution
func (o *Orchestrator) EvaluateToolQuality(ctx context.Context, feedback *ExecutionFeedback) *QualityAssessment {
	return o.evaluator.Evaluate(ctx, feedback)
}

// EvaluateToolQualityWithLLM uses LLM for deeper quality assessment
func (o *Orchestrator) EvaluateToolQualityWithLLM(ctx context.Context, feedback *ExecutionFeedback) (*QualityAssessment, error) {
	return o.evaluator.EvaluateWithLLM(ctx, feedback)
}

// GetToolPatterns returns detected issues patterns for a tool
func (o *Orchestrator) GetToolPatterns(toolName string) []*DetectedPattern {
	return o.patterns.GetToolPatterns(toolName)
}

// GetAllPatterns returns all detected patterns above confidence threshold
func (o *Orchestrator) GetAllPatterns(minConfidence float64) []*DetectedPattern {
	return o.patterns.GetPatterns(minConfidence)
}

// ShouldRefineTool checks if a tool needs improvement based on learnings
func (o *Orchestrator) ShouldRefineTool(toolName string) (bool, []ImprovementSuggestion) {
	learning := o.learnings.GetLearning(toolName)
	if learning == nil {
		return false, nil
	}

	// Check if quality is poor
	if learning.AverageQuality < 0.5 && learning.TotalExecutions >= 3 {
		patterns := o.patterns.GetToolPatterns(toolName)
		suggestions := []ImprovementSuggestion{}
		for _, p := range patterns {
			suggestions = append(suggestions, p.Suggestions...)
		}
		return true, suggestions
	}

	// Check for known issues that are fixable
	if len(learning.KnownIssues) > 0 {
		patterns := o.patterns.GetToolPatterns(toolName)
		for _, p := range patterns {
			if p.Confidence > 0.7 && len(p.Suggestions) > 0 {
				return true, p.Suggestions
			}
		}
	}

	return false, nil
}

// RefineTool generates an improved version of a tool
func (o *Orchestrator) RefineTool(ctx context.Context, toolName string, originalCode string) (*RefinementResult, error) {
	// Gather feedback history
	patterns := o.patterns.GetToolPatterns(toolName)

	// Collect all suggestions
	suggestions := []ImprovementSuggestion{}
	for _, p := range patterns {
		suggestions = append(suggestions, p.Suggestions...)
	}

	req := RefinementRequest{
		ToolName:     toolName,
		OriginalCode: originalCode,
		Patterns:     patterns,
		Suggestions:  suggestions,
	}

	return o.refiner.Refine(ctx, req)
}

// GetToolLearning retrieves accumulated learnings for a tool
func (o *Orchestrator) GetToolLearning(toolName string) *ToolLearning {
	return o.learnings.GetLearning(toolName)
}

// GetAllLearnings returns all accumulated tool learnings
func (o *Orchestrator) GetAllLearnings() []*ToolLearning {
	return o.learnings.GetAllLearnings()
}

// GenerateLearningFacts creates Mangle facts from all learnings
func (o *Orchestrator) GenerateLearningFacts() []string {
	return o.learnings.GenerateMangleFacts()
}

// ExecuteAndEvaluate runs a tool and automatically evaluates quality
func (o *Orchestrator) ExecuteAndEvaluate(ctx context.Context, toolName string, input string) (string, *QualityAssessment, error) {
	start := time.Now()

	output, err := o.ouroboros.ExecuteTool(ctx, toolName, input)

	feedback := &ExecutionFeedback{
		ToolName:   toolName,
		Timestamp:  start,
		Input:      input,
		Output:     output,
		OutputSize: len(output),
		Duration:   time.Since(start),
		Success:    err == nil,
	}

	if err != nil {
		feedback.ErrorMsg = err.Error()
	}

	// Evaluate and record
	o.RecordExecution(ctx, feedback)

	return output, feedback.Quality, err
}

// =============================================================================
// REASONING TRACE WRAPPERS
// =============================================================================
// These methods expose the trace system for capturing tool generation reasoning.

// StartToolTrace begins capturing reasoning for a tool generation
func (o *Orchestrator) StartToolTrace(toolName string, need *ToolNeed, userRequest string) *ReasoningTrace {
	return o.traces.StartTrace(toolName, need, userRequest)
}

// RecordTracePrompt records the prompts sent to the LLM
func (o *Orchestrator) RecordTracePrompt(trace *ReasoningTrace, systemPrompt, userPrompt string) {
	o.traces.RecordPrompt(trace, systemPrompt, userPrompt)
}

// RecordTraceResponse records the LLM response and extracts reasoning
func (o *Orchestrator) RecordTraceResponse(ctx context.Context, trace *ReasoningTrace, response string, tokensUsed int, duration time.Duration) {
	o.traces.RecordResponse(ctx, trace, response, tokensUsed, duration)
}

// FinalizeTrace marks a generation trace as complete
func (o *Orchestrator) FinalizeTrace(trace *ReasoningTrace, success bool, code string, failureReason string) {
	o.traces.FinalizeTrace(trace, success, code, failureReason)
}

// UpdateTraceWithFeedback adds execution feedback to a tool's trace
func (o *Orchestrator) UpdateTraceWithFeedback(toolName string, quality float64, issues []string, notes []string) {
	o.traces.UpdateWithFeedback(toolName, quality, issues, notes)
}

// GetToolTraces retrieves all reasoning traces for a tool
func (o *Orchestrator) GetToolTraces(toolName string) []*ReasoningTrace {
	return o.traces.GetToolTraces(toolName)
}

// GetAllTraces returns all reasoning traces
func (o *Orchestrator) GetAllTraces() []*ReasoningTrace {
	return o.traces.GetAllTraces()
}

// AnalyzeGenerations performs broad analysis across all tool generations
func (o *Orchestrator) AnalyzeGenerations(ctx context.Context) (*GenerationAudit, error) {
	return o.traces.AnalyzeGenerations(ctx)
}

// =============================================================================
// LOGGING INJECTION WRAPPERS
// =============================================================================
// These methods expose the logging injection system for mandatory tool logging.

// InjectLogging adds mandatory logging to generated tool code
func (o *Orchestrator) InjectLogging(code string, toolName string) (string, error) {
	return o.logInjector.InjectLogging(code, toolName)
}

// ValidateLogging checks that required logging is present in tool code
func (o *Orchestrator) ValidateLogging(code string) *LoggingValidation {
	return o.logInjector.ValidateLogging(code)
}

// GenerateToolWithTracing generates a tool with full reasoning trace capture
func (o *Orchestrator) GenerateToolWithTracing(ctx context.Context, need *ToolNeed, userRequest string) (*GeneratedTool, *ReasoningTrace, error) {
	// Start trace
	trace := o.StartToolTrace(need.Name, need, userRequest)

	// Generate tool (the toolgen will populate trace details)
	tool, err := o.toolGen.GenerateTool(ctx, need)
	if err != nil {
		o.FinalizeTrace(trace, false, "", err.Error())
		return nil, trace, err
	}

	// Inject mandatory logging into generated code
	loggedCode, logErr := o.InjectLogging(tool.Code, tool.Name)
	if logErr == nil {
		tool.Code = loggedCode
	}

	// Validate logging
	validation := o.ValidateLogging(tool.Code)
	if !validation.Valid {
		trace.PostExecutionNotes = append(trace.PostExecutionNotes,
			fmt.Sprintf("Logging validation failed: missing %v", validation.Missing))
	}

	// Finalize trace
	o.FinalizeTrace(trace, true, tool.Code, "")

	return tool, trace, nil
}

// ExecuteOuroborosLoopWithTracing runs the full loop with reasoning trace capture
func (o *Orchestrator) ExecuteOuroborosLoopWithTracing(ctx context.Context, need *ToolNeed, userRequest string) (*LoopResult, *ReasoningTrace) {
	// Start trace
	trace := o.StartToolTrace(need.Name, need, userRequest)

	// Execute the loop
	result := o.ouroboros.Execute(ctx, need)

	// Inject logging if successful
	if result.Success && result.ToolHandle != nil {
		// The tool is already compiled, but we record for future generations
		trace.PostExecutionNotes = append(trace.PostExecutionNotes,
			"Tool compiled and registered successfully")
	}

	// Finalize trace
	failureReason := ""
	if result.Error != nil {
		failureReason = result.Error.Error()
	}
	code := ""
	if result.ToolHandle != nil {
		code = fmt.Sprintf("[compiled binary at %s]", result.ToolHandle.BinaryPath)
	}
	o.FinalizeTrace(trace, result.Success, code, failureReason)

	return result, trace
}

// =============================================================================
// QUALITY PROFILE WRAPPERS
// =============================================================================
// These methods expose the tool-specific quality profile system.

// GetToolProfile retrieves the quality profile for a tool
func (o *Orchestrator) GetToolProfile(toolName string) *ToolQualityProfile {
	return o.profiles.GetProfile(toolName)
}

// SetToolProfile stores a quality profile for a tool
func (o *Orchestrator) SetToolProfile(profile *ToolQualityProfile) {
	o.profiles.SetProfile(profile)
}

// GetDefaultToolProfile returns a default profile based on tool type
func (o *Orchestrator) GetDefaultToolProfile(toolName string, toolType ToolType) *ToolQualityProfile {
	return GetDefaultProfile(toolName, toolType)
}

// EvaluateWithProfile performs profile-aware quality evaluation
func (o *Orchestrator) EvaluateWithProfile(ctx context.Context, feedback *ExecutionFeedback) *QualityAssessment {
	profile := o.profiles.GetProfile(feedback.ToolName)
	if profile == nil {
		// Fall back to default evaluation
		return o.evaluator.Evaluate(ctx, feedback)
	}
	return o.evaluator.EvaluateWithProfile(ctx, feedback, profile)
}

// ExecuteAndEvaluateWithProfile runs a tool and evaluates using its quality profile
func (o *Orchestrator) ExecuteAndEvaluateWithProfile(ctx context.Context, toolName string, input string) (string, *QualityAssessment, error) {
	start := time.Now()

	output, err := o.ouroboros.ExecuteTool(ctx, toolName, input)

	feedback := &ExecutionFeedback{
		ToolName:   toolName,
		Timestamp:  start,
		Input:      input,
		Output:     output,
		OutputSize: len(output),
		Duration:   time.Since(start),
		Success:    err == nil,
	}

	if err != nil {
		feedback.ErrorMsg = err.Error()
	}

	// Profile-aware evaluation
	profile := o.profiles.GetProfile(toolName)
	if profile != nil {
		feedback.Quality = o.evaluator.EvaluateWithProfile(ctx, feedback, profile)
	} else {
		feedback.Quality = o.evaluator.Evaluate(ctx, feedback)
	}

	// Record for learning
	o.patterns.RecordExecution(*feedback)
	patterns := o.patterns.GetToolPatterns(feedback.ToolName)
	o.learnings.RecordLearning(feedback.ToolName, feedback, patterns)

	return output, feedback.Quality, err
}

// GenerateToolProfile uses LLM to generate a quality profile during tool creation
func (o *Orchestrator) GenerateToolProfile(ctx context.Context, toolName string, description string, toolCode string) (*ToolQualityProfile, error) {
	prompt := fmt.Sprintf(`Generate a quality profile for this tool. The profile defines expectations for how this tool should perform.

Tool Name: %s
Description: %s
Code (abbreviated):
%s

Based on the tool's purpose and implementation, determine:

1. **Tool Type** - One of:
   - quick_calculation: < 1s, simple computation (e.g., calculator, converter)
   - data_fetch: API call, may paginate (e.g., fetch docs, query database)
   - background_task: Long-running, minutes OK (e.g., indexer, importer)
   - recursive_analysis: Codebase traversal, slow OK (e.g., code analyzer)
   - realtime_query: Must be fast, frequent (e.g., status check, health ping)
   - one_time_setup: Run once, can be slow (e.g., initialization, migration)
   - batch_processor: Processes many items (e.g., bulk update, mass import)
   - monitor: Called repeatedly for status (e.g., metrics collector)

2. **Performance Expectations**:
   - expected_duration_min: Faster than this is suspicious (e.g., didn't do work)
   - expected_duration_max: Slower than this is a problem
   - acceptable_duration: Target duration for good performance
   - timeout_duration: When to give up
   - max_retries: How many retries are acceptable

3. **Output Expectations**:
   - expected_min_size: Smaller output is suspicious
   - expected_max_size: Larger might indicate issue
   - expected_typical_size: Normal output size in bytes
   - expected_format: json, text, csv, etc.
   - expects_pagination: Should we paginate?
   - required_fields: Fields that must be in output (for JSON)
   - must_contain: Strings that must appear
   - must_not_contain: Strings that indicate failure

4. **Usage Pattern**:
   - frequency: once, occasional, frequent, constant
   - is_idempotent: Same input = same output?
   - has_side_effects: Modifies external state?
   - depends_on_external: Needs external service?

5. **Caching**:
   - cacheable: Can results be cached?
   - cache_duration: How long to cache (e.g., "15m", "1h")
   - cache_key: What makes cache key unique (e.g., "input_hash")

Return JSON:
{
  "tool_type": "data_fetch",
  "description": "Brief description of what tool does",
  "performance": {
    "expected_duration_min_ms": 100,
    "expected_duration_max_ms": 30000,
    "acceptable_duration_ms": 5000,
    "timeout_duration_ms": 60000,
    "max_retries": 3,
    "expected_api_calls": 1,
    "scales_with_input_size": false
  },
  "output": {
    "expected_min_size": 100,
    "expected_max_size": 1048576,
    "expected_typical_size": 10240,
    "expected_format": "json",
    "expects_pagination": true,
    "expected_pages": 5,
    "required_fields": ["data", "status"],
    "must_contain": [],
    "must_not_contain": ["error", "failed"]
  },
  "usage_pattern": {
    "frequency": "occasional",
    "calls_per_session": 5,
    "is_idempotent": true,
    "has_side_effects": false,
    "depends_on_external": true
  },
  "caching": {
    "cacheable": true,
    "cache_duration": "15m",
    "cache_key": "input_url"
  },
  "custom_dimensions": [
    {
      "name": "items_fetched",
      "description": "Number of items in response",
      "expected_value": 100,
      "tolerance": 50,
      "weight": 0.3,
      "extract_pattern": "\"count\":\\s*(\\d+)"
    }
  ]
}`,
		toolName,
		description,
		truncateCode(toolCode, 2000),
	)

	resp, err := o.client.Complete(ctx, prompt)
	if err != nil {
		// Return default profile on error
		return GetDefaultProfile(toolName, ToolTypeDataFetch), nil
	}

	// Parse response
	profile, parseErr := parseProfileResponse(toolName, resp)
	if parseErr != nil {
		// Return default profile on parse error
		return GetDefaultProfile(toolName, ToolTypeDataFetch), nil
	}

	// Store the profile
	o.profiles.SetProfile(profile)

	return profile, nil
}

// GenerateToolWithProfile generates a tool and its quality profile together
func (o *Orchestrator) GenerateToolWithProfile(ctx context.Context, need *ToolNeed, userRequest string) (*GeneratedTool, *ToolQualityProfile, *ReasoningTrace, error) {
	// Generate tool with tracing
	tool, trace, err := o.GenerateToolWithTracing(ctx, need, userRequest)
	if err != nil {
		return nil, nil, trace, err
	}

	// Generate quality profile for the tool
	profile, profileErr := o.GenerateToolProfile(ctx, tool.Name, need.Purpose, tool.Code)
	if profileErr != nil {
		// Non-fatal - use default profile
		profile = GetDefaultProfile(tool.Name, ToolTypeDataFetch)
	}

	// Store profile
	o.profiles.SetProfile(profile)

	// Add profile info to trace notes
	trace.PostExecutionNotes = append(trace.PostExecutionNotes,
		fmt.Sprintf("Generated quality profile: type=%s, acceptable_duration=%v",
			profile.ToolType, profile.Performance.AcceptableDuration))

	return tool, profile, trace, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// shouldCheckToolNeed determines if we should check for tool needs
func shouldCheckToolNeed(input string) bool {
	// Check for explicit tool need indicators
	for _, pattern := range missingCapabilityPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// sortActionsByPriority sorts actions by priority (highest first)
func sortActionsByPriority(actions []AutopoiesisAction) {
	for i := 0; i < len(actions); i++ {
		for j := i + 1; j < len(actions); j++ {
			if actions[j].Priority > actions[i].Priority {
				actions[i], actions[j] = actions[j], actions[i]
			}
		}
	}
}

// hashString creates a simple hash of a string
func hashString(s string) string {
	// Simple hash for deduplication
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%08x", h)
}

// complexityLevelString returns string representation of complexity level
func complexityLevelString(level ComplexityLevel) string {
	switch level {
	case ComplexitySimple:
		return "Simple"
	case ComplexityModerate:
		return "Moderate"
	case ComplexityComplex:
		return "Complex"
	case ComplexityEpic:
		return "Epic"
	default:
		return "Unknown"
	}
}

// truncateCode truncates code for LLM prompts while preserving structure
func truncateCode(code string, maxLen int) string {
	if len(code) <= maxLen {
		return code
	}
	// Keep the beginning and note truncation
	return code[:maxLen] + "\n// ... (truncated)"
}

// parseProfileResponse parses LLM response into a ToolQualityProfile
func parseProfileResponse(toolName string, response string) (*ToolQualityProfile, error) {
	// Extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	// Parse into intermediate struct
	var raw struct {
		ToolType    string `json:"tool_type"`
		Description string `json:"description"`
		Performance struct {
			ExpectedDurationMinMS int64   `json:"expected_duration_min_ms"`
			ExpectedDurationMaxMS int64   `json:"expected_duration_max_ms"`
			AcceptableDurationMS  int64   `json:"acceptable_duration_ms"`
			TimeoutDurationMS     int64   `json:"timeout_duration_ms"`
			MaxRetries            int     `json:"max_retries"`
			ExpectedAPICalls      int     `json:"expected_api_calls"`
			MaxMemoryMB           int64   `json:"max_memory_mb"`
			ScalesWithInputSize   bool    `json:"scales_with_input_size"`
			ScalingFactor         float64 `json:"scaling_factor"`
		} `json:"performance"`
		Output struct {
			ExpectedMinSize     int      `json:"expected_min_size"`
			ExpectedMaxSize     int      `json:"expected_max_size"`
			ExpectedTypicalSize int      `json:"expected_typical_size"`
			ExpectedFormat      string   `json:"expected_format"`
			ExpectsPagination   bool     `json:"expects_pagination"`
			ExpectedPages       int      `json:"expected_pages"`
			RequiredFields      []string `json:"required_fields"`
			MustContain         []string `json:"must_contain"`
			MustNotContain      []string `json:"must_not_contain"`
			CompletenessCheck   string   `json:"completeness_check"`
		} `json:"output"`
		UsagePattern struct {
			Frequency         string `json:"frequency"`
			CallsPerSession   int    `json:"calls_per_session"`
			IsIdempotent      bool   `json:"is_idempotent"`
			HasSideEffects    bool   `json:"has_side_effects"`
			DependsOnExternal bool   `json:"depends_on_external"`
		} `json:"usage_pattern"`
		Caching struct {
			Cacheable     bool     `json:"cacheable"`
			CacheDuration string   `json:"cache_duration"`
			CacheKey      string   `json:"cache_key"`
			InvalidateOn  []string `json:"invalidate_on"`
		} `json:"caching"`
		CustomDimensions []struct {
			Name           string  `json:"name"`
			Description    string  `json:"description"`
			ExpectedValue  float64 `json:"expected_value"`
			Tolerance      float64 `json:"tolerance"`
			Weight         float64 `json:"weight"`
			ExtractPattern string  `json:"extract_pattern"`
		} `json:"custom_dimensions"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse profile JSON: %w", err)
	}

	// Convert to ToolQualityProfile
	profile := &ToolQualityProfile{
		ToolName:    toolName,
		ToolType:    ToolType(raw.ToolType),
		Description: raw.Description,
		CreatedAt:   time.Now(),
		Performance: PerformanceExpectations{
			ExpectedDurationMin: time.Duration(raw.Performance.ExpectedDurationMinMS) * time.Millisecond,
			ExpectedDurationMax: time.Duration(raw.Performance.ExpectedDurationMaxMS) * time.Millisecond,
			AcceptableDuration:  time.Duration(raw.Performance.AcceptableDurationMS) * time.Millisecond,
			TimeoutDuration:     time.Duration(raw.Performance.TimeoutDurationMS) * time.Millisecond,
			MaxRetries:          raw.Performance.MaxRetries,
			ExpectedAPIcalls:    raw.Performance.ExpectedAPICalls,
			MaxMemoryMB:         raw.Performance.MaxMemoryMB,
			ScalesWithInputSize: raw.Performance.ScalesWithInputSize,
			ScalingFactor:       raw.Performance.ScalingFactor,
		},
		Output: OutputExpectations{
			ExpectedMinSize:     raw.Output.ExpectedMinSize,
			ExpectedMaxSize:     raw.Output.ExpectedMaxSize,
			ExpectedTypicalSize: raw.Output.ExpectedTypicalSize,
			ExpectedFormat:      raw.Output.ExpectedFormat,
			ExpectsPagination:   raw.Output.ExpectsPagination,
			ExpectedPages:       raw.Output.ExpectedPages,
			RequiredFields:      raw.Output.RequiredFields,
			MustContain:         raw.Output.MustContain,
			MustNotContain:      raw.Output.MustNotContain,
			CompletenessCheck:   raw.Output.CompletenessCheck,
		},
		UsagePattern: UsagePattern{
			Frequency:         UsageFrequency(raw.UsagePattern.Frequency),
			CallsPerSession:   raw.UsagePattern.CallsPerSession,
			IsIdempotent:      raw.UsagePattern.IsIdempotent,
			HasSideEffects:    raw.UsagePattern.HasSideEffects,
			DependsOnExternal: raw.UsagePattern.DependsOnExternal,
		},
		Caching: CachingConfig{
			Cacheable:    raw.Caching.Cacheable,
			CacheKey:     raw.Caching.CacheKey,
			InvalidateOn: raw.Caching.InvalidateOn,
		},
	}

	// Parse cache duration
	if raw.Caching.CacheDuration != "" {
		if dur, err := time.ParseDuration(raw.Caching.CacheDuration); err == nil {
			profile.Caching.CacheDuration = dur
		}
	}

	// Convert custom dimensions
	for _, dim := range raw.CustomDimensions {
		profile.CustomDimensions = append(profile.CustomDimensions, CustomDimension{
			Name:           dim.Name,
			Description:    dim.Description,
			ExpectedValue:  dim.ExpectedValue,
			Tolerance:      dim.Tolerance,
			Weight:         dim.Weight,
			ExtractPattern: dim.ExtractPattern,
		})
	}

	return profile, nil
}
