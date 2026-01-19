package core

import (
	"testing"
)

func TestDreamLearningCollector_New(t *testing.T) {
	collector := NewDreamLearningCollector()
	if collector == nil {
		t.Fatal("NewDreamLearningCollector returned nil")
	}
}

func TestDreamLearningCollector_ExtractLearnings(t *testing.T) {
	collector := NewDreamLearningCollector()

	hypothetical := "Figure out how to deploy"
	consultations := []DreamConsultation{
		{
			ShardName:   "devops",
			ShardType:   "specialist",
			Perspective: "Use Docker for containerization. Run 'docker build' then 'docker push'.",
		},
	}

	learnings := collector.ExtractLearnings(hypothetical, consultations)

	// May or may not extract learnings depending on content patterns
	t.Logf("Extracted %d learnings", len(learnings))

	for _, l := range learnings {
		if l.ID == "" {
			t.Error("Learning missing ID")
		}
		if l.Type == "" {
			t.Error("Learning missing Type")
		}
	}
}

func TestDreamLearningCollector_ConfirmLearnings(t *testing.T) {
	collector := NewDreamLearningCollector()

	// Add some staged learnings
	consultations := []DreamConsultation{
		{
			ShardName:   "coder",
			Perspective: "Always use structured logging. This is a best practice.",
		},
	}
	collector.ExtractLearnings("learn patterns", consultations)

	// Confirm with positive feedback
	confirmed := collector.ConfirmLearnings("Yes, that's correct!")

	t.Logf("Confirmed %d learnings", len(confirmed))
}

func TestDreamLearningCollector_LearnCorrection(t *testing.T) {
	collector := NewDreamLearningCollector()

	correction := "No, we use PostgreSQL not MySQL"
	learning := collector.LearnCorrection(correction, LearningTypePreference)

	if learning == nil {
		t.Fatal("LearnCorrection returned nil")
	}

	if learning.Content != correction {
		t.Errorf("Expected content %q, got %q", correction, learning.Content)
	}

	if learning.Type != LearningTypePreference {
		t.Errorf("Expected type %s, got %s", LearningTypePreference, learning.Type)
	}
}

func TestDreamLearningCollector_GetPendingAndConfirmed(t *testing.T) {
	collector := NewDreamLearningCollector()

	// Initially empty
	pending := collector.GetPendingLearnings()
	confirmed := collector.GetConfirmedLearnings()

	if len(pending) != 0 {
		t.Errorf("Expected 0 pending learnings, got %d", len(pending))
	}
	if len(confirmed) != 0 {
		t.Errorf("Expected 0 confirmed learnings, got %d", len(confirmed))
	}
}

func TestDreamLearningCollector_ClearStaged(t *testing.T) {
	collector := NewDreamLearningCollector()

	// Add some learnings
	collector.ExtractLearnings("test", []DreamConsultation{
		{Perspective: "Important learning here"},
	})

	// Clear
	collector.ClearStaged()

	pending := collector.GetPendingLearnings()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending after clear, got %d", len(pending))
	}
}

func TestDreamLearningCollector_GetStats(t *testing.T) {
	collector := NewDreamLearningCollector()

	stats := collector.GetStats()

	if stats == nil {
		t.Fatal("GetStats returned nil")
	}

	// Should have some keys
	if _, ok := stats["total_extracted"]; !ok {
		t.Log("Warning: stats missing 'total_extracted' key")
	}
}
