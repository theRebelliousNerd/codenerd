package mcp

import (
	"context"
	"fmt"
	"path/filepath"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// IntegrationClient is the interface that VirtualStore expects.
// We define it here to avoid import cycles (mirrors core.IntegrationClient).
type IntegrationClient interface {
	CallTool(ctx context.Context, tool string, args map[string]interface{}) (interface{}, error)
}

// IntegrationAdapter adapts MCPClientManager to the IntegrationClient interface.
// Each adapter is bound to a specific server ID for tool routing.
type IntegrationAdapter struct {
	manager  *MCPClientManager
	serverID string
}

// NewIntegrationAdapter creates a new adapter for a specific MCP server.
func NewIntegrationAdapter(manager *MCPClientManager, serverID string) *IntegrationAdapter {
	return &IntegrationAdapter{
		manager:  manager,
		serverID: serverID,
	}
}

// CallTool implements IntegrationClient by routing to the bound server.
// The tool parameter is the tool name; we construct the full toolID as serverID/tool.
func (a *IntegrationAdapter) CallTool(ctx context.Context, tool string, args map[string]interface{}) (interface{}, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("MCP manager not configured")
	}

	// Construct full toolID: serverID/toolName
	toolID := fmt.Sprintf("%s/%s", a.serverID, tool)

	logging.Get(logging.CategoryTools).Debug("IntegrationAdapter: Calling tool %s", toolID)

	result, err := a.manager.CallTool(ctx, toolID, args)
	if err != nil {
		logging.Get(logging.CategoryTools).Warn("IntegrationAdapter: Tool call failed: %v", err)
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("nil result from MCP call")
	}

	if !result.Success {
		return nil, fmt.Errorf("MCP tool call failed: %s", result.Error)
	}

	return result.Output, nil
}

// MCPIntegrationBridge provides a high-level interface for wiring MCP into the system.
// It manages the lifecycle and provides adapters for VirtualStore.
type MCPIntegrationBridge struct {
	manager     *MCPClientManager
	store       *MCPToolStore
	compiler    *JITToolCompiler
	renderer    *ToolRenderer
	adapters    map[string]*IntegrationAdapter
	initialized bool
}

// NewMCPIntegrationBridge creates a new MCP integration bridge.
func NewMCPIntegrationBridge(workspace string, kernel KernelInterface, embedder embedding.EmbeddingEngine, llmClient LLMClient, serverConfigs map[string]MCPServerConfig) (*MCPIntegrationBridge, error) {
	// Determine database path
	dbPath := filepath.Join(workspace, ".nerd", "mcp_tools.db")

	// Create store
	store, err := NewMCPToolStore(dbPath, embedder)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP tool store: %w", err)
	}

	// Create analyzer
	analyzer := NewToolAnalyzer(llmClient, embedder)

	// Create manager with server configs
	manager := NewMCPClientManager(store, analyzer, serverConfigs)

	// Create compiler
	compiler := NewJITToolCompiler(store, embedder, kernel)

	// Create renderer
	renderer := NewToolRenderer()

	return &MCPIntegrationBridge{
		manager:  manager,
		store:    store,
		compiler: compiler,
		renderer: renderer,
		adapters: make(map[string]*IntegrationAdapter),
	}, nil
}

// GetManager returns the MCP client manager.
func (b *MCPIntegrationBridge) GetManager() *MCPClientManager {
	return b.manager
}

// GetStore returns the MCP tool store.
func (b *MCPIntegrationBridge) GetStore() *MCPToolStore {
	return b.store
}

// GetCompiler returns the JIT tool compiler.
func (b *MCPIntegrationBridge) GetCompiler() *JITToolCompiler {
	return b.compiler
}

// GetRenderer returns the tool renderer.
func (b *MCPIntegrationBridge) GetRenderer() *ToolRenderer {
	return b.renderer
}

// GetAdapter returns an IntegrationAdapter for a specific server.
// Creates the adapter if it doesn't exist.
func (b *MCPIntegrationBridge) GetAdapter(serverID string) *IntegrationAdapter {
	if adapter, ok := b.adapters[serverID]; ok {
		return adapter
	}

	adapter := NewIntegrationAdapter(b.manager, serverID)
	b.adapters[serverID] = adapter
	return adapter
}

// ConnectServer connects to an MCP server by its configured ID.
func (b *MCPIntegrationBridge) ConnectServer(ctx context.Context, serverID string) error {
	return b.manager.Connect(ctx, serverID)
}

// ConnectAll connects to all enabled auto-connect servers.
func (b *MCPIntegrationBridge) ConnectAll(ctx context.Context) error {
	return b.manager.ConnectAll(ctx)
}

// Close closes all connections and the store.
func (b *MCPIntegrationBridge) Close() error {
	b.manager.DisconnectAll()
	if b.store != nil {
		return b.store.Close()
	}
	return nil
}

// CompileToolsForShard compiles tools for a specific shard type and task.
func (b *MCPIntegrationBridge) CompileToolsForShard(ctx context.Context, shardType, taskDescription string, tokenBudget int) (string, error) {
	tcc := ToolCompilationContext{
		ShardType:       shardType,
		TaskDescription: taskDescription,
		TokenBudget:     tokenBudget,
	}

	compiled, err := b.compiler.Compile(ctx, tcc)
	if err != nil {
		return "", err
	}

	return b.renderer.Render(compiled), nil
}

// DiscoverAndAnalyzeTools discovers tools from a server and analyzes them.
func (b *MCPIntegrationBridge) DiscoverAndAnalyzeTools(ctx context.Context, serverID string) error {
	return b.manager.DiscoverTools(ctx, serverID)
}

// Ensure IntegrationAdapter implements the interface.
var _ IntegrationClient = (*IntegrationAdapter)(nil)
