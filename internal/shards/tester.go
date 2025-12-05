// Package shards implements specialized ShardAgent types for the Cortex 1.5.0 architecture.
// This file implements the Tester ShardAgent per §7.0 Sharding.
package shards

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// CONFIGURATION
// =============================================================================

// TesterConfig holds configuration for the tester shard.
type TesterConfig struct {
	Framework     string        // "gotest", "jest", "pytest", "cargo", "auto" (default: auto)
	CoverageGoal  float64       // Target coverage percentage (default: 80.0)
	CoverageCmd   string        // Coverage extraction command (auto-detected if empty)
	AutoFix       bool          // Auto-fix failing tests via TDD loop
	MaxRetries    int           // TDD retry limit (default: 3)
	TestTimeout   time.Duration // Per-test timeout (default: 5 minutes)
	BuildTimeout  time.Duration // Build timeout (default: 2 minutes)
	ParallelTests bool          // Run tests in parallel
	WorkingDir    string        // Workspace directory
	TestPatterns  []string      // Glob patterns for test files
	VerboseOutput bool          // Include detailed output in results
}

// DefaultTesterConfig returns sensible defaults for testing.
func DefaultTesterConfig() TesterConfig {
	return TesterConfig{
		Framework:     "auto",
		CoverageGoal:  80.0,
		CoverageCmd:   "",
		AutoFix:       false,
		MaxRetries:    3,
		TestTimeout:   5 * time.Minute,
		BuildTimeout:  2 * time.Minute,
		ParallelTests: false,
		WorkingDir:    ".",
		TestPatterns:  []string{"*_test.go", "*.test.ts", "test_*.py", "*_test.rs"},
		VerboseOutput: false,
	}
}

// =============================================================================
// TEST RESULT TYPES
// =============================================================================

// TestResult represents the outcome of a test run.
type TestResult struct {
	Passed      bool              `json:"passed"`
	Output      string            `json:"output"`
	Coverage    float64           `json:"coverage"`
	FailedTests []FailedTest      `json:"failed_tests"`
	PassedTests []string          `json:"passed_tests"`
	Duration    time.Duration     `json:"duration"`
	Diagnostics []core.Diagnostic `json:"diagnostics"`
	Retries     int               `json:"retries"`
	Framework   string            `json:"framework"`
}

// FailedTest represents a single failed test.
type FailedTest struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// GeneratedTest represents a newly generated test.
type GeneratedTest struct {
	FilePath        string   `json:"file_path"`
	TargetFile      string   `json:"target_file"`
	Content         string   `json:"content"`
	TestCount       int      `json:"test_count"`
	FunctionsTested []string `json:"functions_tested"`
}

// =============================================================================
// TESTER SHARD
// =============================================================================

// TesterShard is specialized for test generation and TDD loops.
// Per Cortex 1.5.0 §7.0, this is a Type A Ephemeral Generalist or
// can become a Type B Persistent Specialist when hydrated with testing patterns.
type TesterShard struct {
	mu sync.RWMutex

	// Identity
	id     string
	config core.ShardConfig
	state  core.ShardState

	// Tester-specific configuration
	testerConfig TesterConfig

	// Components (required)
	kernel       *core.RealKernel     // Own kernel instance for logic-driven testing
	llmClient    perception.LLMClient // LLM for test generation
	virtualStore *core.VirtualStore   // Action routing

	// TDD Loop integration
	tddLoop *core.TDDLoop

	// State tracking
	startTime   time.Time
	testHistory []TestResult
	diagnostics []core.Diagnostic

	// Autopoiesis tracking (§8.3)
	successPatterns map[string]int // Patterns that pass tests
	failurePatterns map[string]int // Patterns that repeatedly fail
}

// NewTesterShard creates a new Tester shard with default configuration.
func NewTesterShard() *TesterShard {
	return NewTesterShardWithConfig(DefaultTesterConfig())
}

// NewTesterShardWithConfig creates a tester shard with custom configuration.
func NewTesterShardWithConfig(testerConfig TesterConfig) *TesterShard {
	return &TesterShard{
		config:          core.DefaultSpecialistConfig("tester", ""),
		state:           core.ShardStateIdle,
		testerConfig:    testerConfig,
		testHistory:     make([]TestResult, 0),
		diagnostics:     make([]core.Diagnostic, 0),
		successPatterns: make(map[string]int),
		failurePatterns: make(map[string]int),
	}
}

// =============================================================================
// DEPENDENCY INJECTION
// =============================================================================

