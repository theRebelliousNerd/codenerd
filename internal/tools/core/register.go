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
		// The verb that redeems a search_code handle. It is registered
		// alongside the verb that mints one, because a handle whose redemption
		// tool is not in the same catalog is a promise the model cannot keep.
		SearchExpandTool(),
	}

	for _, tool := range allTools {
		if registry.Has(tool.Name) {
			continue
		}
		if err := registry.Register(tool); err != nil {
			return err
		}
	}

	return nil
}
