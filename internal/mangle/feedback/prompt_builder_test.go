package feedback

import (
	"testing"
)

func TestNewPromptBuilder(t *testing.T) {
	pb := NewPromptBuilder()
	if pb == nil {
		t.Fatal("NewPromptBuilder returned nil")
	}
	if pb.MangleSyntaxReminder != defaultSyntaxReminder {
		t.Errorf("Expected MangleSyntaxReminder to be %q, got %q", defaultSyntaxReminder, pb.MangleSyntaxReminder)
	}
}

func TestCleanRuleCandidate(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		expected  string
	}{
		{
			name:      "Simple whitespace trim",
			candidate: "  test(/rule).  ",
			expected:  "test(/rule).",
		},
		{
			name:      "Remove backticks",
			candidate: "`rule(/test).`",
			expected:  "rule(/test).",
		},
		{
			name:      "Remove markdown language prefixes",
			candidate: "mangle\nrule(/test).",
			expected:  "rule(/test).",
		},
		{
			name:      "Convert quoted atoms",
			candidate: `state("active") :- process("running").`,
			expected:  `state(/active) :- process(/running).`,
		},
		{
			name:      "Remove bullet points",
			candidate: "- rule(/one).\n* rule(/two).",
			expected:  "rule(/one).\nrule(/two).",
		},
		{
			name:      "RULE prefix",
			candidate: "RULE: my_rule(/test).",
			expected:  "my_rule(/test).",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanRuleCandidate(tt.candidate)
			if result != tt.expected {
				t.Errorf("cleanRuleCandidate(%q) = %q, want %q", tt.candidate, result, tt.expected)
			}
		})
	}
}
