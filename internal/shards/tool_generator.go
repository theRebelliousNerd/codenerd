// Package shards implements specialized ShardAgent types for the Cortex 1.5.0 architecture.
// This file implements the ToolGenerator ShardAgent for autopoiesis operations.
// The ToolGenerator Shard handles tool creation, refinement, and status queries
// using the autopoiesis Orchestrator's Ouroboros Loop.
package shards

import (
	"codenerd/internal/autopoiesis"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ToolGeneratorConfig holds configuration for the tool generator shard.
type ToolGeneratorConfig struct {
	ToolsDir       string        // Where to store generated tools
	MaxRetries     int           // Retry limit for failed generations
	CompileTimeout time.Duration // Timeout for compilation
	SafetyMode     bool          // Extra safety checks
}

// DefaultToolGeneratorConfig returns sensible defaults.
func DefaultToolGeneratorConfig() ToolGeneratorConfig {
	return ToolGeneratorConfig{
		ToolsDir:       ".nerd/tools",
		MaxRetries:     3,
		CompileTimeout: 30 * time.Second,
		SafetyMode:     true,
	}
}

// ToolGeneratorResult represents the output of a tool generation task.
type ToolGeneratorResult struct {
	Action     string                          `json:"action"` // generate, refine, list, status
	Success    bool                            `json:"success"`
	ToolName   string                          `json:"tool_name,omitempty"`
	Message    string                          `json:"message"`
	LoopResult *autopoiesis.LoopResult         `json:"loop_result,omitempty"`
	Profile    *autopoiesis.ToolQualityProfile `json:"profile,omitempty"`
	Tools      []*autopoiesis.RuntimeTool      `json:"tools,omitempty"`
	Learnings  []*autopoiesis.ToolLearning     `json:"learnings,omitempty"`
	Facts      []core.Fact                     `json:"facts,omitempty"`
	Duration   time.Duration                   `json:"duration"`
}

// ToolGeneratorShard handles autopoiesis tool generation and management.
type ToolGeneratorShard struct {
	mu sync.RWMutex

	// Identity
	id     string
	config core.ShardConfig
	state  core.ShardState

	// ToolGenerator-specific
	generatorConfig ToolGeneratorConfig

	// Components
	kernel       *core.RealKernel
	llmClient    perception.LLMClient
	orchestrator *autopoiesis.Orchestrator

	// State tracking
	startTime time.Time
	stopCh    chan struct{}
}

// NewToolGeneratorShard creates a new tool generator shard.
func NewToolGeneratorShard(id string, config core.ShardConfig) *ToolGeneratorShard {
	return &ToolGeneratorShard{
		id:              id,
		config:          config,
		state:           core.ShardStateIdle,
		generatorConfig: DefaultToolGeneratorConfig(),
		kernel:          core.NewRealKernel(),
		stopCh:          make(chan struct{}),
	}
}

// GetID returns the shard ID.
func (s *ToolGeneratorShard) GetID() string {
	return s.id
}

// GetState returns the current state.
func (s *ToolGeneratorShard) GetState() core.ShardState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetConfig returns the shard configuration.
func (s *ToolGeneratorShard) GetConfig() core.ShardConfig {
	return s.config
}

// GetKernel returns the shard's kernel for fact propagation.
func (s *ToolGeneratorShard) GetKernel() *core.RealKernel {
	return s.kernel
}

// SetLLMClient sets the LLM client for generation.
func (s *ToolGeneratorShard) SetLLMClient(client perception.LLMClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.llmClient = client

	// Initialize orchestrator with LLM client
	if s.orchestrator == nil {
		autoConfig := autopoiesis.Config{
			ToolsDir:  s.generatorConfig.ToolsDir,
			AgentsDir: ".nerd/agents",
		}
		s.orchestrator = autopoiesis.NewOrchestrator(client, autoConfig)

		// Wire kernel to orchestrator for logic-driven orchestration
		if s.kernel != nil {
			kernelAdapter := core.NewKernelAdapter(s.kernel)
			s.orchestrator.SetKernel(kernelAdapter)
		}
	}
}

// Stop stops the shard.
func (s *ToolGeneratorShard) Stop() error {
	close(s.stopCh)
	s.mu.Lock()
	s.state = core.ShardStateCompleted
	s.mu.Unlock()
	return nil
}

// Execute executes a tool generation task.
func (s *ToolGeneratorShard) Execute(ctx context.Context, task string) (string, error) {
	s.mu.Lock()
	s.state = core.ShardStateRunning
	s.startTime = time.Now()
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.state = core.ShardStateCompleted
		s.mu.Unlock()
	}()

	// Parse the task to determine action
	action := s.parseAction(task)

	var result *ToolGeneratorResult
	var err error

	switch action {
	case "generate":
		result, err = s.handleGenerate(ctx, task)
	case "refine":
		result, err = s.handleRefine(ctx, task)
	case "list":
		result, err = s.handleList(ctx)
	case "status":
		result, err = s.handleStatus(ctx, task)
	default:
		// Default to generate if we detect a capability need
		result, err = s.handleGenerate(ctx, task)
	}

	if err != nil {
		return "", err
	}

	// Assert facts to kernel
	for _, fact := range result.Facts {
		_ = s.kernel.Assert(fact)
	}

	// Return JSON result
	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

// parseAction determines what action to take based on the task.
func (s *ToolGeneratorShard) parseAction(task string) string {
	lower := strings.ToLower(task)

	// Check for list action
	if strings.Contains(lower, "list") || strings.Contains(lower, "show") && strings.Contains(lower, "tool") {
		return "list"
	}

	// Check for status action
	if strings.Contains(lower, "status") || strings.Contains(lower, "quality") ||
		strings.Contains(lower, "performance") || strings.Contains(lower, "learning") {
		return "status"
	}

	// Check for refine action
	if strings.Contains(lower, "refine") || strings.Contains(lower, "improve") ||
		strings.Contains(lower, "fix") && strings.Contains(lower, "tool") {
		return "refine"
	}

	// Default to generate
	return "generate"
}

// handleGenerate handles tool generation requests.
func (s *ToolGeneratorShard) handleGenerate(ctx context.Context, task string) (*ToolGeneratorResult, error) {
	s.mu.RLock()
	orch := s.orchestrator
	s.mu.RUnlock()

	if orch == nil {
		return nil, fmt.Errorf("orchestrator not initialized - missing LLM client")
	}

	// Extract capability need from task
	need := s.extractToolNeed(task)

	// Execute the Ouroboros Loop with tracing
	loopResult, trace := orch.ExecuteOuroborosLoopWithTracing(ctx, need, task)

	result := &ToolGeneratorResult{
		Action:     "generate",
		Success:    loopResult.Success,
		ToolName:   need.Name,
		LoopResult: loopResult,
		Duration:   time.Since(s.startTime),
		Facts:      []core.Fact{},
	}

	if loopResult.Success && loopResult.ToolHandle != nil {
		result.Message = fmt.Sprintf("Successfully generated tool '%s'", need.Name)

		// Generate quality profile for the new tool
		profile, _ := orch.GenerateToolProfile(ctx, need.Name, need.Purpose, "")
		result.Profile = profile

		// Assert success facts
		result.Facts = append(result.Facts,
			core.Fact{Predicate: "tool_generated", Args: []interface{}{need.Name, time.Now().Unix()}},
			core.Fact{Predicate: "tool_ready", Args: []interface{}{need.Name}},
		)

		// Add trace info
		if trace != nil {
			result.Facts = append(result.Facts,
				core.Fact{Predicate: "tool_trace", Args: []interface{}{need.Name, trace.TraceID}},
			)
		}
	} else {
		errMsg := "unknown error"
		if loopResult.Error != nil {
			errMsg = loopResult.Error.Error()
		}
		result.Message = fmt.Sprintf("Failed to generate tool: %s", errMsg)

		// Assert failure facts
		result.Facts = append(result.Facts,
			core.Fact{Predicate: "tool_generation_failed", Args: []interface{}{need.Name, errMsg}},
		)
	}

	return result, nil
}

// handleRefine handles tool refinement requests.
func (s *ToolGeneratorShard) handleRefine(ctx context.Context, task string) (*ToolGeneratorResult, error) {
	s.mu.RLock()
	orch := s.orchestrator
	s.mu.RUnlock()

	if orch == nil {
		return nil, fmt.Errorf("orchestrator not initialized")
	}

	// Extract tool name from task
	toolName := s.extractToolName(task)
	if toolName == "" {
		return nil, fmt.Errorf("could not determine which tool to refine")
	}

	// Check if tool needs refinement
	needsRefinement, suggestions := orch.ShouldRefineTool(toolName)
	if !needsRefinement {
		return &ToolGeneratorResult{
			Action:   "refine",
			Success:  true,
			ToolName: toolName,
			Message:  fmt.Sprintf("Tool '%s' does not need refinement", toolName),
			Duration: time.Since(s.startTime),
		}, nil
	}

	// Get the original code (would need to read from file)
	// For now, we'll trigger a regeneration
	result := &ToolGeneratorResult{
		Action:   "refine",
		ToolName: toolName,
		Duration: time.Since(s.startTime),
		Facts:    []core.Fact{},
	}

	// Perform refinement
	refinementResult, err := orch.RefineTool(ctx, toolName, "")
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Refinement failed: %v", err)
		return result, nil
	}

	result.Success = refinementResult.Success
	if refinementResult.Success {
		result.Message = fmt.Sprintf("Successfully refined tool '%s' with %d improvements: %s",
			toolName, len(suggestions), strings.Join(refinementResult.Changes, "; "))
		result.Facts = append(result.Facts,
			core.Fact{Predicate: "tool_refined", Args: []interface{}{toolName, time.Now().Unix()}},
		)
	} else {
		result.Message = fmt.Sprintf("Refinement failed: %s", strings.Join(refinementResult.Changes, "; "))
	}

	return result, nil
}

