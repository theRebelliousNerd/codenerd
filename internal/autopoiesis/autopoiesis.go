// Package autopoiesis implements self-modification capabilities for codeNERD.
// Autopoiesis (from Greek: self-creation) enables the system to:
// 1. Detect when tasks require campaign orchestration (complex multi-phase work)
// 2. Generate new tools when existing capabilities are insufficient
// 3. Create persistent agents when ongoing monitoring/learning is needed
package autopoiesis

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

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
	config      Config
	complexity  *ComplexityAnalyzer
	toolGen     *ToolGenerator
	persistence *PersistenceAnalyzer
	agentCreate *AgentCreator
	ouroboros   *OuroborosLoop  // The Ouroboros Loop for tool self-generation
	client      LLMClient

	// Feedback and Learning System
	evaluator   *QualityEvaluator   // Assess tool output quality
	patterns    *PatternDetector    // Detect recurring issues
	refiner     *ToolRefiner        // Improve suboptimal tools
	learnings   *LearningStore      // Persist learnings
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

	return &Orchestrator{
		config:      config,
		complexity:  NewComplexityAnalyzer(client),
		toolGen:     toolGen,
		persistence: NewPersistenceAnalyzer(client),
		agentCreate: NewAgentCreator(client, config.AgentsDir),
		ouroboros:   NewOuroborosLoop(client, ouroborosConfig),
		client:      client,

		// Initialize feedback and learning system
		evaluator:   NewQualityEvaluator(client),
		patterns:    NewPatternDetector(),
		refiner:     NewToolRefiner(client, toolGen),
		learnings:   NewLearningStore(learningsDir),
	}
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

// DetectToolNeed analyzes input to determine if a new tool is needed
func (o *Orchestrator) DetectToolNeed(ctx context.Context, input string) (*ToolNeed, error) {
	return o.toolGen.DetectToolNeed(ctx, input, "")
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

// ExecuteOuroborosLoop runs the complete tool self-generation cycle
func (o *Orchestrator) ExecuteOuroborosLoop(ctx context.Context, need *ToolNeed) *LoopResult {
	return o.ouroboros.Execute(ctx, need)
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

// RecordExecution records a tool execution and evaluates its quality
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
