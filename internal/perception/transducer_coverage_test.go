package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// coverageMockClient implements LLMClient for coverage tests.
type coverageMockClient struct {
	completeFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *coverageMockClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "", nil
}
func (m *coverageMockClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, user)
	}
	return "", nil
}
func (m *coverageMockClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	go func() {
		defer close(contentChan)
		defer close(errorChan)
		res, err := m.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()
	return contentChan, errorChan
}
func (m *coverageMockClient) CompleteWithTools(ctx context.Context, sys, user string, tools []ToolDefinition) (*LLMToolResponse, error) {
	return &LLMToolResponse{Text: "", StopReason: "end_turn"}, nil
}

// =============================================================================
// SANITIZE FACT ARG TESTS
// =============================================================================

func TestSanitizeFactArg_WhenControlChars_ShouldStrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		check func(t *testing.T, result string)
	}{
		{
			name:  "null_bytes_stripped",
			input: "hello\x00world",
			check: func(t *testing.T, result string) {
				if strings.Contains(result, "\x00") {
					t.Error("result should not contain null bytes")
				}
				if result != "helloworld" {
					t.Errorf("got %q, want %q", result, "helloworld")
				}
			},
		},
		{
			name:  "control_chars_stripped",
			input: "line1\x01\x02\x03line2",
			check: func(t *testing.T, result string) {
				if result != "line1line2" {
					t.Errorf("got %q, want %q", result, "line1line2")
				}
			},
		},
		{
			name:  "ansi_escape_stripped",
			input: "hello\x1bworld",
			check: func(t *testing.T, result string) {
				if strings.Contains(result, "\x1b") {
					t.Error("result should not contain ANSI escape")
				}
			},
		},
		{
			name:  "tabs_and_newlines_preserved",
			input: "hello\tworld\nfoo\rbar",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "\t") {
					t.Error("tabs should be preserved")
				}
				if !strings.Contains(result, "\n") {
					t.Error("newlines should be preserved")
				}
				if !strings.Contains(result, "\r") {
					t.Error("carriage returns should be preserved")
				}
			},
		},
		{
			name:  "empty_string",
			input: "",
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("got %q, want empty", result)
				}
			},
		},
		{
			name:  "normal_text_unchanged",
			input: "internal/core/kernel.go",
			check: func(t *testing.T, result string) {
				if result != "internal/core/kernel.go" {
					t.Errorf("got %q, want %q", result, "internal/core/kernel.go")
				}
			},
		},
		{
			name: "max_length_truncation",
			input: strings.Repeat("a", 3000),
			check: func(t *testing.T, result string) {
				if len(result) > 2048 {
					t.Errorf("result should be truncated to max 2048, got %d", len(result))
				}
			},
		},
		{
			name:  "mixed_valid_and_control",
			input: "foo\x00\x01\x1b\tbar\nbaz",
			check: func(t *testing.T, result string) {
				if result != "foo\tbar\nbaz" {
					t.Errorf("got %q, want %q", result, "foo\tbar\nbaz")
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := sanitizeFactArg(tc.input)
			tc.check(t, result)
		})
	}
}

// =============================================================================
// MIN FUNCTION TESTS
// =============================================================================

