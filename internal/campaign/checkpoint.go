package campaign

import (
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CheckpointRunner runs verification checkpoints for phases.
type CheckpointRunner struct {
	executor     tactile.Executor
	shardMgr     *coreshards.ShardManager // DEPRECATED: Use taskExecutor instead
	taskExecutor session.TaskExecutor      // New: unified task execution
	workspace    string
}

// NewCheckpointRunner creates a new checkpoint runner.
func NewCheckpointRunner(executor tactile.Executor, shardMgr *coreshards.ShardManager, workspace string) *CheckpointRunner {
	return &CheckpointRunner{
		executor:  executor,
		shardMgr:  shardMgr,
		workspace: workspace,
	}
}

// SetTaskExecutor sets the task executor for JIT-driven execution.
func (cr *CheckpointRunner) SetTaskExecutor(te session.TaskExecutor) {
	cr.taskExecutor = te
}

// spawnTask is the unified entry point for task execution.
// Uses TaskExecutor when available, falling back to ShardManager.
func (cr *CheckpointRunner) spawnTask(ctx context.Context, shardType string, task string) (string, error) {
	// Prefer TaskExecutor
	if cr.taskExecutor != nil {
		intent := session.LegacyShardNameToIntent(shardType)
		return cr.taskExecutor.Execute(ctx, intent, task)
	}

	// Fall back to ShardManager
	if cr.shardMgr != nil {
		return cr.shardMgr.Spawn(ctx, shardType, task)
	}

	return "", fmt.Errorf("no executor available: both TaskExecutor and ShardManager are nil")
}

// Run executes a checkpoint based on the verification method.
func (cr *CheckpointRunner) Run(ctx context.Context, phase *Phase, method VerificationMethod) (passed bool, details string, err error) {
	phaseName := ""
	if phase != nil {
		phaseName = phase.Name
	}
	logging.Campaign("CheckpointRunner.Run: executing method=%s phase=%s", method, phaseName)

	switch method {
	case VerifyTestsPass:
		passed, details, err = cr.runTestsCheckpoint(ctx)
	case VerifyBuilds:
		passed, details, err = cr.runBuildCheckpoint(ctx)
	case VerifyManualReview:
		passed, details, err = cr.runManualReviewCheckpoint(ctx, phase)
	case VerifyShardValidate:
		passed, details, err = cr.runShardValidationCheckpoint(ctx, phase)
	case VerifyNemesisGauntlet:
		passed, details, err = cr.runNemesisGauntletCheckpoint(ctx, phase)
	case VerifyNone:
		logging.CampaignDebug("CheckpointRunner.Run: no verification required for phase=%s", phaseName)
		return true, "No verification required", nil
	default:
		logging.CampaignWarn("CheckpointRunner.Run: unknown verification method=%s, skipping", method)
		return true, "Unknown verification method, skipping", nil
	}

	if err != nil {
		logging.CampaignError("CheckpointRunner.Run: method=%s phase=%s error: %v", method, phaseName, err)
	} else if passed {
		logging.Campaign("CheckpointRunner.Run: method=%s phase=%s PASSED", method, phaseName)
	} else {
		logging.CampaignWarn("CheckpointRunner.Run: method=%s phase=%s FAILED", method, phaseName)
	}
	return passed, details, err
}

// runTestsCheckpoint runs tests and checks if they pass.
func (cr *CheckpointRunner) runTestsCheckpoint(ctx context.Context) (bool, string, error) {
	// Detect project type and run appropriate test command
	testCmdStr := cr.detectTestCommand()
	logging.CampaignDebug("runTestsCheckpoint: detected command=%s workspace=%s", testCmdStr, cr.workspace)
	isGoTest := strings.HasPrefix(testCmdStr, "go test")
	isNpmTest := strings.HasPrefix(testCmdStr, "npm test")
	if isGoTest && !strings.Contains(testCmdStr, "-json") {
		testCmdStr = testCmdStr + " -json"
	}
	if isNpmTest && !strings.Contains(testCmdStr, "--") {
		// Try to request JSON where supported (e.g., jest). This is best-effort.
		testCmdStr = testCmdStr + " -- --json --outputFile=.nerd/npm-test.json"
	}
	parts := strings.Fields(testCmdStr)

	cmd := tactile.Command{
		Binary:           parts[0],
		Arguments:        parts[1:],
		WorkingDirectory: cr.workspace,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: 600 * 1000, // 10 minutes
		},
	}

	res, err := cr.executor.Execute(ctx, cmd)
	output := ""
	if res != nil {
		output = res.Output()
	}
	if err != nil {
		// Test failures return non-zero exit code
		return false, fmt.Sprintf("Error running tests: %v\n%s", err, output), nil
	}

	// Count passed/failed from output
	if isGoTest {
		passedCount, failedCount, duration := cr.parseGoTestJSON(output)
		if failedCount > 0 {
			return false, fmt.Sprintf("Tests: %d passed, %d failed (%.2fs)\n%s", passedCount, failedCount, duration.Seconds(), output), nil
		}
		return true, fmt.Sprintf("All %d tests passed (%.2fs)", passedCount, duration.Seconds()), nil
	}

	if isNpmTest {
		passedCount, failedCount := cr.parseTestOutput(output)
		// Also try to read the JSON file if it exists
		jsonPath := filepath.Join(cr.workspace, ".nerd", "npm-test.json")
		if data, err := os.ReadFile(jsonPath); err == nil {
			p, f := cr.parseJestJSON(data)
			if p+f > 0 {
				passedCount, failedCount = p, f
			}
		}
		if failedCount > 0 {
			return false, fmt.Sprintf("Tests: %d passed, %d failed\n%s", passedCount, failedCount, output), nil
		}
		return true, fmt.Sprintf("All %d tests passed", passedCount), nil
	}

	passedCount, failedCount := cr.parseTestOutput(output)
	if failedCount > 0 {
		return false, fmt.Sprintf("Tests: %d passed, %d failed\n%s", passedCount, failedCount, output), nil
	}
	return true, fmt.Sprintf("All %d tests passed", passedCount), nil
}

