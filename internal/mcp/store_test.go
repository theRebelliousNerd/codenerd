package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
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

// -----------------------------------------------------------------------------
// Marathon 18: MCP Store Test Gaps
// -----------------------------------------------------------------------------

// TODO: TEST_GAP: [Null/Undefined/Empty] Store initialization does not guarantee the embedder is non-nil, risking panics during SemanticSearch.
// TODO: TEST_GAP: [Null/Undefined/Empty] SaveServer does not validate if server is nil before dereferencing server.ID.
// TODO: TEST_GAP: [Null/Undefined/Empty] GetServer does not reject empty string IDs, resulting in semantically meaningless database queries.
// TODO: TEST_GAP: [Null/Undefined/Empty] SaveTool lacks nil pointer checks and does not enforce structural requirements on tool before insertion.
// TODO: TEST_GAP: [Null/Undefined/Empty] GetTool processes empty string queries identically to valid lookups.
func TestStore_NullEmptyInputs(t *testing.T) {
	ctx := context.Background()
	_, err := NewMCPToolStore("", nil)
	if err == nil {
		t.Error("Expected error with empty dbPath")
	}

	store := newTestStore(t)

	err = store.SaveServer(ctx, nil)
	if err == nil {
		t.Error("Expected error with nil server")
	}
	err = store.SaveServer(ctx, &MCPServer{})
	if err == nil {
		t.Error("Expected error with empty server")
	}

	err = store.SaveTool(ctx, nil)
	if err == nil {
		t.Error("Expected error with nil tool")
	}
	err = store.SaveTool(ctx, &MCPTool{})
	if err == nil {
		t.Error("Expected error with empty tool")
	}

	_, err = store.GetServer(ctx, "")
	if err == nil {
		t.Error("Expected error with empty serverID")
	}

	_, err = store.GetTool(ctx, "")
	if err == nil {
		t.Error("Expected error with empty toolID")
	}

	_, err = store.SemanticSearch(ctx, nil, 10)
	if err == nil {
		t.Error("Expected error with nil queryEmbedding")
	}
	_, err = store.SemanticSearch(ctx, []float32{}, 10)
	if err == nil {
		t.Error("Expected error with empty queryEmbedding")
	}

	err = store.RecordToolUsage(ctx, "", true, 100)
	if err == nil {
		t.Error("Expected error with empty toolID")
	}
}

// TODO: TEST_GAP: [Type Coercion] GetServer JSON unmarshaling silently fails on corrupted strings, yielding empty properties instead of errors.
// TODO: TEST_GAP: [Type Coercion] SaveTool blindly marshals float32 embeddings to bytes, which can lead to retrieval corruption if sizes are misaligned.
func TestStore_TypeCoercion(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// Direct DB injection of invalid JSON
	store.SaveServer(ctx, &MCPServer{ID: "s1", Endpoint: "e", Protocol: "p"})
	_, err := store.db.Exec(`UPDATE mcp_servers SET capabilities = 'INVALID' WHERE server_id = 's1'`)
	if err != nil {
		t.Fatalf("Failed to inject bad JSON: %v", err)
	}

	server, err := store.GetServer(ctx, "s1")
	if err != nil {
		t.Errorf("GetServer failed on invalid JSON: %v", err)
	}
	if len(server.Capabilities) != 0 {
		t.Errorf("Expected empty capabilities due to unmarshal failure, got %v", server.Capabilities)
	}

	// Corrupt vector data
	store.SaveTool(ctx, &MCPTool{ToolID: "t1", ServerID: "s1", Name: "t1", Embedding: []float32{1, 2}})
	_, err = store.db.Exec(`UPDATE mcp_tools SET embedding = x'00' WHERE tool_id = 't1'`)
	if err != nil {
		t.Fatalf("Failed to inject bad blob: %v", err)
	}

	tool, err := store.GetTool(ctx, "t1")
	if err != nil {
		t.Errorf("GetTool failed on corrupt blob: %v", err)
	}
	if len(tool.Embedding) != 0 {
		t.Errorf("Expected empty embedding, got %v", tool.Embedding)
	}

	// Cosine similarity length mismatch
	sim := cosineSimilarity([]float32{1, 0}, []float32{1, 0, 0})
	if sim != 0 {
		t.Errorf("Expected 0 similarity on mismatched length, got %f", sim)
	}

	// Semantic search with topK <= 0
	_, err = store.SemanticSearch(ctx, []float32{1, 0}, 0)
	if err == nil {
		t.Errorf("Expected error with topK <= 0")
	}
}

// TODO: TEST_GAP: [User Request Extremes] SaveServer allows arbitrary length strings for properties like Endpoint and Capabilities, risking memory exhaustion.
// TODO: TEST_GAP: [User Request Extremes] SemanticSearch does not bound the topK parameter; extreme positive values cause OOMs, while zero/negative values are not rejected.
// TODO: TEST_GAP: [User Request Extremes] RecordToolUsage can suffer from signed integer overflow if adversarial tools consistently report massive latency metrics.
func TestStore_UserRequestExtremes(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	store.SaveServer(ctx, &MCPServer{ID: "s1", Endpoint: "e", Protocol: "p"})

	// Massive latency overflow test
	store.SaveTool(ctx, &MCPTool{ToolID: "t1", ServerID: "s1", Name: "t1"})
	store.RecordToolUsage(ctx, "t1", true, 9223372036854775807) // Max Int64
	store.RecordToolUsage(ctx, "t1", true, 9223372036854775807)

	tool, err := store.GetTool(ctx, "t1")
	if err != nil {
		t.Errorf("GetTool failed: %v", err)
	}
	if tool.AvgLatencyMs < 0 {
		t.Errorf("Average latency overflowed: %d", tool.AvgLatencyMs)
	}
}

// TODO: TEST_GAP: [State Conflicts] Concurrent schema initialization across multiple processes without file locking can lead to database corruption.
// TODO: TEST_GAP: [State Conflicts] RecordToolUsage executes updates that silently fail if the tool was concurrently deleted by another transaction.
func TestStore_StateConflicts(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	store.SaveServer(ctx, &MCPServer{ID: "s1", Endpoint: "e", Protocol: "p"})
	store.SaveTool(ctx, &MCPTool{ToolID: "t1", ServerID: "s1", Name: "t1", Embedding: []float32{1, 2}})

	// Concurrent SaveTool, RecordToolUsage, SemanticSearch
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_ = store.SaveTool(ctx, &MCPTool{ToolID: "t1", ServerID: "s1", Name: "t1", Embedding: []float32{1, 2}})
		})
		wg.Go(func() {
			_ = store.RecordToolUsage(ctx, "t1", true, 100)
		})
		wg.Go(func() {
			_, _ = store.SemanticSearch(ctx, []float32{1, 2}, 10)
		})
	}
	wg.Wait()

	// Update non-existent IDs
	err := store.RecordToolUsage(ctx, "non-existent", true, 100)
	if err != nil {
		t.Errorf("Did not expect error on non-existent RecordToolUsage, got %v", err)
	}

	err = store.UpdateServerStatus(ctx, "non-existent", ServerStatusConnected)
	if err != nil {
		t.Errorf("Did not expect error on non-existent UpdateServerStatus, got %v", err)
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
