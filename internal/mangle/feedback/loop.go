package feedback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/mangle/transpiler"
)

// LLMClient is the interface for making LLM calls during retry.
type LLMClient interface {
	// Complete sends a prompt and returns the response.
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// TracingLLMClient extends LLMClient with context-setting for trace attribution.
// If the LLMClient implements this interface, the feedback loop will set context
// for proper logging attribution of its LLM calls.
type TracingLLMClient interface {
	LLMClient
	SetShardContext(shardID, shardType, shardCategory, sessionID, taskContext string)
	ClearShardContext()
}

// RuleValidator validates Mangle rules via compilation.
type RuleValidator interface {
	// HotLoadRule attempts to load a rule into the kernel sandbox.
	// Returns nil if valid, error otherwise.
	HotLoadRule(rule string) error
	// ValidateLearnedRule validates that a rule only uses declared predicates.
	// Returns nil if valid, error otherwise.
	ValidateLearnedRule(rule string) error
	// GetDeclaredPredicates returns all declared predicate signatures.
	GetDeclaredPredicates() []string
}

// PredicateSelectorInterface allows JIT-style context-aware predicate selection.
type PredicateSelectorInterface interface {
	// SelectForContext returns predicates relevant to the given context.
	SelectForContext(shardType, intentVerb, domain string) ([]string, error)
}

// FeedbackLoop orchestrates the validate-retry cycle for LLM-generated Mangle.
type FeedbackLoop struct {
	config            RetryConfig
	preValidator      *PreValidator
	errorClassifier   *ErrorClassifier
	promptBuilder     *PromptBuilder
	sanitizer         *transpiler.Sanitizer
	budget            *ValidationBudget
	predicateSelector PredicateSelectorInterface // Optional: JIT-style selector
}

// NewFeedbackLoop creates a new feedback loop with the given configuration.
func NewFeedbackLoop(config RetryConfig) *FeedbackLoop {
	return &FeedbackLoop{
		config:          config,
		preValidator:    NewPreValidator(),
		errorClassifier: NewErrorClassifier(),
		promptBuilder:   NewPromptBuilder(),
		sanitizer:       transpiler.NewSanitizer(),
		budget:          NewValidationBudget(config),
	}
}

// SetPredicateSelector sets a JIT-style predicate selector for context-aware selection.
// When set, predicates are selected based on shard type, intent, and domain instead of
// using the full list from the validator.
func (fl *FeedbackLoop) SetPredicateSelector(selector PredicateSelectorInterface) {
	fl.predicateSelector = selector
}

// GenerateResult holds the outcome of a generation attempt.
type GenerateResult struct {
	Rule      string            // The validated rule (empty if failed)
	Valid     bool              // True if rule passed validation
	Attempts  int               // Number of attempts made
	Errors    []ValidationError // Errors from final attempt
	AutoFixed bool              // True if Sanitizer made fixes
}

// GenerateAndValidate generates a Mangle rule with automatic validation and retry.
// This is the main entry point for rule generation.
func (fl *FeedbackLoop) GenerateAndValidate(
	ctx context.Context,
	llmClient LLMClient,
	validator RuleValidator,
	systemPrompt string,
	userPrompt string,
	domain string,
) (*GenerateResult, error) {
	result := &GenerateResult{}

	// Apply total timeout if context has no deadline
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && fl.config.TotalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, fl.config.TotalTimeout)
		defer cancel()
	}

	// Set system context for trace attribution if the client supports it
	if tracingClient, ok := llmClient.(TracingLLMClient); ok {
		sessionID := fmt.Sprintf("feedback-%d", time.Now().UnixNano())
		tracingClient.SetShardContext(sessionID, "feedback_loop", "system", sessionID, "mangle-validation")
		defer tracingClient.ClearShardContext()
	}

	// Get available predicates for feedback - use JIT selector if available
	var predicates []string
	if fl.predicateSelector != nil {
		// JIT-style: select context-relevant predicates (~50-100 instead of 799)
		if selected, err := fl.predicateSelector.SelectForContext("", "", domain); err == nil {
			predicates = selected
			logging.Get(logging.CategoryKernel).Debug("FeedbackLoop: JIT selected %d predicates for domain %q", len(predicates), domain)
		} else {
			logging.Get(logging.CategoryKernel).Warn("FeedbackLoop: JIT selector failed, falling back to full list: %v", err)
			predicates = validator.GetDeclaredPredicates()
		}
	} else {
		predicates = validator.GetDeclaredPredicates()
	}

	// Add syntax guidance to initial prompt
	enhancedPrompt := userPrompt + "\n" + fl.promptBuilder.BuildInitialPromptAdditions(predicates)

	// Hash for budget tracking
	promptHash := hashPrompt(userPrompt)

	var lastRule string
	var lastErrors []ValidationError

	for attempt := 1; attempt <= fl.config.MaxRetries; attempt++ {
		result.Attempts = attempt

		// Check context before each attempt
		if ctx.Err() != nil {
			logging.Get(logging.CategoryKernel).Warn("FeedbackLoop: deadline exhausted before attempt %d", attempt)
			result.Errors = lastErrors
			return result, fmt.Errorf("deadline exhausted before attempt %d: %w", attempt, ctx.Err())
		}

		// Check budget
		canRetry, reason := fl.budget.CanRetry(promptHash)
		if !canRetry {
			logging.Get(logging.CategoryKernel).Warn("FeedbackLoop: budget exhausted: %s", reason)
			result.Errors = lastErrors
			return result, fmt.Errorf("validation budget exhausted: %s", reason)
		}
		fl.budget.RecordAttempt(promptHash)

		logging.KernelDebug("FeedbackLoop: attempt %d/%d", attempt, fl.config.MaxRetries)

		// Build prompt (with feedback on retry attempts)
		currentPrompt := enhancedPrompt
		if attempt > 1 && len(lastErrors) > 0 {
			feedbackCtx := FeedbackContext{
				OriginalPrompt:      userPrompt,
				OriginalRule:        lastRule,
				Errors:              lastErrors,
				AttemptNumber:       attempt,
				MaxAttempts:         fl.config.MaxRetries,
				AvailablePredicates: predicates,
				ValidExamples:       ValidRuleExamples(domain),
			}
			currentPrompt = userPrompt + fl.promptBuilder.BuildFeedbackPrompt(feedbackCtx)
		}

		// Per-attempt timeout
		attemptTimeout := fl.config.PerAttemptTimeout
		if attemptTimeout == 0 {
			attemptTimeout = 60 * time.Second
		}
		attemptCtx, attemptCancel := context.WithTimeout(ctx, attemptTimeout)

		// Call LLM with per-attempt timeout
		response, err := llmClient.Complete(attemptCtx, systemPrompt, currentPrompt)
		attemptCancel() // Cancel immediately after call completes

		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				logging.Get(logging.CategoryKernel).Warn("FeedbackLoop: attempt %d/%d timed out", attempt, fl.config.MaxRetries)
				lastErrors = []ValidationError{{
					Category: CategoryParse,
					Message:  fmt.Sprintf("LLM call timed out after %v", attemptTimeout),
				}}
				continue // Try next attempt
			}
			logging.Get(logging.CategoryKernel).Error("FeedbackLoop: LLM call failed: %v", err)
			return result, fmt.Errorf("LLM call failed: %w", err)
		}

		// Extract rule from response
		rule := ExtractRuleFromResponse(response)
		if rule == "" {
			lastErrors = []ValidationError{{
				Category: CategoryParse,
				Message:  "LLM returned empty or unparseable response",
			}}
			lastRule = response
			logging.KernelDebug("FeedbackLoop: empty rule extracted from response")
			continue
		}
		lastRule = rule

		// Phase 1: Pre-validation (fast regex checks)
		preErrors := fl.preValidator.Validate(rule)
		if len(preErrors) > 0 {
			logging.KernelDebug("FeedbackLoop: pre-validation found %d issues", len(preErrors))
		}

		// Phase 2: Auto-repair via Sanitizer (if enabled)
		sanitizedRule := rule
		if fl.config.EnableAutoRepair {
			// Quick fixes first
			sanitizedRule = fl.preValidator.QuickFix(rule)

			// Full sanitization
			fullSanitized, sanitizeErr := fl.sanitizer.Sanitize(sanitizedRule)
			if sanitizeErr == nil {
				sanitizedRule = fullSanitized
				result.AutoFixed = sanitizedRule != rule
				if result.AutoFixed {
					logging.KernelDebug("FeedbackLoop: Sanitizer auto-fixed rule")
				}
			} else {
				logging.KernelDebug("FeedbackLoop: Sanitizer failed: %v", sanitizeErr)
			}
		}

		// Phase 3: Sandbox compilation (HotLoadRule)
		compileErr := validator.HotLoadRule(sanitizedRule)
		if compileErr != nil {
			// Compilation failed - classify error for feedback
			compileErrors := fl.errorClassifier.ClassifyWithContext(compileErr.Error(), sanitizedRule)
			lastErrors = append(preErrors, compileErrors...)
			logging.KernelDebug("FeedbackLoop: compilation failed: %v", compileErr)
			continue // Retry
		}

		// Phase 4: Schema validation (ensure all predicates are declared)
		schemaErr := validator.ValidateLearnedRule(sanitizedRule)
		if schemaErr != nil {
			// Rule uses undeclared predicates - add to errors for feedback
			schemaError := ValidationError{
				Category:   CategoryUndeclaredPredicate,
				Message:    schemaErr.Error(),
				Suggestion: "Use only predicates declared in schemas. Check available predicates list.",
			}
			lastErrors = append(preErrors, schemaError)
			logging.KernelDebug("FeedbackLoop: schema validation failed: %v", schemaErr)
			continue // Retry
		}

		// Both validations passed!
		result.Rule = sanitizedRule
		result.Valid = true
		logging.Kernel("FeedbackLoop: rule validated successfully on attempt %d", attempt)
		return result, nil
	}

	// All retries exhausted
	result.Errors = lastErrors
	logging.Get(logging.CategoryKernel).Warn("FeedbackLoop: all %d attempts failed", fl.config.MaxRetries)
	return result, fmt.Errorf("rule validation failed after %d attempts", fl.config.MaxRetries)
}

