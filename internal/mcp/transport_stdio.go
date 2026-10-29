package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// StdioTransport implements MCPTransport over stdio.
type StdioTransport struct {
	mu sync.RWMutex

	command string
	args    []string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser

	connected  bool
	serverInfo *MCPCapabilities

	pendingReqs map[int]chan *mcpResponse
	nextID      int

	// failure records why the transport ended itself (an oversized message, the
	// server closing its stdout), so waiting calls report it.
	failure error

	done chan struct{}
	wg   sync.WaitGroup

	// Notification handler (optional)
	notificationHandler func(method string, params json.RawMessage)
}

// mcpNotification represents a server notification (no ID)
type mcpNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitzero"`
}

// SetNotificationHandler sets a handler for server notifications
func (t *StdioTransport) SetNotificationHandler(handler func(method string, params json.RawMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notificationHandler = handler
}

// NewStdioTransport creates a new Stdio transport.
func NewStdioTransport(endpoint string) *StdioTransport {
	parts := strings.Fields(endpoint)
	var cmd string
	var args []string
	if len(parts) > 0 {
		cmd = parts[0]
		args = parts[1:]
	}

	return &StdioTransport{
		command:     cmd,
		args:        args,
		pendingReqs: make(map[int]chan *mcpResponse),
		nextID:      1,
		done:        make(chan struct{}),
	}
}

// Connect starts the subprocess and performs the initialization handshake. The
// transport reports connected only once initialize, capability validation and
// the initialized notification have all succeeded.
func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	if t.connected {
		t.mu.Unlock()
		return nil
	}

	if t.command == "" {
		t.mu.Unlock()
		return fmt.Errorf("empty command for stdio transport")
	}

	t.cmd = exec.Command(t.command, t.args...)

	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		t.mu.Unlock()
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		t.mu.Unlock()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	t.stderr, err = t.cmd.StderrPipe()
	if err != nil {
		t.mu.Unlock()
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := t.cmd.Start(); err != nil {
		t.mu.Unlock()
		return fmt.Errorf("failed to start command %s: %w", t.command, err)
	}

	t.connected = true
	t.failure = nil
	t.serverInfo = nil
	t.done = make(chan struct{})
	t.pendingReqs = make(map[int]chan *mcpResponse)

	// Start stderr reader
	t.wg.Add(1)
	go t.readStderr()

	// Start stdout reader
	t.wg.Add(1)
	go t.readStdout()
	t.mu.Unlock()

	// The lock is released: the reader goroutine needs it to dispatch the
	// initialize response.
	caps, err := t.initialize(ctx)
	if err != nil {
		_ = t.Disconnect()
		return fmt.Errorf("failed to initialize MCP session: %w", err)
	}
	t.mu.Lock()
	t.serverInfo = caps
	t.mu.Unlock()
	logging.Get(logging.CategoryTools).Info("MCP stdio transport connected to %s (protocol %s)", t.command, caps.ProtocolVersion)
	return nil
}

// Disconnect kills the process and cleans up.
func (t *StdioTransport) Disconnect() error {
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		return nil
	}
	t.teardownLocked(nil)
	t.mu.Unlock()

	// Wait for goroutines to finish
	// We should probably use a timeout in case they are stuck (though stdout/stderr close should unblock them)
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		logging.Get(logging.CategoryTools).Warn("Timeout waiting for stdio transport goroutines to exit")
	}

	logging.Get(logging.CategoryTools).Info("MCP Stdio transport disconnected")
	return nil
}

// teardownLocked ends the session: marks it disconnected, kills the process,
// and wakes every pending call. cause is why the transport ended itself, nil
// for a caller's Disconnect. The caller holds the write lock and has checked
// that the transport was connected.
func (t *StdioTransport) teardownLocked(cause error) {
	t.connected = false
	t.failure = cause
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	close(t.done)
	for id, ch := range t.pendingReqs {
		close(ch)
		delete(t.pendingReqs, id)
	}
}

// abort ends the session from the reader goroutine because the stream can no
// longer be trusted: pending calls are woken with the cause instead of waiting
// for a response that will never be read.
func (t *StdioTransport) abort(cause error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.connected {
		return
	}
	logging.Get(logging.CategoryTools).Warn("MCP stdio transport ended: %v", cause)
	t.teardownLocked(cause)
}

// readStderr reads stderr and logs it.
func (t *StdioTransport) readStderr() {
	defer t.wg.Done()
	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		// Server stderr is untrusted output that regularly echoes the
		// credentials the server was configured with.
		logging.Get(logging.CategoryTools).Info("[STDERR] %s", redactForLog(scanner.Text(), maxLoggedPayload))
	}
}

// readStdout reads newline-delimited JSON-RPC messages from stdout.
func (t *StdioTransport) readStdout() {
	defer t.wg.Done()
	scanner := bufio.NewScanner(t.stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), maxWireMessageBytes)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		t.dispatchLine(line)
	}

	cause := scanner.Err()
	switch {
	case errors.Is(cause, bufio.ErrTooLong):
		cause = fmt.Errorf("server message exceeds the %d byte wire limit", maxWireMessageBytes)
	case cause != nil:
		cause = fmt.Errorf("reading server stdout: %w", cause)
	default:
		cause = errors.New("server closed its stdout")
	}
	t.abort(cause)
}

