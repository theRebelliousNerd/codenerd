package core

import (
	"context"
	"math"
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

func TestActionValidator_EmptyResultsSlice(t *testing.T) {
	var results []ValidationResult

	if !ValidateAll(results) {
		t.Error("ValidateAll should return true for empty results")
	}

	if ff := FirstFailure(results); ff != nil {
		t.Error("FirstFailure should return nil for empty results")
	}

	if hc := HighestConfidence(results); hc != nil {
		t.Error("HighestConfidence should return nil for empty results")
	}

	agg := Aggregate(results)
	if !agg.AllVerified || agg.ValidatorCount != 0 {
		t.Error("Aggregate failed for empty results")
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

// TEST_GAP: Type Coercion
func TestActionValidator_ConfidenceExtremes(t *testing.T) {
	results := []ValidationResult{
		{Confidence: math.NaN()},
		{Confidence: math.Inf(1)},
		{Confidence: math.Inf(-1)},
		{Confidence: 0.5},
	}
	hc := HighestConfidence(results)
	// highest confidence will pick the first one that is > current highest.
	// math.NaN() > x is always false. math.Inf(1) > x is true.
	if hc == nil {
		t.Fatal("Expected highest confidence")
	}
	if !math.IsInf(hc.Confidence, 1) {
		t.Error("Expected +Inf to be highest if present in slice")
	}

	// ToFacts clamping
	for _, c := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1.5, -0.5} {
		vr := ValidationResult{Verified: true, Confidence: c}
		facts := vr.ToFacts()
		confVal := facts[0].Args[3].(int64)
		if confVal < 0 || confVal > 100 {
			t.Errorf("Confidence %v clamped incorrectly to %d", c, confVal)
		}
	}
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
	for i := 0; i < 2000; i++ {
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
	details := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		details["key"+itoaValidator(i)] = "value"
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

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r.Register(&dummyValidator{priority: idx})
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Validate(context.Background(), req, res)
		}()
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
