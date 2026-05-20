package store

import (
	"path/filepath"
	"testing"
)

func TestLocalVector_StoreVector(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "local_vector.db")
	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Store vector
	metadata := map[string]interface{}{"key": "value"}
	err = store.StoreVector("test content", metadata)
	if err != nil {
		t.Errorf("StoreVector failed: %v", err)
	}

	// Verify it was stored by querying it back using VectorRecall
	results, err := store.VectorRecall("test", 10)
	if err != nil {
		t.Errorf("VectorRecall failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	} else {
		if results[0].Content != "test content" {
			t.Errorf("Expected 'test content', got %q", results[0].Content)
		}
		if results[0].Metadata["key"] != "value" {
			t.Errorf("Expected metadata 'key' = 'value', got %v", results[0].Metadata["key"])
		}
	}
}
