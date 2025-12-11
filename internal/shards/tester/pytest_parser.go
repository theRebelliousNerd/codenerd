package tester

import (
	"regexp"
	"strconv"
	"strings"

	"codenerd/internal/logging"
)

// =============================================================================
// PYTEST OUTPUT PARSER
// =============================================================================
// Comprehensive state machine parser for pytest verbose output.
// Extracts rich diagnostic information including:
// - Test names and file paths
// - Error types and messages
// - Full tracebacks with root cause identification
// - Assertion expected/actual values
// - Where clause introspection from pytest
//
// This is critical for SWE-bench where we need to understand WHY tests fail,
// not just THAT they fail.

// PytestParserState represents the current parsing context.
type PytestParserState int

const (
	StateIdle PytestParserState = iota
	StateHeader                 // ===== section headers =====
	StateCollecting             // Collecting tests info
	StateFailures               // FAILURES section
	StateTestBlock              // Individual test failure block
	StateTraceback              // Inside traceback
	StateAssertionLine          // E       AssertionError: ...
	StateShortSummary           // short test summary info
	StateResults                // final counts (1 failed, etc.)
)

// Regex patterns for pytest output parsing
var (
	// Section headers: ===== SECTION NAME =====
	sectionHeaderRegex = regexp.MustCompile(`^={3,}\s*(.+?)\s*={3,}$`)

	// Test failure header: __________ TestClass.test_method __________
	testBlockHeaderRegex = regexp.MustCompile(`^_{3,}\s*(.+?)\s*_{3,}$`)

	// Python traceback: File "path/to/file.py", line 123, in function
	pythonTracebackRegex = regexp.MustCompile(`^\s+File "(.+)", line (\d+), in (.+)`)

	// Traceback location: path/to/file.py:123: ErrorType
	tracebackLocationRegex = regexp.MustCompile(`^([^\s].+\.py):(\d+):\s*(\w+(?:Error|Exception|Warning)?)`)

	// Assertion context: >       assert result == expected
	assertionContextRegex = regexp.MustCompile(`^>\s+(.+)$`)

	// Assertion error detail: E       AssertionError: message
	assertionErrorRegex = regexp.MustCompile(`^E\s+(\w+(?:Error|Exception)?):?\s*(.*)$`)

	// Comparison assertion: E       assert 5 == 10
	assertComparisonRegex = regexp.MustCompile(`^E\s+assert\s+(.+?)\s*(==|!=|<|>|<=|>=|in|not in|is|is not)\s+(.+)$`)

	// Where clause: E       where X = func()
	whereClauseRegex = regexp.MustCompile(`^E\s+where\s+(.+?)\s+=\s+(.+)$`)

	// Short summary: FAILED tests/test_file.py::TestClass::test_method - ErrorType: msg
	shortSummaryRegex = regexp.MustCompile(`^FAILED\s+(.+?)::(.+?)\s+-\s+(\w+(?:Error|Exception)?):?\s*(.*)$`)

	// Simple short summary: FAILED tests/test_file.py::test_func
	shortSummarySimpleRegex = regexp.MustCompile(`^FAILED\s+(.+?)::(.+)$`)

	// Results: 1 failed, 5 passed in 0.45s
	resultsRegex = regexp.MustCompile(`(\d+)\s+failed(?:.*?(\d+)\s+passed)?.*?in\s+([\d.]+)s`)
)

// PytestFailure represents a complete failed test with all context.
type PytestFailure struct {
	TestFile   string // tests/test_validation.py
	TestClass  string // TestValidation (if applicable)
	TestMethod string // test_check_is_fitted
	FullName   string // TestValidation.test_check_is_fitted

	ErrorType    string // AssertionError, TypeError, etc.
	ErrorMessage string // "Estimator is not fitted"

	Traceback      []TracebackFrame // Full stack trace
	RootCauseFrame *TracebackFrame  // The non-test file where error originated

	AssertionContext AssertionContext // Expected/actual values

	ShortSummary string // From short test summary section
	RawOutput    string // Original raw output for this test
}

// TracebackFrame represents a single frame in the Python traceback.
type TracebackFrame struct {
	FilePath   string // Full path to file
	Line       int    // Line number
	Function   string // Function/method name
	CodeLine   string // The actual code at that line (if available)
	IsTestFile bool   // true if this frame is in a test file
	Depth      int    // 0 = innermost (where error occurred)
}

