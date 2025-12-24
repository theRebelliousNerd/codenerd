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
