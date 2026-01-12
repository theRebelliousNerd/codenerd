package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// MCPClientManager manages connections to multiple MCP servers.
type MCPClientManager struct {
	mu sync.RWMutex

	servers   map[string]*MCPServerConnection
	store     *MCPToolStore
	analyzer  ToolAnalyzerInterface
	config    map[string]MCPServerConfig
	selection ToolSelectionConfig

	// Callbacks
	onToolDiscovered func(tool *MCPTool)
	onServerStatus   func(serverID string, status ServerStatus)
}

// MCPServerConnection holds the connection state for a single MCP server.
type MCPServerConnection struct {
	Server    *MCPServer
	Transport MCPTransport
	Tools     []*MCPTool
}

// ToolAnalyzerInterface defines the interface for tool analysis.
type ToolAnalyzerInterface interface {
	Analyze(ctx context.Context, schema MCPToolSchema) (*ToolAnalysis, error)
}

// NewMCPClientManager creates a new MCP client manager.
func NewMCPClientManager(store *MCPToolStore, analyzer ToolAnalyzerInterface, config map[string]MCPServerConfig) *MCPClientManager {
	return &MCPClientManager{
		servers:   make(map[string]*MCPServerConnection),
		store:     store,
		analyzer:  analyzer,
		config:    config,
		selection: DefaultToolSelectionConfig(),
	}
}

// SetToolSelectionConfig sets the tool selection configuration.
func (m *MCPClientManager) SetToolSelectionConfig(config ToolSelectionConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selection = config
}

// SetOnToolDiscovered sets the callback for when a new tool is discovered.
func (m *MCPClientManager) SetOnToolDiscovered(fn func(tool *MCPTool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onToolDiscovered = fn
}

// SetOnServerStatus sets the callback for server status changes.
func (m *MCPClientManager) SetOnServerStatus(fn func(serverID string, status ServerStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onServerStatus = fn
}

// ConnectAll connects to all configured servers with auto_connect=true.
func (m *MCPClientManager) ConnectAll(ctx context.Context) error {
	m.mu.RLock()
	configs := make([]MCPServerConfig, 0)
	for _, cfg := range m.config {
		if cfg.AutoConnect && cfg.Enabled {
			configs = append(configs, cfg)
		}
	}
	m.mu.RUnlock()

	var lastErr error
	for _, cfg := range configs {
		if err := m.Connect(ctx, cfg.ID); err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to connect to MCP server %s: %v", cfg.ID, err)
			lastErr = err
		}
	}
	return lastErr
}

// Connect establishes connection to a specific MCP server.
func (m *MCPClientManager) Connect(ctx context.Context, serverID string) error {
	m.mu.Lock()
	cfg, ok := m.config[serverID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown MCP server: %s", serverID)
	}

	// Check if already connected
	if conn, exists := m.servers[serverID]; exists && conn.Transport.IsConnected() {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	// Create transport based on protocol
	var transport MCPTransport
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		timeout = 30 * time.Second
	}

	switch Protocol(cfg.Protocol) {
	case ProtocolHTTP:
		transport = NewHTTPTransport(cfg.BaseURL, timeout)
	case ProtocolStdio:
		transport = NewStdioTransport(cfg.Endpoint)
	case ProtocolSSE:
		// TODO: Implement SSE transport
		return fmt.Errorf("SSE transport not yet implemented")
	default:
		return fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
	}

	// Connect
	m.updateServerStatus(serverID, ServerStatusConnecting)
	if err := transport.Connect(ctx); err != nil {
		m.updateServerStatus(serverID, ServerStatusError)
		return err
	}

	// Get capabilities
	caps, err := transport.GetCapabilities(ctx)
	if err != nil {
		logging.Get(logging.CategoryTools).Warn("Failed to get capabilities from %s: %v", serverID, err)
	}

	// Create server record
	server := &MCPServer{
		ID:            serverID,
		Name:          serverID, // Will be updated from server info
		Endpoint:      cfg.BaseURL,
		Protocol:      Protocol(cfg.Protocol),
		Status:        ServerStatusConnected,
		DiscoveredAt:  time.Now(),
		LastConnected: time.Now(),
	}
	if caps != nil {
		if caps.Tools {
			server.Capabilities = append(server.Capabilities, "tools")
		}
		if caps.Resources {
			server.Capabilities = append(server.Capabilities, "resources")
		}
		if caps.Prompts {
			server.Capabilities = append(server.Capabilities, "prompts")
		}
	}

	// Store connection
	conn := &MCPServerConnection{
		Server:    server,
		Transport: transport,
	}

	m.mu.Lock()
	m.servers[serverID] = conn
	m.mu.Unlock()

	m.updateServerStatus(serverID, ServerStatusConnected)

	// Persist server to store
	if m.store != nil {
		if err := m.store.SaveServer(ctx, server); err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to persist server %s: %v", serverID, err)
		}
	}

	// Discover tools if enabled
	if cfg.AutoDiscoverTools {
		go func() {
			if err := m.DiscoverTools(ctx, serverID); err != nil {
				logging.Get(logging.CategoryTools).Warn("Failed to discover tools from %s: %v", serverID, err)
			}
		}()
	}

	logging.Get(logging.CategoryTools).Info("Connected to MCP server %s at %s", serverID, cfg.BaseURL)
	return nil
}

