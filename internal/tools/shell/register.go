package shell

import (
	"codenerd/internal/tools"
)

// RegisterAll registers all shell execution tools with the given registry.
func RegisterAll(registry *tools.Registry) error {
	allTools := []*tools.Tool{
		RunCommandTool(),
		BashTool(),
		RunBuildTool(),
		RunTestsTool(),
		GitDiffTool(),
		GitLogTool(),
		GitOperationTool(),
	}

	for _, tool := range allTools {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}

	return nil
}
