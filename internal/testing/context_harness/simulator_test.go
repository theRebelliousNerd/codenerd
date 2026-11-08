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

func TestStabilize_InferOperationalMode_PinBranches(t *testing.T) {
	s := &SessionSimulator{}
	cases := []struct {
		name   string
		intent string
		want   string
	}{
		{"debug", "debug", "/debugging"},
		{"analyze", "analyze", "/debugging"},
		{"test", "test", "/testing"},
		{"test-response", "test-response", "/testing"},
		{"implement", "implement", "/coding"},
		{"implement-response", "implement-response", "/coding"},
		{"refactor", "refactor", "/coding"},
		{"refactor-response", "refactor-response", "/coding"},
		{"review", "review", "/reviewing"},
		{"review-response", "review-response", "/reviewing"},
		{"unknown falls back", "plan", "/default"},
		{"empty falls back", "", "/default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.inferOperationalMode(tc.intent); got != tc.want {
				t.Fatalf("inferOperationalMode(%q) = %q, want %q", tc.intent, got, tc.want)
			}
		})
	}
}

func TestStabilize_CalculateFactScore_PinBranches(t *testing.T) {
	cases := []struct {
		name     string
		pred     string
		position int
		total    int
		want     float64
	}{
		{"no recency when total zero", "turn_error_message", 0, 0, 0.7},
		{"topic with full recency", "turn_topic", 0, 4, 0.95},
		{"references file mid recency", "turn_references_file", 2, 4, 0.75},
		{"conversation turn late", "conversation_turn", 3, 4, 0.625},
		{"unknown predicate keeps base plus recency", "other_pred", 0, 2, 0.8},
		{"clamped at one", "turn_error_message", 0, 1, 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := calculateFactScore(core.Fact{Predicate: tc.pred}, tc.position, tc.total)
			diff := got - tc.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 1e-9 {
				t.Fatalf("calculateFactScore(%q,pos=%d,total=%d) = %.4f, want %.4f", tc.pred, tc.position, tc.total, got, tc.want)
			}
			if got < 0.0 || got > 1.0 {
				t.Fatalf("score out of range: %.4f", got)
			}
		})
	}
}

func TestStabilize_BuildScoreReason_PinBranches(t *testing.T) {
	fact := core.Fact{Predicate: "turn_topic"}
	if got := buildScoreReason(fact, 0.7, true); !strings.Contains(got, "Relevant") {
		t.Fatalf("expected Relevant reason, got %q", got)
	}
	if got := buildScoreReason(fact, 0.5, true); !strings.Contains(got, "Retrieved") {
		t.Fatalf("expected Retrieved reason, got %q", got)
	}
	if got := buildScoreReason(fact, 0.6, true); !strings.Contains(got, "Retrieved") {
		t.Fatalf("expected Retrieved at boundary 0.6, got %q", got)
	}
}

func TestConvertToFactActivations(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "turn_topic", Args: []any{"t1", "topic"}},
		{Predicate: "conversation_turn", Args: []any{1, "user", "hello", "debug"}},
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
		{Predicate: "turn_references_file", Args: []any{turn.TurnID, "main.go"}},
		{Predicate: "turn_topic", Args: []any{turn.TurnID, "routing"}},
		{Predicate: "conversation_turn", Args: []any{refTurn, "assistant", "prev", "debug"}},
	}

	sim := &SessionSimulator{}
	precision, recall, measured := sim.calculateRetrievalMetrics(retrieved, turn)
	if !measured {
		t.Fatal("a turn with ground truth was not measured")
	}
	if precision != 1.0 {
		t.Fatalf("precision = %.2f, want 1.0", precision)
	}
	if recall != 1.0 {
		t.Fatalf("recall = %.2f, want 1.0", recall)
	}
}

// An engine that retrieves nothing scores zero, and a turn that names nothing
// to retrieve is not measured. Mock mode used to floor the first at 0.5/0.5
// and score the second 0.95/0.95, both numbers nothing measured. (This test
// pinned the floor; it now pins its removal.)
func TestCalculateRetrievalMetrics_ShouldNotScoreWhatWasNotMeasured(t *testing.T) {
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
	precision, recall, measured := sim.calculateRetrievalMetrics(nil, turn)
	if !measured || precision != 0 || recall != 0 {
		t.Fatalf("nothing retrieved: precision %.2f recall %.2f measured %v, want 0, 0, true", precision, recall, measured)
	}

	if _, _, measured := sim.calculateRetrievalMetrics(nil, &Turn{TurnID: 2, Speaker: "user"}); measured {
		t.Fatal("a turn with no ground truth was scored")
	}
}

// A checkpoint is never passed on retrieval that did not happen. It used to
// substitute its own MustRetrieve list for an empty retrieval, so no engine at
// all scored perfect recall. (This test was TestValidateCheckpointFallback and
// pinned that soft pass.)
func TestValidateCheckpoint_WhenNothingIsRetrieved_ShouldFail(t *testing.T) {
	checkpoint := &Checkpoint{
		AfterTurn:    0,
		Query:        "what happened",
		MustRetrieve: []string{"fact-a", "fact-b"},
		ShouldAvoid:  []string{"fact-x"},
		MinRecall:    0.9,
		MinPrecision: 0.9,
		Description:  "no soft pass",
	}

	noEngine := &SessionSimulator{config: SimulatorConfig{TokenBudget: 100}, metrics: NewMetricsCollector()}
	if result := noEngine.validateCheckpoint(context.Background(), checkpoint); result.Passed || len(result.MissingRequired) != 2 {
		t.Fatalf("no engine: passed=%v missing=%v, want a failure missing both facts", result.Passed, result.MissingRequired)
	}

	empty := &SessionSimulator{config: SimulatorConfig{TokenBudget: 100}, metrics: NewMetricsCollector()}
	empty.SetContextEngine(NewMockContextEngine(nil))
	result := empty.validateCheckpoint(context.Background(), checkpoint)
	if result.Passed || result.Recall != 0 {
		t.Fatalf("empty retrieval: passed=%v recall=%.2f, want a failure at recall 0", result.Passed, result.Recall)
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