// SetLLMClient sets the LLM client for test generation.
func (t *TesterShard) SetLLMClient(client perception.LLMClient) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.llmClient = client
}

// SetKernel sets the Mangle kernel for logic-driven testing.
func (t *TesterShard) SetKernel(k *core.RealKernel) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.kernel = k
}

// SetVirtualStore sets the virtual store for action routing.
func (t *TesterShard) SetVirtualStore(vs *core.VirtualStore) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.virtualStore = vs
}

// =============================================================================
// SHARD INTERFACE IMPLEMENTATION
// =============================================================================

// GetID returns the shard ID.
func (t *TesterShard) GetID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.id
}

// GetState returns the current state.
func (t *TesterShard) GetState() core.ShardState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// GetConfig returns the shard configuration.
func (t *TesterShard) GetConfig() core.ShardConfig {
	return t.config
}

// GetKernel returns the kernel (for fact propagation).
func (t *TesterShard) GetKernel() *core.RealKernel {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.kernel
}

// Stop stops the shard.
func (t *TesterShard) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = core.ShardStateCompleted
	return nil
}

// =============================================================================
// MAIN EXECUTION
// =============================================================================

// Execute performs the testing task.
// Task formats:
//   - "run_tests file:PATH" or "run_tests package:PACKAGE"
//   - "generate_tests file:PATH" or "generate_tests function:NAME in:FILE"
//   - "coverage file:PATH"
//   - "tdd file:PATH" (full TDD repair loop)
func (t *TesterShard) Execute(ctx context.Context, task string) (string, error) {
	t.mu.Lock()
	t.state = core.ShardStateRunning
	t.startTime = time.Now()
	t.id = fmt.Sprintf("tester-%d", time.Now().UnixNano())
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		t.state = core.ShardStateCompleted
		t.mu.Unlock()
	}()

	fmt.Printf("[TesterShard:%s] Starting task: %s\n", t.id, task)

	// Initialize kernel if not set
	if t.kernel == nil {
		t.kernel = core.NewRealKernel()
	}
	// Load tester-specific policy
	_ = t.kernel.LoadPolicyFile("tester.gl")

	// Parse the task
	parsedTask, err := t.parseTask(task)
	if err != nil {
		return "", fmt.Errorf("failed to parse task: %w", err)
	}

	// Assert initial facts to kernel
	t.assertInitialFacts(parsedTask)

	// Route to appropriate handler
	var result *TestResult
	switch parsedTask.Action {
	case "run_tests":
		result, err = t.runTests(ctx, parsedTask)
	case "generate_tests":
		return t.generateTests(ctx, parsedTask)
	case "coverage":
		result, err = t.runCoverage(ctx, parsedTask)
	case "tdd":
		result, err = t.runTDDLoop(ctx, parsedTask)
	default:
		// Default to run_tests
		result, err = t.runTests(ctx, parsedTask)
	}

	if err != nil {
		return "", err
	}

	// Generate facts for propagation
	facts := t.generateFacts(result)
	for _, fact := range facts {
		if t.kernel != nil {
			_ = t.kernel.Assert(fact)
		}
	}

	// Track history
	t.mu.Lock()
	t.testHistory = append(t.testHistory, *result)
	t.mu.Unlock()

	// Format output
	return t.formatResult(result), nil
}

// =============================================================================
// TASK PARSING
// =============================================================================

// TesterTask represents a parsed test task.
type TesterTask struct {
	Action   string // "run_tests", "generate_tests", "coverage", "tdd"
	Target   string // File path, package, or function
	File     string // Specific file (for function tests)
	Function string // Specific function
	Package  string // Package path
	Options  map[string]string
}

// parseTask extracts action and parameters from task string.
func (t *TesterShard) parseTask(task string) (*TesterTask, error) {
	parsed := &TesterTask{
		Action:  "run_tests",
		Options: make(map[string]string),
	}

	parts := strings.Fields(task)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty task")
	}

	// First token is the action
	action := strings.ToLower(parts[0])
	switch action {
	case "run_tests", "test", "run":
		parsed.Action = "run_tests"
	case "generate_tests", "generate", "gen":
		parsed.Action = "generate_tests"
	case "coverage", "cov":
		parsed.Action = "coverage"
	case "tdd", "tdd_loop", "repair":
		parsed.Action = "tdd"
	default:
		// Assume run_tests if action is a file path
		if strings.Contains(action, ".") || strings.Contains(action, "/") {
			parsed.Action = "run_tests"
			parsed.Target = action
		}
	}

	// Parse key:value pairs
	for _, part := range parts[1:] {
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			key := strings.ToLower(kv[0])
			value := kv[1]

			switch key {
			case "file":
				parsed.File = value
				if parsed.Target == "" {
					parsed.Target = value
				}
			case "function", "func":
				parsed.Function = value
			case "package", "pkg":
				parsed.Package = value
				if parsed.Target == "" {
					parsed.Target = value
				}
			case "in":
				parsed.File = value
			default:
				parsed.Options[key] = value
			}
		} else if !strings.HasPrefix(part, "-") {
			// Bare argument - treat as target
			if parsed.Target == "" {
				parsed.Target = part
			}
		}
	}

	// Default target
	if parsed.Target == "" && parsed.Package == "" {
		parsed.Target = "./..."
	}

	return parsed, nil
}

