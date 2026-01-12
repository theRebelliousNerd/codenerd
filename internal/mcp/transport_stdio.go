package mcp

import (
	"bufio"
	"context"
	"encoding/json"
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

	command    string
	args       []string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser

	connected  bool
	serverInfo *MCPCapabilities

	pendingReqs map[int]chan *mcpResponse
	nextID      int

	done       chan struct{}
	wg         sync.WaitGroup
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

// Connect starts the subprocess and the reader loop.
func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	if t.command == "" {
		return fmt.Errorf("empty command for stdio transport")
	}

	t.cmd = exec.Command(t.command, t.args...)

	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	t.stderr, err = t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command %s: %w", t.command, err)
	}

	t.connected = true

	// Start stderr reader
	t.wg.Add(1)
	go t.readStderr()

	// Start stdout reader
	t.wg.Add(1)
	go t.readStdout()

	// Try to get capabilities to verify connection (and initialize)
	// We need to release the lock because getCapabilitiesLocked calls callLocked which waits for response
	// But callLocked needs to register the ID.
	// We can't call callLocked while holding the lock if callLocked sends and waits.
	// Actually, callLocked should send and then wait.
	// If we hold the lock while waiting, readStdout cannot acquire the lock to dispatch the response!
	// So we must NOT hold the lock while waiting for response.

	// We are holding the lock here. We should release it before calling getCapabilities.
	// But we set t.connected = true. So other calls can proceed.
	// However, if we fail to get capabilities, we should disconnect.

	// Let's release the lock temporarily or just rely on the caller (client.go) calling GetCapabilities immediately.
	// Client.go calls Connect then GetCapabilities.
	// But HTTPTransport.Connect calls getCapabilitiesLocked.
	// If we want consistency, we should do it here too.

	// To do it safely:
	// 1. Release lock.
	// 2. Call GetCapabilities.
	// 3. If error, Disconnect and return error.

	return nil
}

// Disconnect kills the process and cleans up.
func (t *StdioTransport) Disconnect() error {
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		return nil
	}
	t.connected = false

	// Close channels to stop readers?
	// Actually we should kill the process.
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}

	// Close stdin to unblock any writes?
	if t.stdin != nil {
		_ = t.stdin.Close()
	}

	// Close done channel to signal shutdown
	close(t.done)

	// Clear pending requests
	for id, ch := range t.pendingReqs {
		close(ch)
		delete(t.pendingReqs, id)
	}
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

// readStderr reads stderr and logs it.
func (t *StdioTransport) readStderr() {
	defer t.wg.Done()
	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		logging.Get(logging.CategoryTools).Info("[STDERR] %s", scanner.Text())
	}
}

// readStdout reads JSON-RPC messages from stdout.
func (t *StdioTransport) readStdout() {
	defer t.wg.Done()
	scanner := bufio.NewScanner(t.stdout)
	// Increase buffer size if needed, but default is usually fine for messages

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(line, &raw); err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to parse JSON from stdout: %v", err)
			continue
		}

		// Check if it's a response (has "id")
		if idVal, ok := raw["id"]; ok {
			// ID can be int or string in JSON-RPC 2.0, but we use int.
			// json.Unmarshal decodes numbers as float64.
			var id int
			switch v := idVal.(type) {
			case float64:
				id = int(v)
			case int:
				id = v
			default:
				// Log unexpected ID type
				continue
			}

			// Decode as mcpResponse
			var resp mcpResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				logging.Get(logging.CategoryTools).Warn("Failed to unmarshal response: %v", err)
				continue
			}

			t.mu.Lock()
			ch, exists := t.pendingReqs[id]
			if exists {
				delete(t.pendingReqs, id)
				ch <- &resp
			} else {
				logging.Get(logging.CategoryTools).Warn("Received response for unknown ID: %d", id)
			}
			t.mu.Unlock()
		} else {
			// Notification or Request from server
			// TODO: Handle server requests/notifications
			logging.Get(logging.CategoryTools).Debug("Received notification: %s", string(line))
		}
	}

	if err := scanner.Err(); err != nil {
		// If process was killed, we might get "file already closed" or similar
		t.mu.RLock()
		connected := t.connected
		t.mu.RUnlock()
		if connected {
			logging.Get(logging.CategoryTools).Error("Error reading stdout: %v", err)
		}
	}
}

// call sends a request and waits for a response.
func (t *StdioTransport) call(ctx context.Context, method string, params interface{}) (*mcpResponse, error) {
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

// ListTools retrieves available tools from the server.
func (t *StdioTransport) ListTools(ctx context.Context) ([]MCPToolSchema, error) {
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
func (t *StdioTransport) CallTool(ctx context.Context, name string, args map[string]interface{}) (*MCPCallResult, error) {
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

	return &MCPCallResult{
		Success:   true,
		Output:    resp.Result,
		LatencyMs: latencyMs,
	}, nil
}

// GetCapabilities returns server capabilities.
func (t *StdioTransport) GetCapabilities(ctx context.Context) (*MCPCapabilities, error) {
	t.mu.RLock()
	if t.serverInfo != nil {
		caps := *t.serverInfo
		t.mu.RUnlock()
		return &caps, nil
	}
	t.mu.RUnlock()

	// Perform initialize handshake
	// Note: In MCP, the first request should be 'initialize'.
	// This method might be called multiple times, but we should only initialize once.
	// However, if serverInfo is nil, we haven't initialized or cached it yet.

	// We need to ensure only one initialize happens.
	// But usually Client calls GetCapabilities once after Connect.

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
		t.mu.Lock()
		t.serverInfo = &simple
		t.mu.Unlock()
		return &simple, nil
	}

	t.mu.Lock()
	t.serverInfo = &result.Capabilities
	t.mu.Unlock()

	// After initialize, we must send 'notifications/initialized'
	// We don't wait for response.
	// We construct a notification (no ID).
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	notifBytes, _ := json.Marshal(notification)
	t.mu.Lock()
	if t.stdin != nil {
		_, _ = t.stdin.Write(append(notifBytes, '\n'))
	}
	t.mu.Unlock()

	return &result.Capabilities, nil
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
