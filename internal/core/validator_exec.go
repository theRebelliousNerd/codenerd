package core

import (
	"context"
	"regexp"
	"strings"
)

// ExecutionValidator verifies that shell commands executed successfully
// by analyzing output for failure patterns, even when exit code is 0.
type ExecutionValidator struct {
	// failurePatterns are regex patterns that indicate failure
	failurePatterns []*regexp.Regexp
	// successPatterns are patterns that indicate success (optional confirmation)
	successPatterns []*regexp.Regexp
}

// NewExecutionValidator creates a new execution validator with common failure patterns.
func NewExecutionValidator() *ExecutionValidator {
	v := &ExecutionValidator{
		failurePatterns: make([]*regexp.Regexp, 0),
		successPatterns: make([]*regexp.Regexp, 0),
	}

	// Common failure patterns (case insensitive via (?i))
	failurePatternStrs := []string{
		`(?i)panic:`,
		`(?i)fatal:`,
		`(?i)FATAL`,
		`(?i)error:`,
		`(?i)ERROR:`,
		`(?i)segmentation fault`,
		`(?i)killed`,
		`(?i)out of memory`,
		`(?i)OOM`,
		`(?i)permission denied`,
		`(?i)access denied`,
		`(?i)no such file or directory`,
		`(?i)command not found`,
		`(?i)cannot find`,
		`(?i)failed to`,
		`(?i)unable to`,
		`(?i)exception`,
		`(?i)traceback`,
		`(?i)stack trace`,
		`(?i)core dumped`,
		`(?i)abort`,
		`(?i)timeout`,
		`(?i)timed out`,
		`(?i)connection refused`,
		`(?i)connection reset`,
		`(?i)ENOENT`,
		`(?i)EACCES`,
		`(?i)EPERM`,
		`(?i)ENOMEM`,
	}

	for _, p := range failurePatternStrs {
		re, err := regexp.Compile(p)
		if err == nil {
			v.failurePatterns = append(v.failurePatterns, re)
		}
	}

	return v
}

// AddFailurePattern adds a custom failure pattern.
func (v *ExecutionValidator) AddFailurePattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	v.failurePatterns = append(v.failurePatterns, re)
	return nil
}

// CanValidate returns true for execution action types.
func (v *ExecutionValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionRunCommand ||
		actionType == ActionBash ||
		actionType == ActionExecCmd ||
		actionType == ActionRunBuild ||
		actionType == ActionRunTests ||
		actionType == ActionGitOperation
}

// Validate scans command output for failure patterns.
func (v *ExecutionValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	// If action already reported failure, trust that
	if !result.Success {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodOutputScan,
			Error:      "command reported failure: " + result.Error,
		}
	}

	output := result.Output

	// Check for failure patterns in output
	for _, pattern := range v.failurePatterns {
		if pattern.MatchString(output) {
			match := pattern.FindString(output)
			// Extract context around the match
			contextStr := extractContext(output, match, 100)

			return ValidationResult{
				Verified:   false,
				Confidence: 0.85, // Not 1.0 because pattern might be false positive
				Method:     ValidationMethodOutputScan,
				Error:      "failure pattern detected in output",
				Details: map[string]interface{}{
					"pattern": pattern.String(),
					"match":   match,
					"context": contextStr,
				},
			}
		}
	}

	// Additional checks for specific command types
	extraResult := v.validateCommandSpecific(ctx, req, result)
	if extraResult != nil && !extraResult.Verified {
		return *extraResult
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 0.8, // Not 1.0 because we only scanned patterns
		Method:     ValidationMethodOutputScan,
		Details: map[string]interface{}{
			"output_length":    len(output),
			"patterns_checked": len(v.failurePatterns),
		},
	}
}