// =============================================================================
// TEST EXECUTION
// =============================================================================

// runTests executes tests for the specified target.
func (t *TesterShard) runTests(ctx context.Context, task *TesterTask) (*TestResult, error) {
	t.mu.RLock()
	framework := t.testerConfig.Framework
	workingDir := t.testerConfig.WorkingDir
	timeout := t.testerConfig.TestTimeout
	t.mu.RUnlock()

	// Auto-detect framework if needed
	if framework == "auto" {
		framework = t.detectFramework(task.Target)
	}

	// Build test command
	cmd := t.buildTestCommand(framework, task)

	// Execute via VirtualStore or direct execution
	var output string
	var err error

	startTime := time.Now()

	if t.virtualStore != nil {
		action := core.Fact{
			Predicate: "next_action",
			Args: []interface{}{
				"/run_tests",
				cmd,
			},
		}
		output, err = t.virtualStore.RouteAction(ctx, action)
	} else {
		// Direct execution fallback
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmdParts := strings.Fields(cmd)
		if len(cmdParts) == 0 {
			return nil, fmt.Errorf("empty test command")
		}

		execCmd := exec.CommandContext(execCtx, cmdParts[0], cmdParts[1:]...)
		execCmd.Dir = workingDir

		outputBytes, execErr := execCmd.CombinedOutput()
		output = string(outputBytes)
		if execErr != nil && !strings.Contains(output, "FAIL") {
			err = execErr
		}
	}

	duration := time.Since(startTime)

	// Parse results
	result := &TestResult{
		Framework:   framework,
		Duration:    duration,
		Output:      output,
		Diagnostics: make([]core.Diagnostic, 0),
	}

	// Determine pass/fail
	if err != nil || t.containsFailure(output) {
		result.Passed = false
		result.FailedTests = t.parseFailedTests(output, framework)
		result.Diagnostics = t.parseDiagnostics(output)

		// Track failure patterns for Autopoiesis
		t.trackFailurePattern(result)
	} else {
		result.Passed = true
		result.PassedTests = t.parsePassedTests(output, framework)

		// Track success patterns for Autopoiesis
		t.trackSuccessPattern(result)
	}

	// Parse coverage if present
	result.Coverage = t.parseCoverage(output, framework)

	return result, nil
}

// runCoverage runs tests with coverage and returns coverage metrics.
func (t *TesterShard) runCoverage(ctx context.Context, task *TesterTask) (*TestResult, error) {
	t.mu.RLock()
	framework := t.testerConfig.Framework
	workingDir := t.testerConfig.WorkingDir
	t.mu.RUnlock()

	if framework == "auto" {
		framework = t.detectFramework(task.Target)
	}

	// Build coverage command
	cmd := t.buildCoverageCommand(framework, task)

	startTime := time.Now()
	var output string

	if t.virtualStore != nil {
		action := core.Fact{
			Predicate: "next_action",
			Args: []interface{}{
				"/run_coverage",
				cmd,
			},
		}
		var err error
		output, err = t.virtualStore.RouteAction(ctx, action)
		if err != nil {
			return nil, err
		}
	} else {
		cmdParts := strings.Fields(cmd)
		execCmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
		execCmd.Dir = workingDir
		outputBytes, _ := execCmd.CombinedOutput()
		output = string(outputBytes)
	}

	duration := time.Since(startTime)

	result := &TestResult{
		Framework: framework,
		Duration:  duration,
		Output:    output,
		Passed:    !t.containsFailure(output),
		Coverage:  t.parseCoverage(output, framework),
	}

	return result, nil
}

// =============================================================================
// TDD LOOP INTEGRATION
// =============================================================================