// handleList handles tool listing requests.
func (s *ToolGeneratorShard) handleList(ctx context.Context) (*ToolGeneratorResult, error) {
	// Check context for cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	orch := s.orchestrator
	s.mu.RUnlock()

	if orch == nil {
		return &ToolGeneratorResult{
			Action:  "list",
			Success: true,
			Message: "No tools available (orchestrator not initialized)",
			Tools:   []*autopoiesis.RuntimeTool{},
		}, nil
	}

	tools := orch.ListGeneratedTools()

	result := &ToolGeneratorResult{
		Action:   "list",
		Success:  true,
		Tools:    tools,
		Duration: time.Since(s.startTime),
	}

	if len(tools) == 0 {
		result.Message = "No generated tools found"
	} else {
		result.Message = fmt.Sprintf("Found %d generated tools", len(tools))
	}

	return result, nil
}

// handleStatus handles tool status queries.
func (s *ToolGeneratorShard) handleStatus(ctx context.Context, task string) (*ToolGeneratorResult, error) {
	// Check context for cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.RLock()
	orch := s.orchestrator
	s.mu.RUnlock()

	if orch == nil {
		return nil, fmt.Errorf("orchestrator not initialized")
	}

	// Extract tool name or get all
	toolName := s.extractToolName(task)

	result := &ToolGeneratorResult{
		Action:    "status",
		Success:   true,
		Duration:  time.Since(s.startTime),
		Learnings: []*autopoiesis.ToolLearning{},
		Facts:     []core.Fact{},
	}

	if toolName != "" {
		// Get specific tool status
		learning := orch.GetToolLearning(toolName)
		profile := orch.GetToolProfile(toolName)
		patterns := orch.GetToolPatterns(toolName)

		result.ToolName = toolName
		result.Profile = profile

		if learning != nil {
			result.Learnings = append(result.Learnings, learning)
			result.Message = fmt.Sprintf("Tool '%s': %d executions, %.1f%% success rate, %.2f avg quality",
				toolName, learning.TotalExecutions, learning.SuccessRate*100, learning.AverageQuality)

			// Add facts
			result.Facts = append(result.Facts,
				core.Fact{Predicate: "tool_learning", Args: []interface{}{
					toolName, learning.TotalExecutions, learning.SuccessRate, learning.AverageQuality,
				}},
			)

			// Check if needs refinement
			if needsRefinement, _ := orch.ShouldRefineTool(toolName); needsRefinement {
				result.Facts = append(result.Facts,
					core.Fact{Predicate: "tool_needs_refinement", Args: []interface{}{toolName}},
				)
			}

			// Add pattern facts
			for _, p := range patterns {
				result.Facts = append(result.Facts,
					core.Fact{Predicate: "tool_issue_pattern", Args: []interface{}{
						toolName, string(p.IssueType), p.Occurrences, p.Confidence,
					}},
				)
			}
		} else {
			result.Message = fmt.Sprintf("No learning data for tool '%s'", toolName)
		}
	} else {
		// Get all learnings
		allLearnings := orch.GetAllLearnings()
		result.Learnings = allLearnings

		if len(allLearnings) == 0 {
			result.Message = "No tool learnings recorded yet"
		} else {
			result.Message = fmt.Sprintf("Status for %d tools", len(allLearnings))
			for _, l := range allLearnings {
				result.Facts = append(result.Facts,
					core.Fact{Predicate: "tool_learning", Args: []interface{}{
						l.ToolName, l.TotalExecutions, l.SuccessRate, l.AverageQuality,
					}},
				)
			}
		}
	}

	return result, nil
}

