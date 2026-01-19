package perception

import (
	"context"
	"strings"
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
	// Setup mock LLM that returns a valid Understanding JSON
	understandingJSON := `{
		"understanding": {
			"primary_intent": "fix bug",
			"semantic_type": "instruction",
			"action_type": "modify",
			"domain": "testing",
			"scope": {
				"target": "login_test.go",
				"file": "internal/auth/login_test.go"
			},
			"user_constraints": ["do not break existing tests"],
			"confidence": 0.95
		},
		"surface_response": "I will fix the bug in login_test.go"
	}`

	mock := &mockClient{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return understandingJSON, nil
		},
	}

	transducer := NewUnderstandingTransducer(mock)
	if transducer == nil {
		t.Fatal("NewUnderstandingTransducer returned nil")
	}

	intent, err := transducer.ParseIntent(context.Background(), "fix the login test")
	if err != nil {
		t.Fatalf("ParseIntent failed: %v", err)
	}

	// Verify mappings
	// mapActionToVerb("modify", "testing") -> "/fix"
	if intent.Verb != "/fix" {
		t.Errorf("Expected verb /fix, got %s", intent.Verb)
	}
	// mapSemanticToCategory("instruction", "modify") -> "/instruction" or "/mutation"
	// Look at mapSemanticToCategory implementation:
	// if semanticType == "instruction" -> return "/instruction"
	if intent.Category != "/instruction" {
		t.Errorf("Expected category /instruction, got %s", intent.Category)
	}
	if intent.Target != "login_test.go" && intent.Target != "internal/auth/login_test.go" {
		t.Errorf("Target mismatch: got %s", intent.Target)
	}
	if intent.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", intent.Confidence)
	}
	if !strings.Contains(intent.Constraint, "do not break") {
		t.Errorf("Constraint mismatch: got %s", intent.Constraint)
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
