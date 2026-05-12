package perception

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

	// TODO: TEST_GAP_EXTREME_01: Add test for extremely large input string (e.g., 50MB) to ensure truncation logic is active and memory is not exhausted before hitting LLM.
	// TODO: TEST_GAP_NULL_03: Add test asserting identical prompt building behavior for `nil` history vs `[]ConversationTurn{}`.

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

func TestUnderstandingTransducer_ParseIntent_EmptyString(t *testing.T) {
	mockClient := &mockLLMClientUT{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			t.Fatal("LLM client should not be called for empty string input")
			return "", nil
		},
	}

	tr := NewUnderstandingTransducer(mockClient)

	// Call ParseIntentWithContext with empty string
	intent, err := tr.ParseIntentWithContext(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("ParseIntentWithContext failed: %v", err)
	}

	// Verify the parsed intent is the fallback intent
	if intent.Verb != "/explain" {
		t.Errorf("Expected Verb /explain, got %s", intent.Verb)
	}
	if intent.Category != "/query" {
		t.Errorf("Expected Category /query, got %s", intent.Category)
	}
	if intent.Response != "Input is empty" {
		t.Errorf("Expected Response 'Input is empty', got %s", intent.Response)
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
		// TODO: TEST_GAP_COERCION_03: Exhaustively test camelCase, snake_case, and erratic casing hallucinated by LLM.
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

	// TODO: TEST_GAP_NULL_01: Add test cases for empty strings ("" and "   ") for both semanticType and actionType to ensure correct fallback to `/query`.
	// TODO: TEST_GAP_EXTREME_02: Ensure mapping logic is resilient to novel domains with strange characters, avoiding invalid Mangle atom creation.

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

// TODO: TEST_GAP_COERCION_01: Mock the LLM to return improperly typed JSON (e.g., a string instead of float64 for confidence) and ensure safe fallback intent.
// TODO: TEST_GAP_COERCION_02: Mock the LLM to return completely empty JSON `{}` and ensure defaults map to safe Intent without nil pointer panics.

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

// TEST_GAP_NULL_01: mapSemanticToCategory with empty strings
func TestUnderstandingTransducer_MapSemanticToCategory_EmptyStrings(t *testing.T) {
	tr := &UnderstandingTransducer{}
	got := tr.mapSemanticToCategory("", "")
	if got != "/query" {
		t.Errorf("Expected /query for empty strings, got %s", got)
	}
	got = tr.mapSemanticToCategory("   ", " \t \n")
	if got != "/query" {
		t.Errorf("Expected /query for whitespace strings, got %s", got)
	}
}

// TEST_GAP_NULL_03: ParseIntent_NilHistory vs EmptyHistory
func TestUnderstandingTransducer_ParseIntent_NilHistory(t *testing.T) {
	mockClient := &mockLLMClientUT{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"understanding":{"action_type":"chat"},"surface_response":"hi"}`, nil
		},
	}
	tr := NewUnderstandingTransducer(mockClient)
	_, err := tr.ParseIntentWithContext(context.Background(), "hello", nil)
	if err != nil {
		t.Errorf("ParseIntentWithContext failed with nil history: %v", err)
	}
	_, err = tr.ParseIntentWithContext(context.Background(), "hello", []ConversationTurn{})
	if err != nil {
		t.Errorf("ParseIntentWithContext failed with empty history: %v", err)
	}
}

// TEST_GAP_COERCION_01 & 02: Malformed JSON
func TestUnderstandingTransducer_MalformedJSON(t *testing.T) {
	mockClient := &mockLLMClientUT{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return `this is not json`, nil
		},
	}
	tr := NewUnderstandingTransducer(mockClient)
	intent, err := tr.ParseIntentWithContext(context.Background(), "do something", nil)
	if err != nil {
		t.Errorf("Expected graceful degradation, got error: %v", err)
	}
	if intent.Verb != "/explain" || intent.Category != "/query" {
		t.Errorf("Expected fallback to /explain /query, got %s %s", intent.Verb, intent.Category)
	}
	if !strings.Contains(intent.Response, "trouble understanding") {
		t.Errorf("Expected fallback response, got %s", intent.Response)
	}
}

// TEST_GAP_EXTREME_01: LargeInput
func TestUnderstandingTransducer_ParseIntent_LargeInput(t *testing.T) {
	mockClient := &mockLLMClientUT{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			// check prompt length to ensure truncation occurred
			if len(prompt) > 60000 {
				t.Errorf("Prompt is too large! Expected truncation, got len=%d", len(prompt))
			}
			return `{"understanding":{"action_type":"chat"},"surface_response":"hi"}`, nil
		},
	}
	tr := NewUnderstandingTransducer(mockClient)
	largeInput := strings.Repeat("A", 100000)
	_, err := tr.ParseIntentWithContext(context.Background(), largeInput, nil)
	if err != nil {
		t.Errorf("Failed with large input: %v", err)
	}
}

// TEST_GAP_EXTREME_02: AssertStabilityFacts_NovelDomain
func TestUnderstandingTransducer_AssertStabilityFacts_NovelDomain(t *testing.T) {
	// sanitizeAtomString testing implicitly through assertStabilityFacts
	res := sanitizeAtomString("Blargh Code! \n  Yes")
	if res != "blargh_code___yes" {
		t.Errorf("Sanitize failed: got %q", res)
	}
}

// TEST_GAP_CONCURRENCY: Concurrency
func TestUnderstandingTransducer_Concurrency(t *testing.T) {
	mockClient := &mockLLMClientUT{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"understanding":{"action_type":"chat"},"surface_response":"hi"}`, nil
		},
	}
	tr := &UnderstandingTransducer{client: mockClient}
	
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _ = tr.ParseIntentWithContext(context.Background(), fmt.Sprintf("msg %d", id), nil)
			tr.mu.RLock()
			_ = tr.lastUnderstanding
			tr.mu.RUnlock()
		}(i)
	}
	wg.Wait()
}
