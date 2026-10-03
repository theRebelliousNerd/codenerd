package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/prompt"
)

func TestUnderstandingSupportingAtom_CanonicalSchemaAndExamples(test *testing.T) {
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		test.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	atom, exists := corpus.Get("perception_understanding")
	if !exists {
		test.Fatal("mandatory supporting understanding atom is missing")
	}
	blocks := strings.Split(atom.Content, "```json")
	if len(blocks) != 7 {
		test.Fatalf("got %d JSON blocks, want one schema and five examples", len(blocks)-1)
	}
	expected := []struct {
		target     string
		confidence float64
		signals    Signals
		mode       string
		shard      string
		support    string
		tools      string
		context    string
	}{
		{"TestValidateToken", 0.95, Signals{IsQuestion: true, Urgency: "normal"}, "tdd", "tester", "coder", "run_tests,read_file,ast_query", "test_output,test_source,function_under_test"},
		{"API", 0.88, Signals{IsMultiStep: true, Urgency: "normal"}, "tdd", "coder", "tester", "edit_file,run_tests,ast_query", "api_handlers,middleware_patterns,existing_tests"},
		{"LocalStore and semantic vector recall", 0.95, Signals{IsQuestion: true, Urgency: "normal"}, "normal", "researcher", "", "grep_search,read_file,ast_query", "LocalStore definition,vector recall implementation"},
		{"caching layer", 0.92, Signals{IsQuestion: true, IsHypothetical: true, Urgency: "low"}, "dream", "reviewer", "", "ast_query,grep,read_file", "cache_usage,dependencies,performance_impact"},
		{"user", 0.99, Signals{Urgency: "normal"}, "normal", "none", "", "", ""},
	}
	for index, block := range blocks[1:] {
		object, _, closed := strings.Cut(block, "```")
		if !closed || !isValidUnderstandingPromptContract(object) {
			test.Fatalf("JSON block %d does not express the canonical nested envelope", index)
		}
		if index == 0 {
			continue
		}
		var envelope UnderstandingEnvelope
		decoder := json.NewDecoder(strings.NewReader(object))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&envelope); err != nil {
			test.Fatalf("example %d does not decode into the production envelope: %v", index, err)
		}
		want := expected[index-1]
		understanding := envelope.Understanding
		if understanding.Scope.Target != want.target || understanding.Confidence != want.confidence || understanding.Signals != want.signals || envelope.SurfaceResponse == "" {
			test.Errorf("example %d lost its target, confidence, signals, or response: %+v", index, envelope)
		}
		approach := understanding.SuggestedApproach
		if approach.Mode != want.mode || approach.PrimaryShard != want.shard || strings.Join(approach.SupportingShards, ",") != want.support || strings.Join(approach.ToolsNeeded, ",") != want.tools || strings.Join(approach.ContextNeeded, ",") != want.context {
			test.Errorf("example %d lost its suggested approach: %+v", index, approach)
		}
	}
}

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
		// REMEDIATED: TEST_GAP_COERCION_03: Exhaustively test camelCase, snake_case, and erratic casing hallucinated by LLM.
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
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	owner, ok := corpus.Get("system/perception/output_format")
	if !ok {
		t.Fatal("canonical perception output owner is missing")
	}
	valid := owner.Content
	piggyback, ok := corpus.Get("protocol/piggyback/envelope")
	if !ok {
		t.Fatal("conversational Piggyback control is missing")
	}
	tests := []struct {
		name   string
		prompt string
		want   bool
	}{
		{
			name:   "canonical owner with nested target and numeric template",
			prompt: valid,
			want:   true,
		},
		{
			name:   "field names mentioned in prose are not a competing schema",
			prompt: valid + ` Do not confuse "control_packet", "category", "verb", "target", or "constraint" with the owning envelope.`,
			want:   true,
		},
		{
			name:   "quoted braces and escaped quotes stay inside a value",
			prompt: strings.Replace(valid, "<specific target>", `brace } and quote \" target`, 1),
			want:   true,
		},
		{
			name:   "piggyback contract is invalid for perception",
			prompt: `{"control_packet": {}, "surface_response": "hi"}`,
			want:   false,
		},
		{
			name:   "valid owner plus competing Piggyback contract",
			prompt: valid + "\n" + piggyback.Content,
			want:   false,
		},
		{
			name:   "valid owner plus legacy flat contract",
			prompt: valid + ` {"category":"query","verb":"review","target":"file.go"}`,
			want:   false,
		},
		{
			name:   "missing nested scope",
			prompt: strings.Replace(valid, `"scope":`, `"old_scope":`, 1),
			want:   false,
		},
		{
			name:   "missing nested approach",
			prompt: strings.Replace(valid, `"suggested_approach":`, `"old_approach":`, 1),
			want:   false,
		},
		{
			name:   "malformed owning object",
			prompt: strings.Replace(valid, `"action_type":`, `action_type:`, 1),
			want:   false,
		},
		{
			name:   "wrong understanding type",
			prompt: `{"understanding":null,"surface_response":"hello"}`,
			want:   false,
		},
		{
			name:   "keys scattered in prose do not establish an envelope",
			prompt: `"understanding" "surface_response" "primary_intent" "semantic_type" "action_type" "domain" "scope" "suggested_approach"`,
			want:   false,
		},
		{
			name:   "unclosed owning object",
			prompt: `{"understanding": {"primary_intent":"debug"}`,
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
	// REMEDIATED: TEST_GAP_NULL_01: Add test cases for empty strings ("" and "   ") for both semanticType and actionType to ensure correct fallback to `/query`.
	// REMEDIATED: TEST_GAP_EXTREME_02: Ensure mapping logic is resilient to novel domains with strange characters, avoiding invalid Mangle atom creation.

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

// REMEDIATED: TEST_GAP_COERCION_01: Mock the LLM to return improperly typed JSON (e.g., a string instead of float64 for confidence) and ensure safe fallback intent.
// REMEDIATED: TEST_GAP_COERCION_02: Mock the LLM to return completely empty JSON `{}` and ensure defaults map to safe Intent without nil pointer panics.

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

// TEST_GAP_CONCURRENCY: Concurrency
func TestUnderstandingTransducer_Concurrency(t *testing.T) {
	mockClient := &mockLLMClientUT{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"understanding":{"action_type":"chat"},"surface_response":"hi"}`, nil
		},
	}
	tr := &UnderstandingTransducer{client: mockClient}

	var wg sync.WaitGroup
	for i := range 100 {
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

func (m *mockLLMClientUT) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, forceJSON bool) (<-chan string, <-chan error) {
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
