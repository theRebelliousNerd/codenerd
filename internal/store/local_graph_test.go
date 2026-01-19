package store

import (
	"testing"
)

func TestStoreLink(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	// Store a link
	err = store.StoreLink("entityA", "related_to", "entityB", 1.5, map[string]interface{}{"source": "manual"})
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