// runTDDLoop runs a full TDD repair loop until tests pass or max retries.
func (t *TesterShard) runTDDLoop(ctx context.Context, task *TesterTask) (*TestResult, error) {
	t.mu.RLock()
	maxRetries := t.testerConfig.MaxRetries
	framework := t.testerConfig.Framework
	workingDir := t.testerConfig.WorkingDir
	testTimeout := t.testerConfig.TestTimeout
	buildTimeout := t.testerConfig.BuildTimeout
	t.mu.RUnlock()

	if framework == "auto" {
		framework = t.detectFramework(task.Target)
	}

	// Create TDD loop configuration
	tddConfig := core.TDDLoopConfig{
		MaxRetries:   maxRetries,
		TestCommand:  t.buildTestCommand(framework, task),
		BuildCommand: t.buildBuildCommand(framework),
		TestTimeout:  testTimeout,
		BuildTimeout: buildTimeout,
		WorkingDir:   workingDir,
	}

	// Create TDD loop with our dependencies
	t.tddLoop = core.NewTDDLoopWithConfig(t.virtualStore, t.kernel, t.llmClient, tddConfig)

	// Run TDD loop to completion
	fmt.Printf("[TesterShard:%s] Starting TDD loop for %s\n", t.id, task.Target)

	startTime := time.Now()
	err := t.tddLoop.RunToCompletion(ctx)
	duration := time.Since(startTime)

	// Collect results
	tddState := t.tddLoop.GetState()
	tddDiagnostics := t.tddLoop.GetDiagnostics()
	retryCount := t.tddLoop.GetRetryCount()

	result := &TestResult{
		Framework:   framework,
		Duration:    duration,
		Passed:      tddState == core.TDDStatePassing,
		Retries:     retryCount,
		Diagnostics: tddDiagnostics,
	}

	// Get detailed output
	tddFacts := t.tddLoop.ToFacts()
	var outputLines []string
	outputLines = append(outputLines, fmt.Sprintf("TDD Loop completed in %s", duration))
	outputLines = append(outputLines, fmt.Sprintf("Final state: %s", tddState))
	outputLines = append(outputLines, fmt.Sprintf("Retries: %d/%d", retryCount, maxRetries))

	for _, fact := range tddFacts {
		if fact.Predicate == "diagnostic" && len(fact.Args) >= 5 {
			outputLines = append(outputLines, fmt.Sprintf("  [%v] %v:%v - %v",
				fact.Args[0], fact.Args[1], fact.Args[2], fact.Args[4]))
		}
	}
	result.Output = strings.Join(outputLines, "\n")

	if err != nil {
		result.Output += fmt.Sprintf("\nError: %v", err)
	}

	return result, nil
}

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

	// Call LLM
	response, err := llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("LLM test generation failed: %w", err)
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
// FRAMEWORK DETECTION AND COMMANDS
// =============================================================================

// detectFramework auto-detects the testing framework based on file extension or project files.
func (t *TesterShard) detectFramework(target string) string {
	ext := strings.ToLower(filepath.Ext(target))

	switch ext {
	case ".go":
		return "gotest"
	case ".ts", ".tsx", ".js", ".jsx":
		return "jest"
	case ".py":
		return "pytest"
	case ".rs":
		return "cargo"
	case ".java":
		return "junit"
	case ".cs":
		return "xunit"
	case ".rb":
		return "rspec"
	case ".php":
		return "phpunit"
	case ".swift":
		return "xctest"
	default:
		// Check for project files
		return "gotest" // Default to Go
	}
}

// buildTestCommand builds the test command for the given framework.
func (t *TesterShard) buildTestCommand(framework string, task *TesterTask) string {
	target := task.Target
	if target == "" {
		target = "./..."
	}

	switch framework {
	case "gotest":
		if t.testerConfig.VerboseOutput {
			return fmt.Sprintf("go test -v %s", target)
		}
		return fmt.Sprintf("go test %s", target)
	case "jest":
		return fmt.Sprintf("npx jest %s", target)
	case "pytest":
		return fmt.Sprintf("pytest %s", target)
	case "cargo":
		return "cargo test"
	case "junit":
		return "mvn test"
	case "xunit":
		return "dotnet test"
	case "rspec":
		return fmt.Sprintf("rspec %s", target)
	case "phpunit":
		return "vendor/bin/phpunit"
	default:
		return fmt.Sprintf("go test %s", target)
	}
}

