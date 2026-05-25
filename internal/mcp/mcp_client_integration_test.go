//go:build integration

package mcp_test

// All tests for MCP client integration.

import (
	"context"
	"fmt"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codenerd/internal/mcp"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
)

type MCPClientIntegrationSuite struct {
	suite.Suite
	server     *httptest.Server
	store      *mcp.MCPToolStore
	client     *mcp.MCPClientManager
	dbPath     string
	serverAddr string
}

func TestMCPClientIntegrationSuite(t *testing.T) {
	suite.Run(t, new(MCPClientIntegrationSuite))
}

func (s *MCPClientIntegrationSuite) SetupTest() {
	// 1. Setup Mock MCP Server (per test)
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		resp := struct {
			JSONRPC string      `json:"jsonrpc"`
			ID      int         `json:"id"`
			Result  interface{} `json:"result,omitempty"`
			Error   interface{} `json:"error,omitempty"`
		}{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

		switch req.Method {
		case "initialize":
			resp.Result = map[string]interface{}{
				"capabilities": map[string]bool{
					"tools": true,
				},
				"serverInfo": map[string]string{
					"name":    "mock-server",
					"version": "1.0.0",
				},
			}
		case "tools/list":
			resp.Result = map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "calculator",
						"description": "Adds two numbers",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"a": map[string]string{"type": "number"},
								"b": map[string]string{"type": "number"},
							},
						},
					},
				},
			}
		case "tools/call":
			var params struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				resp.Error = map[string]interface{}{"code": -32700, "message": "Parse error"}
			} else if params.Name == "calculator" {
				a := params.Arguments["a"].(float64)
				b := params.Arguments["b"].(float64)
				resp.Result = map[string]interface{}{
					"sum": a + b,
				}
			} else {
				resp.Error = map[string]interface{}{"code": -32601, "message": "Method not found"}
			}
		case "ping":
			resp.Result = "pong"
		default:
			resp.Error = map[string]interface{}{"code": -32601, "message": "Method not found"}
		}

		json.NewEncoder(w).Encode(resp)
	}))
	s.serverAddr = s.server.URL

	// 2. Setup Store & Client per test to ensure isolation
	s.dbPath = filepath.Join(s.T().TempDir(), "mcp_test.db")

	var err error
	s.store, err = mcp.NewMCPToolStore(s.dbPath, nil) // nil embedder
	s.Require().NoError(err)

	config := map[string]mcp.MCPServerConfig{
		"test-server": {
			ID:                "test-server",
			Enabled:           true,
			Protocol:          "http", // mcp.ProtocolHTTP is "http"
			BaseURL:           s.serverAddr,
			Timeout:           "1s",
			AutoConnect:       false,
			AutoDiscoverTools: false,
		},
	}

	// nil analyzer for integration test (we don't test LLM part)
	s.client = mcp.NewMCPClientManager(s.store, nil, config)
}

func (s *MCPClientIntegrationSuite) TearDownTest() {
	if s.client != nil {
		s.client.DisconnectAll()
	}
	if s.store != nil {
		s.store.Close()
	}
	if s.server != nil {
		s.server.Close()
	}
	// Verify no goroutine leaks
	goleak.VerifyNone(s.T(), goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
}

func (s *MCPClientIntegrationSuite) TestConnectAndUseTools() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Connect
	err := s.client.Connect(ctx, "test-server")
	s.Require().NoError(err)

	// Verify connected state
	serverConn, ok := s.client.GetServer("test-server")
	s.Require().True(ok)
	s.Require().True(serverConn.Transport.IsConnected())
	s.Equal(mcp.ServerStatusConnected, serverConn.Server.Status)

	// 2. Discover Tools
	err = s.client.DiscoverTools(ctx, "test-server")
	s.Require().NoError(err)

	// Verify tools are found
	tools := s.client.GetAllTools()
	s.Require().Len(tools, 1)
	s.Equal("calculator", tools[0].Name)
	s.Equal("test-server/calculator", tools[0].ToolID)

	// 3. Call Tool
	result, err := s.client.CallTool(ctx, "test-server/calculator", map[string]interface{}{
		"a": 5.0,
		"b": 3.0,
	})
	s.Require().NoError(err)
	s.True(result.Success)

	// Check result
	var out map[string]interface{}
	err = json.Unmarshal(result.Output, &out)
	s.Require().NoError(err)
	s.Equal(8.0, out["sum"])

	// 4. Persistence Verification
	// Verify tool is in DB
	dbTool, err := s.store.GetTool(ctx, "test-server/calculator")
	s.Require().NoError(err)
	s.NotNil(dbTool)
	s.Equal("calculator", dbTool.Name)

	// Verify usage stats updated (async, so use eventually)
	s.Eventually(func() bool {
		t, err := s.store.GetTool(ctx, "test-server/calculator")
		return err == nil && t.UsageCount == 1 && t.SuccessCount == 1
	}, 2*time.Second, 100*time.Millisecond)
}

