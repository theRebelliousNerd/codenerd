package tester

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// =============================================================================
// TEST GENERATION
// =============================================================================

// generateTests uses LLM to generate tests for the target.
func (t *TesterShard) generateTests(ctx context.Context, task *TesterTask) (string, error) {
	t.mu.RLock()
	llmClient := t.llmClient
	framework := t.testerConfig.Framework
	t.mu.RUnlock()

	if llmClient == nil {
		return "", fmt.Errorf("no LLM client configured for test generation")
	}

	if framework == "auto" {
		framework = t.detectFramework(task.Target)
	}

	// Read the target file content
	targetPath := task.Target
	if task.File != "" {
		targetPath = task.File
	}

	var sourceContent string
	if t.virtualStore != nil {
		action := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", targetPath},
		}
		content, err := t.virtualStore.RouteAction(ctx, action)
		if err != nil {
			return "", fmt.Errorf("failed to read target file: %w", err)
		}
		sourceContent = content
	} else {
		return "", fmt.Errorf("virtualStore required for file operations")
	}

	// Build generation prompt
	systemPrompt := t.buildTestGenSystemPrompt(framework)
	userPrompt := t.buildTestGenUserPrompt(sourceContent, task, framework)

	// Call LLM with retry
	response, err := t.llmCompleteWithRetry(ctx, systemPrompt, userPrompt, 3)
	if err != nil {
		return "", fmt.Errorf("LLM test generation failed after retries: %w", err)
	}

	// Parse generated tests
	generated := t.parseGeneratedTests(response, targetPath, framework)

	// Write test file via VirtualStore
	if t.virtualStore != nil && generated.Content != "" {
		writeAction := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/write_file", generated.FilePath, generated.Content},
		}
		_, err := t.virtualStore.RouteAction(ctx, writeAction)
		if err != nil {
			return "", fmt.Errorf("failed to write test file: %w", err)
		}
	}

	// Generate facts
	if t.kernel != nil {
		_ = t.kernel.Assert(core.Fact{
			Predicate: "test_generated",
			Args:      []interface{}{generated.FilePath, generated.TargetFile, int64(generated.TestCount)},
		})
		_ = t.kernel.Assert(core.Fact{
			Predicate: "file_topology",
			Args:      []interface{}{generated.FilePath, hashContent(generated.Content), detectLanguage(generated.FilePath), time.Now().Unix(), true},
		})
	}

	// Format result
	return fmt.Sprintf("Generated %d tests for %s\nTest file: %s\nFunctions tested: %s",
		generated.TestCount, generated.TargetFile, generated.FilePath,
		strings.Join(generated.FunctionsTested, ", ")), nil
}

// buildTestGenSystemPrompt builds the system prompt for test generation.
func (t *TesterShard) buildTestGenSystemPrompt(framework string) string {
	return fmt.Sprintf(`You are an expert test engineer. Generate comprehensive unit tests.

Framework: %s
Guidelines:
- Write thorough tests covering edge cases and error conditions
- Use descriptive test names that explain what is being tested
- Include setup/teardown when appropriate
- Mock external dependencies
- Aim for high coverage of public functions
- Follow best practices for the framework

Return ONLY the test code, no explanations.`, framework)
}

// buildCodeDOMTestContext builds Code DOM context for test generation.
func (t *TesterShard) buildCodeDOMTestContext(targetPath string) string {
	if t.kernel == nil {
		return ""
	}

	var context []string

	// Check for API client functions - need integration tests
	apiClientResults, _ := t.kernel.Query("api_client_function")
	for _, fact := range apiClientResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[1].(string); ok && file == targetPath {
				funcName := "unknown"
				pattern := ""
				if ref, ok := fact.Args[0].(string); ok {
					funcName = ref
				}
				if p, ok := fact.Args[2].(string); ok {
					pattern = p
				}
				context = append(context, fmt.Sprintf("API CLIENT: %s uses %s - mock HTTP client and test error scenarios", funcName, pattern))
			}
		}
	}

	// Check for API handler functions - need request/response tests
	apiHandlerResults, _ := t.kernel.Query("api_handler_function")
	for _, fact := range apiHandlerResults {
		if len(fact.Args) >= 3 {
			if file, ok := fact.Args[1].(string); ok && file == targetPath {
				funcName := "unknown"
				framework := ""
				if ref, ok := fact.Args[0].(string); ok {
					funcName = ref
				}
				if f, ok := fact.Args[2].(string); ok {
					framework = f
				}
				context = append(context, fmt.Sprintf("API HANDLER: %s (%s) - test with httptest, check status codes and JSON responses", funcName, framework))
			}
		}
	}

	// Check requires_integration_test predicate
	integrationResults, _ := t.kernel.Query("requires_integration_test")
	for _, fact := range integrationResults {
		if len(fact.Args) >= 1 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, targetPath) {
				context = append(context, fmt.Sprintf("INTEGRATION TEST RECOMMENDED: %s - consider separate _integration_test.go file", ref))
			}
		}
	}

	// Check for external callers (public API)
	externalResults, _ := t.kernel.Query("has_external_callers")
	for _, fact := range externalResults {
		if len(fact.Args) >= 1 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, targetPath) {
				context = append(context, fmt.Sprintf("PUBLIC API: %s - ensure comprehensive test coverage for public interface", ref))
			}
		}
	}

	if len(context) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nCODE ANALYSIS (from Code DOM):\n")
	for _, c := range context {
		sb.WriteString(fmt.Sprintf("- %s\n", c))
	}
	return sb.String()
}

