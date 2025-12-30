package core

import (
	"context"
	"testing"
	"time"
)

// mockValidator implements ActionValidator for testing
type mockValidator struct {
	name       string
	priority   int
	canHandle  []ActionType
	verifyFunc func(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult
}

func (m *mockValidator) CanValidate(actionType ActionType) bool {
	for _, at := range m.canHandle {
		if at == actionType {
			return true
		}
	}
	return false
}

func (m *mockValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	if m.verifyFunc != nil {
		return m.verifyFunc(ctx, req, result)
	}
	return ValidationResult{
		ActionID:   req.ActionID,
		ActionType: req.Type,
		Verified:   true,
		Confidence: 1.0,
		Method:     "mock",
	}
}

func (m *mockValidator) Name() string     { return m.name }
func (m *mockValidator) Priority() int    { return m.priority }

func TestValidatorRegistry_Register(t *testing.T) {
	r := NewValidatorRegistry()

	v1 := &mockValidator{name: "v1", priority: 10, canHandle: []ActionType{ActionWriteFile}}
	v2 := &mockValidator{name: "v2", priority: 5, canHandle: []ActionType{ActionWriteFile}}
	v3 := &mockValidator{name: "v3", priority: 15, canHandle: []ActionType{ActionReadFile}}

	r.Register(v1)
	r.Register(v2)
	r.Register(v3)

	// Check that validators are sorted by priority
	if len(r.validators) != 3 {
		t.Fatalf("expected 3 validators, got %d", len(r.validators))
	}

	if r.validators[0].Name() != "v2" {
		t.Errorf("expected v2 first (priority 5), got %s", r.validators[0].Name())
	}
	if r.validators[1].Name() != "v1" {
		t.Errorf("expected v1 second (priority 10), got %s", r.validators[1].Name())
	}
	if r.validators[2].Name() != "v3" {
		t.Errorf("expected v3 third (priority 15), got %s", r.validators[2].Name())
	}
}

func TestValidatorRegistry_Validate(t *testing.T) {
	r := NewValidatorRegistry()

	v1 := &mockValidator{
		name:      "hash_validator",
		priority:  10,
		canHandle: []ActionType{ActionWriteFile},
		verifyFunc: func(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
			return ValidationResult{
				Verified:   true,
				Confidence: 1.0,
				Method:     ValidationMethodHash,
			}
		},
	}
	v2 := &mockValidator{
		name:      "syntax_validator",
		priority:  20,
		canHandle: []ActionType{ActionWriteFile},
		verifyFunc: func(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
			return ValidationResult{
				Verified:   true,
				Confidence: 0.9,
				Method:     ValidationMethodSyntax,
			}
		},
	}

	r.Register(v1)
	r.Register(v2)

	req := ActionRequest{
		ActionID: "test-action-1",
		Type:     ActionWriteFile,
		Target:   "/tmp/test.go",
	}
	result := ActionResult{Success: true}

	results := r.Validate(context.Background(), req, result)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result should be from hash_validator (lower priority)
	if results[0].Method != ValidationMethodHash {
		t.Errorf("expected first result method to be hash, got %s", results[0].Method)
	}

	// Second result should be from syntax_validator
	if results[1].Method != ValidationMethodSyntax {
		t.Errorf("expected second result method to be syntax, got %s", results[1].Method)
	}
}

func TestValidatorRegistry_NoValidators(t *testing.T) {
	r := NewValidatorRegistry()

	req := ActionRequest{
		ActionID: "test-action-1",
		Type:     ActionWriteFile,
	}
	result := ActionResult{Success: true}

	results := r.Validate(context.Background(), req, result)

	if len(results) != 1 {
		t.Fatalf("expected 1 result (skipped), got %d", len(results))
	}

	if results[0].Method != ValidationMethodSkipped {
		t.Errorf("expected method to be skipped, got %s", results[0].Method)
	}

	if !results[0].Verified {
		t.Error("expected skipped validation to be marked as verified")
	}
}

func TestValidatorRegistry_ShortCircuitOnFailure(t *testing.T) {
	r := NewValidatorRegistry()

	callCount := 0
	v1 := &mockValidator{
		name:      "failing_validator",
		priority:  10,
		canHandle: []ActionType{ActionWriteFile},
		verifyFunc: func(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
			callCount++
			return ValidationResult{
				Verified:   false,
				Confidence: 0.95, // High confidence failure
				Error:      "hash mismatch",
				Method:     ValidationMethodHash,
			}
		},
	}
	v2 := &mockValidator{
		name:      "second_validator",
		priority:  20,
		canHandle: []ActionType{ActionWriteFile},
		verifyFunc: func(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
			callCount++
			return ValidationResult{
				Verified:   true,
				Confidence: 0.9,
				Method:     ValidationMethodSyntax,
			}
		},
	}

	r.Register(v1)
	r.Register(v2)

	req := ActionRequest{
		ActionID: "test-action-1",
		Type:     ActionWriteFile,
	}
	result := ActionResult{Success: true}

	results := r.Validate(context.Background(), req, result)

	// Should short-circuit after high-confidence failure
	if callCount != 1 {
		t.Errorf("expected 1 validator call (short-circuit), got %d", callCount)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Verified {
		t.Error("expected validation to fail")
	}
}

func TestValidatorRegistry_ContextCancellation(t *testing.T) {
	r := NewValidatorRegistry()

	v := &mockValidator{
		name:      "slow_validator",
		priority:  10,
		canHandle: []ActionType{ActionWriteFile},
		verifyFunc: func(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
			// Simulate slow operation
			select {
			case <-ctx.Done():
				return ValidationResult{Verified: false, Error: "cancelled"}
			case <-time.After(time.Second):
				return ValidationResult{Verified: true, Confidence: 1.0}
			}
		},
	}

	r.Register(v)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := ActionRequest{
		ActionID: "test-action-1",
		Type:     ActionWriteFile,
	}
	result := ActionResult{Success: true}

	results := r.Validate(ctx, req, result)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Verified {
		t.Error("expected validation to fail due to cancellation")
	}
}

func TestValidateAll(t *testing.T) {
	tests := []struct {
		name     string
		results  []ValidationResult
		expected bool
	}{
		{
			name:     "all passed",
			results:  []ValidationResult{{Verified: true}, {Verified: true}},
			expected: true,
		},
		{
			name:     "one failed",
			results:  []ValidationResult{{Verified: true}, {Verified: false}},
			expected: false,
		},
		{
			name:     "empty",
			results:  []ValidationResult{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAll(tt.results); got != tt.expected {
				t.Errorf("ValidateAll() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFirstFailure(t *testing.T) {
	results := []ValidationResult{
		{Verified: true, ActionID: "1"},
		{Verified: false, ActionID: "2", Error: "first error"},
		{Verified: false, ActionID: "3", Error: "second error"},
	}

	failure := FirstFailure(results)
	if failure == nil {
		t.Fatal("expected a failure result")
	}
	if failure.ActionID != "2" {
		t.Errorf("expected first failure to be action 2, got %s", failure.ActionID)
	}

	// Test with no failures
	allPassed := []ValidationResult{{Verified: true}, {Verified: true}}
	if FirstFailure(allPassed) != nil {
		t.Error("expected nil for all-passed results")
	}
}

func TestAggregate(t *testing.T) {
	results := []ValidationResult{
		{Verified: true, Confidence: 0.9},
		{Verified: false, Confidence: 0.8, Error: "error 1"},
		{Verified: true, Confidence: 1.0},
		{Verified: false, Confidence: 0.7, Error: "error 2"},
	}

	agg := Aggregate(results)

	if agg.AllVerified {
		t.Error("expected AllVerified to be false")
	}
	if agg.ValidatorCount != 4 {
		t.Errorf("expected 4 validators, got %d", agg.ValidatorCount)
	}
	if agg.FailureCount != 2 {
		t.Errorf("expected 2 failures, got %d", agg.FailureCount)
	}
	if agg.HighestConfidence != 1.0 {
		t.Errorf("expected highest confidence 1.0, got %f", agg.HighestConfidence)
	}
	if agg.LowestConfidence != 0.7 {
		t.Errorf("expected lowest confidence 0.7, got %f", agg.LowestConfidence)
	}
	if agg.FirstError != "error 1" {
		t.Errorf("expected first error 'error 1', got '%s'", agg.FirstError)
	}
}

func TestValidationResult_ToFacts(t *testing.T) {
	t.Run("verified result", func(t *testing.T) {
		vr := &ValidationResult{
			ActionID:   "action-123",
			ActionType: ActionWriteFile,
			Verified:   true,
			Confidence: 1.0,
			Method:     ValidationMethodHash,
			Timestamp:  time.Now(),
		}

		facts := vr.ToFacts()

		if len(facts) != 2 {
			t.Fatalf("expected 2 facts, got %d", len(facts))
		}

		// First fact should be action_verified
		if facts[0].Predicate != "action_verified" {
			t.Errorf("expected predicate action_verified, got %s", facts[0].Predicate)
		}

		// Second fact should be validation_method_used
		if facts[1].Predicate != "validation_method_used" {
			t.Errorf("expected predicate validation_method_used, got %s", facts[1].Predicate)
		}
	})

	t.Run("failed result", func(t *testing.T) {
		vr := &ValidationResult{
			ActionID:   "action-456",
			ActionType: ActionWriteFile,
			Verified:   false,
			Confidence: 0.9,
			Method:     ValidationMethodHash,
			Error:      "hash mismatch",
			Details:    map[string]interface{}{"expected": "abc123", "actual": "def456"},
			Timestamp:  time.Now(),
		}

		facts := vr.ToFacts()

		if len(facts) != 2 {
			t.Fatalf("expected 2 facts, got %d", len(facts))
		}

		// First fact should be action_validation_failed
		if facts[0].Predicate != "action_validation_failed" {
			t.Errorf("expected predicate action_validation_failed, got %s", facts[0].Predicate)
		}
	})
}
