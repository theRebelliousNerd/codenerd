package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// --- parseToolID ---

func TestParseToolID_WhenValidSlash_ShouldSplit(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantServer string
		wantTool   string
	}{
		{"simple", "server/tool", "server", "tool"},
		{"nested", "org/server/tool", "org/server", "tool"},
		{"no_slash", "toolonly", "", "toolonly"},
		{"trailing_slash", "server/", "server", ""},
		{"leading_slash", "/tool", "", "tool"},
		{"empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, tool := parseToolID(tt.input)
			if server != tt.wantServer {
				t.Errorf("parseToolID(%q) server = %q, want %q", tt.input, server, tt.wantServer)
			}
			if tool != tt.wantTool {
				t.Errorf("parseToolID(%q) tool = %q, want %q", tt.input, tool, tt.wantTool)
			}
		})
	}
}

// --- truncate ---

func TestTruncate_WhenVariousLengths_ShouldTruncateCorrectly(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short_enough", "hello", 10, "hello"},
		{"exact_length", "hello", 5, "hello"},
		{"needs_truncation", "hello world", 8, "hello..."},
		{"very_short_max", "hello", 2, "he"},
		{"zero_max", "hello", 0, ""},
		{"max_3", "hello", 3, "hel"},
		{"max_4", "hello", 4, "h..."},
		{"empty_string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// --- NewMCPClientManager ---

func TestNewMCPClientManager_WhenNilArgs_ShouldNotPanic(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	if mgr == nil {
		t.Fatal("NewMCPClientManager returned nil")
	}
	if mgr.servers == nil {
		t.Error("servers map should be initialized")
	}
}

func TestNewMCPClientManager_WhenConfigProvided_ShouldStoreConfig(t *testing.T) {
	config := map[string]MCPServerConfig{
		"test": {ID: "test", Enabled: true, Protocol: "http"},
	}
	mgr := NewMCPClientManager(nil, nil, config)
	if len(mgr.config) != 1 {
		t.Errorf("expected 1 config entry, got %d", len(mgr.config))
	}
}

// --- Callbacks ---

func TestSetToolSelectionConfig_ShouldUpdate(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	custom := ToolSelectionConfig{
		SkeletonThreshold: 50,
		FullThreshold:     30,
		LogicWeight:       0.5,
		VectorWeight:      0.5,
	}
	mgr.SetToolSelectionConfig(custom)

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	if mgr.selection.SkeletonThreshold != 50 {
		t.Errorf("expected SkeletonThreshold=50, got %d", mgr.selection.SkeletonThreshold)
	}
}

func TestSetOnToolDiscovered_ShouldSetCallback(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	called := false
	mgr.SetOnToolDiscovered(func(tool *MCPTool) {
		called = true
	})

	mgr.mu.RLock()
	cb := mgr.onToolDiscovered
	mgr.mu.RUnlock()

	if cb == nil {
		t.Fatal("callback should be set")
	}
	cb(&MCPTool{Name: "test"})
	if !called {
		t.Error("callback should have been called")
	}
}

func TestSetOnServerStatus_ShouldSetCallback(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	var receivedID string
	var receivedStatus ServerStatus
	mgr.SetOnServerStatus(func(serverID string, status ServerStatus) {
		receivedID = serverID
		receivedStatus = status
	})

	mgr.mu.RLock()
	cb := mgr.onServerStatus
	mgr.mu.RUnlock()

	if cb == nil {
		t.Fatal("callback should be set")
	}
	cb("srv1", ServerStatusConnected)
	if receivedID != "srv1" {
		t.Errorf("expected server ID srv1, got %s", receivedID)
	}
	if receivedStatus != ServerStatusConnected {
		t.Errorf("expected status Connected, got %s", receivedStatus)
	}
}

// --- Connect error paths ---

func TestConnect_WhenEmptyServerID_ShouldError(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	err := mgr.Connect(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty server ID")
	}
	if err.Error() != "server ID cannot be empty" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnect_WhenUnknownServer_ShouldError(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	err := mgr.Connect(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
}

func TestConnect_WhenUnsupportedProtocol_ShouldError(t *testing.T) {
	config := map[string]MCPServerConfig{
		"bad": {ID: "bad", Enabled: true, Protocol: "grpc"},
	}
	mgr := NewMCPClientManager(nil, nil, config)
	err := mgr.Connect(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
}

// --- Disconnect error paths ---

func TestDisconnect_WhenNotConnected_ShouldError(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	err := mgr.Disconnect("nonexistent")
	if err == nil {
		t.Fatal("expected error for disconnecting nonexistent server")
	}
}

// --- mockTransport for unit tests ---

type mockTransport struct {
	connected    bool
	tools        []MCPToolSchema
	caps         *MCPCapabilities
	connectErr   error
	listErr      error
	callErr      error
	callResult   *MCPCallResult
	capsErr      error
	disconnected bool
}

func (m *mockTransport) Connect(ctx context.Context) error {
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}

func (m *mockTransport) Disconnect() error {
	m.connected = false
	m.disconnected = true
	return nil
}

func (m *mockTransport) ListTools(ctx context.Context) ([]MCPToolSchema, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.tools, nil
}

func (m *mockTransport) CallTool(ctx context.Context, name string, args map[string]any) (*MCPCallResult, error) {
	if m.callErr != nil {
		return nil, m.callErr
	}
	if m.callResult != nil {
		return m.callResult, nil
	}
	return &MCPCallResult{Success: true, Output: json.RawMessage(`{"ok":true}`)}, nil
}

func (m *mockTransport) GetCapabilities(ctx context.Context) (*MCPCapabilities, error) {
	if m.capsErr != nil {
		return nil, m.capsErr
	}
	return m.caps, nil
}

func (m *mockTransport) Ping(ctx context.Context) error {
	return nil
}

func (m *mockTransport) IsConnected() bool {
	return m.connected
}

// --- GetServer ---

func TestGetServer_WhenExists_ShouldReturnConnection(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mock := &mockTransport{connected: true}
	mgr.servers["test"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "test"},
		Transport: mock,
	}

	conn, ok := mgr.GetServer("test")
	if !ok {
		t.Fatal("expected server to be found")
	}
	if conn.Server.ID != "test" {
		t.Errorf("expected server ID 'test', got %q", conn.Server.ID)
	}
}

func TestGetServer_WhenNotExists_ShouldReturnFalse(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	_, ok := mgr.GetServer("nonexistent")
	if ok {
		t.Fatal("expected false for nonexistent server")
	}
}

// --- GetConnectedServers ---

func TestGetConnectedServers_ShouldReturnOnlyConnected(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mgr.servers["a"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "a"},
		Transport: &mockTransport{connected: true},
	}
	mgr.servers["b"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "b"},
		Transport: &mockTransport{connected: false},
	}

	servers := mgr.GetConnectedServers()
	if len(servers) != 1 {
		t.Fatalf("expected 1 connected server, got %d", len(servers))
	}
	if servers[0] != "a" {
		t.Errorf("expected server 'a', got %q", servers[0])
	}
}

func TestGetConnectedServers_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	servers := mgr.GetConnectedServers()
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

// --- GetAllTools ---

func TestGetAllTools_ShouldAggregateAcrossServers(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mgr.servers["a"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "a"},
		Transport: &mockTransport{connected: true},
		Tools:     []*MCPTool{{Name: "t1"}, {Name: "t2"}},
	}
	mgr.servers["b"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "b"},
		Transport: &mockTransport{connected: true},
		Tools:     []*MCPTool{{Name: "t3"}},
	}

	tools := mgr.GetAllTools()
	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}
}

