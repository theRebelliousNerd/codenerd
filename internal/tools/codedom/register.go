package codedom

import (
	"codenerd/internal/tools"
)

// RegisterAll registers all Code DOM tools with the given registry.
func RegisterAll(registry *tools.Registry) error {
	allTools := []*tools.Tool{
		// Element operations
		GetElementsTool(),
		GetElementTool(),

		// Structural queries over the workspace-wide index
		FindSymbolTool(),
		PackageOutlineTool(),
		CallersOfTool(),
		CalleesOfTool(),
		UnreferencedSymbolsTool(),
		ImportersOfTool(),
		FindTextTool(),
		PredicateOutlineTool(),

		// Element-addressed edits, validated before anything is written
		EditElementTool(),
		ReplaceElementTool(),
		InsertElementTool(),
		DeleteElementTool(),
		CreateFileTool(),
		RepointTool(),

		// Line operations
		EditLinesTool(),
		InsertLinesTool(),
		DeleteLinesTool(),
		ApplyEditsTool(),

		// Test impact analysis
		RunImpactedTestsTool(),
		GetImpactedTestsTool(),
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
