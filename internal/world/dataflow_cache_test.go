package world

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
)

func TestDataFlowCache_GetOrCompute(t *testing.T) {
	// Create temp directory for cache
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "dataflow_cache")

	cache, err := NewDataFlowCache(cacheDir)
	if err != nil {
		t.Fatalf("NewDataFlowCache() error = %v", err)
	}

	tests := []struct {
		name        string
		file        string
		content     []byte
		wantHit     bool
		computeFunc func() []core.Fact
	}{
		{
			name:    "first access is cache miss",
			file:    "/test/file1.go",
			content: []byte("package main\nfunc main() {}"),
			wantHit: false,
			computeFunc: func() []core.Fact {
				return []core.Fact{
					{Predicate: "assigns", Args: []interface{}{core.MangleAtom("/x"), core.MangleAtom("/nullable"), "/test/file1.go", int64(10)}},
				}
			},
		},
		{
			name:    "same content is cache hit",
			file:    "/test/file1.go",
			content: []byte("package main\nfunc main() {}"),
			wantHit: true,
			computeFunc: func() []core.Fact {
				t.Error("compute should not be called on cache hit")
				return nil
			},
		},
		{
			name:    "modified content is cache miss",
			file:    "/test/file1.go",
			content: []byte("package main\nfunc main() { x := 1 }"),
			wantHit: false,
			computeFunc: func() []core.Fact {
				return []core.Fact{
					{Predicate: "assigns", Args: []interface{}{core.MangleAtom("/y"), core.MangleAtom("/error"), "/test/file1.go", int64(20)}},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statsBefore := cache.Stats()
			facts := cache.GetOrCompute(tt.file, tt.content, tt.computeFunc)
			statsAfter := cache.Stats()

			if tt.wantHit {
				if statsAfter.Hits != statsBefore.Hits+1 {
					t.Errorf("expected cache hit, but hits did not increase")
				}
			} else {
				if statsAfter.Misses != statsBefore.Misses+1 {
					t.Errorf("expected cache miss, but misses did not increase")
				}
			}

			if len(facts) == 0 {
				t.Error("GetOrCompute() returned empty facts")
			}
		})
	}
}

func TestDataFlowCache_Serialization(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "dataflow_cache")

	// Test various argument types
	originalFacts := []core.Fact{
		{
			Predicate: "assigns",
			Args:      []interface{}{core.MangleAtom("/varName"), core.MangleAtom("/nullable"), "/path/to/file.go", int64(42)},
		},
		{
			Predicate: "uses",
			Args:      []interface{}{"/path/to/file.go", core.MangleAtom("/funcName"), core.MangleAtom("/x"), int64(100)},
		},
		{
			Predicate: "guards_return",
			Args:      []interface{}{core.MangleAtom("/err"), core.MangleAtom("/nil_check"), "/file.go", int64(15)},
		},
		{
			Predicate: "test_types",
			Args:      []interface{}{"string_value", int64(123), float64(3.14), true},
		},
	}

	cache, err := NewDataFlowCache(cacheDir)
	if err != nil {
		t.Fatalf("NewDataFlowCache() error = %v", err)
	}

	file := "/test/serialization.go"
	content := []byte("test content")

	// Store facts
	computed := false
	facts := cache.GetOrCompute(file, content, func() []core.Fact {
		computed = true
		return originalFacts
	})

	if !computed {
		t.Error("compute function should have been called")
	}

	// Persist to disk
	if err := cache.Persist(); err != nil {
		t.Fatalf("Persist() error = %v", err)
	}

	// Create new cache instance to test loading from disk
	cache2, err := NewDataFlowCache(cacheDir)
	if err != nil {
		t.Fatalf("NewDataFlowCache() (second) error = %v", err)
	}

	// Lookup should find the cached entry
	loadedFacts, found := cache2.Lookup(file, content)
	if !found {
		t.Fatal("Lookup() did not find cached entry after reload")
	}

	// Verify facts match
	if len(loadedFacts) != len(originalFacts) {
		t.Fatalf("loaded %d facts, want %d", len(loadedFacts), len(originalFacts))
	}

	for i, want := range originalFacts {
		got := loadedFacts[i]
		if got.Predicate != want.Predicate {
			t.Errorf("fact[%d].Predicate = %q, want %q", i, got.Predicate, want.Predicate)
		}
		if len(got.Args) != len(want.Args) {
			t.Errorf("fact[%d].Args length = %d, want %d", i, len(got.Args), len(want.Args))
			continue
		}
		for j := range want.Args {
			if !argsEqual(got.Args[j], want.Args[j]) {
				t.Errorf("fact[%d].Args[%d] = %v (%T), want %v (%T)",
					i, j, got.Args[j], got.Args[j], want.Args[j], want.Args[j])
			}
		}
	}

	// Verify second lookup is a hit
	computed = false
	facts = cache2.GetOrCompute(file, content, func() []core.Fact {
		computed = true
		return nil
	})
	if computed {
		t.Error("compute function should NOT have been called on cache hit")
	}
	if len(facts) != len(originalFacts) {
		t.Errorf("GetOrCompute returned %d facts, want %d", len(facts), len(originalFacts))
	}
}

