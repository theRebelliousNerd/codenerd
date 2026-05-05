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

// TODO: TEST_GAP: Null/Undefined/Empty Inputs
// 1. `NewMCPToolStore` with an empty string `dbPath`.
// 2. `SaveServer` with a `nil` MCPServer or a server with empty ID, Endpoint, Protocol.
// 3. `SaveTool` with a `nil` MCPTool or a tool with empty ToolID, ServerID, Name.
// 4. `GetServer` and `GetTool` with empty string IDs.
// 5. `SemanticSearch` with `nil` or empty `queryEmbedding` slice.
// 6. `RecordToolUsage` with empty `toolID`.

// TODO: TEST_GAP: Type Coercion
// 1. DB containing invalid JSON strings in fields like `input_schema`, `capabilities`, `categories`.
//    Verify that `json.Unmarshal` failures do not crash the `GetTool` / `GetServer` methods and either error gracefully or return partial objects.
// 2. `float32SliceToBytes` and `bytesToFloat32Slice` with byte slices that are not multiples of 4 (e.g., corrupted BLOB data in SQLite).
// 3. Negative `usage_count` or `avg_latency_ms` data pre-existing in the database (e.g. from manual edits or bug).
// 4. `cosineSimilarity` behavior when one slice is empty and the other is not (slices of different lengths).
// 5. `SemanticSearch` with `topK` <= 0.

// TODO: TEST_GAP: User Request Extremes
// 1. Saving an MCPTool with a massive `Embedding` slice (e.g., 100,000 dimensions). Does it hit SQLite BLOB limits or memory issues during float32 -> byte conversion?
// 2. Saving 100,000 tools to the same server and querying them.
// 3. Very large strings (megabytes) for `output_schema` or `input_schema`.
// 4. `RecordToolUsage` with massive `latencyMs` causing integer overflow in the `avg_latency_ms` moving average calculation `((avg_latency_ms * usage_count) + latencyMs) / (usage_count + 1)`.
// 5. `SemanticSearch` with an extreme `topK` value (e.g. `math.MaxInt32`).
// 6. Very long DB paths for `NewMCPToolStore` causing SQLite connection failures.

// TODO: TEST_GAP: State Conflicts
// 1. Concurrent `SaveTool`, `RecordToolUsage`, and `SemanticSearch` operations on the same tool to verify `sync.RWMutex` combined with SQLite WAL mode handles concurrency safely without `database is locked` errors.
// 2. Calling `RecordToolUsage` on a `toolID` that does not exist in the DB.
// 3. Calling `UpdateServerStatus` on a `serverID` that does not exist.
// 4. Updating a tool's vector embedding concurrently while `SemanticSearch` is iterating over `mcp_tools` (brute force) or `mcp_tool_vec` (vec0).
// 5. Instantiating multiple `MCPToolStore` instances pointing to the same file concurrently.

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
