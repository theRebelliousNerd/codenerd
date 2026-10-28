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

	"codenerd/internal/types"
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

	// Validator names the validator that produced this result. The registry
	// sets it.
	Validator string

	// State is what this reading established: pass, fail, skipped (the
	// validator holds no opinion) or unavailable (it could not complete).
	// The registry classifies every result it returns (classifyValidation);
	// a validator may set it itself. Verified alone conflated a deferral
	// ("Verified: true, Confidence: 0") with a pass, and a canceled
	// validator with a failed one.
	State ValidationState
}

// ValidationState is one validator's reading of one action.
type ValidationState string

const (
	ValidationPass        ValidationState = "pass"
	ValidationFail        ValidationState = "fail"
	ValidationSkipped     ValidationState = "skipped"
	ValidationUnavailable ValidationState = "unavailable"
)

// DefinitiveValidationConfidence is the confidence at or above which a failed
// validation is definitive: the result did not do what it claimed. Below it a
// failure is an objection the result stands over. The interactive bridge and
// RouteAction both read it (SummarizeValidation); each used to carry its own
// 0.8.
const DefinitiveValidationConfidence = 0.8

// ValidationMethod constants for consistent method reporting
const (
	ValidationMethodHash           = "hash"
	ValidationMethodSyntax         = "syntax"
	ValidationMethodExistence      = "existence"
	ValidationMethodContentCheck   = "content_check"
	ValidationMethodOutputScan     = "output_scan"
	ValidationMethodCodeDOMRefresh = "codedom_refresh"
	ValidationMethodSkipped        = "skipped"
	ValidationMethodUnavailable    = "unavailable"
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

// validatorTypeAlias maps delegating action types onto the type whose
// validators verify the work. Campaign file/test actions delegate to the file
// and test handlers, so without this their results take the "skipped" branch
// and real mutations go unverified. One map, not per-validator cases: the
// registry comment explains why a duplicated safety list drifts.
//
// Deliberately NOT mapping CampaignDocument: it only writes when content is
// present, and running existence validators over signal-mode requests would
// fail actions that did exactly what they claimed.
var validatorTypeAlias = map[ActionType]ActionType{
	ActionCampaignCreateFile: ActionWriteFile,
	ActionCampaignModifyFile: ActionEditFile,
	ActionCampaignWriteTest:  ActionWriteFile,
	ActionCampaignRunTest:    ActionRunTests,
}

// Validators returns a snapshot of the registered validators for inspection
// (e.g. priming stateful validators before execution).
func (r *ValidatorRegistry) Validators() []ActionValidator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ActionValidator, len(r.validators))
	copy(out, r.validators)
	return out
}

// getValidatorsForType returns validators that can handle the given action type.
// Results are cached for performance.
func (r *ValidatorRegistry) getValidatorsForType(actionType ActionType) []ActionValidator {
	if canon, ok := validatorTypeAlias[actionType]; ok {
		actionType = canon
	}
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
			State:      ValidationSkipped,
			Details:    map[string]any{"reason": "no validators registered for action type"},
			Timestamp:  time.Now(),
		}}
	}

	results := make([]ValidationResult, 0, len(validators))
	for _, v := range validators {
		select {
		case <-ctx.Done():
			// The caller's context ended: this validator, and every one
			// after it, could not run. That is unavailable, not failed --
			// nothing about the result was measured.
			results = append(results, ValidationResult{
				ActionID:   req.ActionID,
				ActionType: req.Type,
				Validator:  v.Name(),
				Method:     ValidationMethodUnavailable,
				State:      ValidationUnavailable,
				Error:      fmt.Sprintf("validation cancelled: %v", ctx.Err()),
				Timestamp:  time.Now(),
			})
			return results
		default:
		}

		valCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		start := time.Now()
		vr := v.Validate(valCtx, req, result)
		timedOut := valCtx.Err() != nil
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
		if vr.Validator == "" {
			vr.Validator = v.Name()
		}
		if vr.State == "" {
			vr.State = classifyValidation(vr, timedOut)
		}

		results = append(results, vr)

		// A definitive failure ends the pass: the result is rejected, and
		// what the remaining validators would say does not change that.
		if vr.State == ValidationFail && vr.Confidence >= DefinitiveValidationConfidence {
			break
		}
	}

	return results
}

// classifyValidation is the state of a reading its validator did not state.
// A failure whose own deadline expired measured nothing; a "verified" result
// with no confidence is the deferral convention ("Verified: true, Confidence:
// 0, defer to other validators"), not a pass.
func classifyValidation(vr ValidationResult, timedOut bool) ValidationState {
	switch {
	case !vr.Verified && timedOut:
		return ValidationUnavailable
	case !vr.Verified:
		return ValidationFail
	case !(vr.Confidence > 0) || vr.Method == ValidationMethodSkipped || strings.HasSuffix(vr.Method, "_skipped"):
		// NaN and non-positive confidence alike affirm nothing.
		return ValidationSkipped
	default:
		return ValidationPass
	}
}

// stateOf is a result's state, classifying one built outside the registry.
func stateOf(vr ValidationResult) ValidationState {
	if vr.State != "" {
		return vr.State
	}
	return classifyValidation(vr, false)
}

