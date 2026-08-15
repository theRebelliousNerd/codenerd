package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
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
	facts     *FactEmitter

	// readiness tracks in-flight initial discovery so callers can wait for the
	// catalog to exist instead of racing an empty store.
	discovering sync.WaitGroup

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

// SetFactEmitter installs the kernel fact emitter. Server and tool state is
// mirrored into Mangle only when this is set; without it the MCP predicates
// stay empty and policy_mcp.mg cannot decide anything.
func (m *MCPClientManager) SetFactEmitter(emitter *FactEmitter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.facts = emitter
}

// factEmitter returns the emitter under the read lock.
func (m *MCPClientManager) factEmitter() *FactEmitter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.facts
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

// WaitForDiscovery blocks until every initial discovery goroutine started by
// Connect has finished, or ctx is done. Discovery runs detached from Connect,
// so without this a caller that compiles a tool set right after ConnectAll
// races an empty catalog.
func (m *MCPClientManager) WaitForDiscovery(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		m.discovering.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Connect establishes connection to a specific MCP server.
func (m *MCPClientManager) Connect(ctx context.Context, serverID string) error {
	if serverID == "" {
		return fmt.Errorf("server ID cannot be empty")
	}

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
	if err != nil || timeout <= 0 {
		// Fall back to default for: parse errors, empty string ("0s" parses
		// fine but produces a useless zero/negative timeout), and explicit
		// non-positive values like "-1s".
		timeout = 30 * time.Second
	}

	// Reject explicitly empty protocol — switch below would also reject it,
	// but this surfaces a clearer error message.
	if cfg.Protocol == "" {
		return fmt.Errorf("protocol cannot be empty for server: %s", serverID)
	}

	switch Protocol(cfg.Protocol) {
	case ProtocolHTTP:
		transport = NewHTTPTransportWithHeaders(cfg.BaseURL, timeout, cfg.Headers)
	case ProtocolStdio:
		// Headers are HTTP-specific; a stdio server is configured through its
		// command line and inherited environment instead.
		transport = NewStdioTransport(cfg.Endpoint)
	case ProtocolSSE:
		transport = NewSSETransportWithHeaders(cfg.BaseURL, timeout, cfg.Headers)
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

	// Publish the server to the kernel before status, so the availability rule
	// in policy_mcp.mg sees a registration to join against.
	m.factEmitter().EmitServer(server)

	m.updateServerStatus(serverID, ServerStatusConnected)

	// Persist server to store
	if m.store != nil {
		if err := m.store.SaveServer(ctx, server); err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to persist server %s: %v", serverID, err)
		}
	}

	// Discover tools if enabled
	if cfg.AutoDiscoverTools {
		m.discovering.Add(1)
		go func() {
			defer m.discovering.Done()
			defer func() {
				if r := recover(); r != nil {
					logging.Get(logging.CategoryTools).Error("Panic in DiscoverTools background goroutine: %v", r)
				}
			}()
			// Use context.Background() to prevent premature cancellation from Connect's context lifecycle
			if err := m.DiscoverTools(context.Background(), serverID); err != nil {
				logging.Get(logging.CategoryTools).Warn("Failed to discover tools from %s: %v", serverID, err)
			}
		}()
	}

	logging.Get(logging.CategoryTools).Info("Connected to MCP server %s at %s", serverID, cfg.BaseURL)
	return nil
}

// Disconnect closes connection to a specific MCP server.
func (m *MCPClientManager) Disconnect(serverID string) error {
	if serverID == "" {
		return fmt.Errorf("server ID cannot be empty")
	}

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
	if len(schemas) == 0 {
		return nil
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
	previous := make([]*MCPTool, 0)
	if conn, ok := m.servers[serverID]; ok {
		previous = append(previous, conn.Tools...)
		conn.Tools = tools
	}
	m.mu.Unlock()

	// A tool the server no longer advertises must lose its facts, otherwise
	// the kernel keeps recommending a call that will fail at the transport.
	if emitter := m.factEmitter(); emitter != nil {
		live := make(map[string]struct{}, len(tools))
		for _, tool := range tools {
			live[tool.ToolID] = struct{}{}
		}
		for _, old := range previous {
			if old == nil {
				continue
			}
			if _, ok := live[old.ToolID]; !ok {
				emitter.RetractTool(old.ToolID)
			}
		}
	}

	return nil
}

// processToolSchema processes a tool schema, checking cache and analyzing if new.
func (m *MCPClientManager) processToolSchema(ctx context.Context, serverID string, schema MCPToolSchema) (*MCPTool, error) {
	toolID := fmt.Sprintf("%s/%s", serverID, schema.Name)
	schemaHash := ToolSchemaHash(schema)

	// Check if already analyzed
	if m.store != nil {
		existing, err := m.store.GetTool(ctx, toolID)
		if err == nil && existing != nil && !existing.AnalyzedAt.IsZero() {
			// Cached analysis is only valid for the schema it was derived from.
			// A server that changes a tool's parameters or description without
			// renaming it would otherwise keep the stale categories,
			// capabilities and embedding forever.
			if existing.SchemaHash == "" || existing.SchemaHash == schemaHash {
				logging.Get(logging.CategoryTools).Debug("Tool %s already analyzed, using cached", toolID)
				if existing.SchemaHash == "" {
					// Backfill for rows written before schema hashing existed.
					existing.SchemaHash = schemaHash
					if err := m.store.SaveTool(ctx, existing); err != nil {
						logging.Get(logging.CategoryTools).Debug("Failed to backfill schema hash for %s: %v", toolID, err)
					}
				}
				m.factEmitter().EmitTool(existing)
				return existing, nil
			}
			logging.Get(logging.CategoryTools).Info("Tool %s schema changed, re-analyzing", toolID)
			m.factEmitter().RetractTool(toolID)
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
		SchemaHash:   schemaHash,
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

	m.factEmitter().EmitTool(tool)

	return tool, nil
}

// CallTool invokes a tool on an MCP server.
func (m *MCPClientManager) CallTool(ctx context.Context, toolID string, args map[string]any) (*MCPCallResult, error) {
	if args == nil {
		args = make(map[string]any)
	}

	// Deep copy/clone arguments map before transport call to prevent map race conditions
	clonedArgs := make(map[string]any, len(args))
	maps.Copy(clonedArgs, args)

	// Ensure args are serializable to prevent transport panic/error later
	if _, err := json.Marshal(clonedArgs); err != nil {
		return nil, fmt.Errorf("invalid arguments: cannot serialize to JSON: %w", err)
	}

	serverID, toolName := parseToolID(toolID)
	if serverID == "" {
		return nil, fmt.Errorf("invalid tool ID: %s", toolID)
	}

	// Sanitize MCP tool names against directory traversal
	if strings.Contains(toolName, "..") || strings.ContainsAny(toolName, "/\\") {
		return nil, fmt.Errorf("invalid tool name: directory traversal detected")
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

	result, err := conn.Transport.CallTool(ctx, toolName, clonedArgs)
	if err != nil {
		// Map unhandled protocol errors cleanly
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("MCP protocol error: tool execution timed out: %w", err)
		}
		if err == context.Canceled {
			return nil, fmt.Errorf("MCP protocol error: tool execution canceled: %w", err)
		}
		return nil, fmt.Errorf("MCP protocol error: %w", err)
	}

	// Cap MCP context memory windows during multi-turn exchanges
	const maxContextWindowBytes = 500 * 1024
	if len(result.Output) > maxContextWindowBytes {
		truncMsg := []byte("\n...[output truncated due to MCP context memory window limit]")
		result.Output = append(result.Output[:maxContextWindowBytes], truncMsg...)
	}

	// Update usage stats defensively with nil checks and panic recovery
	if m.store != nil && result != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logging.Get(logging.CategoryTools).Error("Panic in RecordToolUsage background goroutine: %v", r)
				}
			}()
			if err := m.store.RecordToolUsage(context.Background(), toolID, result.Success, result.LatencyMs); err != nil {
				// Recoverable persistence failure — promote to Warn so it's
				// visible by default. Tool-usage telemetry powers later
				// affinity scoring; silent loss skews the model.
				logging.Get(logging.CategoryTools).Warn("Failed to record tool usage tool=%s: %v", toolID, err)
				return
			}
			// Re-publish counters so the kernel's success-rate boost and
			// slow-tool penalty see the call that just happened.
			if emitter := m.factEmitter(); emitter != nil {
				if updated, err := m.store.GetTool(context.Background(), toolID); err == nil && updated != nil {
					emitter.EmitToolUsage(updated)
				}
			}
		}()
	}

	return result, nil
}