// ValidateOnly validates a rule without generation (for testing existing rules).
func (fl *FeedbackLoop) ValidateOnly(rule string, validator RuleValidator) *ValidationResult {
	result := &ValidationResult{
		Original: rule,
	}

	// Pre-validation
	preErrors := fl.preValidator.Validate(rule)
	result.Errors = preErrors

	// Sanitize
	sanitized, _ := fl.sanitizer.Sanitize(rule)
	result.Sanitized = sanitized

	// Compilation check
	if compileErr := validator.HotLoadRule(sanitized); compileErr != nil {
		compileErrors := fl.errorClassifier.ClassifyWithContext(compileErr.Error(), sanitized)
		result.Errors = append(result.Errors, compileErrors...)
		result.Valid = false
		return result
	}

	// Schema validation (predicate declarations)
	if schemaErr := validator.ValidateLearnedRule(sanitized); schemaErr != nil {
		schemaError := ValidationError{
			Category:   CategoryUndeclaredPredicate,
			Message:    schemaErr.Error(),
			Suggestion: "Use only predicates declared in schemas.",
		}
		result.Errors = append(result.Errors, schemaError)
		result.Valid = false
		return result
	}

	result.Valid = true
	return result
}

// PreValidateOnly runs only pre-validation (for quick checks without compilation).
func (fl *FeedbackLoop) PreValidateOnly(rule string) []ValidationError {
	return fl.preValidator.Validate(rule)
}

