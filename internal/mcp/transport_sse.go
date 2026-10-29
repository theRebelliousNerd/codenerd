package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	// closeCause records why the current stream ended when the transport, not
	// the caller, ended it (an oversized event, a read failure), so calls
	// waiting on the stream report the cause instead of a bare "closed".
	closeCause error

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
			Timeout:       timeout,
			CheckRedirect: refuseCrossHostRedirect,
		},
		// A client-wide timeout also bounds reading the response body. SSE
		// bodies are intentionally long-lived, so only bound response headers;
		// POST requests retain the configured client timeout above.
		streamClient: &http.Client{Transport: streamTransport, CheckRedirect: refuseCrossHostRedirect},
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
	t.closeCause = nil
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

	caps, err := t.initialize(initCtx)
	if err != nil {
		t.finishConnection(generation)
		return fmt.Errorf("failed to initialize MCP session: %w", err)
	}

	t.mu.Lock()
	if t.generation != generation || !t.streamActive {
		t.mu.Unlock()
		return fmt.Errorf("SSE connection closed during initialization")
	}
	t.serverInfo = caps
	t.connected = true
	t.mu.Unlock()

	logging.Get(logging.CategoryTools).Info("MCP SSE transport connected to %s (protocol %s)", t.baseURL, caps.ProtocolVersion)
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

	err := scanSSE(body, maxWireMessageBytes, func(eventType, data string) bool {
		if ctx.Err() != nil {
			return true
		}
		t.handleEvent(generation, eventType, data)
		return false
	})

	if err != nil && ctx.Err() == nil {
		logging.Get(logging.CategoryTools).Warn("SSE read error: %v", err)
		t.mu.Lock()
		if t.generation == generation && t.streamActive {
			t.closeCause = err
		}
		t.mu.Unlock()
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
		msg, err := decodeRPCMessage([]byte(data))
		if err != nil {
			logging.Get(logging.CategoryTools).Warn("Dropping SSE message: %v. Data: %s", err, redactForLog(data, maxLoggedPayload))
			return
		}
		if kind := msg.kind(); kind != rpcResponse {
			// A server request or notification carries an ID or method of its
			// own and is never the answer to one of ours.
			logging.Get(logging.CategoryTools).Debug("Ignoring server %s %q on the SSE stream", kind, msg.Method)
			return
		}
		id, ok := msg.intID()
		if !ok {
			logging.Get(logging.CategoryTools).Warn("Dropping SSE response with unusable id %s", string(msg.ID))
			return
		}

		t.mu.Lock()
		ch, pending := t.pending[id]
		if t.generation != generation || !t.streamActive {
			pending = false
		}
		if pending {
			// The first terminal response completes the call; a duplicate
			// finds no pending entry and is dropped.
			delete(t.pending, id)
			select {
			case ch <- msg.response(id):
			default:
				logging.Get(logging.CategoryTools).Warn("Response channel full for ID %d", id)
			}
		} else {
			logging.Get(logging.CategoryTools).Debug("Dropping response for unknown or completed request ID %d", id)
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

	if err := t.post(ctx, postURL, body); err != nil {
		return nil, err
	}

	// The POST is only acknowledged here; the JSON-RPC response arrives on the
	// event stream, matched to this call by ID.
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, t.closedError()
		}
		if resp.Error != nil {
			return resp, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-connectionDone:
		return nil, t.closedError()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(t.timeout):
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

// post sends one JSON-RPC message to the session endpoint. Any 2xx status
// acknowledges it; the answer, if there is one, comes on the event stream.
func (t *SSETransport) post(ctx context.Context, postURL string, body []byte) error {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", t.resolveURL(postURL), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("server returned status %d: %s", httpResp.StatusCode, diagnosticBody(httpResp.Body))
	}
	return nil
}

// notify sends a notification: no ID and no response to wait for.
func (t *SSETransport) notify(ctx context.Context, notification mcpNotification) error {
	t.mu.RLock()
	active := t.streamActive
	postURL := t.postURL
	t.mu.RUnlock()
	if !active || postURL == "" {
		return fmt.Errorf("not connected to MCP server")
	}
	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	return t.post(ctx, postURL, body)
}

// closedError reports why the stream a call was waiting on ended.
func (t *SSETransport) closedError() error {
	t.mu.RLock()
	cause := t.closeCause
	t.mu.RUnlock()
	if cause != nil {
		return fmt.Errorf("connection closed: %w", cause)
	}
	return fmt.Errorf("connection closed")
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

// GetCapabilities returns the capabilities observed during Connect.
func (t *SSETransport) GetCapabilities(ctx context.Context) (*MCPCapabilities, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.serverInfo == nil {
		return nil, fmt.Errorf("not connected to MCP server")
	}
	caps := *t.serverInfo
	return &caps, nil
}

// initialize runs the handshake on the open stream: initialize, version and
// capability validation, then the initialized notification.
func (t *SSETransport) initialize(ctx context.Context) (*MCPCapabilities, error) {
	resp, err := t.call(ctx, "initialize", initializeParams())
	if err != nil {
		return nil, err
	}
	caps, err := parseInitializeResult(resp.Result)
	if err != nil {
		return nil, err
	}
	if err := t.notify(ctx, initializedNotification()); err != nil {
		return nil, fmt.Errorf("initialized notification: %w", err)
	}
	return caps, nil
}

// ListTools retrieves every page of the server's tools.
func (t *SSETransport) ListTools(ctx context.Context) ([]MCPToolSchema, error) {
	return listToolsVia(ctx, t)
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

	return callResultFromRPC(resp, latencyMs), nil
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
