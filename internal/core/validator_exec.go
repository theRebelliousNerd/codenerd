package core

import (
	"context"
	"regexp"
	"strings"
	"sync"
)

// ANSI escape code regex
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(str string) string {
	return ansiRegex.ReplaceAllString(str, "")
}

// ExecutionValidator verifies that shell commands executed successfully
// by analyzing output for failure patterns, even when exit code is 0.

var (
	commonFailurePatterns []*regexp.Regexp
	buildFailurePatterns  []*regexp.Regexp
	testFailurePatterns   []*regexp.Regexp
)

func init() {
	// Common failure patterns
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
		commonFailurePatterns = append(commonFailurePatterns, regexp.MustCompile(p))
	}

	// Build-specific failure patterns
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
		buildFailurePatterns = append(buildFailurePatterns, regexp.MustCompile(p))
	}

	// Test-specific failure patterns
	testPatterns := []string{
		`(?i)tests? failed`,
		`(?i)FAIL\s+`,
		`(?i)assertion failed`,
		`(?i)expected .* but got`,
		`(?i)test case.*failed`,
	}
	for _, p := range testPatterns {
		testFailurePatterns = append(testFailurePatterns, regexp.MustCompile(p))
	}
}

type ExecutionValidator struct {
	mu sync.RWMutex
	// failurePatterns are regex patterns that indicate failure
	failurePatterns []*regexp.Regexp
	// successPatterns are patterns that indicate success (optional confirmation)
	successPatterns []*regexp.Regexp
}

// NewExecutionValidator creates a new execution validator with common failure patterns.
func NewExecutionValidator() *ExecutionValidator {
	v := &ExecutionValidator{
		// Copy global patterns so AddFailurePattern can mutate safely for this instance
		failurePatterns: append([]*regexp.Regexp{}, commonFailurePatterns...),
		successPatterns: make([]*regexp.Regexp, 0),
	}
	return v
}

// AddFailurePattern adds a custom failure pattern.
func (v *ExecutionValidator) AddFailurePattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
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
	if err := ctx.Err(); err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodOutputScan,
			Error:      "validation cancelled: " + err.Error(),
		}
	}

	// If action already reported failure, trust that
	if !result.Success {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodOutputScan,
			Error:      "command reported failure: " + result.Error,
		}
	}

	// The scan sees the whole stripped output. A head-and-tail cut dropped
	// a failure that sat in the middle of a long log.
	output := stripANSI(result.Output)

	v.mu.RLock()
	patterns := v.failurePatterns
	v.mu.RUnlock()

	// Check for failure patterns in output
	for _, pattern := range patterns {
		if pattern.MatchString(output) {
			match := pattern.FindString(output)
			// Extract context around the match
			contextStr := extractContext(output, match, 100)

			return ValidationResult{
				Verified:   false,
				Confidence: 0.85, // Not 1.0 because pattern might be false positive
				Method:     ValidationMethodOutputScan,
				Error:      "failure pattern detected in output",
				Details: map[string]any{
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
		Details: map[string]any{
			"output_length":    len(output),
			"patterns_checked": len(patterns),
		},
	}
}

// validateCommandSpecific performs additional validation based on command type.
func (v *ExecutionValidator) validateCommandSpecific(ctx context.Context, req ActionRequest, result ActionResult) *ValidationResult {
	if err := ctx.Err(); err != nil {
		return &ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodOutputScan,
			Error:      "validation cancelled: " + err.Error(),
		}
	}
	command := req.Target
	output := stripANSI(result.Output)

	// Go build specific checks
	if strings.Contains(command, "go build") || strings.Contains(command, "go vet") {
		if needle, ok := firstPresent(output, "cannot find package", "undefined:", "imported and not used"); ok {
			return &ValidationResult{
				Verified:   false,
				Confidence: 0.95,
				Method:     ValidationMethodOutputScan,
				Error:      "Go compilation error detected",
				Details:    map[string]any{"output_preview": evidenceLine(output, needle)},
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
				Details:    map[string]any{"output_preview": evidenceLine(output, "FAIL")},
			}
		}
	}

	// npm/yarn specific checks
	if strings.Contains(command, "npm") || strings.Contains(command, "yarn") {
		if needle, ok := firstPresent(output, "npm ERR!", "error "); ok {
			return &ValidationResult{
				Verified:   false,
				Confidence: 0.9,
				Method:     ValidationMethodOutputScan,
				Error:      "npm/yarn error detected",
				Details:    map[string]any{"output_preview": evidenceLine(output, needle)},
			}
		}
	}

	// Python specific checks
	if strings.Contains(command, "python") || strings.Contains(command, "pip") {
		if needle, ok := firstPresent(output, "Traceback (most recent call last)", "SyntaxError", "ModuleNotFoundError"); ok {
			return &ValidationResult{
				Verified:   false,
				Confidence: 0.95,
				Method:     ValidationMethodOutputScan,
				Error:      "Python error detected",
				Details:    map[string]any{"output_preview": evidenceLine(output, needle)},
			}
		}
	}

	// Git specific checks
	if strings.Contains(command, "git") {
		if needle, ok := firstPresent(output, "CONFLICT", "rejected", "not a git repository"); ok {
			return &ValidationResult{
				Verified:   false,
				Confidence: 0.9,
				Method:     ValidationMethodOutputScan,
				Error:      "Git error detected",
				Details:    map[string]any{"output_preview": evidenceLine(output, needle)},
			}
		}
	}

	return nil
}

// Name returns the validator name.
func (v *ExecutionValidator) Name() string { return "execution_validator" }

// Priority returns the validator priority.
func (v *ExecutionValidator) Priority() int { return 10 }

// firstPresent returns the first needle that occurs in output.
func firstPresent(output string, needles ...string) (string, bool) {
	for _, needle := range needles {
		if needle != "" && strings.Contains(output, needle) {
			return needle, true
		}
	}
	return "", false
}

// evidenceLine is the whole line that contains needle. That line is the
// evidence of a command-specific failure. A prefix of the entire output hid
// the match once it sat past the first 200 bytes.
func evidenceLine(output, needle string) string {
	if needle == "" {
		return ""
	}
	idx := strings.Index(output, needle)
	if idx < 0 {
		return needle
	}
	start := 0
	if nl := strings.LastIndex(output[:idx], "\n"); nl >= 0 {
		start = nl + 1
	}
	rest := output[idx:]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		return output[start : idx+nl]
	}
	return output[start:]
}