// GetBudget returns the current validation budget for inspection.
func (fl *FeedbackLoop) GetBudget() *ValidationBudget {
	return fl.budget
}

// ResetBudget resets the validation budget (typically at session start).
func (fl *FeedbackLoop) ResetBudget() {
	fl.budget.Reset()
}

// IsBudgetExhausted checks if the session validation budget has been exhausted.
// This should be called before invoking GenerateAndValidate to avoid unnecessary
// warning spam when budget is depleted.
func (fl *FeedbackLoop) IsBudgetExhausted() bool {
	return fl.budget.IsSessionExhausted()
}

// hashPrompt creates a short hash of a prompt for budget tracking.
func hashPrompt(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(h[:8]) // First 8 bytes = 16 hex chars
}

// BuildEnhancedSystemPrompt enhances a system prompt with Mangle syntax guidance.
func BuildEnhancedSystemPrompt(basePrompt string, predicates []string) string {
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString(defaultSyntaxReminder)

	if len(predicates) > 0 {
		sb.WriteString("\n## Declared Predicates (use ONLY these):\n")
		// Show up to 30 most relevant predicates
		maxShow := 30
		for i, pred := range predicates {
			if i >= maxShow {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(predicates)-maxShow))
				break
			}
			sb.WriteString("- ")
			sb.WriteString(pred)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
