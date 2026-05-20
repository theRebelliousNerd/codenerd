package store

import (
	"testing"
	"math"
	"fmt"
)

func TestLocalStore_KnowledgeGraph_Extra(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// Invalid StoreLink
	err = s.StoreLink("", "rel", "b", 1.0, nil)
	if err == nil {
		t.Errorf("Expected error for empty entityA")
	}
	err = s.StoreLink("a", "rel", "b", math.NaN(), nil)
	if err == nil {
		t.Errorf("Expected error for NaN weight")
	}

	// StoreLink
	err = s.StoreLink("a", "rel", "b", 1.0, map[string]interface{}{"key": "val"})
	if err != nil {
		t.Errorf("StoreLink failed: %v", err)
	}
	s.StoreLink("b", "rel", "c", 1.0, nil)
	s.StoreLink("x", "rel", "y", 1.0, nil)

	// QueryLinks
	links, err := s.QueryLinks("a", "outgoing")
	if err != nil || len(links) != 1 {
		t.Errorf("QueryLinks outgoing failed")
	}
	links, err = s.QueryLinks("b", "incoming")
	if err != nil || len(links) != 1 {
		t.Errorf("QueryLinks incoming failed")
	}
	links, err = s.QueryLinks("b", "both")
	if err != nil || len(links) != 2 { // incoming from a, outgoing to c
		t.Errorf("QueryLinks both failed: expected 2, got %d", len(links))
	}

	// TraversePath
	path, err := s.TraversePath("a", "c", 5)
	if err != nil {
		t.Errorf("TraversePath failed: %v", err)
	}
	if len(path) != 2 {
		t.Errorf("Expected path length 2, got %d", len(path))
	}
	
	// TraversePath with maxDepth limit
	path, err = s.TraversePath("a", "c", 1)
	if err == nil {
		t.Errorf("Expected error when maxDepth is too small")
	}
	
	// TraversePath no path
	path, err = s.TraversePath("a", "x", 5)
	if err == nil {
		t.Errorf("Expected error when no path exists")
	}

	// HydrateKnowledgeGraph
	assertCount := 0
	assertFunc := func(pred string, args []interface{}) error {
		if pred == "knowledge_link" {
			assertCount++
			return nil
		}
		return fmt.Errorf("unexpected pred")
	}
	count, err := s.HydrateKnowledgeGraph(assertFunc)
	if err != nil {
		t.Errorf("HydrateKnowledgeGraph failed: %v", err)
	}
	if count != 3 || assertCount != 3 {
		t.Errorf("Expected 3 assertions, got %d (assertCount: %d)", count, assertCount)
	}
}
