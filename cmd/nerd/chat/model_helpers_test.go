package chat

import (
	"strings"
	"testing"
)

func TestExtractFindings_ParsesSeverities(t *testing.T) {
	input := strings.Join([]string{
		"- [CRITICAL] kernel panic",
		"- [ERROR] nil pointer",
		"- [WARN] slow call",
		"- [INFO] note",
		"unrelated line",
	}, "\n")

	findings := extractFindings(input)
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(findings))
	}

	severityByRaw := make(map[string]string)
	for _, finding := range findings {
		raw, _ := finding["raw"].(string)
		if severity, ok := finding["severity"].(string); ok {
			severityByRaw[raw] = severity
		}
	}

	assertSeverity := func(raw, want string) {
		if got := severityByRaw[raw]; got != want {
			t.Errorf("severity for %q = %q, want %q", raw, got, want)
		}
	}

	assertSeverity("- [CRITICAL] kernel panic", "critical")
	assertSeverity("- [ERROR] nil pointer", "error")
	assertSeverity("- [WARN] slow call", "warning")
	assertSeverity("- [INFO] note", "info")
}

func TestExtractMetrics_ParsesKeyValuePairs(t *testing.T) {
	input := strings.Join([]string{
		"Total lines: 120",
		"functions = 3",
		"complexity=high",
		"nesting: 4",
		"other: 5",
	}, "\n")

	metrics := extractMetrics(input)
	want := map[string]string{
		"Total lines": "120",
		"functions":   "3",
		"complexity":  "high",
		"nesting":     "4",
	}

	if len(metrics) != len(want) {
		t.Fatalf("expected %d metrics, got %d", len(want), len(metrics))
	}

	for key, value := range want {
		got, ok := metrics[key]
		if !ok {
			t.Errorf("missing metric %q", key)
			continue
		}
		if got != value {
			t.Errorf("metric %q = %v, want %q", key, got, value)
		}
	}
}

func TestHardWrap_SplitsLines(t *testing.T) {
	got := hardWrap("abcdef\nghij", 4)
	want := "abcd\nef\nghij"
	if got != want {
		t.Fatalf("hardWrap output = %q, want %q", got, want)
	}
}

func TestHardWrap_ZeroWidthNoop(t *testing.T) {
	input := "abc"
	if got := hardWrap(input, 0); got != input {
		t.Fatalf("hardWrap width 0 = %q, want %q", got, input)
	}
}

func TestExtractClarificationQuestion(t *testing.T) {
	msg := "error: USER_INPUT_REQUIRED: provide input"
	if got := extractClarificationQuestion(msg); got != "provide input" {
		t.Fatalf("extractClarificationQuestion = %q, want %q", got, "provide input")
	}

	plain := "plain error"
	if got := extractClarificationQuestion(plain); got != plain {
		t.Fatalf("extractClarificationQuestion fallback = %q, want %q", got, plain)
	}
}

func TestFeedbackDetectors(t *testing.T) {
	if !isNegativeFeedback("Bad bot, wrong answer") {
		t.Fatal("expected negative feedback to be detected")
	}
	if isNegativeFeedback("looks good") {
		t.Fatal("did not expect negative feedback")
	}

	if !isDreamConfirmation("Yes, do that") {
		t.Fatal("expected dream confirmation to be detected")
	}
	if !isDreamCorrection("No, actually we should change") {
		t.Fatal("expected dream correction to be detected")
	}
	if !isDreamExecutionTrigger("Let's do it") {
		t.Fatal("expected dream execution trigger to be detected")
	}
}

// TestFeedbackDetectors_FalsePositives ensures heuristic functions
// do NOT trigger on natural conversational input that happens to
// contain trigger substrings (the bug that word-boundary matching fixes).
func TestFeedbackDetectors_FalsePositives(t *testing.T) {
	// "fail" as substring within a word — should NOT trigger
	if isNegativeFeedback("The failsafe mechanism caught the error") {
		t.Error("isNegativeFeedback false-positive: 'failsafe'")
	}
	// "stop" as substring within a word — should NOT trigger
	if isNegativeFeedback("I need a stopwatch function") {
		t.Error("isNegativeFeedback false-positive: 'stopwatch'")
	}

	// "correct" embedded as substring in "incorrect" — should NOT trigger
	if isDreamConfirmation("this is incorrect behavior") {
		t.Error("isDreamConfirmation false-positive: 'incorrect' containing 'correct'")
	}

	// "yes" as substring — should NOT trigger (yesterday)
	if isAffirmativeResponse("yesterday I deployed the fix") {
		t.Error("isAffirmativeResponse false-positive: 'yesterday'")
	}

	// "no" as substring — should NOT trigger (another, innovation)
	if isNegativeResponse("another approach would be better") {
		t.Error("isNegativeResponse false-positive: 'another'")
	}
	if isNegativeResponse("innovation is key") {
		t.Error("isNegativeResponse false-positive: 'innovation'")
	}

	// Exact short responses SHOULD work
	if !isAffirmativeResponse("yes") {
		t.Error("isAffirmativeResponse should match exact 'yes'")
	}
	if !isNegativeResponse("no") {
		t.Error("isNegativeResponse should match exact 'no'")
	}
	if !isNegativeResponse("nope") {
		t.Error("isNegativeResponse should match exact 'nope'")
	}
}