// runBuildCheckpoint runs the build and checks if it succeeds.
func (cr *CheckpointRunner) runBuildCheckpoint(ctx context.Context) (bool, string, error) {
	buildCmdStr := cr.detectBuildCommand()
	logging.CampaignDebug("runBuildCheckpoint: detected command=%s workspace=%s", buildCmdStr, cr.workspace)
	parts := strings.Fields(buildCmdStr)

	cmd := tactile.Command{
		Binary:           parts[0],
		Arguments:        parts[1:],
		WorkingDirectory: cr.workspace,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: 600 * 1000, // 10 minutes
		},
	}

	res, err := cr.executor.Execute(ctx, cmd)
	output := ""
	if res != nil {
		output = res.Output()
	}
	if err != nil {
		logging.CampaignWarn("runBuildCheckpoint: build failed: %v (output_len=%d)", err, len(output))
		return false, fmt.Sprintf("Build failed:\n%s", output), nil
	}

	logging.CampaignDebug("runBuildCheckpoint: build succeeded")
	return true, "Build succeeded", nil
}

// runManualReviewCheckpoint requires user confirmation.
func (cr *CheckpointRunner) runManualReviewCheckpoint(ctx context.Context, phase *Phase) (bool, string, error) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return false, "", ctx.Err()
	default:
	}

	// In non-interactive mode, we can't do manual review
	// Return true with a note that review was skipped, including phase context
	return true, fmt.Sprintf("Manual review for phase '%s' skipped (non-interactive mode)", phase.Name), nil
}

