package autopoiesis

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/logging"
)

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
	// Inject learnings from past tool generation into prompts
	o.RefreshLearningsContext()

	result := o.ouroboros.Execute(ctx, need)

	// Wire to kernel: Assert tool registration facts on success
	if result.Success && result.ToolHandle != nil {
		o.assertToolRegistered(result.ToolHandle)

		// GAP-019 FIX: Propagate hot-reload facts to parent kernel
		// The OuroborosLoop's internal engine has these facts, but they need
		// to be synced to the main kernel for spreading activation and JIT
		o.assertToolHotReloaded(result.ToolHandle.Name)
	}

	// Record generation learning for persistence
	o.recordGenerationLearning(ctx, need, result)

	return result
}

// recordGenerationLearning converts a LoopResult to ExecutionFeedback and records it.
// This captures tool generation outcomes (success, safety failures, Thunderdome results)
// as learnings for future reference and analysis.
func (o *Orchestrator) recordGenerationLearning(ctx context.Context, need *ToolNeed, result *LoopResult) {
	if o.learnings == nil {
		return
	}

	// Create execution feedback from generation result
	feedback := &ExecutionFeedback{
		ToolName:    need.Name,
		ExecutionID: fmt.Sprintf("gen_%s_%d", need.Name, time.Now().Unix()),
		Timestamp:   time.Now(),
		Input:       need.Purpose,
		Duration:    result.Duration,
		Success:     result.Success,
	}

	// Add quality assessment based on generation outcome
	var issues []QualityIssue
	if !result.Success {
		feedback.ErrorType = result.Stage.String()
		feedback.ErrorMsg = result.Error

		// Extract issues from safety report if available
		if result.SafetyReport != nil {
			for _, v := range result.SafetyReport.Violations {
				issues = append(issues, QualityIssue{
					Type:        IssueType(v.Type.String()),
					Description: v.Description,
					Severity:    float64(v.Severity) / 10.0,
				})
			}
		}
	}

	// Calculate quality score based on generation stage reached
	score := 0.0
	switch result.Stage {
	case StageComplete, StageRegistration, StageExecution:
		score = 1.0 // Made it all the way - fully successful
	case StageCompilation:
		score = 0.9 // Compiled successfully
	case StageSimulation:
		score = 0.8 // Passed simulation/Thunderdome
	case StageThunderdome:
		score = 0.7 // Passed safety, in Thunderdome
	case StageSafetyCheck:
		score = 0.4 // Generated but failed safety
	case StageSpecification:
		score = 0.2 // Generation started
	default:
		score = 0.1
	}

	feedback.Quality = &QualityAssessment{
		Score:  score,
		Issues: issues,
	}

	// Record the learning
	patterns := o.patterns.GetToolPatterns(need.Name)
	o.learnings.RecordLearning(need.Name, feedback, patterns)

	// Sync learnings to kernel for logic-driven refinement decisions
	if o.kernel != nil {
		o.SyncLearningsToKernel()
	}

	logging.Autopoiesis("Recorded generation learning for %s: success=%v, stage=%s, score=%.2f",
		need.Name, result.Success, result.Stage, score)
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

// ListTools returns tool info for all registered tools (for chat UI)
func (o *Orchestrator) ListTools() []ToolInfo {
	return o.ouroboros.ListTools()
}

// GetToolInfo returns info about a specific tool (for chat UI)
func (o *Orchestrator) GetToolInfo(name string) (*ToolInfo, bool) {
	return o.ouroboros.GetTool(name)
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
