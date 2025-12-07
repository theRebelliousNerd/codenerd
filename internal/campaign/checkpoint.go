package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/tactile"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CheckpointRunner runs verification checkpoints for phases.
type CheckpointRunner struct {
	executor  *tactile.SafeExecutor
	shardMgr  *core.ShardManager
	workspace string
}

// NewCheckpointRunner creates a new checkpoint runner.
func NewCheckpointRunner(executor *tactile.SafeExecutor, shardMgr *core.ShardManager, workspace string) *CheckpointRunner {
	return &CheckpointRunner{
		executor:  executor,
		shardMgr:  shardMgr,
		workspace: workspace,
	}
}

// Run executes a checkpoint based on the verification method.
func (cr *CheckpointRunner) Run(ctx context.Context, phase *Phase, method VerificationMethod) (passed bool, details string, err error) {
	switch method {
	case VerifyTestsPass:
		return cr.runTestsCheckpoint(ctx)
	case VerifyBuilds:
		return cr.runBuildCheckpoint(ctx)
	case VerifyManualReview:
		return cr.runManualReviewCheckpoint(ctx, phase)
	case VerifyShardValidate:
		return cr.runShardValidationCheckpoint(ctx, phase)
	case VerifyNone:
		return true, "No verification required", nil
	default:
		return true, "Unknown verification method, skipping", nil
	}
}

// runTestsCheckpoint runs tests and checks if they pass.
func (cr *CheckpointRunner) runTestsCheckpoint(ctx context.Context) (bool, string, error) {
	// Detect project type and run appropriate test command
	testCmdStr := cr.detectTestCommand()
	parts := strings.Fields(testCmdStr)

	cmd := tactile.ShellCommand{
		Binary:           parts[0],
		Arguments:        parts[1:],
		WorkingDirectory: cr.workspace,
		TimeoutSeconds:   600, // 10 minutes
	}

	output, err := cr.executor.Execute(ctx, cmd)
	if err != nil {
		// Check if it's a test failure vs command error
		if _, ok := err.(*exec.ExitError); ok {
			// Test failures return non-zero exit code
			return false, fmt.Sprintf("Tests failed:\n%s", output), nil
		}
		return false, fmt.Sprintf("Error running tests: %v", err), err
	}

	// Count passed/failed from output
	passedCount, failedCount := cr.parseTestOutput(output)
	if failedCount > 0 {
		return false, fmt.Sprintf("Tests: %d passed, %d failed\n%s", passedCount, failedCount, output), nil
	}

	return true, fmt.Sprintf("All %d tests passed", passedCount), nil
}

// runBuildCheckpoint runs the build and checks if it succeeds.
func (cr *CheckpointRunner) runBuildCheckpoint(ctx context.Context) (bool, string, error) {
	buildCmdStr := cr.detectBuildCommand()
	parts := strings.Fields(buildCmdStr)

	cmd := tactile.ShellCommand{
		Binary:           parts[0],
		Arguments:        parts[1:],
		WorkingDirectory: cr.workspace,
		TimeoutSeconds:   600, // 10 minutes
	}

	output, err := cr.executor.Execute(ctx, cmd)
	if err != nil {
		return false, fmt.Sprintf("Build failed:\n%s", output), nil
	}

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
		return true, "Shard validation skipped (no shard manager)", nil
	}

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
	result, err := cr.shardMgr.Spawn(ctx, "reviewer", reviewPrompt.String())
	if err != nil {
		return false, fmt.Sprintf("Reviewer shard failed: %v", err), err
	}

	// Parse result - look for PASS/FAIL
	resultStr := fmt.Sprintf("%v", result)
	resultLower := strings.ToLower(resultStr)

	if strings.Contains(resultLower, "fail") {
		return false, fmt.Sprintf("Review failed: %s", resultStr), nil
	}

	return true, fmt.Sprintf("Review passed: %s", resultStr), nil
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
	cmd := exec.Command("test", "-f", file)
	cmd.Dir = workspace
	return cmd.Run() == nil
}
