package store_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

// BenchmarkTraversePath measures the performance of path traversal.
// Optimization: Replaced path-copying BFS with predecessor-map BFS.
// Before: ~6.5ms/op, 311 KB/op, 6202 allocs/op
// After:  ~4.1ms/op, 190 KB/op, 6123 allocs/op
// Improvement: ~37% faster, ~39% less memory.
func BenchmarkTraversePath(b *testing.B) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "bench_traversal")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "bench.db")
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// Create a graph: 10x10 Grid for consistent branching
	// Node-i-j connects to Node-i+1-j and Node-i-j+1
	// Create paths from 0-0 to 9-9.

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			curr := fmt.Sprintf("%d-%d", i, j)
			if i < 9 {
				next := fmt.Sprintf("%d-%d", i+1, j)
				_ = db.StoreLink(curr, "right", next, 1.0, nil)
			}
			if j < 9 {
				next := fmt.Sprintf("%d-%d", i, j+1)
				_ = db.StoreLink(curr, "down", next, 1.0, nil)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.TraversePath("0-0", "9-9", 20)
		if err != nil {
			b.Fatal(err)
		}
	}
}
