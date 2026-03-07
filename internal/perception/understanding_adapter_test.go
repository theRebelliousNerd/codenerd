package perception

import (
	"context"
	"testing"
)

// mockLLMClientUT implements LLMClient for understanding adapter tests.
type mockLLMClientUT struct {
	completeFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *mockLLMClientUT) Complete(ctx context.Context, prompt string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "", nil
}
func (m *mockLLMClientUT) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, user)
	}
	return "", nil
}
func (m *mockLLMClientUT) CompleteWithTools(ctx context.Context, sys, user string, tools []ToolDefinition) (*LLMToolResponse, error) {
	return &LLMToolResponse{Text: "", StopReason: "end_turn"}, nil
}

func TestUnderstandingTransducer_ParseIntent_HappyPath(t *testing.T) {
	mockClient := &mockLLMClientUT{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			// Return a JSON representation of an UnderstandingEnvelope
			return `{
				"understanding": {
					"primary_intent": "implement",
					"semantic_type": "",
					"action_type": "implement",
					"domain": "general",
					"scope": {
						"level": "function",
						"target": "myNewFeature"
					},
					"user_constraints": ["keep it simple"],
					"implicit_assumptions": [],
					"confidence": 0.9,
					"signals": {
						"is_question": false,
						"is_hypothetical": false,
						"is_multi_step": false,
						"is_negated": false,
						"requires_confirmation": false,
						"urgency": "normal"
					},
					"suggested_approach": {
						"mode": "normal",
						"primary_shard": "coder",
						"supporting_shards": [],
						"tools_needed": [],
						"context_needed": []
					}
				},
				"surface_response": "I will implement myNewFeature."
			}`, nil
		},
	}

	tr := NewUnderstandingTransducer(mockClient)

	// Call ParseIntentWithContext
	intent, err := tr.ParseIntentWithContext(context.Background(), "implement a new feature", nil)
	if err != nil {
		t.Fatalf("ParseIntentWithContext failed: %v", err)
	}

	// Verify the parsed intent
	if intent.Verb != "/create" {
		t.Errorf("Expected Verb /create, got %s", intent.Verb)
	}
	if intent.Category != "/mutation" {
		t.Errorf("Expected Category /mutation, got %s", intent.Category)
	}
	if intent.Target != "myNewFeature" {
		t.Errorf("Expected Target 'myNewFeature', got %s", intent.Target)
	}
	if intent.Constraint != "keep it simple" {
		t.Errorf("Expected Constraint 'keep it simple', got %s", intent.Constraint)
	}
	if intent.Confidence != 0.9 {
		t.Errorf("Expected Confidence 0.9, got %v", intent.Confidence)
	}
}

func TestUnderstandingTransducer_MapActionToVerb(t *testing.T) {
	tr := &UnderstandingTransducer{}

	tests := []struct {
		action string
		domain string
		want   string
	}{
		{"investigate", "testing", "/debug"},
		{"investigate", "general", "/analyze"},
		{"implement", "", "/create"},
		{"modify", "", "/fix"},
		{"refactor", "", "/refactor"},
		{"verify", "", "/test"},

		{"attack", "", "/assault"},
		{"chat", "", "/converse"},
		{"unknown", "", "/explain"},

		// Case-insensitive matching tests
		{"Investigate", "Testing", "/debug"},
		{"MODIFY", "general", "/fix"},
		{"reFactor", "", "/refactor"},
		{" implement ", "", "/create"},
	}

	for _, tt := range tests {
		got := tr.mapActionToVerb(tt.action, tt.domain)
		if got != tt.want {
			t.Errorf("mapActionToVerb(%q, %q) = %q, want %q", tt.action, tt.domain, got, tt.want)
		}
	}
}

func TestIsValidUnderstandingPromptContract(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   bool
	}{
		{
			name: "valid understanding envelope contract",
			prompt: `Output JSON like:
{
  "understanding": {
    "primary_intent": "debug",
    "semantic_type": "causation",
    "action_type": "investigate",
    "domain": "testing",
    "suggested_approach": {}
  },
  "surface_response": "I will investigate."
}`,
			want: true,
		},
		{
			name:   "piggyback contract is invalid for perception",
			prompt: `{"control_packet": {}, "surface_response": "hi"}`,
			want:   false,
		},
		{
			name:   "legacy flat schema is invalid for perception",
			prompt: `{"category":"query","verb":"review","target":"file.go","constraint":"","confidence":0.8}`,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidUnderstandingPromptContract(tt.prompt); got != tt.want {
				t.Fatalf("isValidUnderstandingPromptContract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnderstandingTransducer_MapSemanticToCategory(t *testing.T) {
	tr := &UnderstandingTransducer{}

	// TODO: TEST_GAP: Add test cases for empty strings and unknown types to verify fallback logic.

	tests := []struct {
		semantic string
		action   string
		want     string
	}{
		{"instruction", "modify", "/instruction"},
		{"definition", "explain", "/query"},
		{"", "implement", "/mutation"},
	}

	for _, tt := range tests {
		got := tr.mapSemanticToCategory(tt.semantic, tt.action)
		if got != tt.want {
			t.Errorf("mapSemanticToCategory(%q, %q) = %q, want %q", tt.semantic, tt.action, got, tt.want)
		}
	}
}

func TestUnderstandingTransducer_ExtractMemoryOperations(t *testing.T) {
	tr := &UnderstandingTransducer{}

	u := &Understanding{
		ActionType: "remember",
		Scope: Scope{
			Target: "prefer-tabs",
		},
	}
	ops := tr.extractMemoryOperations(u)
	if len(ops) != 1 {
		t.Fatalf("Expected 1 memory op, got %d", len(ops))
	}
	if ops[0].Op != "promote_to_long_term" {
		t.Errorf("Expected op promote_to_long_term, got %s", ops[0].Op)
	}
	if ops[0].Value != "prefer-tabs" {
		t.Errorf("Expected value prefer-tabs, got %s", ops[0].Value)
	}

	uForget := &Understanding{
		ActionType: "forget",
		Scope: Scope{
			Target: "prefer-tabs",
		},
	}
	opsForget := tr.extractMemoryOperations(uForget)
	if len(opsForget) != 1 {
		t.Fatalf("Expected 1 memory op for forget, got %d", len(opsForget))
	}
	if opsForget[0].Op != "forget" {
		t.Errorf("Expected op forget, got %s", opsForget[0].Op)
	}
}

func TestUnderstandingTransducer_UnderstandingToIntent_Nil(t *testing.T) {
	// Setup
	tr := &UnderstandingTransducer{}

	// Verify we don't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("understandingToIntent panicked with %v", r)
		}
	}()

	// Execute
	intent := tr.understandingToIntent(nil)

	// Verify safe default
	if intent.Verb != "/explain" {
		t.Errorf("Expected Verb /explain, got %s", intent.Verb)
	}
	if intent.Category != "/query" {
		t.Errorf("Expected Category /query, got %s", intent.Category)
	}
	if intent.Response != "Internal error: understanding is nil" {
		t.Errorf("Expected Response 'Internal error: understanding is nil', got %s", intent.Response)
	}
}

// TODO: TEST_GAP: Add concurrency test (TestUnderstandingTransducer_Concurrency) to detect data race on t.lastUnderstanding.
// Run with -race to confirm.