// Disconnect closes connection to a specific MCP server.
func (m *MCPClientManager) Disconnect(serverID string) error {
	m.mu.Lock()
	conn, ok := m.servers[serverID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("server not connected: %s", serverID)
	}
	delete(m.servers, serverID)
	m.mu.Unlock()

	if err := conn.Transport.Disconnect(); err != nil {
		return err
	}

	m.updateServerStatus(serverID, ServerStatusDisconnected)
	logging.Get(logging.CategoryTools).Info("Disconnected from MCP server %s", serverID)
	return nil
}

// DisconnectAll closes all server connections.
func (m *MCPClientManager) DisconnectAll() {
	m.mu.Lock()
	servers := make([]string, 0, len(m.servers))
	for id := range m.servers {
		servers = append(servers, id)
	}
	m.mu.Unlock()

	for _, id := range servers {
		if err := m.Disconnect(id); err != nil {
			logging.Get(logging.CategoryTools).Warn("Error disconnecting from %s: %v", id, err)
		}
	}
}

// DiscoverTools discovers and analyzes tools from an MCP server.
func (m *MCPClientManager) DiscoverTools(ctx context.Context, serverID string) error {
	m.mu.RLock()
	conn, ok := m.servers[serverID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("server not connected: %s", serverID)
	}
	m.mu.RUnlock()

	// List tools from server
	schemas, err := conn.Transport.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	logging.Get(logging.CategoryTools).Info("Discovered %d tools from %s", len(schemas), serverID)

	// Process each tool
	tools := make([]*MCPTool, 0, len(schemas))
	for _, schema := range schemas {
		tool, err := m.processToolSchema(ctx, serverID, schema)
		if err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to process tool %s: %v", schema.Name, err)
			continue
		}
		tools = append(tools, tool)

		// Notify callback
		m.mu.RLock()
		cb := m.onToolDiscovered
		m.mu.RUnlock()
		if cb != nil {
			cb(tool)
		}
	}

	// Update connection's tool cache
	m.mu.Lock()
	if conn, ok := m.servers[serverID]; ok {
		conn.Tools = tools
	}
	m.mu.Unlock()

	return nil
}

// processToolSchema processes a tool schema, checking cache and analyzing if new.
func (m *MCPClientManager) processToolSchema(ctx context.Context, serverID string, schema MCPToolSchema) (*MCPTool, error) {
	toolID := fmt.Sprintf("%s/%s", serverID, schema.Name)

	// Check if already analyzed
	if m.store != nil {
		existing, err := m.store.GetTool(ctx, toolID)
		if err == nil && existing != nil && !existing.AnalyzedAt.IsZero() {
			logging.Get(logging.CategoryTools).Debug("Tool %s already analyzed, using cached", toolID)
			return existing, nil
		}
	}

	// Create base tool
	tool := &MCPTool{
		ToolID:       toolID,
		ServerID:     serverID,
		Name:         schema.Name,
		Description:  schema.Description,
		InputSchema:  schema.InputSchema,
		OutputSchema: schema.OutputSchema,
		RegisteredAt: time.Now(),
	}

	// Analyze with LLM if analyzer is available
	if m.analyzer != nil {
		analysis, err := m.analyzer.Analyze(ctx, schema)
		if err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to analyze tool %s: %v", toolID, err)
		} else {
			tool.Categories = analysis.Categories
			tool.Capabilities = analysis.Capabilities
			tool.Domain = analysis.Domain
			tool.ShardAffinities = analysis.ShardAffinities
			tool.UseCases = analysis.UseCases
			tool.Condensed = analysis.Condensed
			tool.Embedding = analysis.Embedding
			tool.AnalyzedAt = time.Now()
		}
	}

	// Set default condensed if not set
	if tool.Condensed == "" && tool.Description != "" {
		tool.Condensed = truncate(tool.Description, 80)
	}

	// Persist to store
	if m.store != nil {
		if err := m.store.SaveTool(ctx, tool); err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to persist tool %s: %v", toolID, err)
		}
	}

	return tool, nil
}