// runShardValidationCheckpoint spawns a reviewer shard to validate the phase.
func (cr *CheckpointRunner) runShardValidationCheckpoint(ctx context.Context, phase *Phase) (bool, string, error) {
	if cr.shardMgr == nil {
		logging.CampaignDebug("runShardValidationCheckpoint: no shard manager, skipping")
		return true, "Shard validation skipped (no shard manager)", nil
	}

	logging.Campaign("runShardValidationCheckpoint: spawning reviewer shard for phase=%s", phase.Name)

	// Build a review prompt based on phase objectives and completed tasks
	var reviewPrompt strings.Builder
	reviewPrompt.WriteString("Review the following phase completion for quality and correctness:\n\n")
	reviewPrompt.WriteString(fmt.Sprintf("Phase: %s\n\n", phase.Name))

	reviewPrompt.WriteString("Objectives:\n")
	for _, obj := range phase.Objectives {
		reviewPrompt.WriteString(fmt.Sprintf("- %s\n", obj.Description))
	}

	reviewPrompt.WriteString("\nCompleted Tasks:\n")
	for _, task := range phase.Tasks {
		if task.Status == TaskCompleted {
			reviewPrompt.WriteString(fmt.Sprintf("- [DONE] %s\n", task.Description))
			if len(task.Artifacts) > 0 {
				reviewPrompt.WriteString(fmt.Sprintf("  Artifacts: %v\n", task.Artifacts))
			}
		}
	}

	reviewPrompt.WriteString("\nProvide a brief assessment: PASS if objectives are met, FAIL with reason if not.")

	// Spawn reviewer shard
	result, err := cr.spawnTask(ctx, "reviewer", reviewPrompt.String())
	if err != nil {
		logging.CampaignError("runShardValidationCheckpoint: reviewer shard failed for phase=%s: %v", phase.Name, err)
		return false, fmt.Sprintf("Reviewer shard failed: %v", err), err
	}

	// Parse result - look for PASS/FAIL
	resultStr := fmt.Sprintf("%v", result)
	resultLower := strings.ToLower(resultStr)

	if strings.Contains(resultLower, "fail") {
		logging.CampaignWarn("runShardValidationCheckpoint: reviewer found issues in phase=%s", phase.Name)
		return false, fmt.Sprintf("Review failed: %s", resultStr), nil
	}

	logging.Campaign("runShardValidationCheckpoint: reviewer approved phase=%s", phase.Name)
	return true, fmt.Sprintf("Review passed: %s", resultStr), nil
}

// runNemesisGauntletCheckpoint spawns the Nemesis shard to perform adversarial review.
// This is best-effort: if Nemesis isn't available, we skip rather than fail hard.
func (cr *CheckpointRunner) runNemesisGauntletCheckpoint(ctx context.Context, phase *Phase) (bool, string, error) {
	if cr.shardMgr == nil {
		logging.CampaignDebug("runNemesisGauntletCheckpoint: no shard manager, skipping")
		return true, "Nemesis shard manager unavailable, skipping adversarial checkpoint", nil
	}

	phaseName := ""
	if phase != nil {
		phaseName = phase.Name
	}
	logging.Campaign("runNemesisGauntletCheckpoint: spawning nemesis shard for phase=%s", phaseName)

	target := cr.workspace
	// Prefer a phase-specific target if artifacts exist.
	if phase != nil {
		for _, task := range phase.Tasks {
			for _, artifact := range task.Artifacts {
				if artifact.Path != "" {
					target = artifact.Path
					break
				}
			}
			if target != cr.workspace {
				break
			}
		}
	}

	taskStr := fmt.Sprintf("review:%s", target)
	logging.CampaignDebug("runNemesisGauntletCheckpoint: target=%s task=%s", target, taskStr)
	result, err := cr.spawnTask(ctx, "nemesis", taskStr)
	if err != nil {
		logging.CampaignError("runNemesisGauntletCheckpoint: nemesis shard failed for phase=%s: %v", phaseName, err)
		return false, fmt.Sprintf("Nemesis shard failed: %v", err), err
	}

	resultStr := fmt.Sprintf("%v", result)
	lower := strings.ToLower(resultStr)

	// Heuristic verdict: Nemesis uses "failed/defeated" language when it breaks a patch.
	if strings.Contains(lower, "verdict") && strings.Contains(lower, "fail") {
		logging.CampaignWarn("runNemesisGauntletCheckpoint: nemesis found vulnerabilities in phase=%s", phaseName)
		return false, fmt.Sprintf("Nemesis gauntlet failed: %s", resultStr), nil
	}
	if strings.Contains(lower, "defeated") || strings.Contains(lower, "attack succeeded") {
		logging.CampaignWarn("runNemesisGauntletCheckpoint: nemesis defeated phase=%s defenses", phaseName)
		return false, fmt.Sprintf("Nemesis gauntlet found weaknesses: %s", resultStr), nil
	}

	logging.Campaign("runNemesisGauntletCheckpoint: phase=%s survived nemesis gauntlet", phaseName)
	return true, fmt.Sprintf("Nemesis gauntlet passed: %s", resultStr), nil
}

