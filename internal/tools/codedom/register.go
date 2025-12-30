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

		// Line operations
		EditLinesTool(),
		InsertLinesTool(),
		DeleteLinesTool(),

		// Test impact analysis
		RunImpactedTestsTool(),
		GetImpactedTestsTool(),
	}

	for _, tool := range allTools {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}

	return nil
}
