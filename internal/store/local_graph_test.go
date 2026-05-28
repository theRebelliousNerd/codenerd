package store

import (
	"fmt"
	"math"
	"sync"
	"testing"
)

func TestStoreLink(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	// Store a link
	err = store.StoreLink("entityA", "related_to", "entityB", 1.5, map[string]any{"source": "manual"})
	if err != nil {
		t.Fatalf("StoreLink failed: %v", err)
	}

	// Verify links exist
	links, err := store.QueryLinks("entityA", "outgoing")
	if err != nil {
		t.Fatalf("QueryLinks failed: %v", err)
	}

	if len(links) != 1 {
		t.Fatalf("Expected 1 link, got %d", len(links))
	}

	if links[0].EntityB != "entityB" {
		t.Errorf("Expected EntityB to be 'entityB', got '%s'", links[0].EntityB)
	}
	if links[0].Weight != 1.5 {
		t.Errorf("Expected weight 1.5, got %v", links[0].Weight)
	}
}

func TestTraversePath(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	// A -> B -> C
	store.StoreLink("A", "next", "B", 1.0, nil)
	store.StoreLink("B", "next", "C", 1.0, nil)

	// Traverse from A to B
	paths, err := store.TraversePath("A", "B", 10)
	if err != nil {
		t.Fatalf("TraversePath failed: %v", err)
	}
	// Path should be A->B (1 link)
	if len(paths) != 1 {
		t.Errorf("Expected path length 1, got %d", len(paths))
	}
	if paths[0].EntityB != "B" {
		t.Errorf("Expected to reach B, got %s", paths[0].EntityB)
	}

	// Traverse from A to C
	paths, err = store.TraversePath("A", "C", 10)
	if err != nil {
		t.Fatalf("TraversePath failed: %v", err)
	}
	// Path should be A->B, B->C (2 links)
	if len(paths) != 2 {
		t.Errorf("Expected path length 2, got %d", len(paths))
	}
	if paths[0].EntityB != "B" {
		t.Errorf("Step 1 should be B")
	}
	if paths[1].EntityB != "C" {
		t.Errorf("Step 2 should be C")
	}
}

func TestQueryLinks(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	store.StoreLink("A", "rel1", "B", 1.0, nil)
	store.StoreLink("A", "rel2", "C", 1.0, nil)
	store.StoreLink("X", "rel1", "Y", 1.0, nil)
	store.StoreLink("Z", "rel3", "A", 1.0, nil) // Incoming to A

	// Query Outgoing A
	links, _ := store.QueryLinks("A", "outgoing")
	if len(links) != 2 {
		t.Errorf("Expected 2 outgoing links for A, got %d", len(links))
	}

	// Query Incoming A
	links, _ = store.QueryLinks("A", "incoming")
	if len(links) != 1 {
		t.Errorf("Expected 1 incoming link for A, got %d", len(links))
	}

	// Query Both
	links, _ = store.QueryLinks("A", "both")
	if len(links) != 3 {
		t.Errorf("Expected 3 links for A (both), got %d", len(links))
	}
}

// -----------------------------------------------------------------------------
// QA NEGATIVE TESTING
// -----------------------------------------------------------------------------

func TestTraversePathDeadlock(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	store.StoreLink("A", "next", "B", 1.0, nil)
	store.StoreLink("B", "next", "C", 1.0, nil)

	// To test deadlock, we need TraversePath to run while StoreLink wants a lock.
	// We'll start a slow traversal (or just let it run concurrently with many writes)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range 1000 {
			store.StoreLink("X", "rel", "Y", 1.0, nil)
		}
	}()

	go func() {
		defer wg.Done()
		for range 100 {
			store.TraversePath("A", "C", 5)
		}
	}()

	wg.Wait() // Will deadlock if TraversePath uses QueryLinks and gets blocked by a pending writer
}

func TestJSONMetadataSilentFailure(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	// Create a cyclic map that will fail json.Marshal
	cyclicMap := make(map[string]any)
	cyclicMap["self"] = cyclicMap

	err = store.StoreLink("A", "rel", "B", 1.0, cyclicMap)
	if err == nil {
		t.Error("Expected error when storing link with cyclic metadata")
	}
}

func TestEmptyEntityInputs(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	if err := store.StoreLink("", "rel", "B", 1.0, nil); err == nil {
		t.Error("Expected error with empty EntityA")
	}
	if err := store.StoreLink("A", "", "B", 1.0, nil); err == nil {
		t.Error("Expected error with empty relation")
	}
	if err := store.StoreLink("A", "rel", "", 1.0, nil); err == nil {
		t.Error("Expected error with empty EntityB")
	}
}

func TestNaNAndInfiniteWeights(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	if err := store.StoreLink("A", "rel", "B", math.NaN(), nil); err == nil {
		t.Error("Expected error with NaN weight")
	}
	if err := store.StoreLink("A", "rel", "B", math.Inf(1), nil); err == nil {
		t.Error("Expected error with +Inf weight")
	}
	if err := store.StoreLink("A", "rel", "B", math.Inf(-1), nil); err == nil {
		t.Error("Expected error with -Inf weight")
	}
}

func TestMassiveGraphTraversal(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	// Create a graph with branching factor 5 up to depth 4 (5 + 25 + 125 + 625 = 780 nodes)
	// We want to ensure TraversePath can handle it quickly and returns.

	for depth := range 4 {
		// Just build a flat graph, it's easier and tests the queue
		for branch := range 100 {
			store.StoreLink(fmt.Sprintf("N%d", depth), "next", fmt.Sprintf("N%d_%d", depth+1, branch), 1.0, nil)
		}
	}

	paths, err := store.TraversePath("N0", "N4_50", 10)
	if err == nil {
		t.Error("Did not expect to find path since it's disjointed, but we expect it to finish without blocking forever")
	}
	_ = paths
}
