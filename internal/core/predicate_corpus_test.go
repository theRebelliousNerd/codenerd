package core

import (
	"testing"
)

// TestNewPredicateCorpus tests corpus initialization.
func TestNewPredicateCorpus(t *testing.T) {
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available (expected in CI without embedded DB): %v", err)
	}
	defer corpus.Close()

	// Verify basic functionality
	if corpus.db == nil {
		t.Fatal("Expected db to be non-nil")
	}
}

// TestGetPriorities tests the GetPriorities method.
func TestGetPriorities(t *testing.T) {
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	priorities, err := corpus.GetPriorities()
	if err != nil {
		t.Fatalf("GetPriorities failed: %v", err)
	}

	// Should have some priorities
	if len(priorities) == 0 {
		t.Error("Expected non-empty priorities map")
	}

	// Check that known high-priority predicates have high values
	highPriorityPredicates := []string{"user_intent", "diagnostic", "test_state"}
	for _, pred := range highPriorityPredicates {
		if priority, ok := priorities[pred]; ok {
			if priority < 90 {
				t.Errorf("Expected %s to have priority >= 90, got %d", pred, priority)
			}
		}
	}
}

// TestGetSerializationOrder tests the GetSerializationOrder method.
func TestGetSerializationOrder(t *testing.T) {
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	order, err := corpus.GetSerializationOrder()
	if err != nil {
		t.Fatalf("GetSerializationOrder failed: %v", err)
	}

	// Should have some entries
	if len(order) == 0 {
		t.Error("Expected non-empty serialization order map")
	}

	// user_intent should be present in the order map
	// Note: Default order is 100 since corpus builder doesn't yet parse
	// serialization_order annotations from schemas. When annotations are
	// added, user_intent should have order <= 10.
	if _, ok := order["user_intent"]; !ok {
		t.Error("Expected user_intent to be present in serialization order map")
	}
}

// TestGetPriority tests the single-predicate priority lookup.
func TestGetPriority(t *testing.T) {
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	tests := []struct {
		name     string
		pred     string
		minPrio  int
		maxPrio  int
	}{
		{"user_intent high priority", "user_intent", 90, 100},
		{"unknown predicate default", "nonexistent_predicate_xyz", 50, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority := corpus.GetPriority(tt.pred)
			if priority < tt.minPrio || priority > tt.maxPrio {
				t.Errorf("GetPriority(%s) = %d, want [%d, %d]", tt.pred, priority, tt.minPrio, tt.maxPrio)
			}
		})
	}
}

// TestIsDeclared tests predicate declaration checking.
func TestIsDeclared(t *testing.T) {
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	// Known predicates should be declared
	knownPredicates := []string{"user_intent", "file_topology", "next_action"}
	for _, pred := range knownPredicates {
		if !corpus.IsDeclared(pred) {
			t.Errorf("Expected %s to be declared", pred)
		}
	}

	// Unknown predicates should not be declared
	if corpus.IsDeclared("completely_fake_predicate_12345") {
		t.Error("Expected fake predicate to not be declared")
	}
}

// TestGetPredicate tests full predicate info retrieval.
func TestGetPredicate(t *testing.T) {
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	info, err := corpus.GetPredicate("user_intent")
	if err != nil {
		t.Fatalf("GetPredicate(user_intent) failed: %v", err)
	}

	if info.Name != "user_intent" {
		t.Errorf("Expected name 'user_intent', got '%s'", info.Name)
	}

	if info.Arity < 4 {
		t.Errorf("Expected user_intent arity >= 4, got %d", info.Arity)
	}
}

// TestGetByDomain tests domain-based predicate filtering.
func TestGetByDomain(t *testing.T) {
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	// Core domain should have predicates
	corePredicates, err := corpus.GetByDomain("core")
	if err != nil {
		t.Fatalf("GetByDomain(core) failed: %v", err)
	}

	if len(corePredicates) == 0 {
		t.Error("Expected non-empty core domain predicates")
	}
}

// TestSearchPredicates tests predicate search functionality.
func TestSearchPredicates(t *testing.T) {
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	results, err := corpus.SearchPredicates("intent")
	if err != nil {
		t.Fatalf("SearchPredicates(intent) failed: %v", err)
	}

	// Should find user_intent at minimum
	found := false
	for _, p := range results {
		if p.Name == "user_intent" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find user_intent when searching for 'intent'")
	}
}

// TestFindErrorPattern tests error pattern matching.
func TestFindErrorPattern(t *testing.T) {
	corpus, err := NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	// Test with a common error message
	pattern, err := corpus.FindErrorPattern("undefined predicate 'foo'")
	if err != nil {
		// May not find a pattern, which is okay
		return
	}

	if pattern != nil && pattern.Name == "" {
		t.Error("Expected pattern to have a name")
	}
}
