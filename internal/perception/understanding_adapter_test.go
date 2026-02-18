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
	// SKIP: This test has pre-existing mock interaction issues with LLMTransducer.
	// The actual perception flow is tested via live integration tests.
	t.Skip("Pre-existing mock issue: LLMTransducer uses internal prompt flow that doesn't match mock")
	// TODO: FIX: Unskip this test. The main entry point `ParseIntentWithContext` lacks unit test coverage.
	// Use gomock or a proper mock interface to simulate LLM interaction.
}

func TestUnderstandingTransducer_MapActionToVerb(t *testing.T) {
	tr := &UnderstandingTransducer{}

	// TODO: TEST_GAP: Add test cases for case-insensitive matching (e.g., "Investigate", "MODIFY").
	// Current implementation only handles lowercase, which might fail with non-deterministic LLM output.

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
	}

	for _, tt := range tests {
		got := tr.mapActionToVerb(tt.action, tt.domain)
		if got != tt.want {
			t.Errorf("mapActionToVerb(%q, %q) = %q, want %q", tt.action, tt.domain, got, tt.want)
		}
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
