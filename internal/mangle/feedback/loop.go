package feedback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/mangle/transpiler"
)

// LLMClient is the interface for making LLM calls during retry.
type LLMClient interface {
	// Complete sends a prompt and returns the response.
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// RuleValidator validates Mangle rules via compilation.
type RuleValidator interface {
	// HotLoadRule attempts to load a rule into the kernel sandbox.
	// Returns nil if valid, error otherwise.
	HotLoadRule(rule string) error
	// GetDeclaredPredicates returns all declared predicate signatures.
	GetDeclaredPredicates() []string
}

// FeedbackLoop orchestrates the validate-retry cycle for LLM-generated Mangle.
type FeedbackLoop struct {
	config          RetryConfig
	preValidator    *PreValidator
	errorClassifier *ErrorClassifier
	promptBuilder   *PromptBuilder
	sanitizer       *transpiler.Sanitizer
	budget          *ValidationBudget
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

	// Get available predicates for feedback
	predicates := validator.GetDeclaredPredicates()

	// Add syntax guidance to initial prompt
	enhancedPrompt := userPrompt + "\n" + fl.promptBuilder.BuildInitialPromptAdditions(predicates)

	// Hash for budget tracking
	promptHash := hashPrompt(userPrompt)

	var lastRule string
	var lastErrors []ValidationError

	for attempt := 1; attempt <= fl.config.MaxRetries; attempt++ {
		result.Attempts = attempt

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

		// Call LLM
		response, err := llmClient.Complete(ctx, systemPrompt, currentPrompt)
		if err != nil {
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
		if compileErr == nil {
			// Success!
			result.Rule = sanitizedRule
			result.Valid = true
			logging.Kernel("FeedbackLoop: rule validated successfully on attempt %d", attempt)
			return result, nil
		}

		// Compilation failed - classify error for feedback
		compileErrors := fl.errorClassifier.ClassifyWithContext(compileErr.Error(), sanitizedRule)
		lastErrors = append(preErrors, compileErrors...)

		logging.KernelDebug("FeedbackLoop: compilation failed: %v (found %d errors for feedback)",
			compileErr, len(lastErrors))
	}

	// All retries exhausted
	result.Errors = lastErrors
	logging.Get(logging.CategoryKernel).Warn("FeedbackLoop: all %d attempts failed", fl.config.MaxRetries)
	return result, fmt.Errorf("rule validation failed after %d attempts", fl.config.MaxRetries)
}

// ValidateOnly validates a rule without generation (for testing existing rules).
func (fl *FeedbackLoop) ValidateOnly(rule string, validator RuleValidator) *ValidationResult {
	result := &ValidationResult{
		Original:   rule,
		AttemptNum: 1,
	}

	// Pre-validation
	preErrors := fl.preValidator.Validate(rule)
	result.Errors = append(result.Errors, preErrors...)

	// Auto-repair
	sanitized := rule
	if fl.config.EnableAutoRepair {
		sanitized = fl.preValidator.QuickFix(rule)
		if fullSanitized, err := fl.sanitizer.Sanitize(sanitized); err == nil {
			sanitized = fullSanitized
		}
	}
	result.Sanitized = sanitized

	// Compilation check
	if compileErr := validator.HotLoadRule(sanitized); compileErr != nil {
		compileErrors := fl.errorClassifier.ClassifyWithContext(compileErr.Error(), sanitized)
		result.Errors = append(result.Errors, compileErrors...)
		result.Valid = false
	} else {
		result.Valid = true
	}

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