func TestGetAllTools_WhenNoServers_ShouldReturnNil(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	tools := mgr.GetAllTools()
	if tools != nil {
		t.Errorf("expected nil tools, got %v", tools)
	}
}

// --- ListTools ---

func TestListTools_WhenNoToolsCached_ShouldError(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	_, err := mgr.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected error when no tools cached")
	}
}

func TestListTools_WhenToolsExist_ShouldReturnSchemas(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mgr.servers["srv"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "srv"},
		Transport: &mockTransport{connected: true},
		Tools: []*MCPTool{
			{Name: "tool1", Description: "does stuff", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	}

	schemas, err := mgr.ListTools(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	if schemas[0].Name != "tool1" {
		t.Errorf("expected name 'tool1', got %q", schemas[0].Name)
	}
}

// --- CallTool error paths ---

func TestCallTool_WhenInvalidToolID_ShouldError(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	_, err := mgr.CallTool(context.Background(), "noserver", nil)
	if err == nil {
		t.Fatal("expected error for invalid tool ID")
	}
}

func TestCallTool_WhenServerNotConnected_ShouldReturnFailResult(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	// No server registered
	result, err := mgr.CallTool(context.Background(), "srv/tool", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false for disconnected server")
	}
}

func TestCallTool_WhenTransportFails_ShouldReturnError(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mgr.servers["srv"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "srv"},
		Transport: &mockTransport{connected: true, callErr: fmt.Errorf("transport error")},
	}

	_, err := mgr.CallTool(context.Background(), "srv/tool", nil)
	if err == nil {
		t.Fatal("expected error from transport failure")
	}
}

func TestCallTool_WhenUnserializableArgs_ShouldError(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mgr.servers["srv"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "srv"},
		Transport: &mockTransport{connected: true},
	}

	// Channels can't be serialized to JSON
	args := map[string]any{"ch": make(chan int)}
	_, err := mgr.CallTool(context.Background(), "srv/tool", args)
	if err == nil {
		t.Fatal("expected error for unserializable args")
	}
}

