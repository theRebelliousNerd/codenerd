package perception

import (
	"codenerd/internal/types"
	"context"
	"strings"
	"testing"
)

// Mock for LLM usage in Taxonomy (e.g. Critic)
type mockClient struct {
	completeFunc       func(ctx context.Context, prompt string) (string, error)
	schemaCapable      bool
	schemaCompleteFunc func(ctx context.Context, sys, user, schema string) (string, error)
}

func (m *mockClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "", nil
}
func (m *mockClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, user)
	}
	return "", nil
}
func (m *mockClient) CompleteWithStructuredOutput(ctx context.Context, sys, user string, think bool) (string, error) {
	return "", nil
}
func (m *mockClient) CompleteWithTools(ctx context.Context, sys, user string, tools []ToolDefinition) (*LLMToolResponse, error) {
	return &LLMToolResponse{Text: "", StopReason: "end_turn"}, nil
}
func (m *mockClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan types.StreamResult) {
	return types.StreamWhole(ctx, func(ctx context.Context) (string, error) {
		return m.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	})
}
func (m *mockClient) SetModel(s string) {}
func (m *mockClient) GetModel() string  { return "mock" }
func (m *mockClient) DisableSemaphore() {}

// SchemaCapable indicates this mock can produce structured output.
func (m *mockClient) SchemaCapable() bool {
	return m.schemaCapable
}

// CompleteWithSchema returns structured JSON output.
func (m *mockClient) CompleteWithSchema(ctx context.Context, sys, user, schema string) (string, error) {
	if m.schemaCompleteFunc != nil {
		return m.schemaCompleteFunc(ctx, sys, user, schema)
	}
	if m.completeFunc != nil {
		return m.completeFunc(ctx, user)
	}
	return "", nil
}

func TestTaxonomyEngine_Initialization(t *testing.T) {
	// Simple smoke test for initialization
	// This relies on embedded schemas working. If they fail, we can't test much logic.
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Logf("Skipping taxonomy init test due to env deps: %v", err)
		return
	}
	if engine == nil {
		t.Fatal("NewTaxonomyEngine returned nil")
	}
}

func TestTaxonomyEngine_GetVerbs_Defaults(t *testing.T) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to initialization failure")
	}

	verbs, err := engine.GetVerbs()
	if err != nil {
		t.Fatalf("GetVerbs failed: %v", err)
	}

	if len(verbs) == 0 {
		t.Error("Expected default verbs to be loaded, got 0")
	}

	// Verify /fix exists
	found := false
	for _, v := range verbs {
		if v.Verb == "/fix" {
			found = true
			if v.Category != "/mutation" {
				t.Errorf("/fix category = %s, want /mutation", v.Category)
			}
			break
		}
	}
	if !found {
		t.Error("/fix verb not found in default taxonomy")
	}
}

// TestTaxonomyEngine_GetVerbs_ShardTypesNormalized guards the slash-prefix
// normalization: the taxonomy stores Mangle name constants ("/reviewer") but
// every Go consumer compares against bare names ("reviewer"). Before the fix,
// GetShardTypeForVerb leaked "/reviewer" into shardTypeToTaskRequest, which
// mistook it for an intent verb and ran delegated tasks with a default
// persona.
func TestTaxonomyEngine_GetVerbs_ShardTypesNormalized(t *testing.T) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to initialization failure")
	}

	verbs, err := engine.GetVerbs()
	if err != nil {
		t.Fatalf("GetVerbs failed: %v", err)
	}
	if len(verbs) == 0 {
		t.Fatal("expected default verbs")
	}

	for _, v := range verbs {
		if strings.HasPrefix(v.ShardType, "/") {
			t.Errorf("verb %s: ShardType %q not normalized to bare name", v.Verb, v.ShardType)
		}
		if v.ShardType == "none" {
			t.Errorf("verb %s: ShardType \"none\" should normalize to empty string", v.Verb)
		}
	}

	// Spot-check the canonical mappings.
	want := map[string]string{
		"/review":  "reviewer",
		"/fix":     "coder",
		"/test":    "tester",
		"/explain": "", // /none → ""
	}
	for _, v := range verbs {
		if expected, ok := want[v.Verb]; ok && v.ShardType != expected {
			t.Errorf("verb %s: ShardType = %q, want %q", v.Verb, v.ShardType, expected)
		}
	}
}

