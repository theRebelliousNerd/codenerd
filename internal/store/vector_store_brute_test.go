package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestVectorStore_BruteForceMethods(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create LocalStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	_ = ctx

	// Insert some embeddings
	store.db.Exec("INSERT INTO vectors (content, metadata, embedding) VALUES ('text', '{\"path\":\"path1\", \"k\":\"v\"}', '[0.1, 0.2, 0.3, 0.4]')")
	store.db.Exec("INSERT INTO vectors (content, metadata, embedding) VALUES ('text', '{\"path\":\"path2\", \"k\":\"v2\"}', '[0.5, 0.6, 0.7, 0.8]')")

	// Test vectorRecallBruteForce
	results, err := store.vectorRecallBruteForce("test_query", []float32{0.1, 0.2, 0.3, 0.4}, 10)
	if err != nil {
		t.Fatalf("vectorRecallBruteForce failed: %v", err)
	}
	if len(results) == 0 {
		t.Errorf("Expected results from brute force")
	}

	// Test vectorRecallBruteForceByPaths
	paths := []string{"path1", "path2"}
	_, _ = store.vectorRecallBruteForceByPaths("test_query", []float32{0.1, 0.2, 0.3, 0.4}, 10, paths)

	// Test vectorRecallBruteForceFiltered
	filteredResults, err := store.vectorRecallBruteForceFiltered("test_query", []float32{0.1, 0.2, 0.3, 0.4}, 10, "k", "v")
	if err != nil {
		t.Fatalf("vectorRecallBruteForceFiltered failed: %v", err)
	}
	if len(filteredResults) == 0 {
		t.Errorf("Expected filtered results")
	}
}

func TestVectorStore_FilterUtils(t *testing.T) {
	// buildPathFilteredQuery
	query, args := buildPathFilteredQuery([]string{"path1", "path2"})
	expected := "SELECT id, content, embedding, metadata, created_at FROM vectors WHERE embedding IS NOT NULL AND (json_extract(metadata, ?) = ? OR json_extract(metadata, ?) = ?)"
	if query != expected {
		t.Errorf("Unexpected query: %s", query)
	}
	if len(args) != 4 || args[0] != "$.path" || args[1] != "path1" || args[2] != "$.path" || args[3] != "path2" {
		t.Errorf("Unexpected args: %v", args)
	}

	query2, args2 := buildPathFilteredQuery(nil)
	expected2 := "SELECT id, content, embedding, metadata, created_at FROM vectors WHERE embedding IS NOT NULL"
	if query2 != expected2 || len(args2) != 0 {
		t.Errorf("Unexpected empty paths query: %s", query2)
	}

	// filterByPaths
	cands := []VectorEntry{
		{Metadata: map[string]any{"path": "path1"}},
	}
	filtered := filterByPaths(cands, nil)
	if len(filtered) != 0 {
		t.Errorf("Expected 0 results for empty paths, got %d", len(filtered))
	}
	filtered2 := filterByPaths(cands, []string{"path1"})
	if len(filtered2) != 1 {
		t.Errorf("Expected 1 result for valid path, got %d", len(filtered2))
	}
	filtered3 := filterByPaths(cands, []string{"path2"})
	if len(filtered3) != 0 {
		t.Errorf("Expected 0 results for invalid path, got %d", len(filtered3))
	}
}
