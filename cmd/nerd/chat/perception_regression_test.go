package chat

import (
	"codenerd/internal/perception"
	"testing"
)

// TestIsConversationalIntent_WhenGreeting_ShouldReturnTrue is a regression test
// for the bug where saying "hi" triggered a clarification request instead of a
// friendly greeting. The root cause was that /converse (mapped from action_type
// "chat") was missing from the alwaysConversational map in isConversationalIntent.
func TestIsConversationalIntent_WhenGreeting_ShouldReturnTrue(t *testing.T) {
	tests := []struct {
		name   string
		intent perception.Intent
		want   bool
	}{
		// === Conversational intents: these MUST return true ===
		{
			name:   "greet verb with empty target",
			intent: perception.Intent{Verb: "/greet", Target: ""},
			want:   true,
		},
		{
			name:   "converse verb with empty target (regression: was missing from alwaysConversational)",
			intent: perception.Intent{Verb: "/converse", Target: ""},
			want:   true,
		},
		{
			name:   "converse verb with target none",
			intent: perception.Intent{Verb: "/converse", Target: "none"},
			want:   true,
		},
		{
			name:   "help verb with empty target",
			intent: perception.Intent{Verb: "/help", Target: ""},
			want:   true,
		},
		{
			name:   "knowledge verb with empty target",
			intent: perception.Intent{Verb: "/knowledge", Target: ""},
			want:   true,
		},
		{
			name:   "configure verb with empty target",
			intent: perception.Intent{Verb: "/configure", Target: ""},
			want:   true,
		},
		{
			name:   "dream verb with empty target",
			intent: perception.Intent{Verb: "/dream", Target: ""},
			want:   true,
		},
		{
			name:   "shadow verb with empty target",
			intent: perception.Intent{Verb: "/shadow", Target: ""},
			want:   true,
		},
		// === Actionable intents: these MUST return false ===
		{
			name:   "fix verb with file target is actionable",
			intent: perception.Intent{Verb: "/fix", Target: "auth.go"},
			want:   false,
		},
		{
			name:   "review verb with codebase target is actionable",
			intent: perception.Intent{Verb: "/review", Target: "codebase"},
			want:   false,
		},
		{
			name:   "create verb with feature target is actionable",
			intent: perception.Intent{Verb: "/create", Target: "new_feature"},
			want:   false,
		},
		{
			name:   "explain verb goes through articulation intentionally",
			intent: perception.Intent{Verb: "/explain", Target: "auth"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConversationalIntent(tt.intent)
			if got != tt.want {
				t.Errorf("isConversationalIntent(%+v) = %v, want %v", tt.intent, got, tt.want)
			}
		})
	}
}

// TestShouldClarifyIntent_WhenConversational_ShouldReturnFalse is a regression
// test ensuring conversational intents NEVER trigger clarification, even with
// low confidence, empty targets, or ambiguity entries. This directly validates
// the fix for the "hi" → clarification bug.
func TestShouldClarifyIntent_WhenConversational_ShouldReturnFalse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		intent perception.Intent
		want  bool
	}{
		{
			name:  "converse with low confidence should not clarify",
			input: "hi",
			intent: perception.Intent{
				Verb:       "/converse",
				Target:     "none",
				Confidence: 0.3,
			},
			want: false,
		},
		{
			name:  "greet with very low confidence should not clarify",
			input: "hello",
			intent: perception.Intent{
				Verb:       "/greet",
				Target:     "",
				Confidence: 0.1,
			},
			want: false,
		},
		{
			name:  "help with low confidence should not clarify",
			input: "help",
			intent: perception.Intent{
				Verb:       "/help",
				Target:     "none",
				Confidence: 0.2,
			},
			want: false,
		},
		{
			name:  "actionable verb with high confidence and target should not clarify",
			input: "fix the auth bug",
			intent: perception.Intent{
				Verb:       "/fix",
				Target:     "auth.go",
				Confidence: 0.9,
			},
			want: false,
		},
		{
			name:  "actionable verb with no target and low confidence should clarify",
			input: "fix something",
			intent: perception.Intent{
				Verb:       "/fix",
				Target:     "none",
				Confidence: 0.3,
			},
			want: true,
		},
	}

	// Create a minimal Model value for calling the method.
	m := Model{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := tt.intent // local copy for pointer
			got := m.shouldClarifyIntent(&intent, tt.input)
			if got != tt.want {
				t.Errorf("shouldClarifyIntent(intent=%+v, input=%q) = %v, want %v",
					tt.intent, tt.input, got, tt.want)
			}
		})
	}
}
