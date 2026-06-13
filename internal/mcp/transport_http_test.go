package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mcpTestServer is a minimal JSON-RPC MCP server backed by httptest. It answers
// the handful of methods the HTTP transport issues during a connect / list /
// call / ping lifecycle.
func mcpTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{
				"capabilities": map[string]bool{"tools": true, "resources": false},
				"serverInfo":   map[string]string{"name": "test-mcp", "version": "0.1"},
			}
		case "tools/list":
			result = map[string]any{
				"tools": []map[string]any{
					{"name": "echo", "description": "echoes input", "inputSchema": map[string]any{"type": "object"}},
				},
			}
		case "tools/call":
			result = map[string]any{"content": "echoed"}
		case "ping":
			result = map[string]any{}
		default:
			http.Error(w, "unknown method", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		})
	}))
}

func TestHTTPTransport_Lifecycle(t *testing.T) {
	srv := mcpTestServer(t)
	defer srv.Close()

	tr := NewHTTPTransport(srv.URL, 5*time.Second)
	ctx := context.Background()

	if tr.IsConnected() {
		t.Fatal("transport should start disconnected")
	}
	// Calls before connect are rejected.
	if _, err := tr.ListTools(ctx); err == nil {
		t.Error("ListTools before Connect should error")
	}

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !tr.IsConnected() {
		t.Error("transport should be connected after Connect")
	}

	caps, err := tr.GetCapabilities(ctx)
	if err != nil {
		t.Fatalf("GetCapabilities: %v", err)
	}
	if !caps.Tools {
		t.Errorf("expected Tools capability true, got %+v", caps)
	}

	tools, err := tr.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Errorf("ListTools=%+v, want one 'echo' tool", tools)
	}

	res, err := tr.CallTool(ctx, "echo", map[string]any{"text": "hi"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.Success {
		t.Errorf("CallTool result not successful: %+v", res)
	}

	if err := tr.Ping(ctx); err != nil {
		t.Errorf("Ping: %v", err)
	}

	if err := tr.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if tr.IsConnected() {
		t.Error("transport should be disconnected after Disconnect")
	}
}

func TestHTTPTransport_ConnectFailure(t *testing.T) {
	// Point at a server that 500s on initialize: Connect must fail and leave
	// the transport disconnected.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	tr := NewHTTPTransport(srv.URL, 2*time.Second)
	if err := tr.Connect(context.Background()); err == nil {
		t.Error("Connect to a failing server should error")
	}
	if tr.IsConnected() {
		t.Error("transport should remain disconnected after a failed Connect")
	}
}
