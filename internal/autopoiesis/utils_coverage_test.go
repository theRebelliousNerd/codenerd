package autopoiesis

import (
	"regexp"
	"testing"
)

func compilePatternForTest(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

// --- normalizePercent ---

// want is int64, not float64: tool_learning's numeric slots are declared
// /number, and this Mangle fork compares int64 only — a float here aborts the
// whole kernel fixpoint rather than just failing the tool_quality_* rules.
func TestNormalizePercent_AllBands(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  int64
	}{
		{"zero", 0.0, 0},
		{"negative", -1.0, 0},
		{"fraction", 0.5, 50},
		{"fraction_low", 0.1, 10},
		{"one", 1.0, 1},     // >= 1, read as a percent, not a ratio
		{"fifty", 50.0, 50}, // >= 1, <= 100
		{"hundred", 100.0, 100},
		{"over_hundred", 200.0, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePercent(tt.input)
			if got != tt.want {
				t.Errorf("normalizePercent(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- normalizeCapabilityName ---

func TestNormalizeCapabilityName_AllCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "/unknown"},
		{"  ", "/unknown"},
		{"/already_prefixed", "/already_prefixed"},
		{"tool_name", "/tool_name"},
		{" spaces ", "/spaces"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeCapabilityName(tt.input)
			if got != tt.want {
				t.Errorf("normalizeCapabilityName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- clamp ---

func TestClamp_AllCases(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		min, max float64
		want     float64
	}{
		{"within", 0.5, 0.0, 1.0, 0.5},
		{"at_min", 0.0, 0.0, 1.0, 0.0},
		{"at_max", 1.0, 0.0, 1.0, 1.0},
		{"below_min", -0.5, 0.0, 1.0, 0.0},
		{"above_max", 1.5, 0.0, 1.0, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp(tt.value, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("clamp(%v, %v, %v) = %v, want %v", tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

// --- truncate ---

func TestTruncate_AllCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLen    int
		wantExact string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.wantExact {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.wantExact)
			}
		})
	}
}

// --- hasSuggestion ---

func TestHasSuggestion_WhenPresent_ShouldReturnTrue(t *testing.T) {
	suggestions := []ImprovementSuggestion{
		{Type: SuggestAddPagination},
		{Type: SuggestCaching},
	}
	if !hasSuggestion(suggestions, SuggestAddPagination) {
		t.Error("expected true for existing suggestion")
	}
}

func TestHasSuggestion_WhenAbsent_ShouldReturnFalse(t *testing.T) {
	suggestions := []ImprovementSuggestion{
		{Type: SuggestAddPagination},
	}
	if hasSuggestion(suggestions, SuggestCaching) {
		t.Error("expected false for missing suggestion")
	}
}

func TestHasSuggestion_WhenEmpty_ShouldReturnFalse(t *testing.T) {
	if hasSuggestion(nil, SuggestCaching) {
		t.Error("expected false for nil suggestions")
	}
}

// --- extractMatch ---

func TestExtractMatch_WhenShort_ShouldReturnFull(t *testing.T) {
	got := extractMatch("error in line 5", compilePatternForTest(`error`))
	if got != "error" {
		t.Errorf("got %q, want 'error'", got)
	}
}

// --- NewQualityEvaluator ---

func TestNewQualityEvaluator_ShouldInitialize(t *testing.T) {
	qe := NewQualityEvaluator(nil, nil)
	if qe == nil {
		t.Fatal("expected non-nil QualityEvaluator")
	}
	if len(qe.heuristicRules) == 0 {
		t.Error("expected default heuristic rules")
	}
	if len(qe.completenessHints) == 0 {
		t.Error("expected default completeness hints")
	}
}