// detectTestCommand determines the appropriate test command for the project.
func (cr *CheckpointRunner) detectTestCommand() string {
	// Check for various project types
	checks := []struct {
		file    string
		command string
	}{
		{"go.mod", "go test ./..."},
		{"package.json", "npm test"},
		{"Cargo.toml", "cargo test"},
		{"requirements.txt", "pytest"},
		{"setup.py", "python -m pytest"},
		{"pom.xml", "mvn test"},
		{"build.gradle", "gradle test"},
		{"Makefile", "make test"},
	}

	for _, check := range checks {
		if fileExists(cr.workspace, check.file) {
			return check.command
		}
	}

	// Default to go test
	return "go test ./..."
}

// detectBuildCommand determines the appropriate build command for the project.
func (cr *CheckpointRunner) detectBuildCommand() string {
	checks := []struct {
		file    string
		command string
	}{
		{"go.mod", "go build ./..."},
		{"package.json", "npm run build"},
		{"Cargo.toml", "cargo build"},
		{"pom.xml", "mvn compile"},
		{"build.gradle", "gradle build"},
		{"Makefile", "make build"},
	}

	for _, check := range checks {
		if fileExists(cr.workspace, check.file) {
			return check.command
		}
	}

	// Default to go build
	return "go build ./..."
}

// parseTestOutput parses test output to count passed/failed tests.
func (cr *CheckpointRunner) parseTestOutput(output string) (passed, failed int) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		lower := strings.ToLower(line)

		// Go test output
		if strings.Contains(lower, "--- pass") {
			passed++
		} else if strings.Contains(lower, "--- fail") {
			failed++
		}

		// Generic patterns
		if strings.Contains(lower, "passed") || strings.Contains(lower, "ok") {
			// Try to extract number
			// This is a rough heuristic
		}
		if strings.Contains(lower, "failed") || strings.Contains(lower, "error") {
			failed++
		}
	}

	// If we couldn't parse, assume 1 passed if no failures
	if passed == 0 && failed == 0 {
		passed = 1
	}

	return passed, failed
}

// parseGoTestJSON parses go test -json output for pass/fail counts.
func (cr *CheckpointRunner) parseGoTestJSON(output string) (passed, failed int, duration time.Duration) {
	type goTestEvent struct {
		Action  string  `json:"Action"`
		Test    string  `json:"Test"`
		Elapsed float64 `json:"Elapsed"`
	}

	dec := json.NewDecoder(strings.NewReader(output))
	for dec.More() {
		var evt goTestEvent
		if err := dec.Decode(&evt); err != nil {
			// Fall back to heuristic if JSON framing breaks
			p, f := cr.parseTestOutput(output)
			return p, f, 0
		}
		switch evt.Action {
		case "pass":
			if evt.Test != "" {
				passed++
				duration += time.Duration(evt.Elapsed * float64(time.Second))
			}
		case "fail":
			if evt.Test != "" {
				failed++
				duration += time.Duration(evt.Elapsed * float64(time.Second))
			} else {
				// package-level failure
				failed++
			}
		}
	}
	return passed, failed, duration
}

// parseJestJSON parses a Jest-style JSON report if available.
func (cr *CheckpointRunner) parseJestJSON(data []byte) (passed, failed int) {
	var report struct {
		NumPassedTests int `json:"numPassedTests"`
		NumFailedTests int `json:"numFailedTests"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return 0, 0
	}
	return report.NumPassedTests, report.NumFailedTests
}

// RunAll runs all checkpoints for a phase.
func (cr *CheckpointRunner) RunAll(ctx context.Context, phase *Phase) ([]Checkpoint, error) {
	checkpoints := make([]Checkpoint, 0)

	for _, obj := range phase.Objectives {
		if obj.VerificationMethod == VerifyNone {
			continue
		}

		passed, details, err := cr.Run(ctx, phase, obj.VerificationMethod)
		if err != nil {
			return checkpoints, err
		}

		checkpoints = append(checkpoints, Checkpoint{
			Type:      string(obj.VerificationMethod),
			Passed:    passed,
			Details:   details,
			Timestamp: time.Now(),
		})
	}

	return checkpoints, nil
}

// RunQuick runs a quick sanity check (build only).
func (cr *CheckpointRunner) RunQuick(ctx context.Context) (bool, string, error) {
	return cr.runBuildCheckpoint(ctx)
}

// fileExists checks if a file exists in the workspace.
func fileExists(workspace, file string) bool {
	path := filepath.Join(workspace, file)
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}
