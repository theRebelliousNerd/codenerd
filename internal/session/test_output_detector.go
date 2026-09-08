package session

import (
	"regexp"
	"strings"
)

// responsePresentsTestRunnerOutput reports whether text presents test-runner output.
// It matches only strings that look like quoted runner output, not prose.
// Required markers, case-sensitive where shown:
//   - a line beginning with 'ok  ' followed by a package path
//   - '--- PASS:'
//   - '--- FAIL:'
//   - '=== RUN'
//   - 'PASS' as a standalone token on its own line or after '=>'
//   - 'FAIL' likewise
//   - pytest count summaries (standalone, timed, or delimited)
//   - Jest counts following Tests:, Test Suites:, or Snapshots:
//
// Deliberately does NOT match ordinary prose such as 'tests should pass',
// 'make sure tests pass', 'I did not run the tests', or 'the tests will pass'.
func responsePresentsTestRunnerOutput(text string) bool {
	if strings.Contains(text, "--- PASS:") {
		return true
	}
	if strings.Contains(text, "--- FAIL:") {
		return true
	}
	if strings.Contains(text, "=== RUN") {
		return true
	}
	if okLineRe.MatchString(text) {
		return true
	}
	if passOnlyLineRe.MatchString(text) {
		return true
	}
	if failOnlyLineRe.MatchString(text) {
		return true
	}
	if passArrowRe.MatchString(text) {
		return true
	}
	if failArrowRe.MatchString(text) {
		return true
	}
	if pytestSummaryRe.MatchString(text) || pytestTimedRe.MatchString(text) {
		return true
	}
	if jestSummaryRe.MatchString(text) {
		return true
	}
	return false
}

// ResponsePresentsTestRunnerOutput is the exported wrapper for external callers and tests.
func ResponsePresentsTestRunnerOutput(text string) bool {
	return responsePresentsTestRunnerOutput(text)
}

var (
	okLineRe       = regexp.MustCompile(`(?m)^ok  \s*\S+`)
	passOnlyLineRe = regexp.MustCompile(`(?m)^\s*PASS\s*$`)
	failOnlyLineRe = regexp.MustCompile(`(?m)^\s*FAIL\s*$`)
	passArrowRe    = regexp.MustCompile(`=>\s*PASS\b`)
	failArrowRe    = regexp.MustCompile(`=>\s*FAIL\b`)
	// A bare count anywhere in prose is not a test-runner signature. Browser
	// actions also report "0 succeeded, 1 failed", including expected stale-ref
	// rejections. Require the count grammar or a runner-specific context.
	pytestSummaryRe = regexp.MustCompile(`(?m)^[\t =]*\d+ (?:passed|failed)(?:, \d+ (?:passed|failed|skipped|deselected|xfailed|xpassed|warnings?|errors?))*[\t =]*$`)
	pytestTimedRe   = regexp.MustCompile(`\b\d+ (?:passed|failed)(?:, \d+ (?:passed|failed|skipped|deselected|xfailed|xpassed|warnings?|errors?))* in \d+(?:\.\d+)?s\b`)
	jestSummaryRe   = regexp.MustCompile(`(?m)\b(?:Tests|Test Suites|Snapshots):[^\r\n]*\b\d+ (?:passed|failed)\b`)
)
