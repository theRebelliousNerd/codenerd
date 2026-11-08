package core

import (
	"codenerd/internal/types"
	"context"
	"math"
	"strconv"
	"strings"
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
	// Bounded by the fact-text limit, with the marker saying how much went.
	detailStr := facts[0].Args[3].(string)
	if len(detailStr) > types.MaxFactTextBytes || !strings.Contains(detailStr, "[truncated") {
		t.Errorf("details not bounded with a marker: %d bytes", len(detailStr))
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

func TestValidationResult_ToFacts_ClampsConfidence(t *testing.T) {
	for _, c := range []float64{math.Inf(1), 1.5} {
		vr := ValidationResult{Verified: true, Confidence: c}
		facts := vr.ToFacts()
		confVal := facts[0].Args[3].(int64)
		if confVal < 0 || confVal > 100 {
			t.Errorf("Confidence %v clamped incorrectly to %d", c, confVal)
		}
	}
	// A verified reading with no positive confidence affirms nothing: it
	// is a deferral, not a pass at 0.
	for _, c := range []float64{math.NaN(), math.Inf(-1), -0.5} {
		vr := ValidationResult{Verified: true, Confidence: c}
		if facts := vr.ToFacts(); facts[0].Predicate != "action_validation_skipped" {
			t.Errorf("Confidence %v asserted %s, want action_validation_skipped", c, facts[0].Predicate)
		}
	}
}

func TestSanitizeAtomTail_PinBehavior(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty is unknown", input: "", want: "unknown"},
		{name: "slash only is unknown", input: "/", want: "unknown"},
		{name: "slashes stripped then mapped", input: "///a/b", want: "a_b"},
		{name: "lowercases uppercase", input: "ABC", want: "abc"},
		{name: "keeps dash underscore digits", input: "hello-World_123", want: "hello-world_123"},
		{name: "separators become underscores", input: "a/b.c d", want: "a_b_c_d"},
		{name: "symbols become underscores", input: "!!", want: "__"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeAtomTail(tc.input); got != tc.want {
				t.Errorf("sanitizeAtomTail(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCategorizeValidationError_PinVocabulary(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty is generic", input: "", want: "generic"},
		{name: "hash mismatch", input: "content hash mismatch: got abc want def", want: "hash_mismatch"},
		{name: "edit not applied", input: "file hash unchanged after edit", want: "edit_not_applied"},
		{name: "read back failed qualified", input: "cannot read back file (first read)", want: "read_back_failed"},
		{name: "syntax error", input: "syntax validation failed: bad token", want: "syntax_error"},
		{name: "codedom syntax error", input: "Go syntax error after CodeDOM edit: oops", want: "syntax_error"},
		{name: "element lost", input: "target element no longer exists after edit", want: "element_lost"},
		{name: "fallback leading token", input: "regex: invalid pattern", want: "regex"},
		{name: "fallback sanitizes token", input: "BOOM! failed", want: "boom_"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := categorizeValidationError(tc.input); got != tc.want {
				t.Errorf("categorizeValidationError(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestClassifyValidation_PinStates(t *testing.T) {
	cases := []struct {
		name     string
		vr       ValidationResult
		timedOut bool
		want     ValidationState
	}{
		{name: "timed out failure is unavailable", vr: ValidationResult{Verified: false}, timedOut: true, want: ValidationUnavailable},
		{name: "plain failure is fail", vr: ValidationResult{Verified: false}, timedOut: false, want: ValidationFail},
		{name: "zero confidence deferral is skipped", vr: ValidationResult{Verified: true, Confidence: 0, Method: ValidationMethodHash}, timedOut: false, want: ValidationSkipped},
		{name: "NaN confidence affirms nothing", vr: ValidationResult{Verified: true, Confidence: math.NaN(), Method: ValidationMethodHash}, timedOut: false, want: ValidationSkipped},
		{name: "skipped method is skipped", vr: ValidationResult{Verified: true, Confidence: 0.9, Method: ValidationMethodSkipped}, timedOut: false, want: ValidationSkipped},
		{name: "suffixed skipped method is skipped", vr: ValidationResult{Verified: true, Confidence: 0.9, Method: "hash_skipped"}, timedOut: false, want: ValidationSkipped},
		{name: "verified result is pass", vr: ValidationResult{Verified: true, Confidence: 0.9, Method: ValidationMethodHash}, timedOut: false, want: ValidationPass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyValidation(tc.vr, tc.timedOut); got != tc.want {
				t.Errorf("classifyValidation(%+v, %v) = %q, want %q", tc.vr, tc.timedOut, got, tc.want)
			}
		})
	}
}

func TestStateOf_PinExplicit(t *testing.T) {
	explicit := ValidationResult{State: ValidationPass, Verified: false}
	if got := stateOf(explicit); got != ValidationPass {
		t.Errorf("stateOf with explicit state = %q, want %q", got, ValidationPass)
	}
	derivedFail := ValidationResult{Verified: false}
	if got := stateOf(derivedFail); got != ValidationFail {
		t.Errorf("stateOf derived = %q, want %q", got, ValidationFail)
	}
	derivedPass := ValidationResult{Verified: true, Confidence: 1, Method: ValidationMethodHash}
	if got := stateOf(derivedPass); got != ValidationPass {
		t.Errorf("stateOf derived = %q, want %q", got, ValidationPass)
	}
}

func TestSummarizeValidation_PinPrecedence(t *testing.T) {
	t.Run("empty is unvalidated", func(t *testing.T) {
		got := SummarizeValidation("res", nil)
		if got.State != types.ToolValidationUnvalidated {
			t.Errorf("empty State = %q, want %q", got.State, types.ToolValidationUnvalidated)
		}
		if len(got.Observations) != 0 {
			t.Errorf("empty Observations = %d, want 0", len(got.Observations))
		}
	})
	t.Run("low confidence objection decides nothing", func(t *testing.T) {
		got := SummarizeValidation("res", []ValidationResult{
			{Validator: "v", Method: ValidationMethodHash, Verified: false, Confidence: 0.2, Error: "nope"},
		})
		if got.State != types.ToolValidationUnvalidated {
			t.Errorf("low-confidence State = %q, want %q", got.State, types.ToolValidationUnvalidated)
		}
		if len(got.Observations) != 1 {
			t.Fatalf("Observations = %d, want 1", len(got.Observations))
		}
		if got.Observations[0].State != string(ValidationFail) {
			t.Errorf("observation State = %q, want %q", got.Observations[0].State, ValidationFail)
		}
	})
	t.Run("definitive failure rejects even after low objection", func(t *testing.T) {
		got := SummarizeValidation("res", []ValidationResult{
			{Validator: "a", Method: ValidationMethodHash, Verified: false, Confidence: 0.1, Error: "weak"},
			{Validator: "b", Method: ValidationMethodHash, Verified: false, Confidence: DefinitiveValidationConfidence, Error: "strong"},
		})
		if got.State != types.ToolValidationRejected {
			t.Errorf("State = %q, want %q", got.State, types.ToolValidationRejected)
		}
		if got.Reason != "strong" {
			t.Errorf("Reason = %q, want %q", got.Reason, "strong")
		}
	})
	t.Run("unavailable is incomplete", func(t *testing.T) {
		got := SummarizeValidation("res", []ValidationResult{
			{Validator: "v", Method: ValidationMethodHash, Verified: false, Confidence: 0, Error: "cancelled", State: ValidationUnavailable},
		})
		if got.State != types.ToolValidationIncomplete {
			t.Errorf("State = %q, want %q", got.State, types.ToolValidationIncomplete)
		}
	})
	t.Run("verified pass", func(t *testing.T) {
		got := SummarizeValidation("res", []ValidationResult{
			{Validator: "v", Method: ValidationMethodHash, Verified: true, Confidence: 1, Error: ""},
		})
		if got.State != types.ToolValidationVerified {
			t.Errorf("State = %q, want %q", got.State, types.ToolValidationVerified)
		}
	})
	t.Run("rejected outranks incomplete and verified", func(t *testing.T) {
		got := SummarizeValidation("res", []ValidationResult{
			{Validator: "v1", Method: ValidationMethodHash, Verified: true, Confidence: 1},
			{Validator: "v2", Method: ValidationMethodHash, Verified: false, Confidence: 0, Error: "stalled", State: ValidationUnavailable},
			{Validator: "v3", Method: ValidationMethodHash, Verified: false, Confidence: 1, Error: "broken"},
		})
		if got.State != types.ToolValidationRejected {
			t.Errorf("State = %q, want %q", got.State, types.ToolValidationRejected)
		}
	})
}
