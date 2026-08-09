package research

import (
	"codenerd/internal/tools"
)

// RegisterAll registers all research tools with the given registry.
func RegisterAll(registry *tools.Registry) error {
	allTools := []*tools.Tool{
		// Context7 - LLM-optimized documentation
		Context7Tool(),

		// Web search
		WebSearchTool(),

		// Web fetching
		WebFetchTool(),

		// Browser automation
		BrowserNavigateTool(),
		BrowserExtractTool(),
		BrowserScreenshotTool(),
		BrowserClickTool(),
		BrowserTypeTool(),
		BrowserCloseTool(),
		BrowserObserveTool(),
		BrowserActTool(),
		BrowserMangleTool(),
		BrowserWaitTool(),
		BrowserReasonTool(),
		BrowserEvidenceTool(),
		BrowserSpecsTool(),

		// Caching
		CacheGetTool(),
		CacheSetTool(),
		CacheClearTool(),
		CacheStatsTool(),
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