func TestChunkTextRunes_SemanticBoundaries(t *testing.T) {
	// Words should not be split
	input := "hello world this is a test of semantic chunking"
	chunks := chunkTextRunes(input, 15)
	for i, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk)
		// No chunk should start or end mid-word (no partial words)
		if trimmed != "" && trimmed[len(trimmed)-1] == '-' {
			t.Errorf("chunk %d ends with hyphen (word split): %q", i, chunk)
		}
	}
	// All words from original should appear in chunks
	inputWords := strings.Fields(input)
	allChunkText := strings.Join(chunks, " ")
	for _, word := range inputWords {
		if !strings.Contains(allChunkText, word) {
			t.Errorf("word %q lost during chunking", word)
		}
	}

	// Paragraph boundaries should be preferred
	input2 := "paragraph one content\n\nparagraph two content\n\nparagraph three"
	chunks2 := chunkTextRunes(input2, 30)
	if len(chunks2) < 2 {
		t.Fatalf("expected at least 2 chunks for paragraph split, got %d", len(chunks2))
	}
	// First chunk should contain first paragraph and end at paragraph boundary
	if !strings.Contains(chunks2[0], "paragraph one") {
		t.Errorf("first chunk should contain 'paragraph one': %q", chunks2[0])
	}
}

func TestExtractCorrectionContent(t *testing.T) {
	if got := extractCorrectionContent("No, actually use tests"); got != "use tests" {
		t.Fatalf("extractCorrectionContent = %q, want %q", got, "use tests")
	}
	if got := extractCorrectionContent("learn: prefer clarity"); got != "prefer clarity" {
		t.Fatalf("extractCorrectionContent = %q, want %q", got, "prefer clarity")
	}
	if got := extractCorrectionContent("no marker here"); got != "no marker here" {
		t.Fatalf("extractCorrectionContent fallback = %q, want %q", got, "no marker here")
	}
}

// =============================================================================
// SANITIZATION TESTS (Pre-Chaos Hardening Phase 3.2)
// =============================================================================

func TestSanitizeCommandInput_NullBytes(t *testing.T) {
	// Null bytes should be stripped
	input := "hello\x00world"
	result := sanitizeCommandInput(input)
	if strings.Contains(result, "\x00") {
		t.Error("null bytes should be stripped")
	}
	if result != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", result)
	}
}

func TestSanitizeCommandInput_ANSIEscape(t *testing.T) {
	input := "hello\x1b[31mred\x1b[0mworld"
	result := sanitizeCommandInput(input)
	if strings.Contains(result, "\x1b") {
		t.Error("ANSI escape sequences should be stripped")
	}
}

func TestSanitizeCommandInput_PreservesNewlines(t *testing.T) {
	input := "line1\nline2\ttab\rcarriage"
	result := sanitizeCommandInput(input)
	if !strings.Contains(result, "\n") || !strings.Contains(result, "\t") || !strings.Contains(result, "\r") {
		t.Error("newlines, tabs, and carriage returns should be preserved")
	}
}

func TestSanitizeCommandInput_ControlChars(t *testing.T) {
	input := "hello\x01\x02\x03\x04\x05world"
	result := sanitizeCommandInput(input)
	if result != "helloworld" {
		t.Errorf("control chars should be stripped, got %q", result)
	}
}

func TestSanitizeCommandInput_LengthCap(t *testing.T) {
	// Build a string longer than 10K
	input := strings.Repeat("A", 15000)
	result := sanitizeCommandInput(input)
	if len(result) > 10000 {
		t.Errorf("expected result capped at 10000, got %d", len(result))
	}
}

func TestSanitizeCommandInput_EmptyInput(t *testing.T) {
	result := sanitizeCommandInput("")
	if result != "" {
		t.Errorf("empty input should produce empty output, got %q", result)
	}
}

func TestSanitizeCommandInput_NormalInput(t *testing.T) {
	input := "/review internal/core/kernel.go"
	result := sanitizeCommandInput(input)
	if result != input {
		t.Errorf("normal input should pass through unchanged, got %q", result)
	}
}
