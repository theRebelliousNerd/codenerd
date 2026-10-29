package system

import (
	"context"
	"errors"

	"codenerd/internal/logging"
	"codenerd/internal/mcp"
	"codenerd/internal/tools/mcpctl"
)

// initMCPBridge builds the MCP integration bridge for the configured servers,
// wires each server into the VirtualStore, binds the model-facing control
// plane, and connects in the background. It runs after the JIT compiler is
// built: the bridge's tool analyzer compiles its system prompt from its role
// atoms (bctx.rolePrompts). A bridge that fails to build is logged and MCP
// stays unconfigured; it does not fail boot.
func initMCPBridge(bctx *bootContext) {
	integrationsCfg := bctx.appCfg.GetIntegrations()
	serverConfigs := integrationsCfg.ToMCPServerConfigs()
	if len(serverConfigs) == 0 {
		return
	}
	var mcpLLMClient mcp.LLMClient
	if bctx.llmClient != nil {
		mcpLLMClient = &perceptionLLMAdapter{client: bctx.llmClient}
	}
	mcpBridge, err := mcp.NewMCPIntegrationBridge(bctx.workspace, newMCPKernelAdapter(bctx.kernel), bctx.embeddingEngine, mcpLLMClient, bctx.rolePrompts, serverConfigs)
	if err != nil {
		logging.Get(logging.CategoryTools).Warn("Failed to init MCP bridge: %v", err)
		return
	}
	mcpCtx, mcpCancel := context.WithCancel(bctx.ctx)
	mcpDone := make(chan struct{})
	bctx.mcpBridge = mcpBridge
	mcpBridge.GetManager().SetApprovalBroker(bctx.approvals)
	bctx.mcpCancel = mcpCancel
	bctx.mcpDone = mcpDone
	for serverID := range serverConfigs {
		bctx.virtualStore.SetMCPClient(serverID, mcpBridge.GetAdapter(serverID))
		logging.Get(logging.CategoryTools).Info("Wired MCP integration: %s", serverID)
	}
	// Bind the model-facing control plane. This is what puts MCP in
	// front of the LLM at all: the five mcp_* verbs are registered
	// unconditionally, and until they have a plane they correctly
	// report that nothing is configured. Binding happens before
	// ConnectAll so a tool call racing boot sees an honest empty atlas
	// rather than "MCP is not configured".
	mcpctl.SetControlPlane(mcpBridge.ControlPlane())
	go func() {
		defer close(mcpDone)
		if err := mcpBridge.ConnectAll(mcpCtx); err != nil && !errors.Is(err, context.Canceled) {
			logging.Get(logging.CategoryTools).Warn("MCP auto-connect failed: %v", err)
		}
	}()
}
