package mcpctl

import "codenerd/internal/tools"

// RegisterAll registers the MCP control-plane verbs with the given registry.
//
// Registration is unconditional — the verbs exist whether or not any MCP server
// is configured. A tool that appears only when a server happens to be connected
// is a tool the model cannot learn, and the no-server case has a better answer
// than absence: mcp_map says plainly that nothing is configured.
func RegisterAll(registry *tools.Registry) error {
	allTools := []*tools.Tool{
		MapTool(),
		ProbeTool(),
		CallTool(),
		ExpandTool(),
		ContextTool(),
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