// AssertionContext captures expected vs actual comparison data.
type AssertionContext struct {
	AssertionLine string            // The assert statement
	Expected      string            // Expected value (from comparison)
	Actual        string            // Actual value (from comparison)
	Operator      string            // ==, !=, in, etc.
	WhereValues   map[string]string // Variable introspection from pytest
	ErrorType     string            // AssertionError, etc.
	ErrorMessage  string            // Human-readable error message
}

// PytestOutputParser parses pytest verbose output into structured diagnostics.
type PytestOutputParser struct {
	state            PytestParserState
	currentTest      *PytestFailure
	currentTraceback []TracebackFrame
	failures         []PytestFailure

	// Accumulated context during parsing
	assertionContext string            // The failing assertion line
	assertionLines   []string          // All E-prefixed lines
	whereValues      map[string]string // Variable -> evaluated value
	rawLines         []string          // Raw output for current test
}

// NewPytestOutputParser creates a new parser instance.
func NewPytestOutputParser() *PytestOutputParser {
	return &PytestOutputParser{
		state:       StateIdle,
		whereValues: make(map[string]string),
	}
}

// Parse parses pytest output and returns all failures.
func (p *PytestOutputParser) Parse(output string) []PytestFailure {
	timer := logging.StartTimer(logging.CategoryTester, "ParsePytestOutput")
	defer timer.Stop()

	lines := strings.Split(output, "\n")

	for i, line := range lines {
		p.processLine(line, i, lines)
	}

	// Finalize any pending test
	p.finalizeCurrentTest()

	logging.TesterDebug("Parsed %d pytest failures", len(p.failures))
	return p.failures
}

// processLine handles a single line based on current parser state.
func (p *PytestOutputParser) processLine(line string, index int, allLines []string) {
	// Check for section transitions
	if matches := sectionHeaderRegex.FindStringSubmatch(line); len(matches) > 1 {
		p.handleSectionChange(matches[1])
		return
	}

	// Check for test block header (_____ TestName _____)
	if matches := testBlockHeaderRegex.FindStringSubmatch(line); len(matches) > 1 {
		p.finalizeCurrentTest()
		p.startNewTest(matches[1])
		p.state = StateTestBlock
		return
	}

	switch p.state {
	case StateFailures, StateTestBlock:
		p.handleTestBlock(line, index, allLines)
	case StateShortSummary:
		p.handleShortSummary(line)
	}

	// Track raw output for current test
	if p.currentTest != nil {
		p.rawLines = append(p.rawLines, line)
	}
}

// handleSectionChange transitions parser state based on section header.
func (p *PytestOutputParser) handleSectionChange(sectionName string) {
	sectionLower := strings.ToLower(sectionName)

	switch {
	case strings.Contains(sectionLower, "failures"):
		p.finalizeCurrentTest()
		p.state = StateFailures
	case strings.Contains(sectionLower, "errors"):
		p.finalizeCurrentTest()
		p.state = StateFailures // Handle errors same as failures
	case strings.Contains(sectionLower, "short test summary"):
		p.finalizeCurrentTest()
		p.state = StateShortSummary
	case strings.Contains(sectionLower, "passed") ||
		strings.Contains(sectionLower, "failed") ||
		strings.Contains(sectionLower, "error"):
		p.state = StateResults
	}
}

// startNewTest initializes a new test failure record.
func (p *PytestOutputParser) startNewTest(header string) {
	// Parse "TestClass.test_method" or "test_standalone"
	header = strings.TrimSpace(header)
	parts := strings.Split(header, ".")

	p.currentTest = &PytestFailure{
		FullName: header,
	}

	if len(parts) >= 2 {
		p.currentTest.TestClass = parts[0]
		p.currentTest.TestMethod = parts[len(parts)-1]
	} else {
		p.currentTest.TestMethod = header
	}

	// Reset accumulators
	p.currentTraceback = make([]TracebackFrame, 0)
	p.assertionContext = ""
	p.assertionLines = nil
	p.whereValues = make(map[string]string)
	p.rawLines = nil
}