// DiscoverResources lists a server's resources and publishes them to the
// kernel. Resource availability is a planning input — "is there a resource that
// already answers this?" is a question the executive should be able to ask
// before it spends a tool call.
func (m *MCPClientManager) DiscoverResources(ctx context.Context, serverID string) ([]MCPResource, error) {
	transport, err := m.transportFor(serverID)
	if err != nil {
		return nil, err
	}
	provider, ok := transport.(ResourceCapableTransport)
	if !ok {
		return nil, fmt.Errorf("transport for %s does not support resources", serverID)
	}

	resources, err := provider.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	m.factEmitter().EmitResources(serverID, resources)
	logging.Get(logging.CategoryTools).Info("Discovered %d resources from %s", len(resources), serverID)
	return resources, nil
}

// ReadResource fetches one resource's contents from a server.
func (m *MCPClientManager) ReadResource(ctx context.Context, serverID, uri string) ([]MCPResourceContent, error) {
	transport, err := m.transportFor(serverID)
	if err != nil {
		return nil, err
	}
	provider, ok := transport.(ResourceCapableTransport)
	if !ok {
		return nil, fmt.Errorf("transport for %s does not support resources", serverID)
	}
	return provider.ReadResource(ctx, uri)
}

// DiscoverPrompts lists a server's prompt templates and publishes them.
func (m *MCPClientManager) DiscoverPrompts(ctx context.Context, serverID string) ([]MCPPrompt, error) {
	transport, err := m.transportFor(serverID)
	if err != nil {
		return nil, err
	}
	provider, ok := transport.(PromptCapableTransport)
	if !ok {
		return nil, fmt.Errorf("transport for %s does not support prompts", serverID)
	}

	prompts, err := provider.ListPrompts(ctx)
	if err != nil {
		return nil, err
	}
	m.factEmitter().EmitPrompts(serverID, prompts)
	logging.Get(logging.CategoryTools).Info("Discovered %d prompts from %s", len(prompts), serverID)
	return prompts, nil
}