// dispatchLine routes one message: a response completes the pending call with
// that ID; a notification goes to the handler; a server request is logged, since
// this client declares no capability a server could call.
func (t *StdioTransport) dispatchLine(line []byte) {
	msg, err := decodeRPCMessage(line)
	if err != nil {
		logging.Get(logging.CategoryTools).Warn("Dropping stdout message: %v", err)
		return
	}
	switch msg.kind() {
	case rpcResponse:
		id, ok := msg.intID()
		if !ok {
			logging.Get(logging.CategoryTools).Warn("Dropping response with unusable id %s", string(msg.ID))
			return
		}
		t.mu.Lock()
		ch, exists := t.pendingReqs[id]
		if exists {
			delete(t.pendingReqs, id)
			ch <- msg.response(id)
		} else {
			logging.Get(logging.CategoryTools).Warn("Received response for unknown or completed ID: %d", id)
		}
		t.mu.Unlock()
	case rpcNotification:
		t.handleNotification(mcpNotification{JSONRPC: msg.JSONRPC, Method: msg.Method, Params: msg.Params})
	case rpcRequest:
		logging.Get(logging.CategoryTools).Warn("Ignoring server request %q: this client declares no capability a server may call", msg.Method)
	default:
		logging.Get(logging.CategoryTools).Warn("Dropping stdout message that is neither a response, request nor notification")
	}
}

func (t *StdioTransport) handleNotification(notif mcpNotification) {
	logging.Get(logging.CategoryTools).Debug("Received server notification: method=%s", notif.Method)

	t.mu.RLock()
	handler := t.notificationHandler
	t.mu.RUnlock()

	if handler != nil {
		// Handle in goroutine to avoid blocking reader
		go handler(notif.Method, notif.Params)
		return
	}
	// Common MCP notifications we might receive
	switch notif.Method {
	case "notifications/progress":
		logging.Get(logging.CategoryTools).Debug("Progress update: %s", redactForLog(string(notif.Params), maxLoggedPayload))
	case "notifications/log":
		logging.Get(logging.CategoryTools).Info("Server log: %s", redactForLog(string(notif.Params), maxLoggedPayload))
	case "notifications/cancelled":
		logging.Get(logging.CategoryTools).Debug("Request cancelled: %s", redactForLog(string(notif.Params), maxLoggedPayload))
	default:
		logging.Get(logging.CategoryTools).Debug("Unhandled notification '%s': %s", notif.Method, redactForLog(string(notif.Params), maxLoggedPayload))
	}
}

// call sends a request and waits for a response.
func (t *StdioTransport) call(ctx context.Context, method string, params any) (*mcpResponse, error) {
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		return nil, fmt.Errorf("not connected to MCP server")
	}

	id := t.nextID
	t.nextID++

	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	ch := make(chan *mcpResponse, 1)
	t.pendingReqs[id] = ch

	// Marshal and write
	data, err := json.Marshal(req)
	if err != nil {
		delete(t.pendingReqs, id)
		t.mu.Unlock()
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Write line
	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		delete(t.pendingReqs, id)
		t.mu.Unlock()
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}
	t.mu.Unlock()

	select {
	case resp := <-ch:
		if resp == nil {
			t.mu.RLock()
			cause := t.failure
			t.mu.RUnlock()
			if cause != nil {
				return nil, fmt.Errorf("connection closed: %w", cause)
			}
			return nil, fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pendingReqs, id)
		t.mu.Unlock()
		return nil, ctx.Err()
	}
}

// ListTools retrieves every page of the server's tools.
func (t *StdioTransport) ListTools(ctx context.Context) ([]MCPToolSchema, error) {
	return listToolsVia(ctx, t)
}

// CallTool invokes a tool on the MCP server.
func (t *StdioTransport) CallTool(ctx context.Context, name string, args map[string]any) (*MCPCallResult, error) {
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
func (t *StdioTransport) GetCapabilities(ctx context.Context) (*MCPCapabilities, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.serverInfo == nil {
		return nil, fmt.Errorf("not connected to MCP server")
	}
	caps := *t.serverInfo
	return &caps, nil
}

// initialize runs the handshake: initialize, version and capability
// validation, then the initialized notification.
func (t *StdioTransport) initialize(ctx context.Context) (*MCPCapabilities, error) {
	resp, err := t.call(ctx, "initialize", initializeParams())
	if err != nil {
		return nil, err
	}
	caps, err := parseInitializeResult(resp.Result)
	if err != nil {
		return nil, err
	}
	if err := t.notify(initializedNotification()); err != nil {
		return nil, fmt.Errorf("initialized notification: %w", err)
	}
	return caps, nil
}

// notify writes a notification line: no ID, no response.
func (t *StdioTransport) notify(notification mcpNotification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.connected || t.stdin == nil {
		return fmt.Errorf("not connected to MCP server")
	}
	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}
	return nil
}

// Ping checks if the server is responsive.
func (t *StdioTransport) Ping(ctx context.Context) error {
	_, err := t.call(ctx, "ping", nil)
	return err
}

// IsConnected returns current connection status.
func (t *StdioTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// Ensure StdioTransport implements MCPTransport.
var _ MCPTransport = (*StdioTransport)(nil)