// --- ConnectAll ---

func TestConnectAll_WhenNoAutoConnect_ShouldSkip(t *testing.T) {
	config := map[string]MCPServerConfig{
		"manual": {ID: "manual", Enabled: true, AutoConnect: false, Protocol: "http"},
	}
	mgr := NewMCPClientManager(nil, nil, config)
	err := mgr.ConnectAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No servers should be connected
	if len(mgr.GetConnectedServers()) != 0 {
		t.Error("expected no connected servers when auto_connect=false")
	}
}

func TestConnectAll_WhenDisabled_ShouldSkip(t *testing.T) {
	config := map[string]MCPServerConfig{
		"disabled": {ID: "disabled", Enabled: false, AutoConnect: true, Protocol: "http"},
	}
	mgr := NewMCPClientManager(nil, nil, config)
	err := mgr.ConnectAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DisconnectAll ---

func TestDisconnectAll_ShouldDisconnectAll(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mock1 := &mockTransport{connected: true}
	mock2 := &mockTransport{connected: true}
	mgr.servers["a"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "a"},
		Transport: mock1,
	}
	mgr.servers["b"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "b"},
		Transport: mock2,
	}

	mgr.DisconnectAll()

	if mock1.connected || mock2.connected {
		t.Error("all transports should be disconnected")
	}
}

// --- DiscoverTools ---

func TestDiscoverTools_WhenNotConnected_ShouldError(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	err := mgr.DiscoverTools(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for discovering tools on nonexistent server")
	}
}

func TestDiscoverTools_WhenTransportListFails_ShouldError(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mgr.servers["srv"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "srv"},
		Transport: &mockTransport{connected: true, listErr: fmt.Errorf("list failed")},
	}

	err := mgr.DiscoverTools(context.Background(), "srv")
	if err == nil {
		t.Fatal("expected error when ListTools fails")
	}
}

func TestDiscoverTools_WhenEmptyToolList_ShouldSucceed(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mgr.servers["srv"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "srv"},
		Transport: &mockTransport{connected: true, tools: []MCPToolSchema{}},
	}

	err := mgr.DiscoverTools(context.Background(), "srv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscoverTools_WhenToolsFound_ShouldProcess(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	mgr.servers["srv"] = &MCPServerConnection{
		Server: &MCPServer{ID: "srv"},
		Transport: &mockTransport{
			connected: true,
			tools: []MCPToolSchema{
				{Name: "calc", Description: "Calculator tool"},
			},
		},
	}

	var discoveredTool *MCPTool
	mgr.SetOnToolDiscovered(func(tool *MCPTool) {
		discoveredTool = tool
	})

	err := mgr.DiscoverTools(context.Background(), "srv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check tools were stored in connection
	conn, _ := mgr.GetServer("srv")
	if len(conn.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(conn.Tools))
	}
	if conn.Tools[0].Name != "calc" {
		t.Errorf("expected tool name 'calc', got %q", conn.Tools[0].Name)
	}

	// Check callback was called
	if discoveredTool == nil {
		t.Error("expected tool discovered callback to be called")
	}
}

// --- processToolSchema ---

