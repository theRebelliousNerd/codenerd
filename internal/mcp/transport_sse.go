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
	// connectMu serializes establishment attempts while Disconnect remains free
	// to cancel an in-progress attempt.
	connectMu sync.Mutex

	baseURL      string
	postURL      string
	timeout      time.Duration
	client       *http.Client
	streamClient *http.Client
	connected    bool
	serverInfo   *MCPCapabilities

	// SSE specific
	sseResp        *http.Response
	cancel         context.CancelFunc
	pending        map[int]chan *mcpResponse
	nextID         int
	initSignal     chan struct{}
	connectionDone chan struct{}
	generation     uint64
	streamActive   bool
	endpointReady  bool

	headers map[string]string
}

// NewSSETransportWithHeaders creates an SSE transport that attaches static
// headers to both the event stream request and every POST. Values are
// environment-expanded, so tokens stay out of the workspace config.
func NewSSETransportWithHeaders(baseURL string, timeout time.Duration, headers map[string]string) *SSETransport {
	var streamTransport *http.Transport
	if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
		streamTransport = defaultTransport.Clone()
	} else {
		streamTransport = &http.Transport{}
	}
	streamTransport.ResponseHeaderTimeout = timeout
	return &SSETransport{
		baseURL: baseURL,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		// A client-wide timeout also bounds reading the response body. SSE
		// bodies are intentionally long-lived, so only bound response headers;
		// POST requests retain the configured client timeout above.
		streamClient: &http.Client{Transport: streamTransport},
		pending:      make(map[int]chan *mcpResponse),
		nextID:       1,
		headers:      ExpandHeaderValues(headers),
	}
}

// Connect establishes connection to the MCP server.
func (t *SSETransport) Connect(ctx context.Context) error {
	t.connectMu.Lock()
	defer t.connectMu.Unlock()

	t.mu.RLock()
	if t.connected {
		t.mu.RUnlock()
		return nil
	}
	active := t.streamActive
	t.mu.RUnlock()
	if active {
		_ = t.Disconnect()
	}

	t.mu.Lock()
	t.generation++
	generation := t.generation
	t.streamActive = true
	t.connected = false
	t.serverInfo = nil
	t.postURL = ""
	t.endpointReady = false
	t.initSignal = make(chan struct{})
	t.connectionDone = make(chan struct{})
	t.pending = make(map[int]chan *mcpResponse)
	streamCtx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.mu.Unlock()

	req, err := http.NewRequestWithContext(streamCtx, "GET", t.baseURL, nil)
	if err != nil {
		t.finishConnection(generation)
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	// The stream request must outlive ctx after Connect returns, but ctx still
	// cancels a connection attempt while response headers are being received.
	stopCancel := context.AfterFunc(ctx, cancel)
	resp, err := t.streamClient.Do(req)
	stopCancel()
	if err != nil {
		t.finishConnection(generation)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("failed to connect to SSE endpoint %s: %w", t.baseURL, err)
	}
	if ctx.Err() != nil {
		_ = resp.Body.Close()
		t.finishConnection(generation)
		return ctx.Err()
	}

	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		t.finishConnection(generation)
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	t.mu.Lock()
	if t.generation != generation || !t.streamActive {
		t.mu.Unlock()
		_ = resp.Body.Close()
		return fmt.Errorf("SSE connection closed during connect")
	}
	t.sseResp = resp
	initSignal := t.initSignal
	connectionDone := t.connectionDone
	t.mu.Unlock()
	go t.readLoop(streamCtx, resp.Body, generation)

	logging.Get(logging.CategoryTools).Info("SSE connection established to %s, waiting for endpoint...", t.baseURL)

	// Wait for endpoint event
	endpointTimer := time.NewTimer(t.timeout)
	defer endpointTimer.Stop()
	select {
	case <-initSignal:
		// Got endpoint
	case <-ctx.Done():
		t.finishConnection(generation)
		return ctx.Err()
	case <-connectionDone:
		return fmt.Errorf("SSE connection closed before endpoint event")
	case <-endpointTimer.C:
		t.finishConnection(generation)
		return fmt.Errorf("timeout waiting for endpoint event")
	}

	// Initialization obeys both caller cancellation and the configured request
	// bound. The event stream itself remains owned by the transport.
	initCtx, cancelInit := context.WithTimeout(ctx, t.timeout)
	defer cancelInit()

	caps, err := t.GetCapabilities(initCtx)
	if err != nil {
		t.finishConnection(generation)
		return fmt.Errorf("failed to get capabilities: %w", err)
	}

	t.mu.Lock()
	if t.generation != generation || !t.streamActive {
		t.mu.Unlock()
		return fmt.Errorf("SSE connection closed during initialization")
	}
	t.serverInfo = caps
	t.connected = true
	t.mu.Unlock()

	logging.Get(logging.CategoryTools).Info("MCP SSE transport connected to %s", t.baseURL)
	return nil
}

// Disconnect closes the connection.
func (t *SSETransport) Disconnect() error {
	t.mu.RLock()
	generation := t.generation
	active := t.streamActive
	t.mu.RUnlock()
	if active {
		t.finishConnection(generation)
	}

	logging.Get(logging.CategoryTools).Info("MCP SSE transport disconnected from %s", t.baseURL)
	return nil
}

// finishConnection ends one generation and wakes pending calls without closing
// their response channels. Response delivery and this transition share mu;
// calls observe connectionDone instead of racing channel closure.
func (t *SSETransport) finishConnection(generation uint64) {
	t.mu.Lock()
	if t.generation != generation || !t.streamActive {
		t.mu.Unlock()
		return
	}
	t.streamActive = false
	t.connected = false
	t.endpointReady = false
	t.postURL = ""
	t.serverInfo = nil
	cancel := t.cancel
	response := t.sseResp
	done := t.connectionDone
	t.cancel = nil
	t.sseResp = nil
	t.connectionDone = nil
	t.pending = make(map[int]chan *mcpResponse)
	if done != nil {
		close(done)
	}
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if response != nil {
		_ = response.Body.Close()
	}
	t.streamClient.CloseIdleConnections()
}

// readLoop reads SSE events from the response body.
func (t *SSETransport) readLoop(ctx context.Context, body io.ReadCloser, generation uint64) {
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
			t.handleEvent(generation, eventType, data)

			eventType = "message" // Default event type
			eventData.Reset()
			continue
		}

		if after, ok := strings.CutPrefix(line, "event: "); ok {
			eventType = after
		} else if after, ok := strings.CutPrefix(line, "data: "); ok {
			eventData.WriteString(after)
			eventData.WriteByte('\n')
		} else if strings.HasPrefix(line, ":") ||
			strings.HasPrefix(line, "id: ") ||
			strings.HasPrefix(line, "retry: ") {
			// Comment, id, and retry fields are defined by the SSE spec and
			// deliberately unused here.
		} else {
			// Previously these three recognized-but-unused prefixes were empty
			// else-if branches with no final else, which made the whole chain a
			// no-op: a malformed or unexpected SSE field was indistinguishable
			// from one we chose to skip, and vanished without trace. Naming the
			// unknown case is what makes the branches above mean anything.
			logging.Get(logging.CategoryTools).Debug("SSE: ignoring unrecognized field line: %.80q", line)
		}
	}

	if err := scanner.Err(); err != nil {
		logging.Get(logging.CategoryTools).Warn("SSE read error: %v", err)
	}

	// If the loop exits without an intentional cancellation, the server dropped
	// the event stream. Wake any calls waiting for responses from this stream.
	if ctx.Err() == nil {
		logging.Get(logging.CategoryTools).Warn("SSE connection lost")
	}
	t.finishConnection(generation)
}

