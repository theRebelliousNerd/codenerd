package core

import (
	"codenerd/internal/tools"
)

// RegisterAll registers all core filesystem tools with the given registry.
func RegisterAll(registry *tools.Registry) error {
	allTools := []*tools.Tool{
		// File operations
		ReadFileTool(),
		WriteFileTool(),
		EditFileTool(),
		DeleteFileTool(),
		ListFilesTool(),

		// Search operations
		GlobTool(),
		GrepTool(),
		SearchCodeTool(),
	}

	for _, tool := range allTools {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}

	return nil
}
