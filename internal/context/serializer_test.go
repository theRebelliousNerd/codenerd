package context

import (
	"codenerd/internal/core"
	"strings"
	"testing"
)

// TestNewFactSerializer tests serializer construction.
func TestNewFactSerializer(t *testing.T) {
	fs := NewFactSerializer()
	if fs == nil {
		t.Fatal("Expected non-nil serializer")
	}
	if !fs.includeComments {
		t.Error("Expected includeComments to default to true")
	}
	if !fs.groupByPredicate {
		t.Error("Expected groupByPredicate to default to true")
	}
	if fs.maxLineLength != 120 {
		t.Errorf("Expected maxLineLength 120, got %d", fs.maxLineLength)
	}
}

// TestSetCorpusOrder tests setting corpus-based serialization order.
func TestSetCorpusOrder(t *testing.T) {
	fs := NewFactSerializer()

	order := map[string]int{
		"user_intent": 1,
		"next_action": 2,
		"diagnostic":  10,
	}

	fs.SetCorpusOrder(order)

	if fs.corpusOrder == nil {
		t.Fatal("Expected corpusOrder to be set")
	}

	if fs.corpusOrder["user_intent"] != 1 {
		t.Errorf("Expected user_intent order 1, got %d", fs.corpusOrder["user_intent"])
	}
}

// TestGetSortOrderWithCorpus tests that corpus order takes precedence.
func TestGetSortOrderWithCorpus(t *testing.T) {
	fs := NewFactSerializer()

	// Without corpus order, should use hardcoded
	order := fs.getSortOrder("user_intent")
	if order <= 0 {
		t.Errorf("Expected positive order for user_intent, got %d", order)
	}

	// With corpus order, should use corpus value
	fs.SetCorpusOrder(map[string]int{
		"user_intent": 1,
		"diagnostic":  5,
	})

	if fs.getSortOrder("user_intent") != 1 {
		t.Errorf("Expected corpus order 1 for user_intent, got %d", fs.getSortOrder("user_intent"))
	}

	if fs.getSortOrder("diagnostic") != 5 {
		t.Errorf("Expected corpus order 5 for diagnostic, got %d", fs.getSortOrder("diagnostic"))
	}

	// Predicate not in corpus should fall back to hardcoded (100 is default)
	order = fs.getSortOrder("unknown_predicate_xyz")
	if order != 100 {
		t.Errorf("Expected fallback order 100 for unknown predicate, got %d", order)
	}
}

// TestSerializeFactsGroupedWithCorpusOrder tests that serialization respects corpus order.
func TestSerializeFactsGroupedWithCorpusOrder(t *testing.T) {
	fs := NewFactSerializer().
		WithComments(false).
		WithGrouping(true)

	// Set corpus order: next_action first, then user_intent
	fs.SetCorpusOrder(map[string]int{
		"next_action": 1,
		"user_intent": 2,
	})

	facts := []core.Fact{
		{Predicate: "user_intent", Args: []interface{}{"id1", "/query", "/read", "target", "constraint"}},
		{Predicate: "next_action", Args: []interface{}{"/run_tests"}},
	}

	result := fs.SerializeFacts(facts)

	// next_action should appear before user_intent in output
	nextActionPos := strings.Index(result, "next_action")
	userIntentPos := strings.Index(result, "user_intent")

	if nextActionPos >= userIntentPos {
		t.Errorf("Expected next_action before user_intent based on corpus order.\nResult:\n%s", result)
	}
}

// TestLoadSerializationOrderFromCorpus tests loading order from corpus.
func TestLoadSerializationOrderFromCorpus(t *testing.T) {
	corpus, err := core.NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	fs := NewFactSerializer()
	fs.LoadSerializationOrderFromCorpus(corpus)

	if fs.corpusOrder == nil {
		t.Fatal("Expected corpusOrder to be loaded from corpus")
	}

	if len(fs.corpusOrder) == 0 {
		t.Error("Expected non-empty corpus order")
	}

	// Verify known predicates are present
	if _, ok := fs.corpusOrder["user_intent"]; !ok {
		t.Error("Expected user_intent in corpus order")
	}
}

// TestLoadSerializationOrderFromNilCorpus tests graceful handling of nil corpus.
func TestLoadSerializationOrderFromNilCorpus(t *testing.T) {
	fs := NewFactSerializer()

	// Should not panic
	fs.LoadSerializationOrderFromCorpus(nil)

	// corpusOrder should remain nil
	if fs.corpusOrder != nil {
		t.Error("Expected corpusOrder to remain nil with nil corpus")
	}
}

// TestSerializeFactsFlat tests flat serialization (no grouping).
func TestSerializeFactsFlat(t *testing.T) {
	fs := NewFactSerializer().WithGrouping(false).WithComments(false)

	facts := []core.Fact{
		{Predicate: "user_intent", Args: []interface{}{"id1", "/query"}},
		{Predicate: "next_action", Args: []interface{}{"/run_tests"}},
	}

	result := fs.SerializeFacts(facts)

	// Facts should appear in input order
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	if !strings.HasPrefix(lines[0], "user_intent(") {
		t.Errorf("Expected first line to be user_intent, got: %s", lines[0])
	}
}

// TestSerializeEmptyFacts tests serialization of empty fact list.
func TestSerializeEmptyFacts(t *testing.T) {
	fs := NewFactSerializer()
	result := fs.SerializeFacts(nil)
	if result != "" {
		t.Errorf("Expected empty string for nil facts, got: %s", result)
	}

	result = fs.SerializeFacts([]core.Fact{})
	if result != "" {
		t.Errorf("Expected empty string for empty facts, got: %s", result)
	}
}

// TestMethodChaining tests fluent API chaining.
func TestMethodChaining(t *testing.T) {
	corpus, err := core.NewPredicateCorpus()
	if err != nil {
		t.Skipf("Corpus not available: %v", err)
	}
	defer corpus.Close()

	fs := NewFactSerializer().
		WithComments(false).
		WithGrouping(true).
		LoadSerializationOrderFromCorpus(corpus)

	if fs.includeComments {
		t.Error("Expected comments disabled")
	}
	if !fs.groupByPredicate {
		t.Error("Expected grouping enabled")
	}
	if fs.corpusOrder == nil {
		t.Error("Expected corpus order loaded")
	}
}