// handleTestBlock processes lines within a test failure block.
func (p *PytestOutputParser) handleTestBlock(line string, index int, allLines []string) {
	if p.currentTest == nil {
		return
	}

	// Parse Python traceback frames: File "path", line N, in func
	if matches := pythonTracebackRegex.FindStringSubmatch(line); len(matches) > 3 {
		lineNum, _ := strconv.Atoi(matches[2])
		frame := TracebackFrame{
			FilePath:   matches[1],
			Line:       lineNum,
			Function:   matches[3],
			IsTestFile: isTestFile(matches[1]),
			Depth:      len(p.currentTraceback),
		}

		// Look ahead for code line (next line often shows the code)
		if index+1 < len(allLines) {
			nextLine := allLines[index+1]
			trimmed := strings.TrimSpace(nextLine)
			// Code lines are indented and don't start with E or >
			if len(nextLine) > 0 && nextLine[0] == ' ' &&
				!strings.HasPrefix(trimmed, "E ") &&
				!strings.HasPrefix(trimmed, ">") {
				frame.CodeLine = trimmed
			}
		}

		p.currentTraceback = append(p.currentTraceback, frame)
		return
	}

	// Parse assertion context: >       assert result == expected
	if matches := assertionContextRegex.FindStringSubmatch(line); len(matches) > 1 {
		p.assertionContext = matches[1]
		return
	}

	// Parse E-prefixed assertion details
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "E ") {
		p.handleAssertionLine(trimmed)
		return
	}

	// Parse error location: path/file.py:123: ErrorType
	if matches := tracebackLocationRegex.FindStringSubmatch(line); len(matches) > 3 {
		p.currentTest.ErrorType = matches[3]
		lineNum, _ := strconv.Atoi(matches[2])

		// This is often the root cause location
		frame := TracebackFrame{
			FilePath:   matches[1],
			Line:       lineNum,
			IsTestFile: isTestFile(matches[1]),
		}

		// Prefer non-test file as root cause
		if !frame.IsTestFile && p.currentTest.RootCauseFrame == nil {
			p.currentTest.RootCauseFrame = &frame
		}
	}
}

// handleAssertionLine parses E-prefixed assertion details.
func (p *PytestOutputParser) handleAssertionLine(line string) {
	eLine := strings.TrimPrefix(strings.TrimSpace(line), "E ")
	p.assertionLines = append(p.assertionLines, eLine)

	// Try to parse error type and message: AssertionError: message
	if matches := assertionErrorRegex.FindStringSubmatch(line); len(matches) > 2 {
		if p.currentTest.ErrorType == "" {
			p.currentTest.ErrorType = matches[1]
		}
		if p.currentTest.ErrorMessage == "" {
			p.currentTest.ErrorMessage = strings.TrimSpace(matches[2])
		}
	}

	// Try to parse comparison: assert X == Y
	if matches := assertComparisonRegex.FindStringSubmatch(line); len(matches) > 3 {
		p.currentTest.AssertionContext.Actual = strings.TrimSpace(matches[1])
		p.currentTest.AssertionContext.Operator = matches[2]
		p.currentTest.AssertionContext.Expected = strings.TrimSpace(matches[3])
	}

	// Parse where clauses: where X = func()
	if matches := whereClauseRegex.FindStringSubmatch(line); len(matches) > 2 {
		p.whereValues[matches[1]] = matches[2]
	}
}

// handleShortSummary processes lines in the short test summary section.
func (p *PytestOutputParser) handleShortSummary(line string) {
	// Try full format: FAILED path::test - ErrorType: msg
	if matches := shortSummaryRegex.FindStringSubmatch(line); len(matches) > 4 {
		testFile := matches[1]
		testName := matches[2]
		errorType := matches[3]
		errorMsg := matches[4]

		// Find and update matching failure
		p.updateFailureFromSummary(testFile, testName, errorType, errorMsg, line)
		return
	}

	// Try simple format: FAILED path::test
	if matches := shortSummarySimpleRegex.FindStringSubmatch(line); len(matches) > 2 {
		testFile := matches[1]
		testName := matches[2]
		p.updateFailureFromSummary(testFile, testName, "", "", line)
	}
}