// argsEqual compares two argument values for equality, handling type variations.
func argsEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case core.MangleAtom:
		bv, ok := b.(core.MangleAtom)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case int64:
		switch bv := b.(type) {
		case int64:
			return av == bv
		case int:
			return av == int64(bv)
		}
	case int:
		switch bv := b.(type) {
		case int:
			return av == bv
		case int64:
			return int64(av) == bv
		}
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	}
	return false
}

func TestDataFlowCache_Invalidate(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "dataflow_cache")

	cache, err := NewDataFlowCache(cacheDir)
	if err != nil {
		t.Fatalf("NewDataFlowCache() error = %v", err)
	}

	file := "/test/invalidate.go"
	content := []byte("test content")

	// Add to cache
	cache.GetOrCompute(file, content, func() []core.Fact {
		return []core.Fact{{Predicate: "test", Args: []interface{}{"arg"}}}
	})

	// Verify it's cached
	if _, found := cache.Lookup(file, content); !found {
		t.Error("entry should be cached after GetOrCompute")
	}

	// Persist to disk
	if err := cache.Persist(); err != nil {
		t.Fatalf("Persist() error = %v", err)
	}

	// Invalidate
	cache.Invalidate(file)

	// Verify it's gone from memory
	if _, found := cache.Lookup(file, content); found {
		t.Error("entry should not be found after Invalidate")
	}

	// Verify cache file is removed from disk
	cacheFile := cache.cacheFilePath(file)
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Errorf("cache file should be removed from disk, err = %v", err)
	}
}

func TestDataFlowCache_InvalidateAll(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "dataflow_cache")

	cache, err := NewDataFlowCache(cacheDir)
	if err != nil {
		t.Fatalf("NewDataFlowCache() error = %v", err)
	}

	// Add multiple entries
	files := []string{"/test/file1.go", "/test/file2.go", "/test/file3.go"}
	for _, file := range files {
		cache.GetOrCompute(file, []byte(file), func() []core.Fact {
			return []core.Fact{{Predicate: "test", Args: []interface{}{file}}}
		})
	}

	// Persist all
	if err := cache.Persist(); err != nil {
		t.Fatalf("Persist() error = %v", err)
	}

	// Verify entries exist
	if cache.Size() != 3 {
		t.Errorf("cache.Size() = %d, want 3", cache.Size())
	}

	// Invalidate all
	cache.InvalidateAll()

	// Verify all gone
	if cache.Size() != 0 {
		t.Errorf("cache.Size() after InvalidateAll = %d, want 0", cache.Size())
	}

	// Verify stats reset
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("stats should be reset after InvalidateAll, got hits=%d misses=%d",
			stats.Hits, stats.Misses)
	}
}

