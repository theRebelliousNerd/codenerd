package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
)

// MCPClientManager manages connections to multiple MCP servers.
type MCPClientManager struct {
	mu              sync.RWMutex
	authorityMu     sync.Mutex
	reviewedEffects map[string]ReviewedToolEffect
	resources       map[string][]MCPResource
	prompts         map[string][]MCPPrompt

	servers  map[string]*MCPServerConnection
	store    *MCPToolStore
	analyzer ToolAnalyzerInterface
	config   map[string]MCPServerConfig
	facts    *FactEmitter

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
		servers:  make(map[string]*MCPServerConnection),
		store:    store,
		analyzer: analyzer,
		config:   config,
	}
}

// SetFactEmitter installs the kernel fact emitter. Server and tool state is
// mirrored into Mangle only when this is set; without it the MCP predicates
// stay empty and policy_mcp.mg cannot decide anything.
func (m *MCPClientManager) SetFactEmitter(emitter *FactEmitter) {
	m.authorityMu.Lock()
	defer m.authorityMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.facts = emitter
	m.reviewedEffects = make(map[string]ReviewedToolEffect)
}

// factEmitter returns the emitter under the read lock.
func (m *MCPClientManager) factEmitter() *FactEmitter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.facts
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
		return m.factEmitter().Error()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// transportTimeoutFallbackNanos is the transport timeout used when a
// server's configured timeout is missing, unparseable or non-positive.
// It is installed from integrations.default_timeout by LoadUserConfig;
// the 30s here is the last resort for a process that never loaded a
// config (direct API use, unit tests), because this package cannot
// import internal/config back. The parity test in internal/config pins
// it to that default.
var transportTimeoutFallbackNanos atomic.Int64

func init() {
	transportTimeoutFallbackNanos.Store(int64(30 * time.Second))
}

// SetTransportTimeoutFallback installs the transport fallback timeout.
// A non-positive value resets the 30s last resort. LoadUserConfig is
// the production caller; tests install and restore.
func SetTransportTimeoutFallback(d time.Duration) {
	if d <= 0 {
		d = 30 * time.Second
	}
	transportTimeoutFallbackNanos.Store(int64(d))
}

// DefaultTransportTimeout is the installed transport fallback timeout.
func DefaultTransportTimeout() time.Duration {
	if d := time.Duration(transportTimeoutFallbackNanos.Load()); d > 0 {
		return d
	}
	return 30 * time.Second
}

