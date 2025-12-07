package tester

import (
	"codenerd/internal/core"
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// OUTPUT PARSING
// =============================================================================

// containsFailure checks if output indicates test failure.
func (t *TesterShard) containsFailure(output string) bool {
	lowerOutput := strings.ToLower(output)
	failureIndicators := []string{
		"fail", "failed", "failure",
		"error", "panic",
		"not ok",
		"assertion",
	}
	for _, indicator := range failureIndicators {
		if strings.Contains(lowerOutput, indicator) {
			return true
		}
	}
	return false
}

// parseFailedTests extracts failed test information from output.
func (t *TesterShard) parseFailedTests(output, framework string) []FailedTest {
	failed := make([]FailedTest, 0)
	lines := strings.Split(output, "\n")

	switch framework {
	case "gotest":
		goFailRegex := regexp.MustCompile(`--- FAIL: (\w+)`)
		goErrorRegex := regexp.MustCompile(`^(.+\.go):(\d+): (.+)$`)

		for _, line := range lines {
			if matches := goFailRegex.FindStringSubmatch(line); len(matches) > 1 {
				failed = append(failed, FailedTest{
					Name:    matches[1],
					Message: line,
				})
			}
			if matches := goErrorRegex.FindStringSubmatch(line); len(matches) > 3 {
				lineNum := 0
				fmt.Sscanf(matches[2], "%d", &lineNum)
				failed = append(failed, FailedTest{
					FilePath: matches[1],
					Line:     lineNum,
					Message:  matches[3],
				})
			}
		}

	case "jest":
		jestFailRegex := regexp.MustCompile(`✕ (.+)`)
		for _, line := range lines {
			if matches := jestFailRegex.FindStringSubmatch(line); len(matches) > 1 {
				failed = append(failed, FailedTest{
					Name:    matches[1],
					Message: line,
				})
			}
		}

	case "pytest":
		pytestFailRegex := regexp.MustCompile(`FAILED (.+)::(.+)`)
		for _, line := range lines {
			if matches := pytestFailRegex.FindStringSubmatch(line); len(matches) > 2 {
				failed = append(failed, FailedTest{
					FilePath: matches[1],
					Name:     matches[2],
					Message:  line,
				})
			}
		}

	case "cargo":
		cargoFailRegex := regexp.MustCompile(`test (.+) \.\.\. FAILED`)
		for _, line := range lines {
			if matches := cargoFailRegex.FindStringSubmatch(line); len(matches) > 1 {
				failed = append(failed, FailedTest{
					Name:    matches[1],
					Message: line,
				})
			}
		}
	}

	return failed
}

// parsePassedTests extracts passed test names from output.
func (t *TesterShard) parsePassedTests(output, framework string) []string {
	passed := make([]string, 0)
	lines := strings.Split(output, "\n")

	switch framework {
	case "gotest":
		goPassRegex := regexp.MustCompile(`--- PASS: (\w+)`)
		for _, line := range lines {
			if matches := goPassRegex.FindStringSubmatch(line); len(matches) > 1 {
				passed = append(passed, matches[1])
			}
		}

	case "cargo":
		cargoPassRegex := regexp.MustCompile(`test (.+) \.\.\. ok`)
		for _, line := range lines {
			if matches := cargoPassRegex.FindStringSubmatch(line); len(matches) > 1 {
				passed = append(passed, matches[1])
			}
		}
	}

	return passed
}

// parseCoverage extracts coverage percentage from output.
func (t *TesterShard) parseCoverage(output, framework string) float64 {
	switch framework {
	case "gotest":
		// Look for "coverage: XX.X% of statements"
		re := regexp.MustCompile(`coverage: (\d+\.?\d*)%`)
		if matches := re.FindStringSubmatch(output); len(matches) > 1 {
			var cov float64
			fmt.Sscanf(matches[1], "%f", &cov)
			return cov
		}

	case "jest":
		// Look for "All files | XX.XX"
		re := regexp.MustCompile(`All files\s*\|\s*(\d+\.?\d*)`)
		if matches := re.FindStringSubmatch(output); len(matches) > 1 {
			var cov float64
			fmt.Sscanf(matches[1], "%f", &cov)
			return cov
		}

	case "pytest":
		// Look for "TOTAL XX%"
		re := regexp.MustCompile(`TOTAL\s+\d+\s+\d+\s+(\d+)%`)
		if matches := re.FindStringSubmatch(output); len(matches) > 1 {
			var cov float64
			fmt.Sscanf(matches[1], "%f", &cov)
			return cov
		}
	}

	return 0.0
}

// parseDiagnostics converts test output to diagnostics.
func (t *TesterShard) parseDiagnostics(output string) []core.Diagnostic {
	diagnostics := make([]core.Diagnostic, 0)
	lines := strings.Split(output, "\n")

	// Go error format
	goErrorRegex := regexp.MustCompile(`^(.+\.go):(\d+):(\d+): (.+)$`)

	for _, line := range lines {
		if matches := goErrorRegex.FindStringSubmatch(line); len(matches) > 4 {
			lineNum := 0
			colNum := 0
			fmt.Sscanf(matches[2], "%d", &lineNum)
			fmt.Sscanf(matches[3], "%d", &colNum)
			diagnostics = append(diagnostics, core.Diagnostic{
				Severity: "error",
				FilePath: matches[1],
				Line:     lineNum,
				Column:   colNum,
				Message:  matches[4],
			})
		}
	}

	return diagnostics
}

// =============================================================================
// OUTPUT FORMATTING
// =============================================================================

// formatResult formats a TestResult for human-readable output.
func (t *TesterShard) formatResult(result *TestResult) string {
	var sb strings.Builder

	status := "✓ PASSED"
	if !result.Passed {
		status = "✗ FAILED"
	}

	// Build framework and test type string
	frameworkInfo := result.Framework
	if result.TestType != "" && result.TestType != "unknown" {
		frameworkInfo = fmt.Sprintf("%s [%s]", result.Framework, result.TestType)
	}

	sb.WriteString(fmt.Sprintf("%s (%s, %s)\n", status, frameworkInfo, result.Duration))

	if result.Coverage > 0 {
		coverageStatus := ""
		if result.Coverage < t.testerConfig.CoverageGoal {
			coverageStatus = fmt.Sprintf(" (below goal of %.1f%%)", t.testerConfig.CoverageGoal)
		}
		sb.WriteString(fmt.Sprintf("Coverage: %.1f%%%s\n", result.Coverage, coverageStatus))
	}

	if len(result.PassedTests) > 0 {
		sb.WriteString(fmt.Sprintf("Passed: %d tests\n", len(result.PassedTests)))
	}

	if len(result.FailedTests) > 0 {
		sb.WriteString(fmt.Sprintf("Failed: %d tests\n", len(result.FailedTests)))
		for _, failed := range result.FailedTests {
			if failed.FilePath != "" {
				sb.WriteString(fmt.Sprintf("  - %s (%s:%d)\n", failed.Name, failed.FilePath, failed.Line))
			} else {
				sb.WriteString(fmt.Sprintf("  - %s\n", failed.Name))
			}
		}
	}

	if result.Retries > 0 {
		sb.WriteString(fmt.Sprintf("TDD Retries: %d\n", result.Retries))
	}

	if t.testerConfig.VerboseOutput && result.Output != "" {
		sb.WriteString("\n--- Output ---\n")
		sb.WriteString(truncateString(result.Output, 2000))
	}

	return sb.String()
}

// truncateString truncates a string to max length with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
