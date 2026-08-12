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
//   - 'N passed' / 'N failed' forms from pytest and jest
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
	if nPassedRe.MatchString(text) {
		return true
	}
	if nFailedRe.MatchString(text) {
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
	nPassedRe      = regexp.MustCompile(`\b\d+ passed\b`)
	nFailedRe      = regexp.MustCompile(`\b\d+ failed\b`)
)