// resolveTransportTimeout parses one server's configured timeout,
// falling back to the installed default for: parse errors, empty
// strings ("0s" parses fine but produces a useless zero timeout), and
// explicit non-positive values like "-1s".
func resolveTransportTimeout(raw string) time.Duration {
	if timeout, err := time.ParseDuration(raw); err == nil && timeout > 0 {
		return timeout
	}
	return DefaultTransportTimeout()
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
	timeout := resolveTransportTimeout(cfg.Timeout)

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

	m.authorityMu.Lock()
	m.mu.Lock()
	m.servers[serverID] = conn
	delete(m.resources, serverID)
	delete(m.prompts, serverID)
	for toolID := range m.reviewedEffects {
		if boundServer, _ := parseToolID(toolID); boundServer == serverID {
			delete(m.reviewedEffects, toolID)
		}
	}
	m.mu.Unlock()
	m.authorityMu.Unlock()

	// Publish the server to the kernel before status, so the availability rule
	// in policy_mcp.mg sees a registration to join against.
	m.factEmitter().EmitServer(server)
	if err := m.factEmitter().Error(); err != nil {
		m.mu.Lock()
		delete(m.servers, serverID)
		m.mu.Unlock()
		return errors.Join(fmt.Errorf("publish MCP server %s: %w", serverID, err), transport.Disconnect())
	}

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

	m.authorityMu.Lock()
	m.mu.Lock()
	conn, ok := m.servers[serverID]
	if !ok {
		m.mu.Unlock()
		m.authorityMu.Unlock()
		return fmt.Errorf("server not connected: %s", serverID)
	}
	delete(m.servers, serverID)
	delete(m.resources, serverID)
	delete(m.prompts, serverID)
	for toolID := range m.reviewedEffects {
		if boundServer, _ := parseToolID(toolID); boundServer == serverID {
			delete(m.reviewedEffects, toolID)
		}
	}
	m.mu.Unlock()
	m.authorityMu.Unlock()

	disconnectErr := conn.Transport.Disconnect()
	m.updateServerStatus(serverID, ServerStatusDisconnected)
	logging.Get(logging.CategoryTools).Info("Disconnected from MCP server %s", serverID)
	return errors.Join(disconnectErr, m.factEmitter().Error())
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
		m.invalidateRemoteCatalog(RemoteTool, serverID)
		return fmt.Errorf("failed to list tools: %w", err)
	}
	if len(schemas) == 0 {
		m.invalidateRemoteCatalog(RemoteTool, serverID)
		return m.factEmitter().Error()
	}
	if err := m.beginRemoteToolRefresh(serverID, conn, schemas); err != nil {
		return err
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
	if current, ok := m.servers[serverID]; ok && current == conn {
		previous = append(previous, current.Tools...)
		current.Tools = tools
	} else {
		m.mu.Unlock()
		return fmt.Errorf("MCP connection changed during discovery")
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

	if err := m.factEmitter().Error(); err != nil {
		return fmt.Errorf("publish MCP discovery for %s: %w", serverID, err)
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
				if err := m.factEmitter().Error(); err != nil {
					return nil, fmt.Errorf("publish cached MCP tool %s: %w", toolID, err)
				}
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
	var analysis *ToolAnalysis
	if m.analyzer != nil {
		var err error
		analysis, err = m.analyzer.Analyze(ctx, schema)
		if err != nil || analysis == nil {
			if err == nil {
				err = fmt.Errorf("analyzer returned no metadata")
			}
			logging.Get(logging.CategoryTools).Warn("Failed to analyze tool %s: %v", toolID, err)
			analysis = nil
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

	// Default condensed is the whole description. An 80-rune cut reads as a
	// complete summary while hiding the rest with no way to recall it; the
	// compiler already budgets secondary tools by tier and count, so the
	// text itself stays intact.
	if tool.Condensed == "" && tool.Description != "" {
		tool.Condensed = tool.Description
	}

	// Classify for the control plane. This deliberately runs on every tool,
	// including one whose LLM analysis failed or was never configured: the
	// atlas and the risk gate are what an agent navigates by, and a server that
	// arrives unclassified is a server the agent cannot see. Classification
	// degrades — annotations, then name, then schema — it does not vanish.
	classification := ClassifyTool(schema, analysis)
	tool.Facet = classification.Facet
	tool.Risk = classification.Risk
	tool.FacetSource = classification.FacetSource
	tool.RiskSource = classification.RiskSource
	tool.Annotations = schema.Annotations

	// Persist to store
	if m.store != nil {
		if err := m.store.SaveTool(ctx, tool); err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to persist tool %s: %v", toolID, err)
		}
	}

	m.factEmitter().EmitTool(tool)
	if err := m.factEmitter().Error(); err != nil {
		return nil, fmt.Errorf("publish MCP tool %s: %w", toolID, err)
	}

	return tool, nil
}

// CallTool invokes a tool on an MCP server.
func (m *MCPClientManager) CallTool(ctx context.Context, toolID string, args map[string]any) (*MCPCallResult, error) {
	canonical, frozen, err := canonicalRemoteArgs(args)
	if err != nil {
		return nil, err
	}

	serverID, toolName := parseToolID(toolID)
	if serverID == "" {
		return nil, fmt.Errorf("invalid tool ID: %s", toolID)
	}

	// Sanitize MCP tool names against directory traversal
	if toolName == "" || strings.Contains(toolName, "..") || strings.ContainsAny(toolName, "/\\") {
		return nil, fmt.Errorf("invalid tool name: directory traversal detected")
	}

	m.mu.RLock()
	conn, ok := m.servers[serverID]
	m.mu.RUnlock()

	if !ok || conn == nil || conn.Transport == nil || !conn.Transport.IsConnected() {
		// Return cached offline status
		return &MCPCallResult{
			Success: false,
			Error:   fmt.Sprintf("MCP server %s is not connected", serverID),
		}, nil
	}

	result, err := m.dispatchRemoteCall(ctx, RemoteTool, serverID, toolID, args, canonical, frozen, func(transport MCPTransport, outbound map[string]any) (*MCPCallResult, error) {
		return transport.CallTool(ctx, toolName, outbound)
	})
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

	// The output is returned whole. It used to be cut at 500 KiB here with a
	// marker, which lost the tail of exactly the large results (query dumps,
	// document fetches) a model asks an external tool for; the working
	// context archives every tool result in full and pages it, so size is
	// managed by selection, never by cutting.

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
	m.invalidateRemoteCatalog(RemoteResource, serverID)
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
	m.mu.Lock()
	if connection := m.servers[serverID]; connection == nil || connection.Transport != transport {
		m.mu.Unlock()
		return nil, fmt.Errorf("MCP connection changed during resource discovery")
	}
	if m.resources == nil {
		m.resources = make(map[string][]MCPResource)
	}
	m.resources[serverID] = append([]MCPResource(nil), resources...)
	m.mu.Unlock()
	if err := m.factEmitter().Error(); err != nil {
		return nil, fmt.Errorf("publish MCP resources: %w", err)
	}
	logging.Get(logging.CategoryTools).Info("Discovered %d resources from %s", len(resources), serverID)
	return resources, nil
}

// ReadResource fetches one resource's contents from a server.
func (m *MCPClientManager) ReadResource(ctx context.Context, serverID, uri string) ([]MCPResourceContent, error) {
	args := map[string]any{"uri": uri}
	canonical, frozen, err := canonicalRemoteArgs(args)
	if err != nil {
		return nil, err
	}
	subject := ResourceAuthorityTool(serverID, MCPResource{URI: uri})
	result, err := m.dispatchRemoteCall(ctx, RemoteResource, serverID, subject.ToolID, args, canonical, frozen, func(transport MCPTransport, outbound map[string]any) (*MCPCallResult, error) {
		provider, supported := transport.(ResourceCapableTransport)
		if !supported {
			return nil, fmt.Errorf("transport for %s does not support resources", serverID)
		}
		contents, callErr := provider.ReadResource(ctx, outbound["uri"].(string))
		if callErr != nil {
			return nil, callErr
		}
		encoded, encodeErr := json.Marshal(contents)
		return &MCPCallResult{Success: encodeErr == nil, Output: encoded}, encodeErr
	})
	if err != nil {
		return nil, err
	}
	var contents []MCPResourceContent
	err = json.Unmarshal(result.Output, &contents)
	return contents, err
}

// DiscoverPrompts lists a server's prompt templates and publishes them.
func (m *MCPClientManager) DiscoverPrompts(ctx context.Context, serverID string) ([]MCPPrompt, error) {
	m.invalidateRemoteCatalog(RemotePrompt, serverID)
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
	m.mu.Lock()
	if connection := m.servers[serverID]; connection == nil || connection.Transport != transport {
		m.mu.Unlock()
		return nil, fmt.Errorf("MCP connection changed during prompt discovery")
	}
	if m.prompts == nil {
		m.prompts = make(map[string][]MCPPrompt)
	}
	m.prompts[serverID] = append([]MCPPrompt(nil), prompts...)
	m.mu.Unlock()
	if err := m.factEmitter().Error(); err != nil {
		return nil, fmt.Errorf("publish MCP prompts: %w", err)
	}
	logging.Get(logging.CategoryTools).Info("Discovered %d prompts from %s", len(prompts), serverID)
	return prompts, nil
}

// GetPrompt renders a server-side prompt template.
func (m *MCPClientManager) GetPrompt(ctx context.Context, serverID, name string, args map[string]string) ([]MCPPromptMessage, error) {
	if args == nil {
		args = map[string]string{}
	}
	request := map[string]any{"name": name, "arguments": args}
	canonical, frozen, err := canonicalRemoteArgs(request)
	if err != nil {
		return nil, err
	}
	subject := PromptAuthorityTool(serverID, MCPPrompt{Name: name})
	result, err := m.dispatchRemoteCall(ctx, RemotePrompt, serverID, subject.ToolID, request, canonical, frozen, func(transport MCPTransport, outbound map[string]any) (*MCPCallResult, error) {
		provider, supported := transport.(PromptCapableTransport)
		if !supported {
			return nil, fmt.Errorf("transport for %s does not support prompts", serverID)
		}
		promptArgs := make(map[string]string)
		for key, value := range outbound["arguments"].(map[string]any) {
			promptArgs[key] = value.(string)
		}
		messages, callErr := provider.GetPrompt(ctx, outbound["name"].(string), promptArgs)
		if callErr != nil {
			return nil, callErr
		}
		encoded, encodeErr := json.Marshal(messages)
		return &MCPCallResult{Success: encodeErr == nil, Output: encoded}, encodeErr
	})
	if err != nil {
		return nil, err
	}
	var messages []MCPPromptMessage
	err = json.Unmarshal(result.Output, &messages)
	return messages, err
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
