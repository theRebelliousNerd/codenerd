// Package feedback provides a validation and retry loop for LLM-generated Mangle code.
// It catches common AI errors before expensive Mangle compilation and provides
// structured feedback for retry attempts.
package feedback

import (
	"codenerd/internal/config"
	"sync"
	"time"
)

// ErrorCategory classifies validation errors for targeted feedback.
type ErrorCategory int

const (
	// CategoryParse indicates a Mangle parser failure.
	CategoryParse ErrorCategory = iota
	// CategoryAtomString indicates "string" should be /atom.
	CategoryAtomString
	// CategoryAggregation indicates wrong aggregation syntax (missing |> do fn:).
	CategoryAggregation
	// CategoryMissingPeriod indicates rule lacks terminating period.
	CategoryMissingPeriod
	// CategoryUnboundNegation indicates variable only appears in negation.
	CategoryUnboundNegation
	// CategoryUndeclaredPredicate indicates unknown predicate.
	CategoryUndeclaredPredicate
	// CategoryStratification indicates cyclic negation dependency.
	CategoryStratification
	// CategoryTypeMismatch indicates type inconsistency (e.g., float vs int).
	CategoryTypeMismatch
	// CategoryPrologNegation indicates \+ instead of ! for negation.
	CategoryPrologNegation
	// CategorySyntax indicates general syntax error.
	CategorySyntax
)

// String returns a human-readable name for the error category.
func (c ErrorCategory) String() string {
	switch c {
	case CategoryParse:
		return "parse_error"
	case CategoryAtomString:
		return "atom_string_confusion"
	case CategoryAggregation:
		return "aggregation_syntax"
	case CategoryMissingPeriod:
		return "missing_period"
	case CategoryUnboundNegation:
		return "unbound_negation"
	case CategoryUndeclaredPredicate:
		return "undeclared_predicate"
	case CategoryStratification:
		return "stratification_violation"
	case CategoryTypeMismatch:
		return "type_mismatch"
	case CategoryPrologNegation:
		return "prolog_negation"
	case CategorySyntax:
		return "syntax_error"
	default:
		return "unknown"
	}
}

// IsAutoRepairable returns true if this error type can be auto-fixed by the Sanitizer.
func (c ErrorCategory) IsAutoRepairable() bool {
	switch c {
	case CategoryAtomString, CategoryAggregation, CategoryMissingPeriod, CategoryPrologNegation:
		return true
	case CategoryUnboundNegation:
		return true // Partial - Sanitizer can inject generators
	default:
		return false
	}
}

// ValidationError represents a detected error with context for feedback.
type ValidationError struct {
	Category   ErrorCategory
	Line       int
	Column     int
	Message    string // Human-readable description
	Wrong      string // The problematic code snippet
	Correct    string // Suggested fix (if known)
	Suggestion string // Advice on how to fix
	AutoFixed  bool   // True if Sanitizer repaired this
}

// ValidationResult holds the outcome of a validation attempt.
type ValidationResult struct {
	Valid      bool              // True if code passed all validation
	Errors     []ValidationError // Blocking errors
	Warnings   []ValidationError // Non-blocking issues
	Sanitized  string            // Auto-repaired code (may still have errors)
	Original   string            // Original input code
	AttemptNum int               // Which retry attempt this was
}

// HasErrors returns true if there are any blocking errors.
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// RetryConfig configures the feedback loop behavior.
type RetryConfig struct {
	// MaxRetries is the per-rule retry limit (default: 3).
	MaxRetries int
	// SessionBudget is the total retries allowed per session (default: 20).
	SessionBudget int
	// EnableAutoRepair enables Sanitizer for auto-fixes (default: true).
	EnableAutoRepair bool
	// InjectPredicates includes available predicates in feedback (default: true).
	InjectPredicates bool
	// SimplifyOnLastRetry suggests simplification on final attempt (default: true).
	SimplifyOnLastRetry bool
	// PerAttemptTimeout is the max time per LLM call (default: 60s).
	PerAttemptTimeout time.Duration
	// TotalTimeout is the max total time for all retries (default: 180s).
	TotalTimeout time.Duration
}

// DefaultConfig returns the default retry configuration.
func DefaultConfig() RetryConfig {
	timeouts := config.GetLLMTimeouts()
	perAttempt := timeouts.PerCallTimeout
	if perAttempt <= 0 {
		perAttempt = 60 * time.Second
	}
	maxRetries := 3
	totalTimeout := perAttempt * time.Duration(maxRetries)
	return RetryConfig{
		MaxRetries:          maxRetries,
		SessionBudget:       20,
		EnableAutoRepair:    true,
		InjectPredicates:    true,
		SimplifyOnLastRetry: true,
		PerAttemptTimeout:   perAttempt,
		TotalTimeout:        totalTimeout,
	}
}

// ValidationBudget tracks retry usage across a session.
type ValidationBudget struct {
	mu            sync.Mutex
	maxPerRule    int
	sessionBudget int
	sessionUsed   int
	ruleAttempts  map[string]int // Tracks attempts per rule hash
}

// NewValidationBudget creates a new budget tracker.
func NewValidationBudget(config RetryConfig) *ValidationBudget {
	return &ValidationBudget{
		maxPerRule:    config.MaxRetries,
		sessionBudget: config.SessionBudget,
		ruleAttempts:  make(map[string]int),
	}
}

// CanRetry checks if another retry is allowed for the given rule.
func (b *ValidationBudget) CanRetry(ruleHash string) (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.sessionUsed >= b.sessionBudget {
		return false, "session validation budget exhausted"
	}

	if b.ruleAttempts[ruleHash] >= b.maxPerRule {
		return false, "max retries exceeded for this rule"
	}

	return true, ""
}

// RecordAttempt records a retry attempt.
func (b *ValidationBudget) RecordAttempt(ruleHash string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.sessionUsed++
	b.ruleAttempts[ruleHash]++
}

// GetAttemptCount returns the current attempt count for a rule.
func (b *ValidationBudget) GetAttemptCount(ruleHash string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ruleAttempts[ruleHash]
}

// Reset clears the budget (typically at session start).
func (b *ValidationBudget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.sessionUsed = 0
	b.ruleAttempts = make(map[string]int)
}

// Stats returns current budget statistics.
func (b *ValidationBudget) Stats() (sessionUsed, sessionBudget int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sessionUsed, b.sessionBudget
}

// IsSessionExhausted returns true if the session-wide budget has been exhausted.
// This is useful for early-exit checks before attempting validation.
func (b *ValidationBudget) IsSessionExhausted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sessionUsed >= b.sessionBudget
}

// ErrorPattern defines a regex-based error detection pattern.
type ErrorPattern struct {
	Category       ErrorCategory
	Pattern        string // Regex pattern
	Message        string // Error message template
	WrongExample   string // Example of wrong code
	CorrectFix     string // Example of correct code
	Suggestion     string // How to fix
	AutoRepairable bool   // Can Sanitizer fix this?
}

// FeedbackContext holds context for building feedback prompts.
type FeedbackContext struct {
	OriginalPrompt      string
	OriginalRule        string
	Errors              []ValidationError
	AttemptNumber       int
	MaxAttempts         int
	AvailablePredicates []string
	ValidExamples       []string
	OutputProtocol      OutputProtocol
}

type OutputProtocol string

const (
	OutputProtocolRule  OutputProtocol = "mangle_rule"
	OutputProtocolSynth OutputProtocol = "mangle_synth_json"
)

type SynthMode int

const (
	SynthModeOff SynthMode = iota
	SynthModePrefer
	SynthModeRequire
)