// CallTool invokes a tool on an MCP server.
func (m *MCPClientManager) CallTool(ctx context.Context, toolID string, args map[string]interface{}) (*MCPCallResult, error) {
	// Parse tool ID to get server and tool name
	serverID, toolName := parseToolID(toolID)
	if serverID == "" {
		return nil, fmt.Errorf("invalid tool ID: %s", toolID)
	}

	m.mu.RLock()
	conn, ok := m.servers[serverID]
	m.mu.RUnlock()

	if !ok || !conn.Transport.IsConnected() {
		// Return cached offline status
		return &MCPCallResult{
			Success: false,
			Error:   fmt.Sprintf("MCP server %s is not connected", serverID),
		}, nil
	}

	result, err := conn.Transport.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, err
	}

	// Update usage stats
	if m.store != nil {
		go func() {
			if err := m.store.RecordToolUsage(context.Background(), toolID, result.Success, result.LatencyMs); err != nil {
				logging.Get(logging.CategoryTools).Debug("Failed to record tool usage: %v", err)
			}
		}()
	}

	return result, nil
}

// GetServer returns the connection for a specific server.
func (m *MCPClientManager) GetServer(serverID string) (*MCPServerConnection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conn, ok := m.servers[serverID]
	return conn, ok
}

// GetConnectedServers returns a list of connected server IDs.
func (m *MCPClientManager) GetConnectedServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0, len(m.servers))
	for id, conn := range m.servers {
		if conn.Transport.IsConnected() {
			result = append(result, id)
		}
	}
	return result
}

// GetAllTools returns all tools from all connected servers.
func (m *MCPClientManager) GetAllTools() []*MCPTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []*MCPTool
	for _, conn := range m.servers {
		tools = append(tools, conn.Tools...)
	}
	return tools
}

// ListTools returns cached tool schemas across all connected servers.
func (m *MCPClientManager) ListTools(ctx context.Context) ([]MCPToolSchema, error) {
	_ = ctx

	m.mu.RLock()
	defer m.mu.RUnlock()

	schemas := make([]MCPToolSchema, 0)
	for _, conn := range m.servers {
		for _, tool := range conn.Tools {
			schemas = append(schemas, MCPToolSchema{
				Name:         tool.Name,
				Description:  tool.Description,
				InputSchema:  tool.InputSchema,
				OutputSchema: tool.OutputSchema,
			})
		}
	}

	if len(schemas) == 0 {
		return nil, fmt.Errorf("no tools cached")
	}

	return schemas, nil
}

// updateServerStatus updates server status and notifies callback.
func (m *MCPClientManager) updateServerStatus(serverID string, status ServerStatus) {
	m.mu.RLock()
	cb := m.onServerStatus
	m.mu.RUnlock()

	if cb != nil {
		cb(serverID, status)
	}

	// Update store
	if m.store != nil {
		go func() {
			if err := m.store.UpdateServerStatus(context.Background(), serverID, status); err != nil {
				logging.Get(logging.CategoryTools).Debug("Failed to update server status: %v", err)
			}
		}()
	}
}

// parseToolID parses a tool ID into server ID and tool name.
func parseToolID(toolID string) (serverID, toolName string) {
	for i := len(toolID) - 1; i >= 0; i-- {
		if toolID[i] == '/' {
			return toolID[:i], toolID[i+1:]
		}
	}
	return "", toolID
}

// truncate truncates a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