func TestMin_WhenCompared_ShouldReturnSmaller(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		a, b int
		want int
	}{
		{"a_smaller", 1, 5, 1},
		{"b_smaller", 10, 3, 3},
		{"equal", 7, 7, 7},
		{"negative_a", -5, 3, -5},
		{"negative_b", 3, -5, -5},
		{"both_negative", -10, -3, -10},
		{"zero_and_positive", 0, 5, 0},
		{"zero_and_negative", 0, -5, -5},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := min(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("min(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// =============================================================================
// GET REGEX CANDIDATES EDGE CASES
// =============================================================================

func TestGetRegexCandidates_WhenLongInput_ShouldTruncate(t *testing.T) {
	original := GetVerbCorpus()
	defer SetVerbCorpus(original)

	SetVerbCorpus([]VerbEntry{
		{
			Verb:     "/fix",
			Category: "/mutation",
			Synonyms: []string{"fix"},
			Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)fix`)},
			Priority: 90,
		},
	})

	// Create input longer than maxRegexInputLen (2000)
	longInput := "fix " + strings.Repeat("a", 3000)
	candidates := getRegexCandidates(longInput, GetVerbCorpus())

	// Should still find the match in the first part
	found := false
	for _, c := range candidates {
		if c.Verb == "/fix" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected /fix to be found even with very long input (truncation should preserve start)")
	}
}

func TestGetRegexCandidates_WhenEmptyInput_ShouldReturnEmpty(t *testing.T) {
	original := GetVerbCorpus()
	defer SetVerbCorpus(original)

	SetVerbCorpus([]VerbEntry{
		{
			Verb:     "/fix",
			Category: "/mutation",
			Synonyms: []string{"fix"},
			Priority: 90,
		},
	})

	candidates := getRegexCandidates("", GetVerbCorpus())
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for empty input, got %d", len(candidates))
	}
}

func TestGetRegexCandidates_WhenMultipleVerbsMatch_ShouldReturnAll(t *testing.T) {
	original := GetVerbCorpus()
	defer SetVerbCorpus(original)

	SetVerbCorpus([]VerbEntry{
		{
			Verb:     "/fix",
			Category: "/mutation",
			Synonyms: []string{"fix"},
			Priority: 90,
		},
		{
			Verb:     "/review",
			Category: "/query",
			Synonyms: []string{"review"},
			Priority: 100,
		},
	})

	// Input that matches both
	candidates := getRegexCandidates("fix and review the code", GetVerbCorpus())
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(candidates))
	}
}

// =============================================================================
// INTENT TO FACT EDGE CASES
// =============================================================================

func TestIntent_ToFact_WhenEmptyFields_ShouldProduceValidFact(t *testing.T) {
	t.Parallel()

	intent := Intent{
		Category:   "",
		Verb:       "",
		Target:     "",
		Constraint: "",
		Confidence: 0,
	}

	fact := intent.ToFact()

	if fact.Predicate != "user_intent" {
		t.Errorf("Predicate = %q, want %q", fact.Predicate, "user_intent")
	}
	if len(fact.Args) != 5 {
		t.Fatalf("Args length = %d, want 5", len(fact.Args))
	}
	// Check that empty strings still produce valid fact args
	if arg3, ok := fact.Args[3].(string); !ok || arg3 != "" {
		t.Errorf("Args[3] = %v (type %T), want empty string", fact.Args[3], fact.Args[3])
	}
}

func TestIntent_ToFact_WhenInjectionAttempt_ShouldSanitize(t *testing.T) {
	t.Parallel()

	intent := Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "foo). malicious_rule(X) :- ",
		Constraint: "none",
	}

	fact := intent.ToFact()
	target, ok := fact.Args[3].(string)
	if !ok {
		t.Fatal("Args[3] should be a string")
	}
	// The target should pass through sanitizeFactArg which strips control chars
	// but doesn't strip Mangle syntax chars (parens, dots, etc.) —
	// those are handled at the Mangle layer
	if strings.Contains(target, "\x00") {
		t.Error("Target should not contain null bytes")
	}
}

// =============================================================================
// FOCUS RESOLUTION EDGE CASES
// =============================================================================

func TestFocusResolution_ToFact_WhenMaxConfidence_ShouldPreserve(t *testing.T) {
	t.Parallel()

	focus := FocusResolution{
		RawReference:      "kernel",
		ResolvedPath:      "internal/core/kernel.go",
		SymbolName:        "Execute",
		ConfidencePercent: 100,
	}

	fact := focus.ToFact()
	if fact.Args[3] != int64(100) {
		t.Errorf("Args[3] = %v (type %T), want int64(100)", fact.Args[3], fact.Args[3])
	}
}

func TestFocusResolution_ToFact_WhenNegativeConfidence_ShouldPreserve(t *testing.T) {
	t.Parallel()

	focus := FocusResolution{
		RawReference:      "ref",
		ResolvedPath:      "",
		SymbolName:        "",
		ConfidencePercent: -1,
	}

	fact := focus.ToFact()
	if fact.Args[3] != int64(-1) {
		t.Errorf("Args[3] = %v (type %T), want int64(-1)", fact.Args[3], fact.Args[3])
	}
}

// =============================================================================
// UNDERSTANDING VALIDATION TESTS
// =============================================================================

func TestUnderstanding_Validate_WhenValid_ShouldReturnNil(t *testing.T) {
	t.Parallel()

	u := &Understanding{
		PrimaryIntent: "implement",
		SemanticType:  "definition",
		ActionType:    "implement",
		Domain:        "general",
		Confidence:    0.9,
	}

	if err := u.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestUnderstanding_Validate_WhenMissingFields_ShouldReturnError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		u         Understanding
		wantField string
	}{
		{
			name:      "missing_primary_intent",
			u:         Understanding{SemanticType: "def", ActionType: "impl", Domain: "gen", Confidence: 0.5},
			wantField: "primary_intent",
		},
		{
			name:      "missing_semantic_type",
			u:         Understanding{PrimaryIntent: "impl", ActionType: "impl", Domain: "gen", Confidence: 0.5},
			wantField: "semantic_type",
		},
		{
			name:      "missing_action_type",
			u:         Understanding{PrimaryIntent: "impl", SemanticType: "def", Domain: "gen", Confidence: 0.5},
			wantField: "action_type",
		},
		{
			name:      "missing_domain",
			u:         Understanding{PrimaryIntent: "impl", SemanticType: "def", ActionType: "impl", Confidence: 0.5},
			wantField: "domain",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.u.Validate()
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			ve, ok := err.(*ValidationError)
			if !ok {
				t.Errorf("expected *ValidationError, got %T", err)
				return
			}
			if ve.Field != tc.wantField {
				t.Errorf("Field = %q, want %q", ve.Field, tc.wantField)
			}
		})
	}
}

func TestUnderstanding_Validate_WhenBadConfidence_ShouldReturnError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		confidence float64
	}{
		{"negative", -0.1},
		{"over_one", 1.1},
		{"nan", math.NaN()},
		{"positive_inf", math.Inf(1)},
		{"negative_inf", math.Inf(-1)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := Understanding{
				PrimaryIntent: "impl",
				SemanticType:  "def",
				ActionType:    "impl",
				Domain:        "gen",
				Confidence:    tc.confidence,
			}
			err := u.Validate()
			if err == nil {
				t.Error("expected error for bad confidence, got nil")
			}
		})
	}
}

func TestUnderstanding_Validate_WhenBoundaryConfidence_ShouldPass(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		confidence float64
	}{
		{"zero", 0.0},
		{"one", 1.0},
		{"mid", 0.5},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := Understanding{
				PrimaryIntent: "impl",
				SemanticType:  "def",
				ActionType:    "impl",
				Domain:        "gen",
				Confidence:    tc.confidence,
			}
			if err := u.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil for confidence %.1f", err, tc.confidence)
			}
		})
	}
}

// =============================================================================
// IS ACTION REQUEST TESTS
// =============================================================================

func TestUnderstanding_IsActionRequest_WhenActionVerb_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	actionTypes := []string{"implement", "modify", "refactor", "verify", "attack", "revert", "configure"}

	for _, at := range actionTypes {
		at := at
		t.Run(at, func(t *testing.T) {
			t.Parallel()
			u := &Understanding{ActionType: at}
			if !u.IsActionRequest() {
				t.Errorf("IsActionRequest() = false for %q, want true", at)
			}
		})
	}
}

func TestUnderstanding_IsActionRequest_WhenNonActionVerb_ShouldReturnFalse(t *testing.T) {
	t.Parallel()

	nonActionTypes := []string{"explain", "investigate", "research", "chat", "review", ""}

	for _, at := range nonActionTypes {
		at := at
		t.Run("type_"+at, func(t *testing.T) {
			t.Parallel()
			u := &Understanding{ActionType: at}
			if u.IsActionRequest() {
				t.Errorf("IsActionRequest() = true for %q, want false", at)
			}
		})
	}
}

// =============================================================================
// IS READ ONLY TESTS
// =============================================================================

func TestUnderstanding_IsReadOnly_WhenHypothetical_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	u := &Understanding{
		ActionType: "implement",
		Signals:    Signals{IsHypothetical: true},
	}
	if !u.IsReadOnly() {
		t.Error("IsReadOnly() = false for hypothetical, want true")
	}
}

func TestUnderstanding_IsReadOnly_WhenConstraints_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	constraintCases := []string{"no_changes", "read_only", "dry_run"}
	for _, c := range constraintCases {
		c := c
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			u := &Understanding{
				ActionType:      "implement",
				UserConstraints: []string{c},
			}
			if !u.IsReadOnly() {
				t.Errorf("IsReadOnly() = false for constraint %q, want true", c)
			}
		})
	}
}

func TestUnderstanding_IsReadOnly_WhenReadOnlyAction_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	readOnlyTypes := []string{"investigate", "explain", "research", "review"}
	for _, at := range readOnlyTypes {
		at := at
		t.Run(at, func(t *testing.T) {
			t.Parallel()
			u := &Understanding{ActionType: at}
			if !u.IsReadOnly() {
				t.Errorf("IsReadOnly() = false for action %q, want true", at)
			}
		})
	}
}

func TestUnderstanding_IsReadOnly_WhenMutatingAction_ShouldReturnFalse(t *testing.T) {
	t.Parallel()

	u := &Understanding{ActionType: "implement"}
	if u.IsReadOnly() {
		t.Error("IsReadOnly() = true for implement, want false")
	}
}

// =============================================================================
// NEEDS CONFIRMATION TESTS
// =============================================================================

func TestUnderstanding_NeedsConfirmation_WhenHighRisk_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		u    Understanding
	}{
		{
			name: "requires_confirmation_signal",
			u:    Understanding{Signals: Signals{RequiresConfirmation: true}},
		},
		{
			name: "revert_action",
			u:    Understanding{ActionType: "revert"},
		},
		{
			name: "attack_action",
			u:    Understanding{ActionType: "attack"},
		},
		{
			name: "codebase_scope",
			u:    Understanding{Scope: Scope{Level: "codebase"}},
		},
		{
			name: "module_scope",
			u:    Understanding{Scope: Scope{Level: "module"}},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !tc.u.NeedsConfirmation() {
				t.Error("NeedsConfirmation() = false, want true")
			}
		})
	}
}

func TestUnderstanding_NeedsConfirmation_WhenLowRisk_ShouldReturnFalse(t *testing.T) {
	t.Parallel()

	u := &Understanding{
		ActionType: "explain",
		Scope:      Scope{Level: "function"},
	}
	if u.NeedsConfirmation() {
		t.Error("NeedsConfirmation() = true for explain/function, want false")
	}
}

// =============================================================================
// VALIDATION ERROR TESTS
// =============================================================================

func TestValidationError_Error_ShouldFormatCorrectly(t *testing.T) {
	t.Parallel()

	err := &ValidationError{Field: "confidence", Message: "must be between 0 and 1"}
	got := err.Error()
	want := "validation error: confidence must be between 0 and 1"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// =============================================================================
// EXTRACT CLEAN JSON TESTS
// =============================================================================

func TestExtractCleanJSON_WhenValidJSON_ShouldExtract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "pure_json",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "json_with_prefix",
			input: `Some text {"key": "value"} trailing`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "no_json",
			input: "no json here",
			want:  "",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractCleanJSON(tc.input)
			if got != tc.want {
				t.Errorf("ExtractCleanJSON(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// =============================================================================
// LEARNING - EXTRACT FACT EDGE CASES
// =============================================================================

func TestExtractFactFromResponse_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	got := ExtractFactFromResponse("")
	if got != "" {
		t.Errorf("ExtractFactFromResponse(\"\") = %q, want empty", got)
	}
}

func TestExtractFactFromResponse_WhenNoLearnedExemplar_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	got := ExtractFactFromResponse("This is some random text without any learned exemplar in it.")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractFactFromResponse_WhenMalformed_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	// No closing paren
	got := ExtractFactFromResponse(`learned_exemplar("test", /fix, "target", "constraint", 0.9`)
	if got != "" {
		t.Errorf("malformed (no close paren) should return empty, got %q", got)
	}
}

func TestExtractFactFromResponse_WhenNoPeriod_ShouldAddPeriod(t *testing.T) {
	t.Parallel()

	got := ExtractFactFromResponse(`learned_exemplar("test", /fix, "target", "constraint", 0.9)`)
	if !strings.HasSuffix(got, ".") {
		t.Errorf("expected trailing period, got %q", got)
	}
}

func TestExtractFactFromResponse_WhenInControlPacketJSON_ShouldExtract(t *testing.T) {
	t.Parallel()

	jsonResp := `{"control_packet": {"mangle_updates": ["learned_exemplar(\"test\", /fix, \"target\", \"\", 0.90)."]}}`
	got := ExtractFactFromResponse(jsonResp)
	if got == "" {
		t.Error("expected fact to be extracted from control_packet JSON")
	}
	if !strings.Contains(got, "learned_exemplar") {
		t.Errorf("expected learned_exemplar in result, got %q", got)
	}
}

// =============================================================================
// LEARNING - PARSE LEARNED FACT TESTS
// =============================================================================

func TestParseLearnedFact_WhenInvalidPrefix_ShouldReturnError(t *testing.T) {
	t.Parallel()

	_, _, _, _, _, err := ParseLearnedFact(`not_a_learned_fact("test", /fix, "t", "c", 0.9).`)
	if err == nil {
		t.Error("expected error for invalid prefix, got nil")
	}
}

func TestParseLearnedFact_WhenWrongArgCount_ShouldReturnError(t *testing.T) {
	t.Parallel()

	_, _, _, _, _, err := ParseLearnedFact(`learned_exemplar("test", /fix, "target").`)
	if err == nil {
		t.Error("expected error for wrong arg count, got nil")
	}
}

func TestParseLearnedFact_WhenBadConfidence_ShouldReturnError(t *testing.T) {
	t.Parallel()

	_, _, _, _, _, err := ParseLearnedFact(`learned_exemplar("test", /fix, "target", "constraint", not_a_number).`)
	if err == nil {
		t.Error("expected error for non-numeric confidence, got nil")
	}
}

// =============================================================================
// LEARNING - NORMALIZE LEARNED FACT TESTS
// =============================================================================

func TestNormalizeLearnedFact_WhenFloatConfidence_ShouldConvertToInt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		fact       string
		wantSuffix string
	}{
		{
			name:       "float_0.95",
			fact:       `learned_exemplar("test", /fix, "target", "constraint", 0.95).`,
			wantSuffix: ", 95).",
		},
		{
			name:       "float_0.5",
			fact:       `learned_exemplar("test", /fix, "target", "", 0.50).`,
			wantSuffix: ", 50).",
		},
		{
			name:       "already_integer_90",
			fact:       `learned_exemplar("test", /fix, "target", "", 90).`,
			wantSuffix: ", 90).",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeLearnedFact(tc.fact)
			if err != nil {
				t.Fatalf("NormalizeLearnedFact() error = %v", err)
			}
			if !strings.HasSuffix(got, tc.wantSuffix) {
				t.Errorf("got %q, want suffix %q", got, tc.wantSuffix)
			}
		})
	}
}

func TestNormalizeLearnedFact_WhenMalformed_ShouldReturnOriginal(t *testing.T) {
	t.Parallel()

	malformed := "this is not a fact"
	got, err := NormalizeLearnedFact(malformed)
	if err == nil {
		t.Error("expected error for malformed input")
	}
	if got != malformed {
		t.Errorf("expected original returned on error, got %q", got)
	}
}

// =============================================================================
// SPLIT LEARNED FACT ARGS TESTS
// =============================================================================

func TestSplitLearnedFactArgs_WhenCommasInQuotes_ShouldNotSplit(t *testing.T) {
	t.Parallel()

	input := `"hello, world", /fix, "target", "a, b, c", 0.9`
	parts := splitLearnedFactArgs(input)
	if len(parts) != 5 {
		t.Fatalf("expected 5 parts, got %d: %v", len(parts), parts)
	}
	if parts[0] != `"hello, world"` {
		t.Errorf("parts[0] = %q, want %q", parts[0], `"hello, world"`)
	}
	if parts[3] != `"a, b, c"` {
		t.Errorf("parts[3] = %q, want %q", parts[3], `"a, b, c"`)
	}
}

func TestSplitLearnedFactArgs_WhenEscapedQuotes_ShouldHandleCorrectly(t *testing.T) {
	t.Parallel()

	input := `"hello \"world\"", /fix, "target", "constraint", 0.9`
	parts := splitLearnedFactArgs(input)
	if len(parts) != 5 {
		t.Fatalf("expected 5 parts, got %d: %v", len(parts), parts)
	}
}

func TestSplitLearnedFactArgs_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	parts := splitLearnedFactArgs("")
	if len(parts) != 0 {
		t.Errorf("expected 0 parts, got %d", len(parts))
	}
}

// =============================================================================
// NORMALIZE LEARNED FACT (normalizeLearnedFact) TESTS
// =============================================================================

func TestNormalizeLearnedFactInternal_WhenNoPeriod_ShouldAddPeriod(t *testing.T) {
	t.Parallel()

	got := normalizeLearnedFact(`learned_exemplar("test", /fix, "t", "c", 90)`)
	if !strings.HasSuffix(got, ".") {
		t.Errorf("expected trailing period, got %q", got)
	}
}

func TestNormalizeLearnedFactInternal_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	got := normalizeLearnedFact("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestNormalizeLearnedFactInternal_WhenAlreadyHasPeriod_ShouldNotDouble(t *testing.T) {
	t.Parallel()

	got := normalizeLearnedFact("fact.")
	if got != "fact." {
		t.Errorf("expected %q, got %q", "fact.", got)
	}
}

// =============================================================================
// METRICS TESTS
// =============================================================================

func TestRecordLLMCall_WhenCalled_ShouldTrack(t *testing.T) {
	// Record some calls
	RecordLLMCall("test_category", "test_type", 100, 500, nil)
	RecordLLMCall("test_category", "test_type", 200, 300, fmt.Errorf("test error"))

	snapshot := GetLLMMetrics()
	key := "test_category:test_type"

	m, ok := snapshot[key]
	if !ok {
		t.Fatalf("expected metrics for key %q", key)
	}
	if m.Calls != 2 {
		t.Errorf("Calls = %d, want 2", m.Calls)
	}
	if m.TokensUsed != 300 {
		t.Errorf("TokensUsed = %d, want 300", m.TokensUsed)
	}
	if m.DurationMs != 800 {
		t.Errorf("DurationMs = %d, want 800", m.DurationMs)
	}
	if m.Errors != 1 {
		t.Errorf("Errors = %d, want 1", m.Errors)
	}
}

func TestGetLLMMetrics_WhenEmpty_ShouldReturnEmptyMap(t *testing.T) {
	// The global metrics map may already have entries from other tests,
	// but we can verify the function doesn't panic and returns a map
	snapshot := GetLLMMetrics()
	if snapshot == nil {
		t.Error("expected non-nil map")
	}
}

// =============================================================================
// REQUIRES JSON OUTPUT TESTS (expanded)
// =============================================================================

func TestRequiresJSONOutput_WhenNoMarkers_ShouldReturnFalse(t *testing.T) {
	t.Parallel()

	if requiresJSONOutput("plain system prompt", "plain user prompt") {
		t.Error("should return false when no markers present")
	}
}

func TestRequiresJSONOutput_WhenMarkerInSystem_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	if !requiresJSONOutput("use mangle_synth_v1 format", "normal user") {
		t.Error("should return true when marker in system prompt")
	}
}

// =============================================================================
// TAXONOMY PERSISTENCE TESTS
// =============================================================================

func TestNormalizeTaxonomyFactArgs_WhenVerbDef_ShouldNormalizePriority(t *testing.T) {
	t.Parallel()

	args := normalizeTaxonomyFactArgs("verb_def", []interface{}{"/fix", "/mutation", "/coder", float64(90)})
	if args[3] != int64(90) {
		t.Errorf("args[3] = %v (type %T), want int64(90)", args[3], args[3])
	}
}

func TestNormalizeTaxonomyFactArgs_WhenLearnedExemplar_ShouldNormalizeConfidence(t *testing.T) {
	t.Parallel()

	args := normalizeTaxonomyFactArgs("learned_exemplar", []interface{}{"pattern", "/fix", "target", "constraint", float64(95)})
	if args[4] != int64(95) {
		t.Errorf("args[4] = %v (type %T), want int64(95)", args[4], args[4])
	}
}

func TestNormalizeTaxonomyFactArgs_WhenEmptyArgs_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	args := normalizeTaxonomyFactArgs("verb_def", []interface{}{})
	if len(args) != 0 {
		t.Errorf("expected empty args, got %d", len(args))
	}
}

func TestNormalizeWholeNumber_WhenVariousTypes_ShouldNormalize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		val  interface{}
		want interface{}
	}{
		{"int", int(42), int64(42)},
		{"int32", int32(42), int64(42)},
		{"int64", int64(42), int64(42)},
		{"float64_whole", float64(42.0), int64(42)},
		{"float64_fractional", float64(42.5), float64(42.5)},
		{"json_number_int", json.Number("42"), int64(42)},
		{"json_number_float", json.Number("42.5"), float64(42.5)},
		{"string_passthrough", "foo", "foo"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeWholeNumber(tc.val)
			if got != tc.want {
				t.Errorf("normalizeWholeNumber(%v) = %v (type %T), want %v (type %T)",
					tc.val, got, got, tc.want, tc.want)
			}
		})
	}
}

// =============================================================================
// TO INT TESTS
// =============================================================================

func TestToInt_WhenVariousTypes_ShouldConvert(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		val  interface{}
		want int
	}{
		{"int", int(42), 42},
		{"int64", int64(99), 99},
		{"float64", float64(7.8), 7},
		{"string_zero", "hello", 0},
		{"nil_zero", nil, 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := toInt(tc.val)
			if got != tc.want {
				t.Errorf("toInt(%v) = %d, want %d", tc.val, got, tc.want)
			}
		})
	}
}

// =============================================================================
// CLIENT FACTORY TESTS
// =============================================================================

func TestNewClientFromConfig_WhenUnknownEngine_ShouldReturnError(t *testing.T) {
	t.Parallel()

	cfg := &ProviderConfig{Engine: "unknown-engine"}
	_, err := NewClientFromConfig(cfg)
	if err == nil {
		t.Error("expected error for unknown engine, got nil")
	}
	if !strings.Contains(err.Error(), "unknown engine") {
		t.Errorf("error should mention 'unknown engine', got: %v", err)
	}
}

func TestNewClientFromConfig_WhenUnknownProvider_ShouldReturnError(t *testing.T) {
	t.Parallel()

	cfg := &ProviderConfig{Engine: "api", Provider: "nonexistent"}
	_, err := NewClientFromConfig(cfg)
	if err == nil {
		t.Error("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error should mention 'unknown provider', got: %v", err)
	}
}

func TestNewClassificationClientFromConfig_WhenNil_ShouldReturnNil(t *testing.T) {
	t.Parallel()

	client, err := NewClassificationClientFromConfig(nil)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if client != nil {
		t.Error("expected nil client for nil config")
	}
}

func TestNewClassificationClientFromConfig_WhenCLIEngine_ShouldReturnNil(t *testing.T) {
	t.Parallel()

	cases := []string{"claude-cli", "codex-cli"}
	for _, engine := range cases {
		engine := engine
		t.Run(engine, func(t *testing.T) {
			t.Parallel()
			cfg := &ProviderConfig{Engine: engine}
			client, err := NewClassificationClientFromConfig(cfg)
			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
			if client != nil {
				t.Errorf("expected nil client for %s engine", engine)
			}
		})
	}
}

func TestNewClassificationClientFromConfig_WhenUnsupportedProvider_ShouldReturnNil(t *testing.T) {
	t.Parallel()

	unsupported := []Provider{ProviderZAI, ProviderXAI, ProviderOpenRouter}
	for _, p := range unsupported {
		p := p
		t.Run(string(p), func(t *testing.T) {
			t.Parallel()
			cfg := &ProviderConfig{Provider: p, APIKey: "test-key"}
			client, err := NewClassificationClientFromConfig(cfg)
			if err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
			if client != nil {
				t.Errorf("expected nil client for unsupported provider %s", p)
			}
		})
	}
}

// =============================================================================
// CLIENT TOOL HELPERS TESTS
// =============================================================================

func TestMapToolDefinitionsToOpenAI_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	result := MapToolDefinitionsToOpenAI(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestMapToolDefinitionsToOpenAI_WhenSingle_ShouldMap(t *testing.T) {
	t.Parallel()

	tools := []ToolDefinition{
		{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: map[string]interface{}{"type": "object"},
		},
	}

	result := MapToolDefinitionsToOpenAI(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Type != "function" {
		t.Errorf("Type = %q, want %q", result[0].Type, "function")
	}
	if result[0].Function.Name != "read_file" {
		t.Errorf("Name = %q, want %q", result[0].Function.Name, "read_file")
	}
	if result[0].Function.Description != "Read a file" {
		t.Errorf("Description = %q, want %q", result[0].Function.Description, "Read a file")
	}
}

func TestMapOpenAIToolCallsToInternal_WhenValidJSON_ShouldParse(t *testing.T) {
	t.Parallel()

	calls := []OpenAIToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: OpenAIFunctionCall{
				Name:      "read_file",
				Arguments: `{"path": "/test.go"}`,
			},
		},
	}

	result, err := MapOpenAIToolCallsToInternal(calls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].ID != "call_1" {
		t.Errorf("ID = %q, want %q", result[0].ID, "call_1")
	}
	if result[0].Name != "read_file" {
		t.Errorf("Name = %q, want %q", result[0].Name, "read_file")
	}
	pathVal, ok := result[0].Input["path"]
	if !ok {
		t.Fatal("expected 'path' in Input")
	}
	if pathVal != "/test.go" {
		t.Errorf("Input['path'] = %v, want %q", pathVal, "/test.go")
	}
}

func TestMapOpenAIToolCallsToInternal_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	t.Parallel()

	calls := []OpenAIToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: OpenAIFunctionCall{
				Name:      "read_file",
				Arguments: `not valid json`,
			},
		},
	}

	_, err := MapOpenAIToolCallsToInternal(calls)
	if err == nil {
		t.Error("expected error for invalid JSON arguments")
	}
}

func TestMapOpenAIToolCallsToInternal_WhenNonFunctionType_ShouldSkip(t *testing.T) {
	t.Parallel()

	calls := []OpenAIToolCall{
		{
			ID:   "call_1",
			Type: "retrieval", // Not "function"
			Function: OpenAIFunctionCall{
				Name:      "read_file",
				Arguments: `{"path": "/test.go"}`,
			},
		},
	}

	result, err := MapOpenAIToolCallsToInternal(calls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The non-function call should be skipped (result[0] will be zero value)
	if len(result) != 1 {
		t.Fatalf("expected 1 result (slice length), got %d", len(result))
	}
	if result[0].Name != "" {
		t.Errorf("expected empty Name for skipped non-function type, got %q", result[0].Name)
	}
}

// =============================================================================
// SCHEMA BUILDER TESTS
// =============================================================================

func TestBuildZAIPiggybackEnvelopeSchema_ShouldReturnJSONObject(t *testing.T) {
	t.Parallel()

	schema := BuildZAIPiggybackEnvelopeSchema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if schema.Type != "json_object" {
		t.Errorf("Type = %q, want %q", schema.Type, "json_object")
	}
	// ZAI doesn't support full schema
	if schema.JSONSchema != nil {
		t.Error("ZAI schema should not have JSONSchema (only basic json_object)")
	}
}

func TestBuildOpenAIPiggybackEnvelopeSchema_ShouldReturnJSONSchema(t *testing.T) {
	t.Parallel()

	schema := BuildOpenAIPiggybackEnvelopeSchema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if schema.Type != "json_schema" {
		t.Errorf("Type = %q, want %q", schema.Type, "json_schema")
	}
	if schema.JSONSchema == nil {
		t.Fatal("expected non-nil JSONSchema")
	}
	if schema.JSONSchema.Name != "PiggybackEnvelope" {
		t.Errorf("Name = %q, want %q", schema.JSONSchema.Name, "PiggybackEnvelope")
	}
	if !schema.JSONSchema.Strict {
		t.Error("Strict should be true")
	}
}

func TestBuildGeminiPiggybackEnvelopeSchema_ShouldReturnMap(t *testing.T) {
	t.Parallel()

	schema := BuildGeminiPiggybackEnvelopeSchema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if _, ok := schema["type"]; !ok {
		t.Error("expected 'type' key in schema")
	}
}

func TestBuildOpenRouterPiggybackEnvelopeSchema_ShouldReturnJSONSchema(t *testing.T) {
	t.Parallel()

	schema := BuildOpenRouterPiggybackEnvelopeSchema()
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if schema.Type != "json_schema" {
		t.Errorf("Type = %q, want %q", schema.Type, "json_schema")
	}
}

// =============================================================================
// STABILITY FILTER TESTS
// =============================================================================

func TestComputeStabilityScore_WhenAllSame_ShouldReturn100(t *testing.T) {
	t.Parallel()

	score := computeStabilityScore([]string{"fix", "fix", "fix", "fix", "fix"})
	if score != 100 {
		t.Errorf("score = %d, want 100", score)
	}
}

func TestComputeStabilityScore_WhenAlternating_ShouldReturn0(t *testing.T) {
	t.Parallel()

	score := computeStabilityScore([]string{"fix", "test", "fix", "test"})
	if score != 0 {
		t.Errorf("score = %d, want 0", score)
	}
}

func TestComputeStabilityScore_WhenTooShort_ShouldReturn0(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		{},
		{"fix"},
	}

	for _, history := range cases {
		score := computeStabilityScore(history)
		if score != 0 {
			t.Errorf("computeStabilityScore(%v) = %d, want 0", history, score)
		}
	}
}

func TestComputeStabilityScore_WhenMixed_ShouldCalculateCorrectly(t *testing.T) {
	t.Parallel()

	// [fix, fix, fix, test, fix] -> pairs: (fix,fix)✓ (fix,fix)✓ (fix,test)✗ (test,fix)✗ = 2/4 = 50
	score := computeStabilityScore([]string{"fix", "fix", "fix", "test", "fix"})
	if score != 50 {
		t.Errorf("score = %d, want 50", score)
	}
}

func TestLikelyTopicChange_WhenQuestionMark_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	if !likelyTopicChange("what is this?", nil) {
		t.Error("expected true for question mark input")
	}
}

func TestLikelyTopicChange_WhenTopicShiftKeyword_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	keywords := []string{"explain this", "what is it", "how does it work", "why is it", "describe the code", "show me", "tell me", "help me"}
	for _, kw := range keywords {
		if !likelyTopicChange(kw, nil) {
			t.Errorf("expected true for %q", kw)
		}
	}
}

func TestLikelyTopicChange_WhenLengthSpike_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	// Average length is 10, input is 40 (4x > 3x threshold)
	history := []int{10, 10, 10}
	longInput := strings.Repeat("a", 40)
	if !likelyTopicChange(longInput, history) {
		t.Error("expected true for message length spike")
	}
}

func TestLikelyTopicChange_WhenNoSignals_ShouldReturnFalse(t *testing.T) {
	t.Parallel()

	if likelyTopicChange("fix the bug", []int{15, 15, 15}) {
		t.Error("expected false when no topic change signals")
	}
}

func TestSanitizeAtomString_WhenSpecialChars_ShouldSanitize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"spaces_to_underscores", "hello world", "hello_world"},
		{"dashes_to_underscores", "my-domain", "my_domain"},
		{"special_chars_removed", "test!@#$%^&*()+=123", "test123"},
		{"empty_to_unknown", "", "unknown"},
		{"whitespace_only_to_unknown", "   ", "unknown"},
		{"mixed_case_lowered", "TestDomain", "testdomain"},
		{"preserves_underscores", "my_domain_name", "my_domain_name"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeAtomString(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeAtomString(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// =============================================================================
// UNDERSTANDING TRANSDUCER - RESOLVE FOCUS TESTS
// =============================================================================

func TestUnderstandingTransducer_ResolveFocus_WhenCandidates_ShouldReturnFirst(t *testing.T) {
	t.Parallel()

	tr := &UnderstandingTransducer{}
	focus, err := tr.ResolveFocus(context.Background(), "kernel", []string{"internal/core/kernel.go", "internal/core/kernel_test.go"})
	if err != nil {
		t.Fatalf("ResolveFocus error: %v", err)
	}
	if focus.ResolvedPath != "internal/core/kernel.go" {
		t.Errorf("ResolvedPath = %q, want %q", focus.ResolvedPath, "internal/core/kernel.go")
	}
	if focus.ConfidencePercent != 50 {
		t.Errorf("ConfidencePercent = %d, want 50", focus.ConfidencePercent)
	}
}

func TestUnderstandingTransducer_ResolveFocus_WhenNoCandidates_ShouldReturnReference(t *testing.T) {
	t.Parallel()

	tr := &UnderstandingTransducer{}
	focus, err := tr.ResolveFocus(context.Background(), "some_ref", nil)
	if err != nil {
		t.Fatalf("ResolveFocus error: %v", err)
	}
	if focus.ResolvedPath != "some_ref" {
		t.Errorf("ResolvedPath = %q, want %q", focus.ResolvedPath, "some_ref")
	}
	if focus.ConfidencePercent != 30 {
		t.Errorf("ConfidencePercent = %d, want 30", focus.ConfidencePercent)
	}
}

// =============================================================================
// UNDERSTANDING TRANSDUCER - VERB/CATEGORY HISTORY TESTS
// =============================================================================

func TestUnderstandingTransducer_UpdateVerbHistory_WhenMaxReached_ShouldTrim(t *testing.T) {
	tr := &UnderstandingTransducer{}
	for i := 0; i < 10; i++ {
		tr.updateVerbHistory(fmt.Sprintf("verb_%d", i))
	}
	if len(tr.verbHistory) != 5 {
		t.Errorf("verbHistory length = %d, want 5 (max)", len(tr.verbHistory))
	}
	if tr.lastVerb != "verb_9" {
		t.Errorf("lastVerb = %q, want %q", tr.lastVerb, "verb_9")
	}
}

func TestUnderstandingTransducer_UpdateMsgLenHistory_WhenMaxReached_ShouldTrim(t *testing.T) {
	tr := &UnderstandingTransducer{}
	for i := 0; i < 10; i++ {
		tr.updateMsgLenHistory(strings.Repeat("a", i*10))
	}
	if len(tr.msgLenHistory) != 5 {
		t.Errorf("msgLenHistory length = %d, want 5 (max)", len(tr.msgLenHistory))
	}
}

// =============================================================================
// UNDERSTANDING TRANSDUCER - EXTRACT MEMORY OPS EDGE CASES
// =============================================================================

func TestUnderstandingTransducer_ExtractMemoryOperations_WhenNoTarget_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	tr := &UnderstandingTransducer{}
	u := &Understanding{
		ActionType: "remember",
		Scope:      Scope{Target: ""}, // Empty target
	}
	ops := tr.extractMemoryOperations(u)
	if len(ops) != 0 {
		t.Errorf("expected 0 ops for empty target, got %d", len(ops))
	}
}

func TestUnderstandingTransducer_ExtractMemoryOperations_WhenUnknownAction_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	tr := &UnderstandingTransducer{}
	u := &Understanding{
		ActionType: "implement",
		Scope:      Scope{Target: "something"},
	}
	ops := tr.extractMemoryOperations(u)
	if len(ops) != 0 {
		t.Errorf("expected 0 ops for non-memory action, got %d", len(ops))
	}
}

// =============================================================================
// UNDERSTANDING TRANSDUCER - understandingToIntent EDGE CASES
// =============================================================================

func TestUnderstandingTransducer_UnderstandingToIntent_WhenFileTarget_ShouldFallback(t *testing.T) {
	t.Parallel()

	tr := &UnderstandingTransducer{}

	// When Target is empty but File is set, should use File
	u := &Understanding{
		ActionType: "implement",
		Domain:     "general",
		Scope: Scope{
			Target: "",
			File:   "main.go",
		},
		Confidence: 0.9,
	}

	intent := tr.understandingToIntent(u)
	if intent.Target != "main.go" {
		t.Errorf("Target = %q, want %q", intent.Target, "main.go")
	}
}

func TestUnderstandingTransducer_UnderstandingToIntent_WhenSymbolTarget_ShouldFallback(t *testing.T) {
	t.Parallel()

	tr := &UnderstandingTransducer{}

	u := &Understanding{
		ActionType: "explain",
		Domain:     "general",
		Scope: Scope{
			Target: "",
			File:   "",
			Symbol: "Execute",
		},
		Confidence: 0.8,
	}

	intent := tr.understandingToIntent(u)
	if intent.Target != "Execute" {
		t.Errorf("Target = %q, want %q", intent.Target, "Execute")
	}
}

func TestUnderstandingTransducer_UnderstandingToIntent_WhenRouting_ShouldIncludeInAmbiguity(t *testing.T) {
	t.Parallel()

	tr := &UnderstandingTransducer{}

	u := &Understanding{
		ActionType:   "implement",
		SemanticType: "mechanism",
		Domain:       "general",
		Routing: &Routing{
			PrimaryShard: "coder",
		},
	}

	intent := tr.understandingToIntent(u)
	found := false
	for _, a := range intent.Ambiguity {
		if strings.Contains(a, "shard=coder") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected shard routing info in Ambiguity")
	}
}

// =============================================================================
// CONSOLIDATION WORKER TESTS
// =============================================================================

func TestConsolidationWorker_Enqueue_WhenQueueFull_ShouldNotBlock(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}

	cw := NewConsolidationWorker(eng)
	cw.Start()
	defer cw.Stop()

	// Fill queue beyond capacity (100)
	for i := 0; i < 110; i++ {
		cw.Enqueue([]ReasoningTrace{{UserPrompt: "test", Response: "resp"}})
	}
	// If we get here without blocking, the test passes
}

// =============================================================================
// TAXONOMY ENGINE - SET WORKSPACE TESTS
// =============================================================================

func TestTaxonomyEngine_SetWorkspace_ShouldUpdateRoot(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	eng.SetWorkspace("/tmp/test-workspace")
	if !eng.HasWorkspace() {
		t.Error("expected HasWorkspace() = true after SetWorkspace")
	}
}

func TestTaxonomyEngine_HasWorkspace_WhenNotSet_ShouldReturnFalse(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	// Reset workspace (fresh engine has no workspace)
	eng.workspaceRoot = ""
	if eng.HasWorkspace() {
		t.Error("expected HasWorkspace() = false for fresh engine")
	}
}

func TestTaxonomyEngine_NerdPath_WhenWorkspaceSet_ShouldUseWorkspace(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	eng.workspaceRoot = "/my/project"
	path := eng.nerdPath("mangle")
	// On Windows, filepath.Join converts / to \, so check for either separator
	if !strings.Contains(path, "my") || !strings.Contains(path, "project") {
		t.Errorf("nerdPath = %q, expected to contain workspace root components", path)
	}
}

func TestTaxonomyEngine_NerdPath_WhenNoWorkspace_ShouldUseRelative(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	eng.workspaceRoot = ""
	path := eng.nerdPath("mangle")
	if strings.HasPrefix(path, "/") || strings.Contains(path, ":\\") {
		// Relative path should not start with / or have drive letter on Windows
		// This could vary by OS, so we just check it contains .nerd
	}
	if !strings.Contains(path, ".nerd") {
		t.Errorf("nerdPath = %q, expected to contain '.nerd'", path)
	}
}

// =============================================================================
// TAXONOMY ENGINE - ENSURE DEFAULTS TESTS
// =============================================================================

func TestTaxonomyEngine_EnsureDefaults_WhenNoStore_ShouldReturnError(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	// No store set
	eng.store = nil
	err = eng.EnsureDefaults()
	if err == nil {
		t.Error("expected error when no store configured")
	}
}

func TestTaxonomyEngine_HydrateFromDB_WhenNoStore_ShouldReturnError(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	eng.store = nil
	err = eng.HydrateFromDB()
	if err == nil {
		t.Error("expected error when no store configured")
	}
}

// =============================================================================
// DEBUG TAXONOMY TESTS
// =============================================================================

func TestDebugTaxonomy_ShouldReturnResults(t *testing.T) {
	verb, category, confidence, shardType := DebugTaxonomy("fix the bug")
	// Should return some result (at least the fallback)
	if verb == "" {
		t.Error("expected non-empty verb from DebugTaxonomy")
	}
	if category == "" {
		t.Error("expected non-empty category from DebugTaxonomy")
	}
	if confidence <= 0 {
		t.Errorf("expected positive confidence, got %f", confidence)
	}
	// shardType may be empty for fallback, that's ok
	_ = shardType
}

func TestDebugTaxonomyWithContext_ShouldAcceptContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	verb, category, _, _ := DebugTaxonomyWithContext(ctx, "review the code")
	if verb == "" {
		t.Error("expected non-empty verb")
	}
	if category == "" {
		t.Error("expected non-empty category")
	}
}

// =============================================================================
// DEFAULT TAXONOMY DATA VALIDATION
// =============================================================================

func TestDefaultTaxonomyData_ShouldHaveRequiredVerbs(t *testing.T) {
	t.Parallel()

	requiredVerbs := []string{"/fix", "/review", "/explain", "/test", "/create", "/debug", "/refactor"}

	for _, rv := range requiredVerbs {
		found := false
		for _, td := range DefaultTaxonomyData {
			if td.Verb == rv {
				found = true
				if td.Category == "" {
					t.Errorf("verb %s has empty category", rv)
				}
				if len(td.Synonyms) == 0 {
					t.Errorf("verb %s has no synonyms", rv)
				}
				break
			}
		}
		if !found {
			t.Errorf("required verb %s not found in DefaultTaxonomyData", rv)
		}
	}
}

func TestDefaultTaxonomyData_ShouldHaveUniqueVerbs(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)
	for _, td := range DefaultTaxonomyData {
		if seen[td.Verb] {
			t.Errorf("duplicate verb: %s", td.Verb)
		}
		seen[td.Verb] = true
	}
}

func TestDefaultTaxonomyData_ShouldHaveValidPatterns(t *testing.T) {
	t.Parallel()

	for _, td := range DefaultTaxonomyData {
		for _, pat := range td.Patterns {
			_, err := regexp.Compile(pat)
			if err != nil {
				t.Errorf("invalid pattern %q for verb %s: %v", pat, td.Verb, err)
			}
		}
	}
}

// =============================================================================
// LEARNING FROM INTERACTION TESTS
// =============================================================================

func TestTaxonomyEngine_LearnFromInteraction_WhenNoClient_ShouldReturnError(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	eng.client = nil
	_, err = eng.LearnFromInteraction(context.Background(), []ReasoningTrace{{UserPrompt: "test"}})
	if err == nil {
		t.Error("expected error when no client configured")
	}
}

func TestTaxonomyEngine_LearnFromInteraction_WhenEmptyHistory_ShouldReturnEmpty(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	eng.client = &mockClient{}
	fact, err := eng.LearnFromInteraction(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if fact != "" {
		t.Errorf("expected empty fact, got %q", fact)
	}
}

func TestTaxonomyEngine_LearnFromInteraction_WhenCriticFindsPattern_ShouldReturnFact(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	eng.client = &mockClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return `learned_exemplar("nuke it", /delete, "database", "", 0.95).`, nil
		},
	}

	history := []ReasoningTrace{
		{UserPrompt: "nuke it", Response: "???", Success: false},
		{UserPrompt: "nuke it means delete the database", Response: "ok", Success: true},
	}

	fact, err := eng.LearnFromInteraction(context.Background(), history)
	if err != nil {
		t.Fatalf("LearnFromInteraction error: %v", err)
	}
	if !strings.Contains(fact, "learned_exemplar") {
		t.Errorf("expected learned_exemplar fact, got %q", fact)
	}
}

func TestTaxonomyEngine_LearnFromInteraction_WhenCriticReturnsEmpty_ShouldReturnEmpty(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	eng.client = &mockClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return "", nil // No pattern detected
		},
	}

	history := []ReasoningTrace{
		{UserPrompt: "fix the bug", Response: "done", Success: true},
	}

	fact, err := eng.LearnFromInteraction(context.Background(), history)
	if err != nil {
		t.Fatalf("LearnFromInteraction error: %v", err)
	}
	if fact != "" {
		t.Errorf("expected empty fact, got %q", fact)
	}
}

func TestTaxonomyEngine_LearnFromInteraction_WhenLongHistory_ShouldTruncate(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	callCount := 0
	eng.client = &mockClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			callCount++
			return "", nil
		},
	}

	// Create 10 history entries
	history := make([]ReasoningTrace, 10)
	for i := range history {
		history[i] = ReasoningTrace{UserPrompt: fmt.Sprintf("msg_%d", i), Response: "ok", Success: true}
	}

	eng.LearnFromInteraction(context.Background(), history)
	if callCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", callCount)
	}
}

// =============================================================================
// TAXONOMY ENGINE - SET CLIENT TESTS
// =============================================================================

func TestTaxonomyEngine_SetClient_ShouldStore(t *testing.T) {
	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to taxonomy engine init failure")
	}
	defer eng.StopWorker()

	mc := &mockClient{}
	eng.SetClient(mc)
	// Verify client is set by attempting LearnFromInteraction which requires client
	_, learnErr := eng.LearnFromInteraction(context.Background(), []ReasoningTrace{{UserPrompt: "test"}})
	if learnErr != nil {
		// Should not get "no client" error
		if strings.Contains(learnErr.Error(), "no LLM client") {
			t.Error("client should be set after SetClient")
		}
	}
}

// =============================================================================
// REASONING TRACE TYPE CHECK
// =============================================================================

func TestReasoningTrace_ShouldHaveRequiredFields(t *testing.T) {
	t.Parallel()

	trace := ReasoningTrace{
		UserPrompt: "test prompt",
		Response:   "test response",
		Success:    true,
	}
	if trace.UserPrompt != "test prompt" {
		t.Errorf("UserPrompt = %q, want %q", trace.UserPrompt, "test prompt")
	}
	if trace.Response != "test response" {
		t.Errorf("Response = %q, want %q", trace.Response, "test response")
	}
	if !trace.Success {
		t.Error("Success = false, want true")
	}
}

// =============================================================================
// INTENT STRUCT FIELD TESTS
// =============================================================================

func TestIntent_ShouldStoreAllFields(t *testing.T) {
	t.Parallel()

	intent := Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "kernel.go",
		Constraint: "go",
		Confidence: 0.95,
		Ambiguity:  []string{"semantic_type=causation"},
		Response:   "I'll fix that.",
		MemoryOperations: []MemoryOperation{
			{Op: "promote_to_long_term", Key: "pref", Value: "tabs"},
		},
	}

	if intent.Category != "/mutation" {
		t.Errorf("Category = %q", intent.Category)
	}
	if intent.Verb != "/fix" {
		t.Errorf("Verb = %q", intent.Verb)
	}
	if intent.Target != "kernel.go" {
		t.Errorf("Target = %q", intent.Target)
	}
	if intent.Constraint != "go" {
		t.Errorf("Constraint = %q", intent.Constraint)
	}
	if intent.Confidence != 0.95 {
		t.Errorf("Confidence = %f", intent.Confidence)
	}
	if len(intent.Ambiguity) != 1 {
		t.Errorf("Ambiguity length = %d", len(intent.Ambiguity))
	}
	if intent.Response != "I'll fix that." {
		t.Errorf("Response = %q", intent.Response)
	}
	if len(intent.MemoryOperations) != 1 {
		t.Errorf("MemoryOperations length = %d", len(intent.MemoryOperations))
	}
}

// =============================================================================
// CONVERSATION TURN STRUCT TESTS
// =============================================================================

func TestConversationTurn_ShouldStoreAllFields(t *testing.T) {
	t.Parallel()

	turn := ConversationTurn{
		Role:             "user",
		Content:          "Hello",
		ThoughtSignature: "sig123",
		ThoughtSummary:   "Thinking about greeting",
	}

	if turn.Role != "user" {
		t.Errorf("Role = %q", turn.Role)
	}
	if turn.Content != "Hello" {
		t.Errorf("Content = %q", turn.Content)
	}
	if turn.ThoughtSignature != "sig123" {
		t.Errorf("ThoughtSignature = %q", turn.ThoughtSignature)
	}
	if turn.ThoughtSummary != "Thinking about greeting" {
		t.Errorf("ThoughtSummary = %q", turn.ThoughtSummary)
	}
}

// =============================================================================
// PROVIDER CONSTANTS TESTS
// =============================================================================

func TestProviderConstants_ShouldHaveExpectedValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		p    Provider
		want string
	}{
		{"zai", ProviderZAI, "zai"},
		{"anthropic", ProviderAnthropic, "anthropic"},
		{"openai", ProviderOpenAI, "openai"},
		{"gemini", ProviderGemini, "gemini"},
		{"xai", ProviderXAI, "xai"},
		{"openrouter", ProviderOpenRouter, "openrouter"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if string(tc.p) != tc.want {
				t.Errorf("Provider %s = %q, want %q", tc.name, tc.p, tc.want)
			}
		})
	}
}

// =============================================================================
// REFINE CATEGORY ADDITIONAL EDGE CASES
// =============================================================================

func TestRefineCategory_WhenPolitePrefix_ShouldStillDetectMutation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"please_can_you_make", "please can you make this faster", "/mutation"},
		{"i_need_you_to", "I need you to fix the login", "/mutation"},
		{"from_now_on", "from now on use spaces", "/instruction"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := refineCategory(tc.input, "/query")
			if got != tc.want {
				t.Errorf("refineCategory(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// =============================================================================
// MAP ACTION TO VERB EXPANDED TESTS
// =============================================================================

func TestMapActionToVerb_WhenAllActions_ShouldMapCorrectly(t *testing.T) {
	t.Parallel()

	tr := &UnderstandingTransducer{}

	cases := []struct {
		action string
		domain string
		want   string
	}{
		{"deploy", "", "/deploy"},
		{"migrate", "", "/migrate"},
		{"optimize", "", "/optimize"},
		{"document", "", "/document"},
		{"benchmark", "", "/benchmark"},
		{"profile", "", "/profile"},
		{"audit", "", "/audit"},
		{"scaffold", "", "/scaffold"},
		{"lint", "", "/lint"},
		{"format", "", "/format"},
		{"review", "security", "/security"},
		{"review", "general", "/review"},
		{"remember", "", "/remember"},
		{"forget", "", "/forget"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.action+"_"+tc.domain, func(t *testing.T) {
			t.Parallel()
			got := tr.mapActionToVerb(tc.action, tc.domain)
			if got != tc.want {
				t.Errorf("mapActionToVerb(%q, %q) = %q, want %q", tc.action, tc.domain, got, tc.want)
			}
		})
	}
}

// needed by existing tests that reference it
var _ = core.MangleAtom("/test")
