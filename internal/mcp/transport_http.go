package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
)

// HTTPTransport implements MCPTransport over a single HTTP endpoint: every
// JSON-RPC request is a POST, and the response is either one JSON document or a
// request-scoped event stream carrying it. It speaks the 2024-11-05
// initialization profile only; it neither sends nor honors the later
// Mcp-Session-Id lifecycle, and a server that requires it is refused at
// initialize rather than half-spoken to.
type HTTPTransport struct {
	mu sync.RWMutex

	baseURL    string
	timeout    time.Duration
	client     *http.Client
	connected  bool
	serverInfo *MCPCapabilities
	headers    map[string]string

	// nextID issues request IDs that are never reused for the life of the
	// transport. The 2024-11-05 revision forbids reuse within a session, even
	// for sequential completed calls.
	nextID atomic.Int64
}

// NewHTTPTransportWithHeaders creates an HTTP transport that attaches static
// headers (auth tokens, tenant IDs) to every request. Header values are
// environment-expanded at construction time so a secret never has to be stored
// in the workspace config.
func NewHTTPTransportWithHeaders(baseURL string, timeout time.Duration, headers map[string]string) *HTTPTransport {
	return &HTTPTransport{
		baseURL: baseURL,
		timeout: timeout,
		client: &http.Client{
			Timeout:       timeout,
			CheckRedirect: refuseCrossHostRedirect,
		},
		headers: ExpandHeaderValues(headers),
	}
}

// mcpRequest represents a JSON-RPC style MCP request.
type mcpRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitzero"`
}

// mcpResponse represents a JSON-RPC style MCP response.
type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitzero"`
	Error   *mcpError       `json:"error,omitzero"`
}

// mcpError represents an error in MCP response.
type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitzero"`
}

// Connect performs the initialization handshake: initialize, version and
// capability validation, then the initialized notification. Only after all of
// it does the transport report connected.
func (t *HTTPTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	caps, err := t.initializeLocked(ctx)
	if err != nil {
		t.connected = false
		t.serverInfo = nil
		return fmt.Errorf("failed to connect to MCP server at %s: %w", t.baseURL, err)
	}

	t.serverInfo = caps
	t.connected = true
	logging.Get(logging.CategoryTools).Info("MCP HTTP transport connected to %s (protocol %s)", t.baseURL, caps.ProtocolVersion)
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

// ListTools retrieves every page of the server's tools.
func (t *HTTPTransport) ListTools(ctx context.Context) ([]MCPToolSchema, error) {
	t.mu.RLock()
	if !t.connected {
		t.mu.RUnlock()
		return nil, fmt.Errorf("not connected to MCP server")
	}
	t.mu.RUnlock()

	tools, err := listToolsVia(ctx, t)
	if err == nil {
		logging.Get(logging.CategoryTools).Debug("MCP server returned %d tools", len(tools))
	}
	return tools, err
}

// CallTool invokes a tool on the MCP server.
func (t *HTTPTransport) CallTool(ctx context.Context, name string, args map[string]any) (*MCPCallResult, error) {
	t.mu.RLock()
	if !t.connected {
		t.mu.RUnlock()
		return nil, fmt.Errorf("not connected to MCP server")
	}
	t.mu.RUnlock()

	start := time.Now()

	params := map[string]any{
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
	return callResultFromRPC(resp, latencyMs), nil
}

// GetCapabilities returns the capabilities observed during Connect.
func (t *HTTPTransport) GetCapabilities(ctx context.Context) (*MCPCapabilities, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.serverInfo == nil {
		return nil, fmt.Errorf("not connected to MCP server")
	}
	caps := *t.serverInfo
	return &caps, nil
}

// initializeLocked runs the handshake (must hold the write lock).
func (t *HTTPTransport) initializeLocked(ctx context.Context) (*MCPCapabilities, error) {
	resp, err := t.callLocked(ctx, "initialize", initializeParams())
	if err != nil {
		return nil, err
	}
	caps, err := parseInitializeResult(resp.Result)
	if err != nil {
		return nil, err
	}
	if err := t.notifyLocked(ctx, initializedNotification()); err != nil {
		return nil, fmt.Errorf("initialized notification: %w", err)
	}
	return caps, nil
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
func (t *HTTPTransport) call(ctx context.Context, method string, params any) (*mcpResponse, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.callLocked(ctx, method, params)
}

func (t *HTTPTransport) newPost(ctx context.Context, body []byte) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range t.headers {
		httpReq.Header.Set(k, v)
	}
	return httpReq, nil
}

// callLocked makes a JSON-RPC call (must hold at least read lock). The response
// must be the terminal response to this request's ID, as one JSON document or
// as the matching event of a request-scoped event stream; anything else is an
// error, never a success.
func (t *HTTPTransport) callLocked(ctx context.Context, method string, params any) (*mcpResponse, error) {
	id := int(t.nextID.Add(1))
	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := t.newPost(ctx, body)
	if err != nil {
		return nil, err
	}

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned status %d: %s", httpResp.StatusCode, diagnosticBody(httpResp.Body))
	}

	mediaType, _, _ := mime.ParseMediaType(httpResp.Header.Get("Content-Type"))
	var resp *mcpResponse
	switch mediaType {
	case "application/json":
		data, err := readBounded(httpResp.Body, int64(maxWireMessageBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		if resp, err = responseFor(id, data); err != nil {
			return nil, fmt.Errorf("invalid response to %s: %w", method, err)
		}
	case "text/event-stream":
		if resp, err = t.responseFromStream(id, httpResp.Body); err != nil {
			return nil, fmt.Errorf("invalid response to %s: %w", method, err)
		}
	default:
		return nil, fmt.Errorf("unsupported response content type %q for %s (want application/json or text/event-stream)", httpResp.Header.Get("Content-Type"), method)
	}

	if resp.Error != nil {
		return resp, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp, nil
}

// responseFromStream reads a request-scoped event stream until the response to
// request id arrives. Server notifications and requests on the stream are not
// this call's answer and are skipped; a response to any other ID, or a stream
// that ends first, is an error.
func (t *HTTPTransport) responseFromStream(id int, body io.Reader) (*mcpResponse, error) {
	var found *mcpResponse
	var streamErr error
	err := scanSSE(body, maxWireMessageBytes, func(event, data string) bool {
		if event != "message" {
			return false
		}
		msg, err := decodeRPCMessage([]byte(data))
		if err != nil {
			streamErr = err
			return true
		}
		if msg.kind() != rpcResponse {
			logging.Get(logging.CategoryTools).Debug("MCP HTTP stream: skipping server %s %q", msg.kind(), msg.Method)
			return false
		}
		found, streamErr = responseFor(id, []byte(data))
		return true
	})
	if streamErr != nil {
		return nil, streamErr
	}
	if err != nil {
		return nil, fmt.Errorf("event stream: %w", err)
	}
	if found == nil {
		return nil, fmt.Errorf("event stream ended without a response to request %d", id)
	}
	return found, nil
}

// notifyLocked sends a notification: no ID, and success is any 2xx status. The
// reply body, if any, is not a JSON-RPC answer and is discarded.
func (t *HTTPTransport) notifyLocked(ctx context.Context, notification mcpNotification) error {
	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	httpReq, err := t.newPost(ctx, body)
	if err != nil {
		return err
	}
	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("server returned status %d: %s", httpResp.StatusCode, diagnosticBody(httpResp.Body))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(httpResp.Body, maxDiagnosticBody))
	return nil
}

// Ensure HTTPTransport implements MCPTransport.
var _ MCPTransport = (*HTTPTransport)(nil)
