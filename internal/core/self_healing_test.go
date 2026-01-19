package core

import (
	"context"
	"testing"
)

// mockActionExecutor implements ActionExecutor for testing
type mockActionExecutor struct {
	execCount  int
	shouldFail bool
}

func (m *mockActionExecutor) Execute(ctx context.Context, req ActionRequest) (ActionResult, error) {
	m.execCount++
	if m.shouldFail {
		return ActionResult{Success: false, Error: "mock failure"}, nil
	}
	return ActionResult{Success: true}, nil
}

func TestSelfHealer_New(t *testing.T) {
	k := setupMockKernel(t)
	config := DefaultSelfHealerConfig()

	healer := NewSelfHealer(k, nil, config)
	if healer == nil {
		t.Fatal("NewSelfHealer returned nil")
	}
}

func TestSelfHealer_SetExecutor(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	executor := &mockActionExecutor{}
	healer.SetExecutor(executor)

	// No error means success
}

func TestSelfHealer_HandleValidationFailure_Retry(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	executor := &mockActionExecutor{shouldFail: false}
	healer.SetExecutor(executor)

	req := ActionRequest{
		ActionID: "action-1",
		Type:     ActionWriteFile,
	}
	vr := ValidationResult{
		Verified: false,
		Error:    "content mismatch",
	}

	ctx := context.Background()
	result, err := healer.HandleValidationFailure(ctx, req, vr)

	if err != nil {
		t.Logf("HandleValidationFailure returned error: %v", err)
	}

	if result != nil {
		t.Logf("Healing result: strategy=%s, success=%v", result.Strategy, result.Success)
	}
}

func TestSelfHealer_DetermineHealingType(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	vr := ValidationResult{
		Verified: false,
		Error:    "file not found",
	}

	healingType := healer.determineHealingType("action-1", vr)

	// Should return some healing type
	if healingType == "" {
		t.Error("Expected non-empty healing type")
	}
}

func TestSelfHealer_ClearHealingAttempts(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	// Clear should not panic
	healer.ClearHealingAttempts("action-1")

	// Get attempts should be 0
	attempts := healer.GetHealingAttempts("action-1")
	if attempts != 0 {
		t.Errorf("Expected 0 attempts after clear, got %d", attempts)
	}
}

func TestDefaultSelfHealerConfig(t *testing.T) {
	config := DefaultSelfHealerConfig()

	if config.MaxRetries <= 0 {
		t.Errorf("Expected positive MaxRetries, got %d", config.MaxRetries)
	}

	if config.RetryBackoff <= 0 {
		t.Error("Expected positive RetryBackoff")
	}
}
