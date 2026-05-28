// Package core provides the post-action validation infrastructure.
// Every action executed by VirtualStore is verified after execution
// to ensure it actually succeeded, not just returned without error.
package core

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// ValidationResult captures the outcome of post-action verification.
type ValidationResult struct {
	// ActionID identifies the action that was validated
	ActionID string

	// ActionType is the type of action that was validated
	ActionType ActionType

	// Verified indicates whether the action was successfully verified
	Verified bool

	// Confidence is a score from 0.0-1.0 indicating verification certainty
	// 1.0 = hash match, 0.8 = output scan, 0.5 = existence check only
	Confidence float64

	// Method describes how verification was performed
	// Values: "hash", "syntax", "existence", "content_check", "output_scan", "codedom_refresh"
	Method string

	// Error describes what went wrong if Verified is false
	Error string

	// Details contains method-specific information
	Details map[string]any

	// Duration is how long validation took
	Duration time.Duration

	// Timestamp when validation completed
	Timestamp time.Time
}

// ValidationMethod constants for consistent method reporting
const (
	ValidationMethodHash           = "hash"
	ValidationMethodSyntax         = "syntax"
	ValidationMethodExistence      = "existence"
	ValidationMethodContentCheck   = "content_check"
	ValidationMethodOutputScan     = "output_scan"
	ValidationMethodCodeDOMRefresh = "codedom_refresh"
	ValidationMethodSkipped        = "skipped"
)

// ActionValidator verifies that an action actually succeeded after execution.
// Validators are registered by action type and called after each action completes.
type ActionValidator interface {
	// CanValidate returns true if this validator handles the given action type
	CanValidate(actionType ActionType) bool

	// Validate checks that the action actually succeeded.
	// req contains the original action request, result contains what the action returned.
	// Returns a ValidationResult indicating whether the action was verified.
	Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult

	// Name returns a human-readable name for this validator
	Name() string

	// Priority returns the order in which this validator should run (lower = first)
	// Multiple validators can handle the same action type; they run in priority order
	Priority() int
}

// ValidatorRegistry holds all registered validators and orchestrates validation.
type ValidatorRegistry struct {
	mu         sync.RWMutex
	validators []ActionValidator
	// byType caches validators by action type for fast lookup
	byType map[ActionType][]ActionValidator
}

// NewValidatorRegistry creates a new empty validator registry.
func NewValidatorRegistry() *ValidatorRegistry {
	return &ValidatorRegistry{
		validators: make([]ActionValidator, 0),
		byType:     make(map[ActionType][]ActionValidator),
	}
}

// Register adds a validator to the registry.
// Validators are sorted by priority (lower priority runs first).
func (r *ValidatorRegistry) Register(v ActionValidator) {
	if v == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.validators = append(r.validators, v)

	// Sort by priority
	for i := len(r.validators) - 1; i > 0; i-- {
		if r.validators[i].Priority() < r.validators[i-1].Priority() {
			r.validators[i], r.validators[i-1] = r.validators[i-1], r.validators[i]
		}
	}

	// Invalidate cache
	r.byType = make(map[ActionType][]ActionValidator)
}

// getValidatorsForType returns validators that can handle the given action type.
// Results are cached for performance.
func (r *ValidatorRegistry) getValidatorsForType(actionType ActionType) []ActionValidator {
	r.mu.RLock()
	if cached, ok := r.byType[actionType]; ok {
		r.mu.RUnlock()
		return cached
	}
	r.mu.RUnlock()

	// Build list of applicable validators
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if cached, ok := r.byType[actionType]; ok {
		return cached
	}

	applicable := make([]ActionValidator, 0)
	for _, v := range r.validators {
		if v.CanValidate(actionType) {
			applicable = append(applicable, v)
		}
	}

	r.byType[actionType] = applicable
	return applicable
}

