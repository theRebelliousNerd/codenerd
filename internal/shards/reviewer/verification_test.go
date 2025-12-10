package reviewer

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseVerdicts(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     []Verdict
		wantErr  bool
	}{
		{
			name: "valid JSON array in markdown block",
			response: `Here are my verdicts:

` + "```json" + `
[
  {
    "hypothesis_type": "unchecked_error",
    "file": "main.go",
    "line": 42,
    "decision": "CONFIRMED",
    "reasoning": "Error from db.Query is ignored",
    "confidence": 0.95,
    "fix": "if err != nil { return err }",
    "false_positive": false,
    "pattern_note": ""
  }
]
` + "```" + `
`,
			want: []Verdict{
				{
					HypothesisType: HypothesisUncheckedError,
					File:           "main.go",
					Line:           42,
					Decision:       "CONFIRMED",
					Reasoning:      "Error from db.Query is ignored",
					Confidence:     0.95,
					Fix:            "if err != nil { return err }",
					FalsePositive:  false,
				},
			},
			wantErr: false,
		},
		{
			name: "dismissed verdict",
			response: `
` + "```json" + `
[
  {
    "hypothesis_type": "sql_injection",
    "file": "handler.go",
    "line": 100,
    "decision": "dismissed",
    "reasoning": "Query uses parameterized arguments",
    "confidence": 0.9,
    "false_positive": true,
    "pattern_note": "Parameterized query pattern"
  }
]
` + "```" + `
`,
			want: []Verdict{
				{
					HypothesisType: HypothesisSQLInjection,
					File:           "handler.go",
					Line:           100,
					Decision:       "DISMISSED",
					Reasoning:      "Query uses parameterized arguments",
					Confidence:     0.9,
					FalsePositive:  true,
					PatternNote:    "Parameterized query pattern",
				},
			},
			wantErr: false,
		},
		{
			name: "multiple verdicts",
			response: `[
  {"hypothesis_type": "nil_channel", "file": "a.go", "line": 10, "decision": "CONFIRMED", "reasoning": "Channel never initialized", "confidence": 0.8},
  {"hypothesis_type": "race_condition", "file": "a.go", "line": 20, "decision": "DISMISSED", "reasoning": "Protected by mutex", "confidence": 0.85, "false_positive": true}
]`,
			want: []Verdict{
				{HypothesisType: HypothesisNilChannel, File: "a.go", Line: 10, Decision: "CONFIRMED", Reasoning: "Channel never initialized", Confidence: 0.8},
				{HypothesisType: HypothesisRaceCondition, File: "a.go", Line: 20, Decision: "DISMISSED", Reasoning: "Protected by mutex", Confidence: 0.85, FalsePositive: true},
			},
			wantErr: false,
		},
		{
			name:     "no JSON in response",
			response: "I analyzed the code and found no issues.",
			want:     nil,
			wantErr:  true,
		},
		{
			name: "confidence clamping - over 1.0",
			response: `[{"hypothesis_type": "test", "file": "x.go", "line": 1, "decision": "CONFIRMED", "reasoning": "test", "confidence": 1.5}]`,
			want: []Verdict{
				{HypothesisType: "test", File: "x.go", Line: 1, Decision: "CONFIRMED", Reasoning: "test", Confidence: 1.0},
			},
			wantErr: false,
		},
		{
			name: "confidence clamping - negative",
			response: `[{"hypothesis_type": "test", "file": "x.go", "line": 1, "decision": "CONFIRMED", "reasoning": "test", "confidence": -0.5}]`,
			want: []Verdict{
				{HypothesisType: "test", File: "x.go", Line: 1, Decision: "CONFIRMED", Reasoning: "test", Confidence: 0},
			},
			wantErr: false,
		},
		{
			name: "invalid decision defaults to DISMISSED",
			response: `[{"hypothesis_type": "test", "file": "x.go", "line": 1, "decision": "MAYBE", "reasoning": "uncertain", "confidence": 0.5}]`,
			want: []Verdict{
				{HypothesisType: "test", File: "x.go", Line: 1, Decision: "DISMISSED", Reasoning: "uncertain", Confidence: 0.5},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVerdicts(tt.response)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVerdicts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("parseVerdicts() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExtractJSONFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     string
	}{
		{
			name:     "markdown json block",
			response: "Here is the result:\n```json\n[{\"a\": 1}]\n```\nDone.",
			want:     "[{\"a\": 1}]",
		},
		{
			name:     "plain code block",
			response: "```\n{\"key\": \"value\"}\n```",
			want:     "{\"key\": \"value\"}",
		},
		{
			name:     "raw JSON array",
			response: "The analysis returned [{\"x\": 1}, {\"x\": 2}] as results.",
			want:     "[{\"x\": 1}, {\"x\": 2}]",
		},
		{
			name:     "raw JSON object",
			response: "Result: {\"status\": \"ok\"}.",
			want:     "{\"status\": \"ok\"}",
		},
		{
			name:     "nested brackets",
			response: `[{"nested": {"deep": [1,2,3]}}]`,
			want:     `[{"nested": {"deep": [1,2,3]}}]`,
		},
		{
			name:     "no JSON",
			response: "This response has no JSON content.",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONFromResponse(tt.response)
			if got != tt.want {
				t.Errorf("extractJSONFromResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetermineSeverity(t *testing.T) {
	tests := []struct {
		name       string
		hypoType   HypothesisType
		confidence float64
		want       string
	}{
		{
			name:       "SQL injection is critical",
			hypoType:   HypothesisSQLInjection,
			confidence: 0.9,
			want:       "critical",
		},
		{
			name:       "command injection is critical",
			hypoType:   HypothesisCommandInjection,
			confidence: 0.85,
			want:       "critical",
		},
		{
			name:       "low confidence critical downgraded to error",
			hypoType:   HypothesisSQLInjection,
			confidence: 0.5,
			want:       "error",
		},
		{
			name:       "nil deref is error",
			hypoType:   HypothesisUnsafeDeref,
			confidence: 0.8,
			want:       "error",
		},
		{
			name:       "unchecked error is warning",
			hypoType:   HypothesisUncheckedError,
			confidence: 0.7,
			want:       "warning",
		},
		{
			name:       "high confidence warning upgraded to error",
			hypoType:   HypothesisUncheckedError,
			confidence: 0.95,
			want:       "error",
		},
		{
			name:       "generic type is info",
			hypoType:   HypothesisGeneric,
			confidence: 0.6,
			want:       "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineSeverity(tt.hypoType, tt.confidence)
			if got != tt.want {
				t.Errorf("determineSeverity(%v, %.2f) = %q, want %q",
					tt.hypoType, tt.confidence, got, tt.want)
			}
		})
	}
}

func TestFindMatchingHypothesis(t *testing.T) {
	hypos := []Hypothesis{
		{Type: HypothesisUncheckedError, File: "main.go", Line: 10, Variable: "err"},
		{Type: HypothesisSQLInjection, File: "handler.go", Line: 50, Variable: "query"},
		{Type: HypothesisNilChannel, File: "worker.go", Line: 100, Variable: "ch"},
	}

	tests := []struct {
		name    string
		verdict Verdict
		want    *Hypothesis
	}{
		{
			name:    "exact match",
			verdict: Verdict{File: "main.go", Line: 10, HypothesisType: HypothesisUncheckedError},
			want:    &hypos[0],
		},
		{
			name:    "exact match different type",
			verdict: Verdict{File: "handler.go", Line: 50, HypothesisType: HypothesisSQLInjection},
			want:    &hypos[1],
		},
		{
			name:    "fuzzy match within 3 lines",
			verdict: Verdict{File: "worker.go", Line: 102, HypothesisType: HypothesisNilChannel},
			want:    &hypos[2],
		},
		{
			name:    "same file and type fallback",
			verdict: Verdict{File: "main.go", Line: 999, HypothesisType: HypothesisUncheckedError},
			want:    &hypos[0],
		},
		{
			name:    "no match - wrong file",
			verdict: Verdict{File: "other.go", Line: 10, HypothesisType: HypothesisUncheckedError},
			want:    nil,
		},
		{
			name:    "no match - wrong type and line",
			verdict: Verdict{File: "main.go", Line: 999, HypothesisType: HypothesisSQLInjection},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMatchingHypothesis(hypos, tt.verdict)
			if tt.want == nil {
				if got != nil {
					t.Errorf("findMatchingHypothesis() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("findMatchingHypothesis() = nil, want %+v", tt.want)
				return
			}
			if got.File != tt.want.File || got.Line != tt.want.Line || got.Type != tt.want.Type {
				t.Errorf("findMatchingHypothesis() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildDismissalReason(t *testing.T) {
	tests := []struct {
		name    string
		verdict Verdict
		want    string
	}{
		{
			name:    "reasoning only",
			verdict: Verdict{Reasoning: "Code is safe"},
			want:    "Code is safe",
		},
		{
			name:    "with pattern note",
			verdict: Verdict{Reasoning: "Safe", PatternNote: "Guarded by mutex"},
			want:    "Safe; Pattern: Guarded by mutex",
		},
		{
			name:    "confirmed false positive",
			verdict: Verdict{Reasoning: "Not exploitable", FalsePositive: true},
			want:    "Not exploitable; Confirmed false positive",
		},
		{
			name:    "all fields",
			verdict: Verdict{Reasoning: "Safe", PatternNote: "Test pattern", FalsePositive: true},
			want:    "Safe; Pattern: Test pattern; Confirmed false positive",
		},
		{
			name:    "empty verdict",
			verdict: Verdict{},
			want:    "Dismissed by LLM verification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDismissalReason(tt.verdict)
			if got != tt.want {
				t.Errorf("buildDismissalReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAbsInt(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{5, 5},
		{-5, 5},
		{0, 0},
		{-1, 1},
		{1, 1},
	}

	for _, tt := range tests {
		got := absInt(tt.input)
		if got != tt.want {
			t.Errorf("absInt(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestTruncatePreservingHypothesisLines(t *testing.T) {
	// Create a 20-line file
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, "line "+string(rune('0'+i%10)))
	}
	code := ""
	for _, l := range lines {
		code += l + "\n"
	}

	tests := []struct {
		name   string
		hypos  []Hypothesis
		maxLen int
	}{
		{
			name:   "preserves hypothesis at line 10",
			hypos:  []Hypothesis{{Line: 10}},
			maxLen: 500,
		},
		{
			name:   "preserves multiple hypotheses",
			hypos:  []Hypothesis{{Line: 5}, {Line: 15}},
			maxLen: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncatePreservingHypothesisLines(code, tt.hypos, tt.maxLen)

			// Verify hypothesis lines are present
			for _, h := range tt.hypos {
				expectedLine := "line " + string(rune('0'+h.Line%10))
				if !containsSubstring(result, expectedLine) {
					t.Errorf("truncatePreservingHypothesisLines() missing line %d content", h.Line)
				}
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s[1:], substr) || s[:len(substr)] == substr)
}

func TestToFindings(t *testing.T) {
	verified := []VerifiedFinding{
		{
			ReviewFinding: ReviewFinding{File: "a.go", Line: 10, Severity: "error", Message: "Issue 1"},
			LogicTrace:    "trace1",
			Reasoning:     "reason1",
		},
		{
			ReviewFinding: ReviewFinding{File: "b.go", Line: 20, Severity: "warning", Message: "Issue 2"},
			LogicTrace:    "trace2",
			Reasoning:     "reason2",
		},
	}

	findings := ToFindings(verified)

	if len(findings) != len(verified) {
		t.Errorf("ToFindings() returned %d findings, want %d", len(findings), len(verified))
	}

	for i, f := range findings {
		if f.File != verified[i].File || f.Line != verified[i].Line {
			t.Errorf("ToFindings()[%d] = %+v, want File=%s Line=%d",
				i, f, verified[i].File, verified[i].Line)
		}
	}
}
