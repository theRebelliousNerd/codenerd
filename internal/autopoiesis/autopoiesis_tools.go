package autopoiesis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// GenerateTool creates a new tool based on the detected need.
//
// This is the production entry point used by chat `generate_tool` and by the
// verification corrective-action path. It runs the full Ouroboros pipeline —
// safety audit, Thunderdome, Mangle transition simulation, compile, register —
// and NOT the bare LLM call it used to run. The old body called
// toolGen.GenerateTool directly, so a tool created from chat was written to
// disk having passed only the generator's own syntactic self-check: no
// go_safety.mg audit, no adversarial pass, no kernel registration facts. The
// deep path and the shallow path also drifted independently, which is exactly
// the dual-maintenance problem OPEN-QUESTIONS Q1 describes.
//
// Errors are now returned for tools the pipeline rejected; callers that only
// logged "generated tool X" will now correctly report the rejection instead.
func (o *Orchestrator) GenerateTool(ctx context.Context, need *ToolNeed) (*GeneratedTool, error) {
	if need == nil {
		return nil, fmt.Errorf("tool need is nil")
	}

	result := o.ExecuteOuroborosLoop(ctx, need)
	if result == nil {
		return nil, fmt.Errorf("ouroboros returned no result for %q", need.Name)
	}
	if !result.Success {
		reason := result.Error
		if reason == "" {
			reason = "no reason reported"
		}
		return nil, fmt.Errorf("ouroboros rejected tool %q at stage %s: %s", need.Name, result.Stage, reason)
	}

	return o.generatedToolFromResult(need, result), nil
}

// generatedToolFromResult reconstructs the GeneratedTool view callers expect
// from a committed LoopResult. The source is read back from disk because the
// loop commits source before compiling; a read failure is not fatal since the
// tool is already registered and executable.
func (o *Orchestrator) generatedToolFromResult(need *ToolNeed, result *LoopResult) *GeneratedTool {
	name := result.ToolName
	if name == "" {
		name = need.Name
	}

	tool := &GeneratedTool{
		Name:        name,
		Description: need.Purpose,
		FilePath:    filepath.Join(o.config.ToolsDir, name+".go"),
		Validated:   true,
	}
	if handle := result.ToolHandle; handle != nil {
		if handle.Name != "" {
			tool.Name = handle.Name
		}
		if handle.Description != "" {
			tool.Description = handle.Description
		}
	}
	if src, err := os.ReadFile(tool.FilePath); err == nil {
		tool.Code = string(src)
	}
	return tool
}

// WriteAndRegisterTool writes the generated tool to disk and registers it.
//
// UNAUDITED PATH: this bypasses go_safety.mg, the Thunderdome and compilation.
// It exists for tests and diagnostics that need a registry entry without the
// full pipeline; production tool creation must go through ExecuteOuroborosLoop
// or GenerateTool. tool_creation_routing_test.go enforces that no production
// call site uses it.
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
	if o == nil || need == nil || result == nil {
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

	o.RecordExecution(ctx, feedback)

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
	return o.ouroboros.ListRuntimeTools()
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
	_, exists := o.ouroboros.GetRuntimeTool(name)
	return exists
}

// CheckToolSafety validates tool code without compiling
func (o *Orchestrator) CheckToolSafety(code string) *SafetyReport {
	return o.ouroboros.CheckToolSafety(code)
}