func (t *SSETransport) handleEvent(generation uint64, eventType, data string) {
	switch eventType {
	case "endpoint":
		t.mu.Lock()
		if t.generation == generation && t.streamActive && !t.endpointReady {
			t.postURL = data
			t.endpointReady = true
			close(t.initSignal)
		}
		t.mu.Unlock()

		logging.Get(logging.CategoryTools).Debug("Received SSE endpoint: %s", redactForLog(data, maxLoggedPayload))

	case "message":
		var resp mcpResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to unmarshal SSE message: %v. Data: %s", err, redactForLog(data, maxLoggedPayload))
			return
		}

		t.mu.Lock()
		ch, ok := t.pending[resp.ID]
		if t.generation != generation || !t.streamActive {
			ok = false
		}

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
		t.mu.Unlock()

	default:
		logging.Get(logging.CategoryTools).Debug("Ignored SSE event type: %s", eventType)
	}
}

// call makes a JSON-RPC call.
func (t *SSETransport) call(ctx context.Context, method string, params any) (*mcpResponse, error) {
	t.mu.Lock()
	if !t.streamActive || t.connectionDone == nil {
		t.mu.Unlock()
		return nil, fmt.Errorf("not connected to MCP server")
	}
	id := t.nextID
	t.nextID++
	ch := make(chan *mcpResponse, 1)
	t.pending[id] = ch
	postURL := t.postURL
	connectionDone := t.connectionDone
	generation := t.generation
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		if t.generation == generation && t.pending[id] == ch {
			delete(t.pending, id)
		}
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
	for k, v := range t.headers {
		httpReq.Header.Set(k, v)
	}

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
	case <-connectionDone:
		return nil, fmt.Errorf("connection closed")
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

	resp, err := t.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
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
func (t *SSETransport) CallTool(ctx context.Context, name string, args map[string]any) (*MCPCallResult, error) {
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
