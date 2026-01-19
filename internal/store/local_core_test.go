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

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float64
		b    []float64
		want float64
	}{
		{
			name: "Identical",
			a:    []float64{1, 0, 0},
			b:    []float64{1, 0, 0},
			want: 1.0,
		},
		{
			name: "Orthogonal",
			a:    []float64{1, 0, 0},
			b:    []float64{0, 1, 0},
			want: 0.0,
		},
		{
			name: "Opposite",
			a:    []float64{1, 0, 0},
			b:    []float64{-1, 0, 0},
			want: -1.0,
		},
		{
			name: "Length Mismatch",
			a:    []float64{1, 0},
			b:    []float64{1, 0, 0},
			want: 0.0,
		},
		{
			name: "Zero Vector",
			a:    []float64{0, 0, 0},
			b:    []float64{1, 1, 1},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.a, tt.b)
			if abs(got-tt.want) > 0.0001 {
				t.Errorf("CosineSimilarity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