// extractContext extracts text around a match for context.
func extractContext(text, match string, contextChars int) string {
	before, _, ok := strings.Cut(text, match)
	if !ok {
		return match
	}

	runes := []rune(text)
	matchRunes := []rune(match)

	// Convert byte index to rune index
	startRuneIdx := len([]rune(before))

	start := max(startRuneIdx-contextChars, 0)

	end := min(startRuneIdx+len(matchRunes)+contextChars, len(runes))

	result := string(runes[start:end])
	if start > 0 {
		result = "..." + result
	}
	if end < len(runes) {
		result = result + "..."
	}

	return result
}

// BuildValidator specifically validates build command results.
// Embeds *ExecutionValidator (not by value) because ExecutionValidator
// contains a sync.RWMutex that must not be copied.
type BuildValidator struct {
	*ExecutionValidator
}

// NewBuildValidator creates a validator specialized for build commands.
func NewBuildValidator() *BuildValidator {
	exec := NewExecutionValidator()
	exec.mu.Lock()
	exec.failurePatterns = append(exec.failurePatterns, buildFailurePatterns...)
	exec.mu.Unlock()
	return &BuildValidator{ExecutionValidator: exec}
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
// Embeds *ExecutionValidator (not by value) because ExecutionValidator
// contains a sync.RWMutex that must not be copied.
type TestValidator struct {
	*ExecutionValidator
}

// NewTestValidator creates a validator specialized for test commands.
func NewTestValidator() *TestValidator {
	exec := NewExecutionValidator()
	exec.mu.Lock()
	exec.failurePatterns = append(exec.failurePatterns, testFailurePatterns...)
	exec.mu.Unlock()
	return &TestValidator{ExecutionValidator: exec}
}

// CanValidate returns true for test action types.
func (v *TestValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionRunTests
}

// Name returns the validator name.
func (v *TestValidator) Name() string { return "test_validator" }

// Priority returns the validator priority.
func (v *TestValidator) Priority() int { return 8 }