// validateCommandSpecific performs additional validation based on command type.
func (v *ExecutionValidator) validateCommandSpecific(ctx context.Context, req ActionRequest, result ActionResult) *ValidationResult {
	if err := ctx.Err(); err != nil {
		return nil
	}
	command := req.Target
	output := result.Output

	// Go build specific checks
	if strings.Contains(command, "go build") || strings.Contains(command, "go vet") {
		if strings.Contains(output, "cannot find package") ||
			strings.Contains(output, "undefined:") ||
			strings.Contains(output, "imported and not used") {
			return &ValidationResult{
				Verified:   false,
				Confidence: 0.95,
				Method:     ValidationMethodOutputScan,
				Error:      "Go compilation error detected",
				Details:    map[string]interface{}{"output_preview": truncateStr(output, 200)},
			}
		}
	}

	// Go test specific checks
	if strings.Contains(command, "go test") {
		if strings.Contains(output, "FAIL") && !strings.Contains(output, "ok") {
			return &ValidationResult{
				Verified:   false,
				Confidence: 0.95,
				Method:     ValidationMethodOutputScan,
				Error:      "Go test failure detected",
				Details:    map[string]interface{}{"output_preview": truncateStr(output, 200)},
			}
		}
	}

	// npm/yarn specific checks
	if strings.Contains(command, "npm") || strings.Contains(command, "yarn") {
		if strings.Contains(output, "npm ERR!") || strings.Contains(output, "error ") {
			return &ValidationResult{
				Verified:   false,
				Confidence: 0.9,
				Method:     ValidationMethodOutputScan,
				Error:      "npm/yarn error detected",
				Details:    map[string]interface{}{"output_preview": truncateStr(output, 200)},
			}
		}
	}

	// Python specific checks
	if strings.Contains(command, "python") || strings.Contains(command, "pip") {
		if strings.Contains(output, "Traceback (most recent call last)") ||
			strings.Contains(output, "SyntaxError") ||
			strings.Contains(output, "ModuleNotFoundError") {
			return &ValidationResult{
				Verified:   false,
				Confidence: 0.95,
				Method:     ValidationMethodOutputScan,
				Error:      "Python error detected",
				Details:    map[string]interface{}{"output_preview": truncateStr(output, 200)},
			}
		}
	}

	// Git specific checks
	if strings.Contains(command, "git") {
		if strings.Contains(output, "CONFLICT") ||
			strings.Contains(output, "rejected") ||
			strings.Contains(output, "not a git repository") {
			return &ValidationResult{
				Verified:   false,
				Confidence: 0.9,
				Method:     ValidationMethodOutputScan,
				Error:      "Git error detected",
				Details:    map[string]interface{}{"output_preview": truncateStr(output, 200)},
			}
		}
	}

	return nil
}

// Name returns the validator name.
func (v *ExecutionValidator) Name() string { return "execution_validator" }

// Priority returns the validator priority.
func (v *ExecutionValidator) Priority() int { return 10 }

// extractContext extracts text around a match for context.
func extractContext(text, match string, contextChars int) string {
	idx := strings.Index(text, match)
	if idx == -1 {
		return match
	}

	start := idx - contextChars
	if start < 0 {
		start = 0
	}

	end := idx + len(match) + contextChars
	if end > len(text) {
		end = len(text)
	}

	result := text[start:end]
	if start > 0 {
		result = "..." + result
	}
	if end < len(text) {
		result = result + "..."
	}

	return result
}

// BuildValidator specifically validates build command results.
type BuildValidator struct {
	ExecutionValidator
}

// NewBuildValidator creates a validator specialized for build commands.
func NewBuildValidator() *BuildValidator {
	exec := NewExecutionValidator()

	// Add build-specific failure patterns
	buildPatterns := []string{
		`(?i)compilation failed`,
		`(?i)build failed`,
		`(?i)linker error`,
		`(?i)undefined reference`,
		`(?i)unresolved external`,
		`(?i)cannot find -l`,
		`(?i)missing required`,
	}

	for _, p := range buildPatterns {
		_ = exec.AddFailurePattern(p)
	}

	return &BuildValidator{ExecutionValidator: *exec}
}

// CanValidate returns true for build action types.
func (v *BuildValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionRunBuild
}

// Name returns the validator name.
func (v *BuildValidator) Name() string { return "build_validator" }

// Priority returns the validator priority (higher priority = runs first for builds).
func (v *BuildValidator) Priority() int { return 8 }

// TestValidator specifically validates test command results.
type TestValidator struct {
	ExecutionValidator
}

// NewTestValidator creates a validator specialized for test commands.
func NewTestValidator() *TestValidator {
	exec := NewExecutionValidator()

	// Add test-specific failure patterns
	testPatterns := []string{
		`(?i)tests? failed`,
		`(?i)FAIL\s+`,
		`(?i)assertion failed`,
		`(?i)expected .* but got`,
		`(?i)test case.*failed`,
	}

	for _, p := range testPatterns {
		_ = exec.AddFailurePattern(p)
	}

	return &TestValidator{ExecutionValidator: *exec}
}

// CanValidate returns true for test action types.
func (v *TestValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionRunTests
}

// Name returns the validator name.
func (v *TestValidator) Name() string { return "test_validator" }

// Priority returns the validator priority.
func (v *TestValidator) Priority() int { return 8 }