// -----------------------------------------------------------------------------
// Marathon 17: MCP Client Integration Test Gaps
// -----------------------------------------------------------------------------

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify empty ServerID rejects strictly when calling Connect.
func (s *MCPClientIntegrationSuite) TestConnect_EmptyServerID() {
	err := s.client.Connect(context.Background(), "")
	s.Require().Error(err)
	s.Contains(err.Error(), "cannot be empty")
}

// TODO: TEST_GAP: [Type Coercion] Verify invalid argument types (like unmarshalable structs) properly fail JSON marshaling before transmission.
func (s *MCPClientIntegrationSuite) TestCallTool_NilArgs() {
	// CallTool with nil args
	ctx := context.Background()
	s.Require().NoError(s.client.Connect(ctx, "test-server"))
	
	result, err := s.client.CallTool(ctx, "test-server/ping", nil) // ping doesn't take args
	// Wait, the mock server doesn't have a "ping" tool, it has a "ping" method.
	// We'll call a non-existent tool with nil args, it should fail with "Method not found" from server,
	// but it shouldn't crash our client.
	s.Require().NoError(err)
	s.False(result.Success)
	s.Contains(result.Error, "Method not found")
}

// TODO: TEST_GAP: [User Request Extremes] Verify DiscoverTools behaves safely and aborts gracefully when given an extremely large list of tools or malformed empty responses.
func (s *MCPClientIntegrationSuite) TestDiscoverTools_EmptyList() {
	// Let's create an empty mock server
	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Method  string `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
		}
		if req.Method == "tools/list" {
			resp["result"] = map[string]interface{}{"tools": []interface{}{}}
		} else {
			resp["result"] = map[string]interface{}{}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer emptyServer.Close()

	// Update config to use empty server
	s.client = mcp.NewMCPClientManager(s.store, nil, map[string]mcp.MCPServerConfig{
		"empty": {ID: "empty", Enabled: true, Protocol: "http", BaseURL: emptyServer.URL},
	})

	ctx := context.Background()
	s.Require().NoError(s.client.Connect(ctx, "empty"))
	
	// DiscoverTools should not error if the list is empty
	err := s.client.DiscoverTools(ctx, "empty")
	s.Require().NoError(err)
	s.Len(s.client.GetAllTools(), 0)
}

func (s *MCPClientIntegrationSuite) TestConnectAll_NilConfig() {
	client := mcp.NewMCPClientManager(s.store, nil, nil) // nil config
	err := client.ConnectAll(context.Background())
	s.Require().NoError(err) // Should silently succeed
}

func (s *MCPClientIntegrationSuite) TestCallTool_InvalidArgsTypes() {
	ctx := context.Background()
	s.Require().NoError(s.client.Connect(ctx, "test-server"))

	// Pass a channel, which cannot be JSON marshaled
	args := map[string]interface{}{
		"ch": make(chan int),
	}
	_, err := s.client.CallTool(ctx, "test-server/calculator", args)
	s.Require().Error(err)
	s.Contains(err.Error(), "cannot serialize to JSON")
}

func (s *MCPClientIntegrationSuite) TestListTools_Extremes() {
	// Directly inject 10000 tools into a server connection
	s.Require().NoError(s.client.Connect(context.Background(), "test-server"))
	
	conn, ok := s.client.ServersForTest()["test-server"]
	s.Require().True(ok)
	
	tools := make([]*mcp.MCPTool, 10000)
	for i := 0; i < 10000; i++ {
		tools[i] = &mcp.MCPTool{Name: fmt.Sprintf("t%d", i)}
	}
	conn.Tools = tools

	// ListTools should be fast and not hold mutex too long
	start := time.Now()
	schemas, err := s.client.ListTools(context.Background())
	s.Require().NoError(err)
	s.Len(schemas, 10000)
	s.True(time.Since(start) < time.Second) // Should be extremely fast
}

func (s *MCPClientIntegrationSuite) TestConnect_LongTimeout() {
	client := mcp.NewMCPClientManager(s.store, nil, map[string]mcp.MCPServerConfig{
		"test": {ID: "test", Enabled: true, Protocol: "http", BaseURL: s.serverAddr, Timeout: "1000h"},
	})
	err := client.Connect(context.Background(), "test")
	s.Require().NoError(err) // Should succeed parsing long timeout
}

func (s *MCPClientIntegrationSuite) TestConcurrentConnectDisconnect() {
	client := mcp.NewMCPClientManager(s.store, nil, map[string]mcp.MCPServerConfig{
		"test": {ID: "test", Enabled: true, Protocol: "http", BaseURL: s.serverAddr},
	})

	// Run concurrent Connect and Disconnect
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.Connect(context.Background(), "test")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.Disconnect("test")
		}()
	}
	wg.Wait()
}

// TODO: TEST_GAP: [State Conflicts] Verify concurrent execution of CallTool and Disconnect does not trigger deadlocks or nil pointer dereferences.
func (s *MCPClientIntegrationSuite) TestCallToolConcurrentDisconnect() {
	s.Require().NoError(s.client.Connect(context.Background(), "test-server"))

	var wg sync.WaitGroup
	
	// We want to trigger CallTool while Disconnect happens
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.client.Disconnect("test-server")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = s.client.CallTool(context.Background(), "test-server/calculator", nil)
	}()

	wg.Wait()
}