// buildCoverageCommand builds the coverage command for the given framework.
func (t *TesterShard) buildCoverageCommand(framework string, task *TesterTask) string {
	target := task.Target
	if target == "" {
		target = "./..."
	}

	switch framework {
	case "gotest":
		return fmt.Sprintf("go test -cover -coverprofile=coverage.out %s", target)
	case "jest":
		return fmt.Sprintf("npx jest --coverage %s", target)
	case "pytest":
		return fmt.Sprintf("pytest --cov=%s", target)
	case "cargo":
		return "cargo tarpaulin"
	default:
		return fmt.Sprintf("go test -cover %s", target)
	}
}

// buildBuildCommand builds the build command for the given framework.
func (t *TesterShard) buildBuildCommand(framework string) string {
	switch framework {
	case "gotest":
		return "go build ./..."
	case "jest":
		return "npm run build"
	case "pytest":
		return "python -m py_compile"
	case "cargo":
		return "cargo build"
	default:
		return "go build ./..."
	}
}

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
// FACT GENERATION
// =============================================================================

// assertInitialFacts asserts initial facts to the kernel.
func (t *TesterShard) assertInitialFacts(task *TesterTask) {
	if t.kernel == nil {
		return
	}

	_ = t.kernel.Assert(core.Fact{
		Predicate: "tester_task",
		Args:      []interface{}{t.id, "/" + task.Action, task.Target, time.Now().Unix()},
	})

	_ = t.kernel.Assert(core.Fact{
		Predicate: "coverage_goal",
		Args:      []interface{}{t.testerConfig.CoverageGoal},
	})
}

// generateFacts generates facts from test results for propagation.
func (t *TesterShard) generateFacts(result *TestResult) []core.Fact {
	facts := make([]core.Fact, 0)

	// Test state
	stateAtom := "/passing"
	if !result.Passed {
		stateAtom = "/failing"
	}
	facts = append(facts, core.Fact{
		Predicate: "test_state",
		Args:      []interface{}{stateAtom},
	})

	// Test output
	facts = append(facts, core.Fact{
		Predicate: "test_output",
		Args:      []interface{}{truncateString(result.Output, 1000)},
	})

	// Coverage metric
	if result.Coverage > 0 {
		facts = append(facts, core.Fact{
			Predicate: "coverage_metric",
			Args:      []interface{}{result.Coverage},
		})

		// Check against goal
		if result.Coverage < t.testerConfig.CoverageGoal {
			facts = append(facts, core.Fact{
				Predicate: "coverage_below_goal",
				Args:      []interface{}{result.Coverage, t.testerConfig.CoverageGoal},
			})
		}
	}

	// Retry count
	facts = append(facts, core.Fact{
		Predicate: "retry_count",
		Args:      []interface{}{int64(result.Retries)},
	})

	// Failed tests
	for _, failed := range result.FailedTests {
		facts = append(facts, core.Fact{
			Predicate: "failed_test",
			Args:      []interface{}{failed.Name, failed.FilePath, failed.Message},
		})
	}

	// Diagnostics
	for _, diag := range result.Diagnostics {
		facts = append(facts, diag.ToFact())
	}

	// Autopoiesis facts
	t.mu.RLock()
	for pattern, count := range t.failurePatterns {
		if count >= 3 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"avoid_pattern", pattern},
			})
		}
	}
	for pattern, count := range t.successPatterns {
		if count >= 5 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"test_template", pattern},
			})
		}
	}
	t.mu.RUnlock()

	return facts
}

// =============================================================================
// AUTOPOIESIS (SELF-IMPROVEMENT)
// =============================================================================

// trackFailurePattern tracks recurring test failure patterns for Autopoiesis (§8.3).
func (t *TesterShard) trackFailurePattern(result *TestResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, failed := range result.FailedTests {
		// Create pattern key from failure message
		pattern := normalizePattern(failed.Message)
		t.failurePatterns[pattern]++
	}
}

// trackSuccessPattern tracks successful test patterns for Autopoiesis (§8.3).
func (t *TesterShard) trackSuccessPattern(result *TestResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, passed := range result.PassedTests {
		// Create pattern key from test name structure
		pattern := normalizePattern(passed)
		t.successPatterns[pattern]++
	}
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

	sb.WriteString(fmt.Sprintf("%s (%s, %s)\n", status, result.Framework, result.Duration))

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

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// NOTE: hashContent and detectLanguage are defined in coder.go (same package)

// normalizePattern normalizes a string into a pattern key.
func normalizePattern(s string) string {
	// Remove numbers and specific values, keep structure
	re := regexp.MustCompile(`\d+`)
	normalized := re.ReplaceAllString(s, "N")
	// Limit length
	if len(normalized) > 100 {
		normalized = normalized[:100]
	}
	return strings.ToLower(normalized)
}

// truncateString truncates a string to max length with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