// extractToolNeed parses a task to create a ToolNeed.
func (s *ToolGeneratorShard) extractToolNeed(task string) *autopoiesis.ToolNeed {
	// Basic extraction - in practice this would be more sophisticated
	// possibly using LLM to parse the intent

	// Try to extract purpose from common patterns
	purpose := task
	patterns := []string{
		`(?i)(?:create|make|generate|build)\s+(?:a\s+)?tool\s+(?:for|to)\s+(.+)`,
		`(?i)(?:i|we)\s+need\s+(?:a\s+)?tool\s+(?:for|to)\s+(.+)`,
		`(?i)tool\s+(?:for|to)\s+(.+)`,
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if matches := re.FindStringSubmatch(task); len(matches) > 1 {
			purpose = matches[1]
			break
		}
	}

	// Generate a name from purpose
	name := s.generateToolName(purpose)

	return &autopoiesis.ToolNeed{
		Name:       name,
		Purpose:    purpose,
		InputType:  "string",
		OutputType: "string",
		Priority:   0.8,
		Confidence: 0.7,
		Reasoning:  fmt.Sprintf("User requested: %s", task),
	}
}

// extractToolName extracts a tool name from a task string.
func (s *ToolGeneratorShard) extractToolName(task string) string {
	// Look for quoted names
	re := regexp.MustCompile(`['"]([^'"]+)['"]`)
	if matches := re.FindStringSubmatch(task); len(matches) > 1 {
		return matches[1]
	}

	// Look for "the X tool" pattern
	re = regexp.MustCompile(`(?i)the\s+(\w+)\s+tool`)
	if matches := re.FindStringSubmatch(task); len(matches) > 1 {
		return matches[1]
	}

	// Look for "tool X" pattern
	re = regexp.MustCompile(`(?i)tool\s+(\w+)`)
	if matches := re.FindStringSubmatch(task); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// generateToolName creates a valid tool name from a purpose string.
func (s *ToolGeneratorShard) generateToolName(purpose string) string {
	// Simple slugification
	name := strings.ToLower(purpose)
	name = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")

	// Truncate if too long
	if len(name) > 30 {
		name = name[:30]
	}

	// Ensure valid Go identifier
	if len(name) == 0 {
		name = "generated_tool"
	}

	return name
}
