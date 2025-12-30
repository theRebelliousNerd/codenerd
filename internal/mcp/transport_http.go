package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// HTTPTransport implements MCPTransport over HTTP.
type HTTPTransport struct {
	mu sync.RWMutex

	baseURL    string
	timeout    time.Duration
	client     *http.Client
	connected  bool
	serverInfo *MCPCapabilities
}

// NewHTTPTransport creates a new HTTP transport for MCP communication.
func NewHTTPTransport(baseURL string, timeout time.Duration) *HTTPTransport {
	return &HTTPTransport{
		baseURL: baseURL,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// mcpRequest represents a JSON-RPC style MCP request.
type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// mcpResponse represents a JSON-RPC style MCP response.
type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

// mcpError represents an error in MCP response.
type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Connect establishes connection to the MCP server.
func (t *HTTPTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Try to get capabilities to verify connection
	caps, err := t.getCapabilitiesLocked(ctx)
	if err != nil {
		t.connected = false
		return fmt.Errorf("failed to connect to MCP server at %s: %w", t.baseURL, err)
	}

	t.serverInfo = caps
	t.connected = true
	logging.Get(logging.CategoryTools).Info("MCP HTTP transport connected to %s", t.baseURL)
	return nil
}

// Disconnect closes the connection.
func (t *HTTPTransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false
	t.serverInfo = nil
	logging.Get(logging.CategoryTools).Info("MCP HTTP transport disconnected from %s", t.baseURL)
	return nil
}

// ListTools retrieves available tools from the server.
func (t *HTTPTransport) ListTools(ctx context.Context) ([]MCPToolSchema, error) {
	t.mu.RLock()
	if !t.connected {
		t.mu.RUnlock()
		return nil, fmt.Errorf("not connected to MCP server")
	}
	t.mu.RUnlock()

	resp, err := t.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	var result struct {
		Tools []MCPToolSchema `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools response: %w", err)
	}

	logging.Get(logging.CategoryTools).Debug("MCP server returned %d tools", len(result.Tools))
	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (t *HTTPTransport) CallTool(ctx context.Context, name string, args map[string]interface{}) (*MCPCallResult, error) {
	t.mu.RLock()
	if !t.connected {
		t.mu.RUnlock()
		return nil, fmt.Errorf("not connected to MCP server")
	}
	t.mu.RUnlock()

	start := time.Now()

	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	resp, err := t.call(ctx, "tools/call", params)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		return &MCPCallResult{
			Success:   false,
			Error:     err.Error(),
			LatencyMs: latencyMs,
		}, nil
	}

	if resp.Error != nil {
		return &MCPCallResult{
			Success:   false,
			Error:     resp.Error.Message,
			LatencyMs: latencyMs,
		}, nil
	}

	return &MCPCallResult{
		Success:   true,
		Output:    resp.Result,
		LatencyMs: latencyMs,
	}, nil
}

// GetCapabilities returns server capabilities.
func (t *HTTPTransport) GetCapabilities(ctx context.Context) (*MCPCapabilities, error) {
	t.mu.RLock()
	if t.serverInfo != nil {
		caps := *t.serverInfo
		t.mu.RUnlock()
		return &caps, nil
	}
	t.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()
	return t.getCapabilitiesLocked(ctx)
}

// getCapabilitiesLocked fetches capabilities (must hold lock).
func (t *HTTPTransport) getCapabilitiesLocked(ctx context.Context) (*MCPCapabilities, error) {
	resp, err := t.callLocked(ctx, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "codeNERD",
			"version": "1.0.0",
		},
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Capabilities MCPCapabilities `json:"capabilities"`
		ServerInfo   struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		// Try parsing as simpler structure
		var simple MCPCapabilities
		if err2 := json.Unmarshal(resp.Result, &simple); err2 != nil {
			return nil, fmt.Errorf("failed to parse capabilities: %w", err)
		}
		return &simple, nil
	}

	return &result.Capabilities, nil
}

// Ping checks if the server is responsive.
func (t *HTTPTransport) Ping(ctx context.Context) error {
	t.mu.RLock()
	if !t.connected {
		t.mu.RUnlock()
		return fmt.Errorf("not connected to MCP server")
	}
	t.mu.RUnlock()

	_, err := t.call(ctx, "ping", nil)
	if err != nil {
		// Try a simple HTTP GET as fallback
		req, err2 := http.NewRequestWithContext(ctx, "GET", t.baseURL+"/health", nil)
		if err2 != nil {
			return err
		}
		resp, err2 := t.client.Do(req)
		if err2 != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("server returned status %d", resp.StatusCode)
		}
	}
	return nil
}

// IsConnected returns current connection status.
func (t *HTTPTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// call makes a JSON-RPC call to the MCP server.
func (t *HTTPTransport) call(ctx context.Context, method string, params interface{}) (*mcpResponse, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.callLocked(ctx, method, params)
}

// callLocked makes a JSON-RPC call (must hold at least read lock).
func (t *HTTPTransport) callLocked(ctx context.Context, method string, params interface{}) (*mcpResponse, error) {
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var resp mcpResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.Error != nil {
		return &resp, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return &resp, nil
}

// Ensure HTTPTransport implements MCPTransport.
var _ MCPTransport = (*HTTPTransport)(nil)
