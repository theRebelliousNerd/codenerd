package context_harness

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/core"
)

func TestCalculateFactScoreAndReason(t *testing.T) {
	fact := core.Fact{Predicate: "turn_error_message"}
	score := calculateFactScore(fact, 0, 1)
	if score != 1.0 {
		t.Fatalf("calculateFactScore = %.2f, want 1.0", score)
	}

	reason := buildScoreReason(fact, score, true)
	if !strings.Contains(reason, "High-priority") {
		t.Fatalf("expected high-priority reason, got %q", reason)
	}

	pruned := buildScoreReason(fact, 0.1, false)
	if !strings.Contains(pruned, "Pruned") {
		t.Fatalf("expected pruned reason, got %q", pruned)
	}
}

func TestConvertToFactActivations(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "turn_topic", Args: []interface{}{"t1", "topic"}},
		{Predicate: "conversation_turn", Args: []interface{}{1, "user", "hello", "debug"}},
	}

	activations := convertToFactActivations(facts, true)
	if len(activations) != len(facts) {
		t.Fatalf("activations len = %d, want %d", len(activations), len(facts))
	}
	for _, act := range activations {
		if !act.Selected {
			t.Fatalf("expected Selected to be true")
		}
		if act.Reason == "" {
			t.Fatalf("expected Reason to be set")
		}
	}
}

func TestCalculateRetrievalMetrics(t *testing.T) {
	refTurn := 2
	turn := &Turn{
		TurnID:  3,
		Speaker: "user",
		Message: "question",
		Metadata: TurnMetadata{
			FilesReferenced:         []string{"main.go"},
			Topics:                  []string{"routing"},
			ReferencesBackToTurn:    &refTurn,
			IsQuestionReferringBack: true,
		},
	}

	retrieved := []core.Fact{
		{Predicate: "turn_references_file", Args: []interface{}{turn.TurnID, "main.go"}},
		{Predicate: "turn_topic", Args: []interface{}{turn.TurnID, "routing"}},
		{Predicate: "conversation_turn", Args: []interface{}{refTurn, "assistant", "prev", "debug"}},
	}

	sim := &SessionSimulator{}
	precision, recall := sim.calculateRetrievalMetrics(retrieved, turn)
	if precision != 1.0 {
		t.Fatalf("precision = %.2f, want 1.0", precision)
	}
	if recall != 1.0 {
		t.Fatalf("recall = %.2f, want 1.0", recall)
	}
}

func TestCalculateRetrievalMetricsFloors(t *testing.T) {
	turn := &Turn{
		TurnID:  1,
		Speaker: "user",
		Message: "question",
		Metadata: TurnMetadata{
			FilesReferenced:         []string{"missing.go"},
			IsQuestionReferringBack: false,
		},
	}

	sim := &SessionSimulator{}
	precision, recall := sim.calculateRetrievalMetrics(nil, turn)
	if precision != 0.5 {
		t.Fatalf("precision = %.2f, want 0.5", precision)
	}
	if recall != 0.5 {
		t.Fatalf("recall = %.2f, want 0.5", recall)
	}
}

func TestValidateCheckpointFallback(t *testing.T) {
	sim := &SessionSimulator{
		config: SimulatorConfig{TokenBudget: 100},
	}
	checkpoint := &Checkpoint{
		AfterTurn:    0,
		Query:        "what happened",
		MustRetrieve: []string{"fact-a", "fact-b"},
		ShouldAvoid:  []string{"fact-x"},
		MinRecall:    0.9,
		MinPrecision: 0.9,
		Description:  "fallback",
	}

	result := sim.validateCheckpoint(context.Background(), checkpoint)
	if !result.Passed {
		t.Fatalf("expected checkpoint to pass, got %s", result.FailureReason)
	}
	if len(result.MissingRequired) != 0 {
		t.Fatalf("expected no missing required facts")
	}
	if len(result.UnwantedNoise) != 0 {
		t.Fatalf("expected no unwanted noise")
	}
}

func TestMeetsExpectations(t *testing.T) {
	sim := &SessionSimulator{}
	actual := &Metrics{
		CompressionRatio:      1.0,
		AvgRetrievalRecall:    0.6,
		AvgRetrievalPrec:      0.6,
		TokenBudgetViolations: 1,
	}
	expected := &Metrics{
		CompressionRatio:      1.2,
		AvgRetrievalRecall:    0.6,
		AvgRetrievalPrec:      0.6,
		TokenBudgetViolations: 1,
	}

	if sim.meetsExpectations(actual, expected) {
		t.Fatalf("expected meetsExpectations to be false")
	}
}

func TestContainsAndSortByScore(t *testing.T) {
	if !contains("same", "same") {
		t.Fatalf("expected contains to match equal strings")
	}
	if contains("same", "diff") {
		t.Fatalf("expected contains to be false for mismatched strings")
	}

	facts := []scoredFact{
		{score: 0.1},
		{score: 0.9},
		{score: 0.5},
	}
	sortByScore(facts)
	if facts[0].score != 0.9 || facts[1].score != 0.5 || facts[2].score != 0.1 {
		t.Fatalf("scores not sorted desc: %+v", facts)
	}
}

// contains checks if two strings are equal (simple string equality).
func contains(a, b string) bool {
	return a == b
}