// Validate runs all applicable validators for the given action.
// Returns all validation results (one per validator that ran).
func (r *ValidatorRegistry) Validate(ctx context.Context, req ActionRequest, result ActionResult) []ValidationResult {
	validators := r.getValidatorsForType(req.Type)
	if len(validators) == 0 {
		// No validators for this action type - return a single "skipped" result
		return []ValidationResult{{
			ActionID:   req.ActionID,
			ActionType: req.Type,
			Verified:   true,
			Confidence: 0.0,
			Method:     ValidationMethodSkipped,
			Details:    map[string]any{"reason": "no validators registered for action type"},
			Timestamp:  time.Now(),
		}}
	}

	results := make([]ValidationResult, 0, len(validators))
	for _, v := range validators {
		select {
		case <-ctx.Done():
			// Context cancelled - return partial results with error
			results = append(results, ValidationResult{
				ActionID:   req.ActionID,
				ActionType: req.Type,
				Verified:   false,
				Error:      fmt.Sprintf("validation cancelled: %v", ctx.Err()),
				Timestamp:  time.Now(),
			})
			return results
		default:
		}

		valCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		start := time.Now()
		vr := v.Validate(valCtx, req, result)
		cancel()

		vr.Duration = time.Since(start)
		vr.Timestamp = time.Now()

		// Ensure ActionID and ActionType are set
		if vr.ActionID == "" {
			vr.ActionID = req.ActionID
		}
		if vr.ActionType == "" {
			vr.ActionType = req.Type
		}

		results = append(results, vr)

		// If a high-confidence validator fails, we can short-circuit
		if !vr.Verified && vr.Confidence >= 0.8 {
			break
		}
	}

	return results
}

// ValidateAll checks if all validation results passed.
func ValidateAll(results []ValidationResult) bool {
	for _, r := range results {
		if !r.Verified {
			return false
		}
	}
	return true
}

// FirstFailure returns the first failed validation result, or nil if all passed.
func FirstFailure(results []ValidationResult) *ValidationResult {
	for _, r := range results {
		if !r.Verified {
			return &r
		}
	}
	return nil
}

// HighestConfidence returns the validation result with the highest confidence score.
func HighestConfidence(results []ValidationResult) *ValidationResult {
	if len(results) == 0 {
		return nil
	}
	highest := &results[0]
	for i := 1; i < len(results); i++ {
		// If current highest is NaN, or the new one is larger, update highest
		if math.IsNaN(highest.Confidence) || results[i].Confidence > highest.Confidence {
			highest = &results[i]
		}
	}
	return highest
}

// AggregateResult combines multiple validation results into a summary.
type AggregateResult struct {
	// AllVerified is true if all validators passed
	AllVerified bool

	// HighestConfidence is the maximum confidence score across all results
	HighestConfidence float64

	// LowestConfidence is the minimum confidence score across all results
	LowestConfidence float64

	// FirstError is the first error encountered, if any
	FirstError string

	// ValidatorCount is how many validators ran
	ValidatorCount int

	// FailureCount is how many validators failed
	FailureCount int

	// Results contains all individual results
	Results []ValidationResult
}

// Aggregate combines multiple validation results into a summary.
func Aggregate(results []ValidationResult) AggregateResult {
	if len(results) == 0 {
		return AggregateResult{
			AllVerified:       true,
			HighestConfidence: 0.0,
			LowestConfidence:  0.0,
			ValidatorCount:    0,
			Results:           results,
		}
	}

	agg := AggregateResult{
		AllVerified:       true,
		HighestConfidence: 0.0,
		LowestConfidence:  1.0,
		ValidatorCount:    len(results),
		Results:           results,
	}

	for _, r := range results {
		if !r.Verified {
			agg.AllVerified = false
			agg.FailureCount++
			if agg.FirstError == "" {
				agg.FirstError = r.Error
			}
		}
		if r.Confidence > agg.HighestConfidence {
			agg.HighestConfidence = r.Confidence
		}
		if r.Confidence < agg.LowestConfidence {
			agg.LowestConfidence = r.Confidence
		}
	}

	return agg
}

