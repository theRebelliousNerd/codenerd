package autopoiesis

import (
	"testing"
)

// --- calculatePatternConfidence ---

func TestCalculatePatternConfidence_AllBands(t *testing.T) {
	tests := []struct {
		name        string
		occurrences int
		want        float64
	}{
		{"single", 1, 0.3},
		{"two", 2, 0.5},
		{"three", 3, 0.7},
		{"four", 4, 0.7},
		{"five", 5, 0.9},
		{"many", 100, 0.9},
		{"zero", 0, 0.3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePatternConfidence(tt.occurrences)
			if got != tt.want {
				t.Errorf("calculatePatternConfidence(%d) = %v, want %v", tt.occurrences, got, tt.want)
			}
		})
	}
}

// --- NewPatternDetector ---

func TestNewPatternDetector_ShouldReturnInitialized(t *testing.T) {
	pd := NewPatternDetector()
	if pd == nil {
		t.Fatal("expected non-nil PatternDetector")
	}
	if pd.patterns == nil {
		t.Error("expected patterns map to be initialized")
	}
	if pd.history == nil {
		t.Error("expected history slice to be initialized")
	}
}

// --- GetPatterns ---

func TestGetPatterns_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	pd := NewPatternDetector()
	got := pd.GetPatterns(0.0)
	if len(got) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(got))
	}
}

func TestGetPatterns_WhenAboveThreshold_ShouldFilter(t *testing.T) {
	pd := NewPatternDetector()
	pd.patterns["low"] = &DetectedPattern{Confidence: 0.3}
	pd.patterns["high"] = &DetectedPattern{Confidence: 0.8}

	got := pd.GetPatterns(0.5)
	if len(got) != 1 {
		t.Errorf("expected 1 pattern above 0.5, got %d", len(got))
	}
}

// --- GetToolPatterns ---

func TestGetToolPatterns_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	pd := NewPatternDetector()
	got := pd.GetToolPatterns("nonexistent")
	if len(got) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(got))
	}
}

func TestGetToolPatterns_WhenMatching_ShouldFilter(t *testing.T) {
	pd := NewPatternDetector()
	pd.patterns["tool1:issue"] = &DetectedPattern{ToolName: "tool1", Confidence: 0.8}
	pd.patterns["tool2:issue"] = &DetectedPattern{ToolName: "tool2", Confidence: 0.8}

	got := pd.GetToolPatterns("tool1")
	if len(got) != 1 {
		t.Errorf("expected 1 pattern for tool1, got %d", len(got))
	}
}

// --- RecordExecution ---

func TestRecordExecution_WhenNoQuality_ShouldJustAppend(t *testing.T) {
	pd := NewPatternDetector()
	pd.RecordExecution(ExecutionFeedback{
		ToolName: "test_tool",
		Success:  true,
	})
	if len(pd.history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(pd.history))
	}
	if len(pd.patterns) != 0 {
		t.Errorf("expected 0 patterns without quality, got %d", len(pd.patterns))
	}
}

func TestRecordExecution_WhenQualityIssues_ShouldCreatePatterns(t *testing.T) {
	pd := NewPatternDetector()
	pd.RecordExecution(ExecutionFeedback{
		ToolName: "test_tool",
		Success:  false,
		Quality: &QualityAssessment{
			Issues: []QualityIssue{
				{Type: IssuePartialFailure, Evidence: "nil pointer"},
			},
		},
	})
	if len(pd.patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(pd.patterns))
	}
}
