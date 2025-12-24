package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestMCPToolStoreServerAndToolLifecycle(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	server := &MCPServer{
		ID:            "server-1",
		Endpoint:      "http://localhost:9999",
		Protocol:      ProtocolHTTP,
		Name:          "Server One",
		Version:       "1.0.0",
		Status:        ServerStatusConnected,
		Capabilities:  []string{"tools"},
		DiscoveredAt:  time.Now(),
		LastConnected: time.Now(),
	}

	if err := store.SaveServer(ctx, server); err != nil {
		t.Fatalf("SaveServer failed: %v", err)
	}

	gotServer, err := store.GetServer(ctx, server.ID)
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if gotServer == nil || gotServer.Name != server.Name {
		t.Fatalf("unexpected server: %+v", gotServer)
	}

	tool := &MCPTool{
		ToolID:          "tool-1",
		ServerID:        server.ID,
		Name:            "read_file",
		Description:     "Read file contents",
		InputSchema:     json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		OutputSchema:    json.RawMessage(`{"type":"object"}`),
		Categories:      []string{"filesystem"},
		Capabilities:    []string{"/read"},
		Domain:          "/general",
		ShardAffinities: map[string]int{"coder": 80},
		UseCases:        []string{"read files"},
		Condensed:       "Read file contents",
		Embedding:       []float32{1, 0},
		EmbeddingModel:  "test",
		RegisteredAt:    time.Now(),
		AnalyzedAt:      time.Now(),
	}

	if err := store.SaveTool(ctx, tool); err != nil {
		t.Fatalf("SaveTool failed: %v", err)
	}

	gotTool, err := store.GetTool(ctx, tool.ToolID)
	if err != nil {
		t.Fatalf("GetTool failed: %v", err)
	}
	if gotTool == nil || gotTool.Name != tool.Name {
		t.Fatalf("unexpected tool: %+v", gotTool)
	}
	if len(gotTool.Embedding) != 2 || gotTool.Embedding[0] != 1 {
		t.Fatalf("unexpected embedding: %v", gotTool.Embedding)
	}

	if err := store.RecordToolUsage(ctx, tool.ToolID, true, 100); err != nil {
		t.Fatalf("RecordToolUsage failed: %v", err)
	}
	updated, err := store.GetTool(ctx, tool.ToolID)
	if err != nil {
		t.Fatalf("GetTool after usage failed: %v", err)
	}
	if updated.UsageCount != 1 || updated.SuccessCount != 1 {
		t.Fatalf("unexpected usage counts: %+v", updated)
	}
	if updated.AvgLatencyMs != 100 {
		t.Fatalf("AvgLatencyMs = %d, want 100", updated.AvgLatencyMs)
	}

	if err := store.RecordToolUsage(ctx, tool.ToolID, false, 300); err != nil {
		t.Fatalf("RecordToolUsage second failed: %v", err)
	}
	updated, _ = store.GetTool(ctx, tool.ToolID)
	if updated.UsageCount != 2 || updated.SuccessCount != 1 {
		t.Fatalf("unexpected usage counts after second call: %+v", updated)
	}
	if updated.AvgLatencyMs != 200 {
		t.Fatalf("AvgLatencyMs = %d, want 200", updated.AvgLatencyMs)
	}

	tool2 := &MCPTool{
		ToolID:       "tool-2",
		ServerID:     server.ID,
		Name:         "write_file",
		Description:  "Write file",
		Condensed:    "Write file contents",
		Embedding:    []float32{0, 1},
		RegisteredAt: time.Now(),
		AnalyzedAt:   time.Now(),
	}
	if err := store.SaveTool(ctx, tool2); err != nil {
		t.Fatalf("SaveTool tool2 failed: %v", err)
	}

	all, err := store.GetAllTools(ctx)
	if err != nil {
		t.Fatalf("GetAllTools failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("GetAllTools len = %d, want 2", len(all))
	}

	results, err := store.SemanticSearch(ctx, []float32{1, 0}, 2)
	if err != nil {
		t.Fatalf("SemanticSearch failed: %v", err)
	}
	if len(results) == 0 || results[0].ToolID != tool.ToolID {
		t.Fatalf("unexpected semantic search results: %+v", results)
	}
}

func newTestStore(t *testing.T) *MCPToolStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "mcp.db")
	store, err := NewMCPToolStore(dbPath, nil)
	if err != nil {
		t.Fatalf("NewMCPToolStore failed: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}
