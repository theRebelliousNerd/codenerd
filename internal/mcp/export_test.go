package mcp

import "context"

const (
	mcpSchemaDir = "../core/defaults"
	mcpPolicyRel = "policy/policy_mcp.mg"
)

// ServersForTest allows tests outside the mcp package to access the unexported servers map.
func (m *MCPClientManager) ServersForTest() map[string]*MCPServerConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.servers
}

// MangleSelectForTest exposes the kernel-backed selection path. The end-to-end
// kernel test must live in package mcp_test (internal/config imports
// internal/mcp, so an in-package test cannot import internal/core), which is
// why this wrapper exists.
func (c *JITToolCompiler) MangleSelectForTest(ctx context.Context, shardType string) ([]SelectedTool, error) {
	return c.mangleSelect(ctx, ToolCompilationContext{ShardType: shardType})
}

func ServerFactsForTest(server *MCPServer) []string {
	return serverFacts(server)
}

func ToolFactsForTest(tool *MCPTool) []string {
	return toolFacts(tool)
}
