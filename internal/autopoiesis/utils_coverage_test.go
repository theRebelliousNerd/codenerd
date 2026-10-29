package autopoiesis

import (
	"regexp"
	"strings"
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
		// Both inputs are 0..1 ratios (feedback.go keeps SuccessRate as a
		// running hit rate and AverageQuality as a running mean of 0..1 scores;
		// autopoiesis_feedback.go tests AverageQuality < 0.5). This case used to
		// expect 1, because normalizePercent guessed that anything >= 1 was
		// already a percent -- so a tool that had never failed was recorded as a
		// 1% success rate and tool_quality_* (SuccessRate > 50) queued it for
		// deprecation. Out-of-range inputs saturate rather than being
		// reinterpreted.
		{"perfect ratio saturates", 1.0, 100},
		{"out of range saturates", 50.0, 100},
		{"far out of range saturates", 200.0, 100},
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

// --- known-issue names ---

// tool_known_issue is rendered into learned.mg text. The old
// normalizeCapabilityName only prefixed a slash, so an issue type with a space
// or capital letter became a token that does not parse and broke the whole
// file at the next boot. types.Atom always yields a valid name.
func TestGenerateMangleFacts_IssueTypeIsAValidName(t *testing.T) {
	store := NewLearningStore(t.TempDir())
	store.RecordLearning("t", &ExecutionFeedback{
		ToolName: "t",
		Success:  true,
		Quality:  &QualityAssessment{Score: 0.5, Issues: []QualityIssue{{Type: IssueType("Slow Path")}}},
	}, nil)
	var found bool
	for _, fact := range store.GenerateMangleFacts() {
		if strings.HasPrefix(fact, "tool_known_issue(") {
			found = true
			if !strings.HasSuffix(fact, ", /slow_path).") {
				t.Errorf("issue type not rendered as a valid name: %s", fact)
			}
		}
	}
	if !found {
		t.Fatal("no tool_known_issue fact rendered")
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
