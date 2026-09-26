package core

import (
	"context"
	"math"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TEST_GAP: Null/Empty
func TestActionValidator_NilRegister(t *testing.T) {
	r := NewValidatorRegistry()
	r.Register(nil) // Should handle gracefully
	if len(r.validators) != 0 {
		t.Errorf("Expected 0 validators, got %d", len(r.validators))
	}
}

func TestValidationResult_ToFacts_Empty(t *testing.T) {
	vr := ValidationResult{
		Verified: true,
	}
	facts := vr.ToFacts()
	if len(facts) != 2 {
		t.Errorf("Expected 2 facts for empty fields, got %d", len(facts))
	}
	// ActionID and ActionType will just be empty strings, which is fine
}

type dummyValidator struct {
	priority int
	sleep    time.Duration
}

func (d *dummyValidator) CanValidate(t ActionType) bool { return true }
func (d *dummyValidator) Name() string                  { return "dummy" }
func (d *dummyValidator) Priority() int                 { return d.priority }
func (d *dummyValidator) Validate(ctx context.Context, req ActionRequest, res ActionResult) ValidationResult {
	if d.sleep > 0 {
		time.Sleep(d.sleep)
	}
	return ValidationResult{Verified: true, Confidence: 0.5}
}

// TEST_GAP: User Request Extremes
func TestActionValidator_MassiveValidators(t *testing.T) {
	r := NewValidatorRegistry()
	for i := range 2000 {
		r.Register(&dummyValidator{priority: i})
	}
	req := ActionRequest{Type: ActionExecCmd}
	res := ActionResult{}
	results := r.Validate(context.Background(), req, res)
	if len(results) != 2000 {
		t.Errorf("Expected 2000 results, got %d", len(results))
	}
}

func TestValidationResult_ToFacts_MassiveDetails(t *testing.T) {
	details := make(map[string]any)
	for i := range 1000 {
		details["key"+strconv.Itoa(i)] = "value"
	}
	vr := ValidationResult{
		Verified: false,
		Details:  details,
	}
	facts := vr.ToFacts()
	// Length should be clamped to 1024
	detailStr := facts[0].Args[3].(string)
	if len(detailStr) > 1024 {
		t.Errorf("Details string not truncated, length: %d", len(detailStr))
	}
}

// TEST_GAP: State Conflicts
func TestActionValidator_ConcurrentRegister(t *testing.T) {
	r := NewValidatorRegistry()
	var wg sync.WaitGroup
	req := ActionRequest{Type: ActionExecCmd}
	res := ActionResult{}

	for i := range 50 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r.Register(&dummyValidator{priority: idx})
		}(i)

		wg.Go(func() {
			r.Validate(context.Background(), req, res)
		})
	}
	wg.Wait()
}

func TestActionValidator_PriorityStability(t *testing.T) {
	r := NewValidatorRegistry()
	r.Register(&dummyValidator{priority: 10})
	r.Register(&dummyValidator{priority: 10})
	r.Register(&dummyValidator{priority: 10})
	if len(r.validators) != 3 {
		t.Error("Expected 3 validators")
	}
	// just ensure it didn't panic or drop
}

func TestActionValidator_EmptyResultsSlice(t *testing.T) {
	var results []ValidationResult

	if !ValidateAll(results) {
		t.Error("ValidateAll should return true for empty results")
	}

	if ff := FirstFailure(results); ff != nil {
		t.Error("FirstFailure should return nil for empty results")
	}
}

func TestValidationResult_ToFacts_ClampsConfidence(t *testing.T) {
	for _, c := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1.5, -0.5} {
		vr := ValidationResult{Verified: true, Confidence: c}
		facts := vr.ToFacts()
		confVal := facts[0].Args[3].(int64)
		if confVal < 0 || confVal > 100 {
			t.Errorf("Confidence %v clamped incorrectly to %d", c, confVal)
		}
	}
}