// buildTestGenUserPrompt builds the user prompt for test generation.
func (t *TesterShard) buildTestGenUserPrompt(source string, task *TesterTask, framework string) string {
	var sb strings.Builder
	sb.WriteString("Generate unit tests for the following code:\n\n")
	sb.WriteString("```\n")
	sb.WriteString(source)
	sb.WriteString("\n```\n\n")

	if task.Function != "" {
		sb.WriteString(fmt.Sprintf("Focus on testing the function: %s\n", task.Function))
	}

	sb.WriteString(fmt.Sprintf("Use the %s framework.\n", framework))
	sb.WriteString("Include tests for:\n")
	sb.WriteString("- Normal operation\n")
	sb.WriteString("- Edge cases\n")
	sb.WriteString("- Error conditions\n")

	// Add Code DOM context for API-aware test generation
	targetPath := task.Target
	if task.File != "" {
		targetPath = task.File
	}
	codeDOMContext := t.buildCodeDOMTestContext(targetPath)
	if codeDOMContext != "" {
		sb.WriteString(codeDOMContext)
	}

	return sb.String()
}

// parseGeneratedTests parses LLM response into a GeneratedTest struct.
func (t *TesterShard) parseGeneratedTests(response, targetPath, framework string) GeneratedTest {
	// Determine test file path
	testPath := t.getTestFilePath(targetPath, framework)

	// Extract code block if present
	content := response
	if idx := strings.Index(response, "```"); idx != -1 {
		endIdx := strings.LastIndex(response, "```")
		if endIdx > idx {
			content = response[idx+3 : endIdx]
			// Remove language tag if present
			if newlineIdx := strings.Index(content, "\n"); newlineIdx != -1 {
				firstLine := strings.TrimSpace(content[:newlineIdx])
				if !strings.Contains(firstLine, " ") && len(firstLine) < 20 {
					content = content[newlineIdx+1:]
				}
			}
		}
	}

	// Count test functions
	testCount := 0
	functionsTested := make([]string, 0)

	switch framework {
	case "gotest":
		re := regexp.MustCompile(`func (Test\w+)\(`)
		matches := re.FindAllStringSubmatch(content, -1)
		testCount = len(matches)
		for _, m := range matches {
			functionsTested = append(functionsTested, m[1])
		}
	case "jest":
		testCount = strings.Count(content, "test(") + strings.Count(content, "it(")
	case "pytest":
		re := regexp.MustCompile(`def (test_\w+)\(`)
		matches := re.FindAllStringSubmatch(content, -1)
		testCount = len(matches)
		for _, m := range matches {
			functionsTested = append(functionsTested, m[1])
		}
	case "cargo":
		re := regexp.MustCompile(`#\[test\]\s*fn (\w+)\(`)
		matches := re.FindAllStringSubmatch(content, -1)
		testCount = len(matches)
		for _, m := range matches {
			functionsTested = append(functionsTested, m[1])
		}
	}

	return GeneratedTest{
		FilePath:        testPath,
		TargetFile:      targetPath,
		Content:         strings.TrimSpace(content),
		TestCount:       testCount,
		FunctionsTested: functionsTested,
	}
}

// getTestFilePath generates the test file path from source file path.
func (t *TesterShard) getTestFilePath(sourcePath, framework string) string {
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	switch framework {
	case "gotest":
		return filepath.Join(dir, name+"_test.go")
	case "jest":
		return filepath.Join(dir, name+".test"+ext)
	case "pytest":
		return filepath.Join(dir, "test_"+name+".py")
	case "cargo":
		// Rust tests typically go in the same file or tests/ dir
		return filepath.Join(dir, name+"_test.rs")
	default:
		return filepath.Join(dir, name+"_test"+ext)
	}
}

// =============================================================================
// LLM HELPERS
// =============================================================================

// llmCompleteWithRetry calls LLM with exponential backoff retry logic.
func (t *TesterShard) llmCompleteWithRetry(ctx context.Context, systemPrompt, userPrompt string, maxRetries int) (string, error) {
	if t.llmClient == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	var lastErr error
	baseDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("[TesterShard:%s] LLM retry attempt %d/%d\n", t.id, attempt+1, maxRetries)

			delay := baseDelay * time.Duration(1<<uint(attempt))
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		response, err := t.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err == nil {
			return response, nil
		}

		lastErr = err

		if !isRetryableError(err) {
			return "", fmt.Errorf("non-retryable error: %w", err)
		}
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// isRetryableError determines if an error should be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	retryablePatterns := []string{
		"timeout", "connection", "network", "temporary",
		"rate limit", "503", "502", "429",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return true // Default to retry
}
