//go:build integration

package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