// ToFacts converts validation results to Mangle facts for kernel assertion.
func (vr *ValidationResult) ToFacts() []Fact {
	facts := make([]Fact, 0, 2)

	confidence := vr.Confidence
	if math.IsNaN(confidence) || math.IsInf(confidence, 0) {
		confidence = 0.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	} else if confidence > 1.0 {
		confidence = 1.0
	}

	actionTypeStr := string(vr.ActionType)
	if len(actionTypeStr) > 256 {
		actionTypeStr = actionTypeStr[:256]
	}

	errorStr := vr.Error
	if len(errorStr) > 1024 {
		errorStr = errorStr[:1024]
	}

	// Schema (schemas_execution.mg) declares ActionType and Method as
	// /name and Reason as /name. The raw strings ("exec_cmd", "hash",
	// "hash mismatch") would otherwise be ingested as /string and the
	// row would fail schema validation. Coerce explicitly here.
	actionTypeAtom := MangleAtom("/" + sanitizeAtomTail(actionTypeStr))
	methodAtom := MangleAtom("/" + sanitizeAtomTail(vr.Method))

	if vr.Verified {
		// action_verified(ActionID, ActionType, Method, Confidence, Timestamp)
		facts = append(facts, Fact{
			Predicate: "action_verified",
			Args: []any{
				vr.ActionID,
				actionTypeAtom,
				methodAtom,
				int64(confidence * 100), // Scale 0.0-1.0 → 0-100 integer per schema
				vr.Timestamp.Unix(),
			},
		})
	} else {
		// action_validation_failed(ActionID, ActionType, Reason, Details, Timestamp).
		// The error text is free-form so we derive a stable /name category
		// for Reason and stash the full text in Details (/string).
		reasonAtom := MangleAtom("/" + categorizeValidationError(errorStr))
		detailsStr := errorStr
		if vr.Details != nil {
			// Simple serialization for Mangle
			for k, v := range vr.Details {
				detailsStr += fmt.Sprintf("; %s=%v", k, v)
			}
		}
		if len(detailsStr) > 1024 {
			detailsStr = detailsStr[:1024]
		}
		facts = append(facts, Fact{
			Predicate: "action_validation_failed",
			Args: []any{
				vr.ActionID,
				actionTypeAtom,
				reasonAtom,
				detailsStr,
				vr.Timestamp.Unix(),
			},
		})
	}

	// Always emit the method used
	facts = append(facts, Fact{
		Predicate: "validation_method_used",
		Args: []any{
			vr.ActionID,
			vr.Method,
		},
	})

	return facts
}

// sanitizeAtomTail produces a safe single-segment Mangle name-constant
// tail by stripping leading slashes, replacing path separators and
// whitespace with underscores, and lowercasing the rest. Used when
// schemas require /name slots but the source is a free-form ActionType
// or method string.
func sanitizeAtomTail(s string) string {
	if s == "" {
		return "unknown"
	}
	s = strings.TrimLeft(s, "/")
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '_', r == '-':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r == ' ', r == '\t', r == '\n', r == '\r', r == '/', r == '.':
			return '_'
		default:
			return '_'
		}
	}, s)
	if mapped == "" {
		return "unknown"
	}
	return mapped
}

// categorizeValidationError extracts a /name-friendly category from a
// free-form validation error message (e.g. "hash mismatch" → "hash",
// "regex: invalid pattern" → "regex"). Falls back to "generic" for
// empty or non-classifiable errors. Keeps the /name slot bounded to a
// small enum so policy rules can branch on it without exploding the
// stratum count.
func categorizeValidationError(msg string) string {
	if msg == "" {
		return "generic"
	}
	first := msg
	if i := strings.IndexAny(msg, ": \t"); i > 0 {
		first = msg[:i]
	}
	return sanitizeAtomTail(first)
}