// updateFailureFromSummary enriches a failure with short summary info.
func (p *PytestOutputParser) updateFailureFromSummary(testFile, testName, errorType, errorMsg, line string) {
	// Extract full test name (handle TestClass::test_method format)
	fullName := testName
	if strings.Contains(testName, "::") {
		parts := strings.Split(testName, "::")
		fullName = strings.Join(parts, ".")
	}

	// Find matching failure
	for i := range p.failures {
		if p.failures[i].FullName == fullName ||
			p.failures[i].TestMethod == testName ||
			strings.HasSuffix(p.failures[i].FullName, testName) {

			p.failures[i].ShortSummary = line
			p.failures[i].TestFile = testFile

			if p.failures[i].ErrorType == "" && errorType != "" {
				p.failures[i].ErrorType = errorType
			}
			if p.failures[i].ErrorMessage == "" && errorMsg != "" {
				p.failures[i].ErrorMessage = errorMsg
			}
			return
		}
	}

	// If no matching failure found, create one from summary
	failure := PytestFailure{
		TestFile:     testFile,
		FullName:     fullName,
		TestMethod:   testName,
		ErrorType:    errorType,
		ErrorMessage: errorMsg,
		ShortSummary: line,
	}
	p.failures = append(p.failures, failure)
}

// finalizeCurrentTest saves the current test and resets state.
func (p *PytestOutputParser) finalizeCurrentTest() {
	if p.currentTest == nil {
		return
	}

	// Set traceback
	p.currentTest.Traceback = p.currentTraceback

	// Find root cause frame (first non-test file in traceback, from innermost)
	if p.currentTest.RootCauseFrame == nil {
		for i := len(p.currentTraceback) - 1; i >= 0; i-- {
			if !p.currentTraceback[i].IsTestFile {
				frameCopy := p.currentTraceback[i]
				p.currentTest.RootCauseFrame = &frameCopy
				break
			}
		}
	}

	// Set assertion context
	p.currentTest.AssertionContext.AssertionLine = p.assertionContext
	p.currentTest.AssertionContext.WhereValues = p.whereValues
	p.currentTest.AssertionContext.ErrorType = p.currentTest.ErrorType
	p.currentTest.AssertionContext.ErrorMessage = p.currentTest.ErrorMessage

	// Set raw output
	p.currentTest.RawOutput = strings.Join(p.rawLines, "\n")

	// Append to failures
	p.failures = append(p.failures, *p.currentTest)

	// Reset
	p.currentTest = nil
	p.currentTraceback = nil
	p.assertionContext = ""
	p.assertionLines = nil
	p.whereValues = make(map[string]string)
	p.rawLines = nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// isTestFile determines if a file path is a test file.
func isTestFile(path string) bool {
	// Normalize path separators
	path = strings.ReplaceAll(path, "\\", "/")

	// Check filename patterns
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		if strings.HasPrefix(filename, "test_") ||
			strings.HasSuffix(filename, "_test.py") ||
			filename == "conftest.py" {
			return true
		}
	}

	// Check path patterns
	return strings.Contains(path, "/tests/") ||
		strings.Contains(path, "/test/") ||
		strings.Contains(path, "/testing/")
}

// =============================================================================
// INTEGRATION WITH EXISTING TESTER
// =============================================================================

// ParsePytestOutput is the main entry point for parsing pytest output.
// Returns both the structured failures and a simplified FailedTest slice.
func ParsePytestOutput(output string) ([]PytestFailure, []FailedTest) {
	parser := NewPytestOutputParser()
	failures := parser.Parse(output)

	// Convert to FailedTest for backwards compatibility
	failed := make([]FailedTest, 0, len(failures))
	for _, f := range failures {
		ft := FailedTest{
			Name:    f.FullName,
			Message: f.ErrorMessage,
		}

		// Use root cause location if available
		if f.RootCauseFrame != nil {
			ft.FilePath = f.RootCauseFrame.FilePath
			ft.Line = f.RootCauseFrame.Line
		} else if f.TestFile != "" {
			ft.FilePath = f.TestFile
		}

		// Include expected/actual if available
		if f.AssertionContext.Expected != "" {
			ft.Expected = f.AssertionContext.Expected
			ft.Actual = f.AssertionContext.Actual
		}

		failed = append(failed, ft)
	}

	return failures, failed
}

// IsPytestOutput detects if output is from pytest.
func IsPytestOutput(output string) bool {
	return strings.Contains(output, "pytest") ||
		strings.Contains(output, "===") && strings.Contains(output, "FAILURES") ||
		strings.Contains(output, "FAILED") && strings.Contains(output, "::") ||
		strings.Contains(output, "short test summary info")
}
