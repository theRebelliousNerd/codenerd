package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// IntegrationClient is the interface that VirtualStore expects.
// We define it here to avoid import cycles (mirrors core.IntegrationClient).
type IntegrationClient interface {
	CallTool(ctx context.Context, tool string, args map[string]any) (any, error)
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
func (a *IntegrationAdapter) CallTool(ctx context.Context, tool string, args map[string]any) (any, error) {
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
	mu       sync.RWMutex
	manager  *MCPClientManager
	store    *MCPToolStore
	compiler *JITToolCompiler
	renderer *ToolRenderer
	facts    *FactEmitter
	kernel   KernelInterface
	adapters map[string]*IntegrationAdapter

	// ready closes once ConnectAll and the initial per-server discovery have
	// finished, so callers can gate tool compilation on a populated catalog.
	ready     chan struct{}
	readyOnce sync.Once
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

	// Mirror server/tool state into the kernel. Without this the MCP
	// predicates stay empty and policy_mcp.mg has nothing to reason over.
	emitter := NewFactEmitter(kernel)
	manager.SetFactEmitter(emitter)

	return &MCPIntegrationBridge{
		manager:  manager,
		store:    store,
		compiler: compiler,
		renderer: renderer,
		facts:    emitter,
		kernel:   kernel,
		adapters: make(map[string]*IntegrationAdapter),
		ready:    make(chan struct{}),
	}, nil
}

// GetFactEmitter returns the kernel fact emitter (nil when no kernel is wired).
func (b *MCPIntegrationBridge) GetFactEmitter() *FactEmitter {
	return b.facts
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
	b.mu.Lock()
	defer b.mu.Unlock()

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

// ConnectAll connects to all enabled auto-connect servers and then waits for
// the initial tool discovery each connection kicked off. It marks the bridge
// ready before returning, whether or not individual servers failed: readiness
// means "the catalog is as complete as it is going to get", not "everything
// connected". A connect error is still returned to the caller.
func (b *MCPIntegrationBridge) ConnectAll(ctx context.Context) error {
	connectErr := b.manager.ConnectAll(ctx)

	if err := b.manager.WaitForDiscovery(ctx); err != nil {
		// Context died mid-discovery (shutdown). Signal readiness anyway so
		// nothing blocks forever on a bridge that will never progress.
		b.markReady()
		if connectErr != nil {
			return connectErr
		}
		return err
	}

	b.markReady()
	return connectErr
}

// readyChan returns the readiness channel, creating it on demand so a bridge
// built as a bare struct literal (as tests do) is still usable.
func (b *MCPIntegrationBridge) readyChan() chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ready == nil {
		b.ready = make(chan struct{})
	}
	return b.ready
}

// markReady closes the readiness channel exactly once and publishes the
// resulting catalog size to the kernel.
func (b *MCPIntegrationBridge) markReady() {
	ch := b.readyChan()
	b.readyOnce.Do(func() {
		servers := b.manager.GetConnectedServers()
		toolCount := 0
		if b.store != nil {
			if tools, err := b.store.GetAllTools(context.Background()); err == nil {
				toolCount = len(tools)
			}
		}
		if b.facts != nil {
			b.facts.EmitReady(len(servers), toolCount)
		}
		logging.Get(logging.CategoryTools).Info(
			"MCP integration ready: %d connected server(s), %d tool(s) in catalog", len(servers), toolCount)
		close(ch)
	})
}

// Ready returns a channel closed once ConnectAll has finished connecting and
// discovering. Reading from a bridge before this fires can observe an empty
// tool catalog.
func (b *MCPIntegrationBridge) Ready() <-chan struct{} {
	return b.readyChan()
}

// WaitReady blocks until the bridge is ready or ctx is done.
func (b *MCPIntegrationBridge) WaitReady(ctx context.Context) error {
	select {
	case <-b.readyChan():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsReady reports readiness without blocking.
func (b *MCPIntegrationBridge) IsReady() bool {
	select {
	case <-b.readyChan():
		return true
	default:
		return false
	}
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
