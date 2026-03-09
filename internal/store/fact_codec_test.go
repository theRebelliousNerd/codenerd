package store

import (
	"testing"

	"codenerd/internal/types"
)

func TestColdStoragePreservesTypedArgs(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	args := []interface{}{
		types.MangleAtom("/go"),
		int64(7),
		float64(3.5),
		"/tmp/file.go",
		true,
	}

	if err := store.StoreFact("typed_fact", args, "fact", 5); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	facts, err := store.LoadFacts("typed_fact")
	if err != nil {
		t.Fatalf("LoadFacts failed: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("Expected 1 fact, got %d", len(facts))
	}

	got := facts[0].Args
	if atom, ok := got[0].(types.MangleAtom); !ok || atom != types.MangleAtom("/go") {
		t.Fatalf("Expected MangleAtom /go, got %#v", got[0])
	}
	if n, ok := got[1].(int64); !ok || n != 7 {
		t.Fatalf("Expected int64(7), got %#v", got[1])
	}
	if f, ok := got[2].(float64); !ok || f != 3.5 {
		t.Fatalf("Expected float64(3.5), got %#v", got[2])
	}
	if s, ok := got[3].(string); !ok || s != "/tmp/file.go" {
		t.Fatalf("Expected path string, got %#v", got[3])
	}
	if b, ok := got[4].(bool); !ok || !b {
		t.Fatalf("Expected bool true, got %#v", got[4])
	}
}

func TestWorldCachePreservesTypedArgs(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	facts := []WorldFactInput{
		{
			Predicate: "file_topology",
			Args: []interface{}{
				"/workspace/main.go",
				types.MangleAtom("/go"),
				int64(12),
			},
		},
	}

	if err := store.ReplaceWorldFactsForFile("/workspace/main.go", "fast", "fp-1", facts); err != nil {
		t.Fatalf("ReplaceWorldFactsForFile failed: %v", err)
	}

	loaded, fp, err := store.LoadWorldFactsForFile("/workspace/main.go", "fast")
	if err != nil {
		t.Fatalf("LoadWorldFactsForFile failed: %v", err)
	}
	if fp != "fp-1" {
		t.Fatalf("Expected fingerprint fp-1, got %q", fp)
	}
	if len(loaded) != 1 {
		t.Fatalf("Expected 1 world fact, got %d", len(loaded))
	}

	got := loaded[0].Args
	if s, ok := got[0].(string); !ok || s != "/workspace/main.go" {
		t.Fatalf("Expected path string, got %#v", got[0])
	}
	if atom, ok := got[1].(types.MangleAtom); !ok || atom != types.MangleAtom("/go") {
		t.Fatalf("Expected MangleAtom /go, got %#v", got[1])
	}
	if n, ok := got[2].(int64); !ok || n != 12 {
		t.Fatalf("Expected int64(12), got %#v", got[2])
	}

	all, err := store.LoadAllWorldFacts("fast")
	if err != nil {
		t.Fatalf("LoadAllWorldFacts failed: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("Expected 1 cached world fact, got %d", len(all))
	}
}
