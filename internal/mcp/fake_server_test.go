package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeMCPServer is an in-process JSON-RPC MCP server. The transport tests
// previously only asserted against hand-rolled response bodies, which could not
// catch a wrong method name or a malformed request envelope — the server here
// validates what the client actually sends.
type fakeMCPServer struct {
	mu sync.Mutex

	*httptest.Server

	// Recorded observations.
	methods     []string
	authHeaders []string
	lastArgs    map[string]any

	// Behavior switches.
	failCall    bool
	toolErrCode int
}

func newFakeMCPServer(t *testing.T) *fakeMCPServer {
	t.Helper()
	f := &fakeMCPServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeMCPServer) handle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.JSONRPC != "2.0" {
		http.Error(w, "missing jsonrpc version", http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	f.methods = append(f.methods, req.Method)
	f.authHeaders = append(f.authHeaders, r.Header.Get("Authorization"))
	failCall := f.failCall
	errCode := f.toolErrCode
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	write := func(result any) {
		payload, _ := json.Marshal(result)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  json.RawMessage(payload),
		})
	}

	switch req.Method {
	case "initialize":
		write(map[string]any{
			"capabilities": map[string]bool{"tools": true, "resources": true},
			"serverInfo":   map[string]string{"name": "fake", "version": "1.0"},
		})
	case "tools/list":
		write(map[string]any{
			"tools": []map[string]any{
				{
					"name":        "read_file",
					"description": "Read a file from disk",
					"inputSchema": map[string]any{"type": "object"},
				},
				{
					"name":        "run_cmd",
					"description": "Execute a shell command",
					"inputSchema": map[string]any{"type": "object"},
				},
			},
		})
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &params)
		f.mu.Lock()
		f.lastArgs = params.Arguments
		f.mu.Unlock()

		if failCall {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"error":   map[string]any{"code": errCode, "message": "tool exploded"},
			})
			return
		}
		write(map[string]any{"content": "ok:" + params.Name})
	case "ping":
		write(map[string]any{})
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error":   map[string]any{"code": -32601, "message": "method not found"},
		})
	}
}

func (f *fakeMCPServer) observedMethods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.methods...)
}

func TestHTTPTransport_WhenServerLive_ShouldListToolsOverJSONRPC(t *testing.T) {
	server := newFakeMCPServer(t)
	transport := NewHTTPTransport(server.URL, 5*time.Second)

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !transport.IsConnected() {
		t.Fatal("transport should report connected after a successful initialize")
	}

	tools, err := transport.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "read_file" || tools[1].Name != "run_cmd" {
		t.Errorf("unexpected tool names: %+v", tools)
	}

	methods := server.observedMethods()
	if len(methods) < 2 || methods[0] != "initialize" || methods[1] != "tools/list" {
		t.Errorf("expected initialize then tools/list, got %v", methods)
	}
}

