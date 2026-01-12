package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// SSETransport implements MCPTransport over SSE (Server-Sent Events).
type SSETransport struct {
	mu sync.RWMutex

	baseURL    string
	postURL    string
	timeout    time.Duration
	client     *http.Client
	connected  bool
	serverInfo *MCPCapabilities

	// SSE specific
	sseResp    *http.Response
	cancel     context.CancelFunc
	pending    map[int]chan *mcpResponse
	nextID     int
	initSignal chan struct{}
	initOnce   sync.Once
}

// NewSSETransport creates a new SSE transport for MCP communication.
func NewSSETransport(baseURL string, timeout time.Duration) *SSETransport {
	return &SSETransport{
		baseURL: baseURL,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		pending:    make(map[int]chan *mcpResponse),
		nextID:     1,
		initSignal: make(chan struct{}),
	}
}

// Connect establishes connection to the MCP server.
func (t *SSETransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	if t.connected {
		t.mu.Unlock()
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", t.baseURL, nil)
	if err != nil {
		t.mu.Unlock()
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := t.client.Do(req)
	if err != nil {
		t.mu.Unlock()
		return fmt.Errorf("failed to connect to SSE endpoint %s: %w", t.baseURL, err)
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		t.mu.Unlock()
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	t.sseResp = resp

	readCtx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	go t.readLoop(readCtx, resp.Body)
	t.mu.Unlock()

	logging.Get(logging.CategoryTools).Info("SSE connection established to %s, waiting for endpoint...", t.baseURL)

	// Wait for endpoint event
	select {
	case <-t.initSignal:
		// Got endpoint
	case <-ctx.Done():
		t.Disconnect()
		return ctx.Err()
	case <-time.After(t.timeout):
		t.Disconnect()
		return fmt.Errorf("timeout waiting for endpoint event")
	}

	// Get capabilities
	// Use a fresh context for initialization that respects the timeout
	initCtx, cancelInit := context.WithTimeout(context.Background(), t.timeout)
	defer cancelInit()

	caps, err := t.GetCapabilities(initCtx)
	if err != nil {
		t.Disconnect()
		return fmt.Errorf("failed to get capabilities: %w", err)
	}

	t.mu.Lock()
	t.serverInfo = caps
	t.connected = true
	t.mu.Unlock()

	logging.Get(logging.CategoryTools).Info("MCP SSE transport connected to %s", t.baseURL)
	return nil
}

// Disconnect closes the connection.
func (t *SSETransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}

	if t.sseResp != nil {
		t.sseResp.Body.Close()
		t.sseResp = nil
	}

	t.connected = false
	t.serverInfo = nil

	// Close any pending channels to unblock callers
	for id, ch := range t.pending {
		close(ch)
		delete(t.pending, id)
	}

	logging.Get(logging.CategoryTools).Info("MCP SSE transport disconnected from %s", t.baseURL)
	return nil
}

// readLoop reads SSE events from the response body.
func (t *SSETransport) readLoop(ctx context.Context, body io.ReadCloser) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var eventType string
	var eventData bytes.Buffer

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			// End of event
			data := eventData.String()
			if len(data) > 0 && data[len(data)-1] == '\n' {
				data = data[:len(data)-1]
			}
			t.handleEvent(eventType, data)

			eventType = "message" // Default event type
			eventData.Reset()
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			eventData.WriteString(strings.TrimPrefix(line, "data: "))
			eventData.WriteByte('\n')
		} else if strings.HasPrefix(line, ":") {
			// Comment, ignore
		} else if strings.HasPrefix(line, "id: ") {
			// ID, ignore for now
		} else if strings.HasPrefix(line, "retry: ") {
			// Retry, ignore for now
		}
	}

	if err := scanner.Err(); err != nil {
		logging.Get(logging.CategoryTools).Warn("SSE read error: %v", err)
	}

	// If loop exits, we disconnected
	t.mu.Lock()
	if t.connected {
		t.connected = false
		logging.Get(logging.CategoryTools).Warn("SSE connection lost")
	}
	t.mu.Unlock()
}

func (t *SSETransport) handleEvent(eventType, data string) {
	switch eventType {
	case "endpoint":
		t.mu.Lock()
		t.postURL = data
		t.mu.Unlock()

		t.initOnce.Do(func() {
			close(t.initSignal)
		})
		logging.Get(logging.CategoryTools).Debug("Received SSE endpoint: %s", data)

	case "message":
		var resp mcpResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to unmarshal SSE message: %v. Data: %s", err, data)
			return
		}

		t.mu.RLock()
		ch, ok := t.pending[resp.ID]
		t.mu.RUnlock()

		if ok {
			select {
			case ch <- &resp:
			default:
				logging.Get(logging.CategoryTools).Warn("Response channel full for ID %d", resp.ID)
			}
		} else {
			// Could be a notification or unsolicited message (not supported yet)
			logging.Get(logging.CategoryTools).Debug("Received unsolicited message ID %d", resp.ID)
		}

	default:
		logging.Get(logging.CategoryTools).Debug("Ignored SSE event type: %s", eventType)
	}
}

// call makes a JSON-RPC call.
func (t *SSETransport) call(ctx context.Context, method string, params interface{}) (*mcpResponse, error) {
	t.mu.Lock()
	id := t.nextID
	t.nextID++
	ch := make(chan *mcpResponse, 1)
	t.pending[id] = ch
	postURL := t.postURL
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
	}()

	if postURL == "" {
		return nil, fmt.Errorf("no endpoint available")
	}

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

	fullURL := t.resolveURL(postURL)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", fullURL, bytes.NewReader(body))
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

	// For SSE, the response is usually "Accepted" (202) or just 200 OK with empty body,
	// and the actual result comes via SSE.
	// However, standard MCP might return result immediately if it's not async?
	// The spec says: "The server MUST respond to the POST request with a 200 OK status code... The actual JSON-RPC response ... will be sent via the SSE connection."

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return resp, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(t.timeout):
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

func (t *SSETransport) resolveURL(u string) string {
	base, err := url.Parse(t.baseURL)
	if err != nil {
		return u
	}
	ref, err := url.Parse(u)
	if err != nil {
		return u
	}
	return base.ResolveReference(ref).String()
}

// GetCapabilities returns server capabilities.
func (t *SSETransport) GetCapabilities(ctx context.Context) (*MCPCapabilities, error) {
	t.mu.RLock()
	if t.serverInfo != nil {
		caps := *t.serverInfo
		t.mu.RUnlock()
		return &caps, nil
	}
	t.mu.RUnlock()

	resp, err := t.call(ctx, "initialize", map[string]interface{}{
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

// ListTools retrieves available tools from the server.
func (t *SSETransport) ListTools(ctx context.Context) ([]MCPToolSchema, error) {
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
	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (t *SSETransport) CallTool(ctx context.Context, name string, args map[string]interface{}) (*MCPCallResult, error) {
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

	// Error handling is done in call(), but if resp still has error (shouldn't happen with my implementation of call)
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

// Ping checks if the server is responsive.
func (t *SSETransport) Ping(ctx context.Context) error {
	_, err := t.call(ctx, "ping", nil)
	return err
}

// IsConnected returns current connection status.
func (t *SSETransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// Ensure SSETransport implements MCPTransport.
var _ MCPTransport = (*SSETransport)(nil)
