package store

import (
	"testing"
)

func TestNewLocalStore(t *testing.T) {
	// Use in-memory database
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	if store.db == nil {
		t.Error("Database connection is nil")
	}

	if store.GetDB() == nil {
		t.Error("GetDB returned nil")
	}

	// Check if tables exist
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	// Just check a few key tables to ensure schema initialization ran
	requiredTables := []string{"vectors", "knowledge_graph", "cold_storage", "session_history"}
	for _, table := range requiredTables {
		if _, ok := stats[table]; !ok {
			t.Errorf("Stats missing table: %s", table)
		}
	}
}

func TestGetTraceStore(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	traceStore := store.GetTraceStore()
	if traceStore == nil {
		t.Error("GetTraceStore returned nil")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
