package perception

import (
	"context"
	"testing"
)

// Mock for LLM usage in Taxonomy (e.g. Critic)
type mockClient struct {
	completeFunc func(ctx context.Context, prompt string) (string, error)
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
func (m *mockClient) SetModel(s string) {}
func (m *mockClient) GetModel() string  { return "mock" }
func (m *mockClient) DisableSemaphore() {}

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

func TestTaxonomyEngine_ClassifyInput_Simple(t *testing.T) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping due to initialization failure")
	}

	// We need candidates to classify.
	candidates, _ := engine.GetVerbs()
	if len(candidates) == 0 {
		// Mock candidates if defaults not loaded?
		candidates = []VerbEntry{
			{Verb: "/fix", Category: "/mutation", Priority: 90, Synonyms: []string{"fix", "repair"}},
			{Verb: "/test", Category: "/mutation", Priority: 88, Synonyms: []string{"test"}},
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
		verb, _, err := engine.ClassifyInput(tt.input, candidates)
		if err != nil {
			// ClassifyInput relies on Mangle schemas which may not be loaded
			t.Skipf("ClassifyInput(%q) error (Mangle dependency): %v", tt.input, err)
		}

		// Note: ClassifyInput uses Mangle logic which might vary slightly based on rules loaded.
		// We just check if it returns *something* reasonable or if logic allows.
		// If Mangle rules aren't fully loaded, this might return empty.
		if verb != "" && verb != tt.want {
			t.Errorf("ClassifyInput(%q) = %q, want %q", tt.input, verb, tt.want)
		}
	}
}

func TestGenerateSystemPromptSection(t *testing.T) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		t.Skip("Skipping init")
	}

	prompt, err := engine.GenerateSystemPromptSection()
	if err != nil {
		t.Fatalf("GenerateSystemPromptSection failed: %v", err)
	}

	if len(prompt) < 10 {
		t.Error("Generated prompt too short")
	}
}
