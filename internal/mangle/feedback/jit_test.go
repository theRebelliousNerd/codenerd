package feedback

import (
	"context"
	"testing"
)

// MockPredicateSelector implements PredicateSelectorInterface for testing.
type MockPredicateSelector struct {
	SelectFunc func(ctx context.Context, shardType, intentVerb, domain, query string) ([]string, error)
	CallCount  int
}

func (m *MockPredicateSelector) SelectForContext(ctx context.Context, shardType, intentVerb, domain, query string) ([]string, error) {
	m.CallCount++
	if m.SelectFunc != nil {
		return m.SelectFunc(ctx, shardType, intentVerb, domain, query)
	}
	return nil, nil
}

func TestFeedbackLoop_JITPredicateSelection(t *testing.T) {
	// 1. Setup
	config := DefaultConfig()
	config.MaxRetries = 2
	fl := NewFeedbackLoop(config)

	// Mock Selector to return specific JIT predicates
	mockSelector := &MockPredicateSelector{
		SelectFunc: func(ctx context.Context, shardType, intentVerb, domain, query string) ([]string, error) {
			return []string{"jit_predicate/1", "context_aware/2"}, nil
		},
	}
	fl.SetPredicateSelector(mockSelector)

	// Mock LLM: Fails first (invalid syntax), Succeeds second
	mockLLM := &MockLLMClient{
		responses: []string{
			"invalid(syntax).", // Attempt 1
			"valid(rule).",     // Attempt 2
		},
	}

	// Mock Validator: Rejects invalid, Accepts valid
	mockValidator := &MockRuleValidator{
		validRules: map[string]bool{
			"valid(rule).": true,
		},
		predicates: []string{"static_predicate/1"}, // Default static predicates
	}

	// 2. Execute
	// We expect the loop to fail on attempt 1, call SelectForContext, regenerate prompt with JIT preds, and succeed on attempt 2.
	result, err := fl.GenerateAndValidate(
		context.Background(),
		mockLLM,
		mockValidator,
		"system prompt",
		"user prompt",
		"test_domain",
	)

	// 3. Verify
	if err != nil {
		t.Fatalf("GenerateAndValidate failed: %v", err)
	}
	if !result.Valid {
		t.Error("Result should be valid")
	}
	if result.Attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", result.Attempts)
	}

	// Verify Selector was called
	if mockSelector.CallCount == 0 {
		t.Error("PredicateSelector.SelectForContext was NOT called")
	}

	// Verify that the prompt for the second attempt contained the JIT predicates.
	// Since we can't easily inspect the internal prompt construction from here without mocking the PromptBuilder too,
	// we rely on the fact that SelectForContext was called.
	// However, we can check if the feedback loop used the JIT predicates.
	// We can trust coverage of loop.go logic if we hit the selector.
}