// SummarizeValidation is what a validation pass established, typed: rejected
// when ANY reading failed definitively (not only the first failure -- a
// low-confidence objection ahead of a definitive one used to hide it),
// incomplete when a validator could not complete, verified when one affirmed
// the result, and unvalidated when none held an opinion. A failure below
// DefinitiveValidationConfidence is an objection the result stands over: it
// stays visible in the observations and decides nothing.
func SummarizeValidation(resource string, results []ValidationResult) types.ToolValidation {
	out := types.ToolValidation{State: types.ToolValidationUnvalidated}
	var rejected, incomplete, passed bool
	var rejectReason, incompleteReason string
	for _, r := range results {
		state := stateOf(r)
		out.Observations = append(out.Observations, types.ValidatorObservation{
			Resource:   resource,
			Validator:  r.Validator,
			Method:     r.Method,
			State:      string(state),
			Confidence: r.Confidence,
			Detail:     r.Error,
		})
		switch state {
		case ValidationFail:
			if r.Confidence >= DefinitiveValidationConfidence && !rejected {
				rejected, rejectReason = true, r.Error
			}
		case ValidationUnavailable:
			if !incomplete {
				incomplete, incompleteReason = true, r.Error
			}
		case ValidationPass:
			passed = true
		}
	}
	switch {
	case rejected:
		out.State, out.Reason = types.ToolValidationRejected, rejectReason
	case incomplete:
		out.State, out.Reason = types.ToolValidationIncomplete, incompleteReason
	case passed:
		out.State = types.ToolValidationVerified
	}
	return out
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

	switch stateOf(*vr) {
	case ValidationSkipped:
		// action_validation_skipped(ActionID, ActionType, Method, Timestamp):
		// the validator held no opinion. It used to be action_verified at
		// confidence 0, an affirmative row for a reading that affirmed
		// nothing.
		facts = append(facts, Fact{
			Predicate: "action_validation_skipped",
			Args:      []any{vr.ActionID, actionTypeAtom, methodAtom, vr.Timestamp.Unix()},
		})
	case ValidationUnavailable:
		// action_validation_unavailable(ActionID, ActionType, Details,
		// Timestamp): the validator could not complete. It used to be
		// action_validation_failed, which sent a canceled check down the
		// self-healing arms as a broken artifact.
		facts = append(facts, Fact{
			Predicate: "action_validation_unavailable",
			Args:      []any{vr.ActionID, actionTypeAtom, errorStr, vr.Timestamp.Unix()},
		})
	case ValidationPass:
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
	default:
		// action_validation_failed(ActionID, ActionType, Reason, Details, Timestamp).
		// The error text is free-form so we derive a stable /name category
		// for Reason and stash the full text in Details (/string).
		reasonAtom := MangleAtom("/" + categorizeValidationError(errorStr))
		var detailsStr string
		if len(vr.Details) > 0 {
			var sb strings.Builder
			// Estimate roughly: length of error + average 32 bytes per map entry
			sb.Grow(len(errorStr) + len(vr.Details)*32)
			sb.WriteString(errorStr)
			// Simple serialization for Mangle
			for k, v := range vr.Details {
				fmt.Fprintf(&sb, "; %s=%v", k, v)
			}
			detailsStr = sb.String()
		} else {
			detailsStr = errorStr
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

	// Always emit the method used. Method is a /name in the schema; the raw
	// string failed schema validation, so this row never landed.
	facts = append(facts, Fact{
		Predicate: "validation_method_used",
		Args: []any{
			vr.ActionID,
			methodAtom,
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

// validationReasonPrefixes maps the validation error messages the validators
// actually emit onto the reason atoms policy/validation.mg branches on. It is a
// prefix table because validator_paranoid.go qualifies the same failure with
// "(first read)" / "(second read)".
//
// It exists because the first-token fallback below is not a vocabulary. On
// "content hash mismatch" it yields /content, and on "cannot read back file"
// /cannot - categories no rule author would ever write, and ones that change
// silently the moment somebody rewords a message. Every entry here has a
// matching rule in policy/validation.mg; adding a message without adding its
// rule just routes the failure to the /escalate arm, which is the safe default.
var validationReasonPrefixes = []struct{ prefix, reason string }{
	{"content hash mismatch", "hash_mismatch"},
	{"file hash unchanged after edit", "edit_not_applied"},
	{"cannot read back file", "read_back_failed"},
	{"syntax validation failed", "syntax_error"},
	{"Go syntax error after CodeDOM edit", "syntax_error"},
	{"target element no longer exists after edit", "element_lost"},
}

// categorizeValidationError extracts a /name-friendly category from a
// free-form validation error message. Known validator messages map to the
// stable vocabulary in validationReasonPrefixes; anything else falls back to
// the leading token (e.g. "regex: invalid pattern" → "regex"), and an empty
// error to "generic". Keeps the /name slot bounded to a small enum so policy
// rules can branch on it without exploding the stratum count.
func categorizeValidationError(msg string) string {
	if msg == "" {
		return "generic"
	}
	for _, m := range validationReasonPrefixes {
		if strings.HasPrefix(msg, m.prefix) {
			return m.reason
		}
	}
	first := msg
	if i := strings.IndexAny(msg, ": \t"); i > 0 {
		first = msg[:i]
	}
	return sanitizeAtomTail(first)
}
