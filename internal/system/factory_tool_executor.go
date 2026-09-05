package system

import (
	"context"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// orchestratorToolExecutor is the VirtualStore's tool executor: it runs a
// generated tool through the autopoiesis Orchestrator's evaluate-and-profile
// path so every execution feeds the tool's learning record, and triggers an
// asynchronous refinement when the orchestrator says the tool needs one.
// Moved from cmd/nerd/chat (ToolExecutorAdapter) so every boot path — not
// only the TUI — executes tools with the feedback loop; the factory used to
// hand the VirtualStore the bare OuroborosLoop, which executes without it.
type orchestratorToolExecutor struct {
	orchestrator *autopoiesis.Orchestrator
}

func newOrchestratorToolExecutor(orch *autopoiesis.Orchestrator) *orchestratorToolExecutor {
	return &orchestratorToolExecutor{orchestrator: orch}
}

// ExecuteTool runs a registered tool with the given input.
func (a *orchestratorToolExecutor) ExecuteTool(ctx context.Context, toolName string, input string) (string, error) {
	output, _, err := a.orchestrator.ExecuteAndEvaluateWithProfile(ctx, toolName, input)
	if err != nil {
		return output, err
	}

	// ExecuteAndEvaluateWithProfile already records execution feedback.
	// Only the refinement decision remains here.
	go func() {
		log := logging.Get(logging.CategoryAutopoiesis)
		needsRefinement, suggestions := a.orchestrator.ShouldRefineTool(toolName)
		if !needsRefinement {
			return
		}
		log.Info("Tool '%s' needs refinement based on %d patterns", toolName, len(suggestions))
		refinementCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		result, err := a.orchestrator.RefineTool(refinementCtx, toolName, "")
		if err != nil {
			log.Warn("Tool refinement for '%s' failed: %v", toolName, err)
		} else if result.Success {
			log.Info("Tool '%s' refined successfully: %v", toolName, result.Changes)
		}
	}()

	return output, nil
}

// ListTools returns all registered tools.
func (a *orchestratorToolExecutor) ListTools() []core.ToolInfo {
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

// GetTool returns info about a specific tool.
func (a *orchestratorToolExecutor) GetTool(name string) (*core.ToolInfo, bool) {
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

var _ core.ToolExecutor = (*orchestratorToolExecutor)(nil)
