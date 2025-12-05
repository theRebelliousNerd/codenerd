package main

import (
	"strings"
	"testing"
)

func TestExtractTarget(t *testing.T) {
	scenario := "please delete src/auth/handler.go soon"
	got := extractTarget(scenario, "delete")
	if got != "src/auth/handler.go soon" {
		t.Fatalf("unexpected target: %s", got)
	}
}

func TestExtractClarificationQuestion(t *testing.T) {
	errMsg := "clarification_needed: Which service should be restarted?"
	got := extractClarificationQuestion(errMsg)
	if !strings.Contains(got, "Which service") {
		t.Fatalf("expected question to include prompt, got: %s", got)
	}
}

func TestFormatClarificationRequestIncludesOptions(t *testing.T) {
	m := chatModel{selectedOption: 1}
	state := ClarificationState{
		Question: "Pick one?",
		Options:  []string{"A", "B", "C"},
	}
	out := m.formatClarificationRequest(state)
	if !strings.Contains(out, "B") || !strings.Contains(out, "→") {
		t.Fatalf("expected selected option to be highlighted, got: %s", out)
	}
}