// GetPrompt renders a server-side prompt template.
func (m *MCPClientManager) GetPrompt(ctx context.Context, serverID, name string, args map[string]string) ([]MCPPromptMessage, error) {
	transport, err := m.transportFor(serverID)
	if err != nil {
		return nil, err
	}
	provider, ok := transport.(PromptCapableTransport)
	if !ok {
		return nil, fmt.Errorf("transport for %s does not support prompts", serverID)
	}
	return provider.GetPrompt(ctx, name, args)
}

// transportFor returns the live transport for a connected server.
func (m *MCPClientManager) transportFor(serverID string) (MCPTransport, error) {
	if serverID == "" {
		return nil, fmt.Errorf("server ID cannot be empty")
	}
	m.mu.RLock()
	conn, ok := m.servers[serverID]
	m.mu.RUnlock()
	if !ok || conn.Transport == nil || !conn.Transport.IsConnected() {
		return nil, fmt.Errorf("server not connected: %s", serverID)
	}
	return conn.Transport, nil
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
	var allTools []*MCPTool
	for _, conn := range m.servers {
		allTools = append(allTools, conn.Tools...)
	}
	m.mu.RUnlock()

	schemas := make([]MCPToolSchema, 0, len(allTools))
	for _, tool := range allTools {
		schemas = append(schemas, MCPToolSchema{
			Name:         tool.Name,
			Description:  tool.Description,
			InputSchema:  tool.InputSchema,
			OutputSchema: tool.OutputSchema,
		})
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
	emitter := m.facts
	m.mu.RUnlock()

	// Availability in policy_mcp.mg keys off mcp_server_status, so this is the
	// fact that has to move on every transition — including disconnect.
	emitter.EmitServerStatus(serverID, status)

	if cb != nil {
		cb(serverID, status)
	}

	// Update store
	if m.store != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logging.Get(logging.CategoryTools).Error("Panic in UpdateServerStatus background goroutine: %v", r)
				}
			}()
			if err := m.store.UpdateServerStatus(context.Background(), serverID, status); err != nil {
				// Server-status drift makes degraded MCP servers invisible
				// in the registry; surface at Warn so it shows up by default.
				logging.Get(logging.CategoryTools).Warn("Failed to update server status server=%s: %v", serverID, err)
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
