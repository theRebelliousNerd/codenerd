package context_harness

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	ctxcompress "codenerd/internal/context"
)

// TestFeedbackTracerOutput verifies the FeedbackTracer produces expected output.
func TestFeedbackTracerOutput(t *testing.T) {
	var buf bytes.Buffer
	tracer := NewFeedbackTracer(&buf, true) // verbose mode

	snapshot := &FeedbackSnapshot{
		Timestamp:         time.Now(),
		TurnNumber:        5,
		IntentVerb:        "/fix",
		OverallUsefulness: 0.85,
		HelpfulFacts:      []string{"file_topology", "test_state", "error_context"},
		NoiseFacts:        []string{"browser_state", "dom_node"},
		MissingContext:    "dependency graph would have been helpful",
		ActivePredicates: []PredicateFeedbackState{
			{
				Predicate:      "file_topology",
				HelpfulCount:   15,
				NoiseCount:     2,
				TotalMentions:  17,
				UsefulnessScore: 0.76,
				ScoreComponent:  15.2,
			},
			{
				Predicate:      "browser_state",
				HelpfulCount:   1,
				NoiseCount:     12,
				TotalMentions:  13,
				UsefulnessScore: -0.85,
				ScoreComponent:  -17.0,
			},
		},
		TotalFeedbackSamples: 42,
	}

	tracer.TraceFeedback(snapshot)

	output := buf.String()

	// Verify key elements are present
	expectedStrings := []string{
		"CONTEXT FEEDBACK - TURN 5",
		"Intent: /fix",
		"Overall Usefulness: 0.85",
		"Helpful Predicates (3):",
		"file_topology",
		"test_state",
		"error_context",
		"Noise Predicates (2):",
		"browser_state",
		"dom_node",
		"Missing Context: dependency graph",
		"Total Feedback Samples: 42",
		"Active Predicates",
	}

	for _, expected := range expectedStrings {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", expected, output)
		}
	}

	t.Logf("FeedbackTracer output:\n%s", output)
}

// TestPiggybackTracerContextFeedback verifies PiggybackTracer logs context feedback.
func TestPiggybackTracerContextFeedback(t *testing.T) {
	var buf bytes.Buffer
	tracer := NewPiggybackTracer(&buf, false)

	event := &PiggybackEvent{
		Timestamp:       time.Now(),
		TurnNumber:      10,
		Speaker:         "assistant",
		SurfaceText:     "I've fixed the authentication bug.",
		ResponseTokens:  250,
		ResponseLatency: 1500 * time.Millisecond,
		ControlPacket: &ControlPacket{
			IntentClassification: IntentClassification{
				Category:   "/mutation",
				Verb:       "/fix",
				Target:     "auth.go",
				Confidence: 0.95,
			},
			MangleUpdates: []string{"task_status(/fix, /complete)"},
			ContextFeedback: &ContextFeedback{
				OverallUsefulness: 0.9,
				HelpfulFacts:      []string{"error_context", "test_state"},
				NoiseFacts:        []string{"campaign_context"},
				MissingContext:    "",
			},
		},
	}

	tracer.TracePiggyback(event)

	output := buf.String()

	// Verify context feedback section is present
	expectedStrings := []string{
		"PIGGYBACK PROTOCOL - TURN 10",
		"Context Feedback:",
		"Overall Usefulness: 0.90",
		"Helpful Predicates (2):",
		"error_context",
		"test_state",
		"Noise Predicates (1):",
		"campaign_context",
	}

	for _, expected := range expectedStrings {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", expected, output)
		}
	}

	t.Logf("PiggybackTracer output:\n%s", output)
}

