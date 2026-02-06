package campaign

import (
	"context"
	"errors"
	"testing"
)

func TestReplanner_RecursionFix(t *testing.T) {
	// Setup
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "Mock response", nil
		},
	}

	// Create replanner with nil kernel (not needed for this test)
	// We pass the mock as the LLMClient
	r := NewReplanner(nil, mockLLM)

	// Context
	ctx := context.Background()

	// Execution
	// This should NOT panic with stack overflow
	resp, err := r.completeWithGrounding(ctx, "Test prompt")

	// Verification
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if resp != "Mock response" {
		t.Errorf("Expected 'Mock response', got '%s'", resp)
	}
}

func TestReplanner_RecursionFix_ErrorPropagates(t *testing.T) {
	// Setup
	expectedErr := errors.New("LLM error")
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "", expectedErr
		},
	}

	r := NewReplanner(nil, mockLLM)
	ctx := context.Background()

	// Execution
	_, err := r.completeWithGrounding(ctx, "Test prompt")

	// Verification
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}