func TestProcessToolSchema_WhenNoAnalyzer_ShouldSetCondensed(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	schema := MCPToolSchema{
		Name:        "long_tool",
		Description: "This is a very long description that should be truncated to fit within the condensed limit for display purposes in the LLM context window",
	}

	tool, err := mgr.processToolSchema(context.Background(), "srv", schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool.Condensed == "" {
		t.Error("expected condensed description to be set")
	}
	if len(tool.Condensed) > 83 { // 80 + "..."
		t.Errorf("expected condensed <= 83 chars, got %d", len(tool.Condensed))
	}
}

func TestProcessToolSchema_WhenEmptyDescription_ShouldNotSetCondensed(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	schema := MCPToolSchema{
		Name:        "no_desc",
		Description: "",
	}

	tool, err := mgr.processToolSchema(context.Background(), "srv", schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool.Condensed != "" {
		t.Errorf("expected empty condensed for empty description, got %q", tool.Condensed)
	}
}

// --- updateServerStatus ---

func TestUpdateServerStatus_WhenCallbackSet_ShouldNotify(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	var gotID string
	var gotStatus ServerStatus
	mgr.SetOnServerStatus(func(id string, s ServerStatus) {
		gotID = id
		gotStatus = s
	})

	mgr.updateServerStatus("srv1", ServerStatusError)
	if gotID != "srv1" {
		t.Errorf("expected ID 'srv1', got %q", gotID)
	}
	if gotStatus != ServerStatusError {
		t.Errorf("expected status Error, got %s", gotStatus)
	}
}

func TestUpdateServerStatus_WhenNoCallback_ShouldNotPanic(t *testing.T) {
	mgr := NewMCPClientManager(nil, nil, nil)
	// Should not panic
	mgr.updateServerStatus("srv1", ServerStatusConnected)
}

// --- DefaultToolSelectionConfig ---

func TestDefaultToolSelectionConfig_ShouldReturnSensibleDefaults(t *testing.T) {
	cfg := DefaultToolSelectionConfig()
	if cfg.SkeletonThreshold != 90 {
		t.Errorf("expected SkeletonThreshold=90, got %d", cfg.SkeletonThreshold)
	}
	if cfg.FullThreshold != 70 {
		t.Errorf("expected FullThreshold=70, got %d", cfg.FullThreshold)
	}
	if cfg.LogicWeight != 0.7 {
		t.Errorf("expected LogicWeight=0.7, got %f", cfg.LogicWeight)
	}
	if cfg.VectorWeight != 0.3 {
		t.Errorf("expected VectorWeight=0.3, got %f", cfg.VectorWeight)
	}
	if cfg.MaxFullTools != 10 {
		t.Errorf("expected MaxFullTools=10, got %d", cfg.MaxFullTools)
	}
	if cfg.TokenBudget != 4000 {
		t.Errorf("expected TokenBudget=4000, got %d", cfg.TokenBudget)
	}
}

// --- ToolAvailableEntry ---

func TestToolAvailableEntry_IsMCPTool(t *testing.T) {
	entry := ToolAvailableEntry{Type: "mcp"}
	if !entry.IsMCPTool() {
		t.Error("expected IsMCPTool() == true for type 'mcp'")
	}

	entry2 := ToolAvailableEntry{Type: "static"}
	if entry2.IsMCPTool() {
		t.Error("expected IsMCPTool() == false for type 'static'")
	}

	entry3 := ToolAvailableEntry{}
	if entry3.IsMCPTool() {
		t.Error("expected IsMCPTool() == false for empty type")
	}
}

// --- Connect with already-connected transport ---

func TestConnect_WhenAlreadyConnected_ShouldReturnNil(t *testing.T) {
	config := map[string]MCPServerConfig{
		"srv": {ID: "srv", Enabled: true, Protocol: "http", BaseURL: "http://localhost:1234"},
	}
	mgr := NewMCPClientManager(nil, nil, config)
	mock := &mockTransport{connected: true}
	mgr.servers["srv"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "srv"},
		Transport: mock,
	}

	err := mgr.Connect(context.Background(), "srv")
	if err != nil {
		t.Fatalf("expected nil error for already-connected server, got: %v", err)
	}
}

// --- Connect timeout parsing ---

func TestConnect_WhenInvalidTimeout_ShouldUseDefault(t *testing.T) {
	config := map[string]MCPServerConfig{
		"srv": {ID: "srv", Enabled: true, Protocol: "http", BaseURL: "http://192.0.2.1:1", Timeout: "not-a-duration"},
	}
	mgr := NewMCPClientManager(nil, nil, config)
	// This will try to connect to a non-routable IP, which will fail, but it should parse the timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	// We just verify it doesn't panic on the bad timeout — the connect itself will timeout/fail
	_ = mgr.Connect(ctx, "srv")
}