// TestContextFeedbackStoreIntegration tests the complete feedback storage flow.
func TestContextFeedbackStoreIntegration(t *testing.T) {
	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "feedback_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "context_feedback.db")

	// Create feedback store
	store, err := ctxcompress.NewContextFeedbackStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create feedback store: %v", err)
	}
	defer store.Close()

	// Store some feedback samples
	testCases := []struct {
		turnID       int
		usefulness   float64
		intentVerb   string
		helpfulFacts []string
		noiseFacts   []string
	}{
		{1, 0.9, "/fix", []string{"file_topology", "test_state"}, []string{"browser_state"}},
		{2, 0.8, "/fix", []string{"file_topology", "error_context"}, []string{"dom_node"}},
		{3, 0.7, "/fix", []string{"file_topology"}, []string{"browser_state", "campaign_context"}},
		{4, 0.6, "/test", []string{"test_state"}, []string{"file_topology"}},
		{5, 0.85, "/fix", []string{"file_topology", "test_state"}, []string{}},
		// Add more samples to exceed minimum threshold (10)
		{6, 0.9, "/fix", []string{"file_topology"}, []string{"browser_state"}},
		{7, 0.8, "/fix", []string{"file_topology"}, []string{"browser_state"}},
		{8, 0.85, "/fix", []string{"file_topology"}, []string{}},
		{9, 0.75, "/fix", []string{"file_topology"}, []string{"browser_state"}},
		{10, 0.9, "/fix", []string{"file_topology"}, []string{"browser_state"}},
		{11, 0.8, "/fix", []string{"file_topology"}, []string{"browser_state"}},
		{12, 0.85, "/fix", []string{"file_topology"}, []string{"browser_state"}},
	}

	for _, tc := range testCases {
		err := store.StoreFeedback(tc.turnID, "", tc.usefulness, tc.intentVerb, tc.helpfulFacts, tc.noiseFacts)
		if err != nil {
			t.Fatalf("Failed to store feedback for turn %d: %v", tc.turnID, err)
		}
	}

	// Query overall stats
	totalFeedback, avgUsefulness, err := store.GetOverallStats()
	if err != nil {
		t.Fatalf("Failed to get overall stats: %v", err)
	}

	t.Logf("Overall stats: total=%d, avg_usefulness=%.2f", totalFeedback, avgUsefulness)

	if totalFeedback != len(testCases) {
		t.Errorf("Expected %d feedback entries, got %d", len(testCases), totalFeedback)
	}

	// Query predicate usefulness (should have enough samples now)
	fileTopologyScore := store.GetPredicateUsefulness("file_topology")
	t.Logf("file_topology usefulness score: %.2f", fileTopologyScore)

	// file_topology was helpful in most cases, so score should be positive
	// (exact value depends on time decay, but should be > 0)
	if fileTopologyScore < 0 {
		t.Errorf("Expected positive score for file_topology, got %.2f", fileTopologyScore)
	}

	browserStateScore := store.GetPredicateUsefulness("browser_state")
	t.Logf("browser_state usefulness score: %.2f", browserStateScore)

	// browser_state was mostly noise, so score should be negative
	if browserStateScore > 0 {
		t.Errorf("Expected negative score for browser_state, got %.2f", browserStateScore)
	}

	// Query intent-specific usefulness
	fileTopologyFixScore := store.GetPredicateUsefulnessForIntent("file_topology", "/fix")
	t.Logf("file_topology usefulness for /fix intent: %.2f", fileTopologyFixScore)

	// Get top helpful predicates
	topHelpful, err := store.GetTopHelpfulPredicates(5)
	if err != nil {
		t.Fatalf("Failed to get top helpful predicates: %v", err)
	}

	t.Logf("Top helpful predicates:")
	for _, p := range topHelpful {
		t.Logf("  %s: score=%.2f (helpful=%d, noise=%d)",
			p.Predicate, p.WeightedScore, p.HelpfulCount, p.NoiseCount)
	}

	// Get top noise predicates
	topNoise, err := store.GetTopNoisePredicates(5)
	if err != nil {
		t.Fatalf("Failed to get top noise predicates: %v", err)
	}

	t.Logf("Top noise predicates:")
	for _, p := range topNoise {
		t.Logf("  %s: score=%.2f (helpful=%d, noise=%d)",
			p.Predicate, p.WeightedScore, p.HelpfulCount, p.NoiseCount)
	}
}

// TestFeedbackScoreImpact tests TraceScoreImpact output.
func TestFeedbackScoreImpact(t *testing.T) {
	var buf bytes.Buffer
	tracer := NewFeedbackTracer(&buf, true)

	impacts := []PredicateScoreImpact{
		{
			Predicate:   "file_topology",
			BaseScore:   50.0,
			FeedbackMod: 15.0,
			FinalScore:  65.0,
			ScoreDelta:  15.0,
		},
		{
			Predicate:   "browser_state",
			BaseScore:   40.0,
			FeedbackMod: -18.0,
			FinalScore:  22.0,
			ScoreDelta:  -18.0,
		},
		{
			Predicate:   "test_state",
			BaseScore:   45.0,
			FeedbackMod: 10.0,
			FinalScore:  55.0,
			ScoreDelta:  10.0,
		},
	}

	tracer.TraceScoreImpact(15, impacts)

	output := buf.String()

	expectedStrings := []string{
		"FEEDBACK SCORE IMPACT - TURN 15",
		"browser_state",
		"-18.0",
		"file_topology",
		"+15.0",
	}

	for _, expected := range expectedStrings {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", expected, output)
		}
	}

	t.Logf("TraceScoreImpact output:\n%s", output)
}