func TestTaxonomyEngine_ClassifyInput_Simple(t *testing.T) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatalf("NewTaxonomyEngine: %v", err)
	}
	defer engine.StopWorker()

	// Classify against the engine's loaded corpus. An empty or unreadable corpus
	// is a broken behavior oracle, not a reason to substitute test-local verbs.
	candidates, err := engine.GetVerbs()
	if err != nil {
		t.Fatalf("GetVerbs: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("GetVerbs returned an empty verb corpus")
	}
	corpusVerbs := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		corpusVerbs[candidate.Verb] = true
	}
	for _, verb := range []string{"/fix", "/test"} {
		if !corpusVerbs[verb] {
			t.Fatalf("real verb corpus does not contain %q", verb)
		}
	}

	tests := []struct {
		input string
		want  string
	}{
		{"fix this bug", "/fix"},
		{"run tests", "/test"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			verb, _, err := engine.ClassifyInput(tt.input, candidates)
			if err != nil {
				t.Fatalf("ClassifyInput(%q): %v", tt.input, err)
			}
			if verb != tt.want {
				t.Fatalf("ClassifyInput(%q) = %q, want %q", tt.input, verb, tt.want)
			}
		})
	}
}

// TestTaxonomyEngine_ClassifyInput_Idempotent verifies bug #18 fix preserves
// behavioral equivalence: classifying the same input across multiple calls,
// including with other inputs interleaved, must return the same result. This
// catches state leakage between calls if Clear() ever fails to wipe transient
// facts that influence scoring.
func TestTaxonomyEngine_ClassifyInput_Idempotent(t *testing.T) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Skipf("init failed: %v", err)
	}
	defer engine.StopWorker()

	candidates, err := engine.GetVerbs()
	if err != nil || len(candidates) == 0 {
		t.Skip("no candidates available")
	}

	inputA := "fix this bug in the parser"
	inputB := "explain how the kernel routes intents"

	verbA1, _, errA1 := engine.ClassifyInput(inputA, candidates)
	if errA1 != nil {
		t.Skipf("classify A1 failed (mangle dependency): %v", errA1)
	}
	verbB, _, errB := engine.ClassifyInput(inputB, candidates)
	if errB != nil {
		t.Fatalf("classify B failed: %v", errB)
	}
	verbA2, _, errA2 := engine.ClassifyInput(inputA, candidates)
	if errA2 != nil {
		t.Fatalf("classify A2 failed: %v", errA2)
	}

	if verbA1 != verbA2 {
		t.Errorf("ClassifyInput not idempotent: first call returned %q, second returned %q (intervening input: %q -> %q)", verbA1, verbA2, inputB, verbB)
	}
}

// TestTaxonomyEngine_SchemasLoadedOnce verifies that ClassifyInput does NOT
// reload static schemas on every call (bug #18). It checks the schemasLoaded
// flag is set after construction and remains set after multiple calls.
func TestTaxonomyEngine_SchemasLoadedOnce(t *testing.T) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Skipf("init failed: %v", err)
	}
	defer engine.StopWorker()

	if !engine.schemasLoaded {
		t.Fatal("schemasLoaded should be true after NewTaxonomyEngine")
	}

	candidates, _ := engine.GetVerbs()
	if len(candidates) == 0 {
		candidates = []VerbEntry{{Verb: "/fix", Priority: 90}}
	}

	for i := range 5 {
		_, _, _ = engine.ClassifyInput("fix bug", candidates)
		if !engine.schemasLoaded {
			t.Fatalf("schemasLoaded flipped to false after call %d", i)
		}
	}
}
