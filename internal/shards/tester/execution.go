package tester

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

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
		TestType:    "unknown",
	}

	// Detect test type from target file
	if task.Target != "" && task.Target != "./..." {
		result.TestType = t.detectTestType(ctx, task.Target)
	}

	// Determine pass/fail
	if err != nil || t.containsFailure(output) {
		result.Passed = false
		result.FailedTests = t.parseFailedTests(output, framework)
		result.Diagnostics = t.parseDiagnostics(output)

		// Check for stale mock errors and attempt regeneration
		if t.isMockError(output) && task.Target != "" {
			fmt.Printf("[TesterShard] Detected mock-related test failure, checking for stale mocks...\n")
			staleMocks, mockErr := t.detectStaleMocks(ctx, task.Target)
			if mockErr == nil && len(staleMocks) > 0 {
				fmt.Printf("[TesterShard] Found %d stale mock(s), attempting regeneration...\n", len(staleMocks))
				for _, interfacePath := range staleMocks {
					regenErr := t.regenerateMock(ctx, interfacePath)
					if regenErr != nil {
						fmt.Printf("[TesterShard] Warning: failed to regenerate mock for %s: %v\n", interfacePath, regenErr)
					}
				}
			}
		}

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
		TestType:  "unknown",
	}

	// Detect test type from target file
	if task.Target != "" && task.Target != "./..." {
		result.TestType = t.detectTestType(ctx, task.Target)
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
		TestType:    "unknown",
	}

	// Detect test type from target file
	if task.Target != "" && task.Target != "./..." {
		result.TestType = t.detectTestType(ctx, task.Target)
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