func TestDataFlowCache_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "dataflow_cache")

	cache, err := NewDataFlowCache(cacheDir)
	if err != nil {
		t.Fatalf("NewDataFlowCache() error = %v", err)
	}

	// Initial stats
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Entries != 0 {
		t.Errorf("initial stats should be zero, got %+v", stats)
	}

	// Generate some hits and misses
	file := "/test/stats.go"
	content := []byte("test")

	// Miss
	cache.GetOrCompute(file, content, func() []core.Fact {
		return []core.Fact{{Predicate: "test"}}
	})

	// Hit
	cache.GetOrCompute(file, content, func() []core.Fact {
		return nil
	})

	// Hit
	cache.GetOrCompute(file, content, func() []core.Fact {
		return nil
	})

	stats = cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("stats.Hits = %d, want 2", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("stats.Misses = %d, want 1", stats.Misses)
	}
	if stats.Entries != 1 {
		t.Errorf("stats.Entries = %d, want 1", stats.Entries)
	}

	// Test hit rate calculation
	expectedRate := float64(2) / float64(3) * 100
	if stats.HitRate() != expectedRate {
		t.Errorf("stats.HitRate() = %f, want %f", stats.HitRate(), expectedRate)
	}
}

func TestDataFlowCache_Store(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "dataflow_cache")

	cache, err := NewDataFlowCache(cacheDir)
	if err != nil {
		t.Fatalf("NewDataFlowCache() error = %v", err)
	}

	file := "/test/direct.go"
	content := []byte("direct store test")
	facts := []core.Fact{
		{Predicate: "direct", Args: []interface{}{"stored"}},
	}

	// Store directly
	cache.Store(file, content, facts)

	// Lookup should find it
	loaded, found := cache.Lookup(file, content)
	if !found {
		t.Fatal("Lookup() should find directly stored entry")
	}

	if len(loaded) != 1 || loaded[0].Predicate != "direct" {
		t.Errorf("loaded facts don't match stored facts: %+v", loaded)
	}
}

func TestCacheStats_HitRate(t *testing.T) {
	tests := []struct {
		name   string
		stats  CacheStats
		want   float64
	}{
		{
			name:   "no lookups",
			stats:  CacheStats{Hits: 0, Misses: 0},
			want:   0,
		},
		{
			name:   "all hits",
			stats:  CacheStats{Hits: 10, Misses: 0},
			want:   100,
		},
		{
			name:   "all misses",
			stats:  CacheStats{Hits: 0, Misses: 10},
			want:   0,
		},
		{
			name:   "50% hit rate",
			stats:  CacheStats{Hits: 5, Misses: 5},
			want:   50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stats.HitRate(); got != tt.want {
				t.Errorf("HitRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataFlowCache_VersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "dataflow_cache")

	cache, err := NewDataFlowCache(cacheDir)
	if err != nil {
		t.Fatalf("NewDataFlowCache() error = %v", err)
	}

	file := "/test/version.go"
	content := []byte("version test")

	// Add entry
	cache.GetOrCompute(file, content, func() []core.Fact {
		return []core.Fact{{Predicate: "test"}}
	})

	// Manually modify the version in the entry
	cache.mu.Lock()
	if entry, ok := cache.entries[file]; ok {
		entry.Version = 999 // Invalid version
	}
	cache.mu.Unlock()

	// GetOrCompute should treat this as a miss due to version mismatch
	statsBefore := cache.Stats()
	cache.GetOrCompute(file, content, func() []core.Fact {
		return []core.Fact{{Predicate: "recomputed"}}
	})
	statsAfter := cache.Stats()

	// Should be a miss, not a hit
	if statsAfter.Misses != statsBefore.Misses+1 {
		t.Error("version mismatch should cause cache miss")
	}
}
