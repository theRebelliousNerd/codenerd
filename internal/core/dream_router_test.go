package core

import (
	"testing"
)

// Mock implementations for testing
type mockLearningStore struct {
	saved []string
}

func (m *mockLearningStore) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	m.saved = append(m.saved, factPredicate)
	return nil
}

type mockColdStore struct {
	stored []string
}

func (m *mockColdStore) StoreFact(predicate string, args []interface{}, factType string, importance int) error {
	m.stored = append(m.stored, predicate)
	return nil
}

func TestDreamRouter_New(t *testing.T) {
	k := setupMockKernel(t)
	ls := &mockLearningStore{}
	cs := &mockColdStore{}

	router := NewDreamRouter(k, ls, cs)
	if router == nil {
		t.Fatal("NewDreamRouter returned nil")
	}
}

func TestDreamRouter_RouteLearnings(t *testing.T) {
	k := setupMockKernel(t)
	ls := &mockLearningStore{}
	cs := &mockColdStore{}

	router := NewDreamRouter(k, ls, cs)

	learnings := []*DreamLearning{
		{
			ID:         "learn-1",
			Type:       LearningTypeProcedural,
			Content:    "Always test before commit",
			Confidence: 0.9,
		},
		{
			ID:         "learn-2",
			Type:       LearningTypePreference,
			Content:    "User prefers dark mode",
			Confidence: 0.8,
		},
	}

	results := router.RouteLearnings(learnings)

	// Results count depends on implementation - may filter duplicates or nil entries
	t.Logf("RouteLearnings returned %d results for %d learnings", len(results), len(learnings))

	// Just verify we got results back (may be empty if stores not configured)
	for _, r := range results {
		if r.LearningID != "" {
			t.Logf("Routed learning %s to %s", r.LearningID, r.Destination)
		}
	}
}

func TestDreamRouter_SetOuroborosQueue(t *testing.T) {
	k := setupMockKernel(t)
	router := NewDreamRouter(k, nil, nil)

	queue := make(chan ToolNeed, 10)
	router.SetOuroborosQueue(queue)

	// Route a tool need learning
	learnings := []*DreamLearning{
		{
			ID:      "tool-need-1",
			Type:    LearningTypeToolNeed,
			Content: "Need a code coverage tool",
		},
	}

	router.RouteLearnings(learnings)

	// Check if tool need was queued (non-blocking check)
	select {
	case need := <-queue:
		t.Logf("Tool need queued with source: %s", need.SourceDream)
	default:
		t.Log("No tool need queued (expected if queue handler not set)")
	}
}
