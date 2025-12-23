// Package chat provides the interactive TUI chat interface for codeNERD.
// This file provides adapters between the autopoiesis and core packages.
package chat

import (
	"context"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// ToolExecutorAdapter adapts autopoiesis.Orchestrator to implement core.ToolExecutor interface
type ToolExecutorAdapter struct {
	orchestrator *autopoiesis.Orchestrator
}

// NewToolExecutorAdapter creates a new adapter for the Orchestrator
func NewToolExecutorAdapter(orch *autopoiesis.Orchestrator) *ToolExecutorAdapter {
	return &ToolExecutorAdapter{orchestrator: orch}
}

// ExecuteTool runs a registered tool with the given input
func (a *ToolExecutorAdapter) ExecuteTool(ctx context.Context, toolName string, input string) (string, error) {
	// Use ExecuteAndEvaluateWithProfile for full feedback loop
	output, assessment, err := a.orchestrator.ExecuteAndEvaluateWithProfile(ctx, toolName, input)
	if err != nil {
		return output, err
	}

	// Log quality assessment for learning (async to not block)
	go func() {
		log := logging.Get(logging.CategoryAutopoiesis)
		feedback := &autopoiesis.ExecutionFeedback{
			ToolName:  toolName,
			Input:     input,
			Output:    output,
			Duration:  time.Since(time.Now()), // Approximate - real duration tracked internally
			Success:   true,
			Timestamp: time.Now(),
			Quality:   assessment, // Already a *QualityAssessment
		}
		a.orchestrator.RecordExecution(context.Background(), feedback)

		// GAP-005 FIX: Check if tool needs refinement after recording execution
		needsRefinement, suggestions := a.orchestrator.ShouldRefineTool(toolName)
		if needsRefinement {
			log.Info("Tool '%s' needs refinement based on %d patterns", toolName, len(suggestions))
			// Trigger async refinement (doesn't block user flow)
			refinementCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			result, err := a.orchestrator.RefineTool(refinementCtx, toolName, "")
			if err != nil {
				log.Warn("Tool refinement for '%s' failed: %v", toolName, err)
			} else if result.Success {
				log.Info("Tool '%s' refined successfully: %v", toolName, result.Changes)
			}
		}
	}()

	return output, nil
}

// ListTools returns all registered tools
func (a *ToolExecutorAdapter) ListTools() []core.ToolInfo {
	autoTools := a.orchestrator.ListTools()
	coreTools := make([]core.ToolInfo, len(autoTools))
	for i, t := range autoTools {
		coreTools[i] = core.ToolInfo{
			Name:         t.Name,
			Description:  t.Description,
			BinaryPath:   t.BinaryPath,
			Hash:         t.Hash,
			RegisteredAt: t.RegisteredAt,
			ExecuteCount: t.ExecuteCount,
		}
	}
	return coreTools
}

// GetTool returns info about a specific tool
func (a *ToolExecutorAdapter) GetTool(name string) (*core.ToolInfo, bool) {
	autoInfo, exists := a.orchestrator.GetToolInfo(name)
	if !exists {
		return nil, false
	}
	return &core.ToolInfo{
		Name:         autoInfo.Name,
		Description:  autoInfo.Description,
		BinaryPath:   autoInfo.BinaryPath,
		Hash:         autoInfo.Hash,
		RegisteredAt: autoInfo.RegisteredAt,
		ExecuteCount: autoInfo.ExecuteCount,
	}, true
}

// Compile-time check that ToolExecutorAdapter implements core.ToolExecutor
var _ core.ToolExecutor = (*ToolExecutorAdapter)(nil)