func TestHTTPTransport_WhenToolCalled_ShouldForwardArgumentsAndReturnOutput(t *testing.T) {
	server := newFakeMCPServer(t)
	transport := NewHTTPTransport(server.URL, 5*time.Second)
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	result, err := transport.CallTool(ctx, "read_file", map[string]any{"path": "main.go"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %+v", result)
	}
	if !strings.Contains(string(result.Output), "ok:read_file") {
		t.Errorf("unexpected output: %s", result.Output)
	}

	server.mu.Lock()
	args := server.lastArgs
	server.mu.Unlock()
	if args["path"] != "main.go" {
		t.Errorf("arguments not forwarded: %+v", args)
	}
}

func TestHTTPTransport_WhenServerReturnsRPCError_ShouldSurfaceSoftFailure(t *testing.T) {
	server := newFakeMCPServer(t)
	server.mu.Lock()
	server.failCall = true
	server.toolErrCode = -32000
	server.mu.Unlock()

	transport := NewHTTPTransport(server.URL, 5*time.Second)
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// A tool-level failure is data, not a transport error: the shard must be
	// able to see the message and pick another tool.
	result, err := transport.CallTool(ctx, "run_cmd", nil)
	if err != nil {
		t.Fatalf("CallTool returned a transport error for a tool-level failure: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure result")
	}
	if !strings.Contains(result.Error, "tool exploded") {
		t.Errorf("error message lost: %q", result.Error)
	}
}

func TestHTTPTransport_WhenHeadersConfigured_ShouldSendThemOnEveryRequest(t *testing.T) {
	server := newFakeMCPServer(t)
	t.Setenv("MCP_TEST_TOKEN", "s3cr3t-value")

	transport := NewHTTPTransportWithHeaders(server.URL, 5*time.Second, map[string]string{
		"Authorization": "Bearer ${MCP_TEST_TOKEN}",
		"X-Empty":       "${MCP_TEST_UNSET_VAR}",
	})

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := transport.ListTools(ctx); err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.authHeaders) < 2 {
		t.Fatalf("expected at least 2 requests, got %d", len(server.authHeaders))
	}
	for i, got := range server.authHeaders {
		if got != "Bearer s3cr3t-value" {
			t.Errorf("request %d Authorization = %q, want the expanded token", i, got)
		}
	}
}

// TestManager_WhenConnectedToFakeServer_ShouldDiscoverAndEmitFacts exercises the
// full manager path against a real HTTP server: connect, discover, analyze
// (heuristics, no LLM), persist, and mirror into the kernel.
func TestManager_WhenConnectedToFakeServer_ShouldDiscoverAndEmitFacts(t *testing.T) {
	server := newFakeMCPServer(t)
	store, err := NewMCPToolStore("file::memory:?cache=shared", nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	kernel := &recordingKernel{}
	manager := NewMCPClientManager(store, NewToolAnalyzer(nil, nil), map[string]MCPServerConfig{
		"fake": {
			ID:                "fake",
			Enabled:           true,
			Protocol:          "http",
			BaseURL:           server.URL,
			Timeout:           "5s",
			AutoConnect:       true,
			AutoDiscoverTools: true,
		},
	})
	manager.SetFactEmitter(NewFactEmitter(kernel))

	ctx := context.Background()
	if err := manager.ConnectAll(ctx); err != nil {
		t.Fatalf("ConnectAll: %v", err)
	}
	if err := manager.WaitForDiscovery(ctx); err != nil {
		t.Fatalf("WaitForDiscovery: %v", err)
	}

	tools, err := store.GetAllTools(ctx)
	if err != nil {
		t.Fatalf("GetAllTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 persisted tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool.SchemaHash == "" {
			t.Errorf("tool %s persisted without a schema hash", tool.ToolID)
		}
	}

	asserted := kernel.assertedFacts()
	wantPrefixes := []string{
		`mcp_server_registered("fake"`,
		`mcp_server_status("fake", /connected)`,
		`mcp_tool_registered("fake/read_file"`,
		`mcp_tool_capability("fake/read_file"`,
		`mcp_tool_shard_affinity("fake/read_file"`,
	}
	for _, want := range wantPrefixes {
		if !containsPrefix(asserted, want) {
			t.Errorf("expected a fact starting with %q; asserted: %v", want, asserted)
		}
	}
}

func containsPrefix(values []string, prefix string) bool {
	for _, v := range values {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

// recordingKernel captures the fact strings the emitter produces.
type recordingKernel struct {
	mu        sync.Mutex
	asserted  []string
	retracted []string
}

func (k *recordingKernel) Assert(fact string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.asserted = append(k.asserted, fact)
	return nil
}

func (k *recordingKernel) Retract(fact string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.retracted = append(k.retracted, fact)
	return nil
}

func (k *recordingKernel) Query(string) ([]map[string]any, error) { return nil, nil }

func (k *recordingKernel) assertedFacts() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]string(nil), k.asserted...)
}

func (k *recordingKernel) retractedFacts() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]string(nil), k.retracted...)
}
