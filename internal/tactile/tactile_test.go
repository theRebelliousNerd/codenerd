package tactile

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDirectExecutor_Execute(t *testing.T) {
	executor := NewDirectExecutor()

	// Test simple command
	var cmd Command
	if runtime.GOOS == "windows" {
		cmd = Command{
			Binary:    "cmd",
			Arguments: []string{"/c", "echo", "hello"},
		}
	} else {
		cmd = Command{
			Binary:    "echo",
			Arguments: []string{"hello"},
		}
	}

	result, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Error)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Output(), "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", result.Output())
	}
}

func TestDirectExecutor_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Timeout test unreliable on Windows due to ping not respecting context cancellation")
	}

	executor := NewDirectExecutor()

	// Command that sleeps for longer than timeout
	cmd := Command{
		Binary:    "sleep",
		Arguments: []string{"10"},
		Limits: &ResourceLimits{
			TimeoutMs: 500, // 500ms timeout
		},
	}

	start := time.Now()
	result, err := executor.Execute(context.Background(), cmd)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Killed {
		t.Errorf("Expected command to be killed")
	}

	if !strings.Contains(result.KillReason, "timeout") {
		t.Errorf("Expected kill reason to mention timeout, got: %s", result.KillReason)
	}

	// Should complete quickly (within 2 seconds)
	if elapsed > 2*time.Second {
		t.Errorf("Timeout didn't work, elapsed: %v", elapsed)
	}
}

func TestDirectExecutor_NonZeroExit(t *testing.T) {
	executor := NewDirectExecutor()

	var cmd Command
	if runtime.GOOS == "windows" {
		cmd = Command{
			Binary:    "cmd",
			Arguments: []string{"/c", "exit", "1"},
		}
	} else {
		cmd = Command{
			Binary:    "sh",
			Arguments: []string{"-c", "exit 1"},
		}
	}

	result, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Success should be true (command ran)
	if !result.Success {
		t.Errorf("Expected success=true for non-zero exit, got: %s", result.Error)
	}

	// But exit code should be 1
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}
}

func TestDirectExecutor_InvalidCommand(t *testing.T) {
	executor := NewDirectExecutor()

	cmd := Command{
		Binary: "nonexistent_command_12345",
	}

	result, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute returned error instead of result: %v", err)
	}

	if result.Success {
		t.Errorf("Expected failure for invalid command")
	}

	if result.Error == "" {
		t.Errorf("Expected error message for invalid command")
	}
}

func TestDirectExecutor_WorkingDirectory(t *testing.T) {
	executor := NewDirectExecutor()

	// Get temp directory
	tempDir := os.TempDir()

	var cmd Command
	if runtime.GOOS == "windows" {
		cmd = Command{
			Binary:           "cmd",
			Arguments:        []string{"/c", "cd"},
			WorkingDirectory: tempDir,
		}
	} else {
		cmd = Command{
			Binary:           "pwd",
			WorkingDirectory: tempDir,
		}
	}

	result, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got: %s", result.Error)
	}

	// Output should contain the temp directory path
	output := strings.TrimSpace(result.Output())
	if !strings.Contains(strings.ToLower(output), strings.ToLower(strings.TrimSuffix(tempDir, string(os.PathSeparator)))) {
		t.Errorf("Expected output to contain %s, got: %s", tempDir, output)
	}
}

func TestDirectExecutor_OutputCapture(t *testing.T) {
	executor := NewDirectExecutor()

	var cmd Command
	if runtime.GOOS == "windows" {
		cmd = Command{
			Binary:    "cmd",
			Arguments: []string{"/c", "echo", "stdout"},
		}
	} else {
		cmd = Command{
			Binary:    "sh",
			Arguments: []string{"-c", "echo stdout; echo stderr >&2"},
		}
	}

	result, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success")
	}

	if !strings.Contains(result.Stdout, "stdout") {
		t.Errorf("Expected stdout to contain 'stdout', got: %s", result.Stdout)
	}

	// On Windows with echo, stderr is empty
	if runtime.GOOS != "windows" {
		if !strings.Contains(result.Stderr, "stderr") {
			t.Errorf("Expected stderr to contain 'stderr', got: %s", result.Stderr)
		}
	}
}

func TestDirectExecutor_OutputTruncation(t *testing.T) {
	config := DefaultExecutorConfig()
	config.MaxOutputBytes = 50 // Very small limit
	config.DefaultLimits.MaxOutputBytes = 50
	executor := NewDirectExecutorWithConfig(config)

	// Generate output larger than the limit by repeating a string
	var cmd Command
	if runtime.GOOS == "windows" {
		// Windows: use cmd to echo a long repeated string
		cmd = Command{
			Binary:    "cmd",
			Arguments: []string{"/c", "echo AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		}
	} else {
		cmd = Command{
			Binary:    "sh",
			Arguments: []string{"-c", "echo AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		}
	}

	result, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Truncated {
		t.Errorf("Expected output to be truncated, got output of len=%d, combined=%d", len(result.Stdout), len(result.Combined))
	}

	if result.TruncatedBytes == 0 {
		t.Errorf("Expected truncated bytes > 0")
	}
}

func TestDirectExecutor_ContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Context cancellation test unreliable on Windows due to ping not respecting context cancellation")
	}

	executor := NewDirectExecutor()

	ctx, cancel := context.WithCancel(context.Background())

	cmd := Command{
		Binary:    "sleep",
		Arguments: []string{"10"},
	}

	// Cancel after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, err := executor.Execute(ctx, cmd)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Killed {
		t.Errorf("Expected command to be killed")
	}

	if !strings.Contains(result.KillReason, "canceled") {
		t.Errorf("Expected kill reason to mention canceled, got: %s", result.KillReason)
	}

	if elapsed > 2*time.Second {
		t.Errorf("Cancellation didn't work quickly, elapsed: %v", elapsed)
	}
}

func TestDirectExecutor_Capabilities(t *testing.T) {
	executor := NewDirectExecutor()
	caps := executor.Capabilities()

	if caps.Name != "direct" {
		t.Errorf("Expected name 'direct', got: %s", caps.Name)
	}

	if caps.Platform != runtime.GOOS {
		t.Errorf("Expected platform %s, got: %s", runtime.GOOS, caps.Platform)
	}

	if !caps.SupportsStdin {
		t.Errorf("Expected stdin support")
	}

	// Should support SandboxNone
	found := false
	for _, mode := range caps.SupportedSandboxModes {
		if mode == SandboxNone {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected SandboxNone in supported modes")
	}
}

func TestDirectExecutor_Validate(t *testing.T) {
	executor := NewDirectExecutor()

	// Valid command
	cmd := Command{Binary: "echo"}
	if err := executor.Validate(cmd); err != nil {
		t.Errorf("Expected valid command to pass validation: %v", err)
	}

	// Invalid: empty binary
	cmd = Command{Binary: ""}
	if err := executor.Validate(cmd); err == nil {
		t.Errorf("Expected empty binary to fail validation")
	}

	// Invalid: wrong sandbox mode
	cmd = Command{
		Binary:  "echo",
		Sandbox: &SandboxConfig{Mode: SandboxDocker},
	}
	if err := executor.Validate(cmd); err == nil {
		t.Errorf("Expected Docker sandbox to fail validation on DirectExecutor")
	}
}

func TestCommand_CommandString(t *testing.T) {
	cmd := Command{
		Binary:    "git",
		Arguments: []string{"commit", "-m", "test message"},
	}

	str := cmd.CommandString()
	if str != "git commit -m test message" {
		t.Errorf("Unexpected command string: %s", str)
	}

	// Empty arguments
	cmd = Command{Binary: "ls"}
	if cmd.CommandString() != "ls" {
		t.Errorf("Unexpected command string for no args: %s", cmd.CommandString())
	}
}

func TestExecutionResult_Helpers(t *testing.T) {
	// Test IsError
	result := &ExecutionResult{Success: true}
	if result.IsError() {
		t.Errorf("Expected IsError=false for successful result")
	}

	result = &ExecutionResult{Success: false, Error: "something failed"}
	if !result.IsError() {
		t.Errorf("Expected IsError=true for failed result")
	}

	// Test IsNonZeroExit
	result = &ExecutionResult{Success: true, ExitCode: 0}
	if result.IsNonZeroExit() {
		t.Errorf("Expected IsNonZeroExit=false for exit code 0")
	}

	result = &ExecutionResult{Success: true, ExitCode: 1}
	if !result.IsNonZeroExit() {
		t.Errorf("Expected IsNonZeroExit=true for exit code 1")
	}

	// Test Output
	result = &ExecutionResult{Combined: "combined output"}
	if result.Output() != "combined output" {
		t.Errorf("Expected Output to return Combined")
	}

	result = &ExecutionResult{Stdout: "stdout", Stderr: "stderr"}
	output := result.Output()
	if !strings.Contains(output, "stdout") || !strings.Contains(output, "stderr") {
		t.Errorf("Expected Output to contain both stdout and stderr, got: %s", output)
	}
}

func TestResourceUsage_TotalCPUTimeMs(t *testing.T) {
	usage := &ResourceUsage{
		UserTimeMs:   100,
		SystemTimeMs: 50,
	}

	if usage.TotalCPUTimeMs() != 150 {
		t.Errorf("Expected total CPU time 150, got: %d", usage.TotalCPUTimeMs())
	}
}

func TestShellCommand_ToCommand(t *testing.T) {
	legacy := ShellCommand{
		Binary:           "git",
		Arguments:        []string{"status"},
		WorkingDirectory: "/tmp",
		TimeoutSeconds:   30,
		EnvironmentVars:  []string{"GIT_DIR=/path"},
	}

	cmd := legacy.ToCommand()

	if cmd.Binary != "git" {
		t.Errorf("Expected binary 'git', got: %s", cmd.Binary)
	}

	if len(cmd.Arguments) != 1 || cmd.Arguments[0] != "status" {
		t.Errorf("Unexpected arguments: %v", cmd.Arguments)
	}

	if cmd.WorkingDirectory != "/tmp" {
		t.Errorf("Expected working dir '/tmp', got: %s", cmd.WorkingDirectory)
	}

	if cmd.Limits == nil || cmd.Limits.TimeoutMs != 30000 {
		t.Errorf("Expected timeout 30000ms, got: %v", cmd.Limits)
	}

	if len(cmd.Environment) != 1 || cmd.Environment[0] != "GIT_DIR=/path" {
		t.Errorf("Unexpected environment: %v", cmd.Environment)
	}
}

func TestExecutorConfig_Merge(t *testing.T) {
	config := DefaultExecutorConfig()
	config.DefaultWorkingDir = "/default"
	config.DefaultTimeout = 60 * time.Second
	config.MaxTimeout = 5 * time.Minute

	// Command with no values should get defaults
	cmd := Command{Binary: "echo"}
	merged := config.Merge(cmd)

	if merged.WorkingDirectory != "/default" {
		t.Errorf("Expected default working dir, got: %s", merged.WorkingDirectory)
	}

	// Command with values should keep them
	cmd = Command{
		Binary:           "echo",
		WorkingDirectory: "/custom",
		Limits:           &ResourceLimits{TimeoutMs: 10000},
	}
	merged = config.Merge(cmd)

	if merged.WorkingDirectory != "/custom" {
		t.Errorf("Expected custom working dir, got: %s", merged.WorkingDirectory)
	}

	// Timeout should not exceed max
	cmd = Command{
		Binary: "echo",
		Limits: &ResourceLimits{TimeoutMs: 600000}, // 10 minutes
	}
	merged = config.Merge(cmd)

	if merged.Limits.TimeoutMs > int64(config.MaxTimeout/time.Millisecond) {
		t.Errorf("Timeout should be capped at max, got: %d", merged.Limits.TimeoutMs)
	}
}

func TestAuditEvent_ToFacts(t *testing.T) {
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command: Command{
			Binary:    "go",
			Arguments: []string{"test"},
			RequestID: "req-123",
			SessionID: "session-456",
		},
		Result: &ExecutionResult{
			Success:     true,
			ExitCode:    0,
			Duration:    100 * time.Millisecond,
			Stdout:      "PASS",
			SandboxUsed: SandboxNone,
		},
		SessionID:    "session-456",
		ExecutorName: "direct",
	}

	facts := event.ToFacts()

	if len(facts) == 0 {
		t.Errorf("Expected facts to be generated")
	}

	// Check for specific facts
	hasCompleted := false
	hasSuccess := false
	for _, fact := range facts {
		if fact.Predicate == "execution_completed" {
			hasCompleted = true
		}
		if fact.Predicate == "execution_success" {
			hasSuccess = true
		}
	}

	if !hasCompleted {
		t.Errorf("Expected execution_completed fact")
	}

	if !hasSuccess {
		t.Errorf("Expected execution_success fact")
	}
}

func TestAuditLogger(t *testing.T) {
	logger := NewAuditLogger()

	// Track callback invocations
	var events []AuditEvent
	logger.AddCallback(func(e AuditEvent) {
		events = append(events, e)
	})

	// Track facts
	var facts []Fact
	logger.SetFactCallback(func(f Fact) {
		facts = append(facts, f)
	})

	// Log an event
	event := AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command: Command{
			Binary:    "test",
			RequestID: "req-1",
		},
		ExecutorName: "test",
	}
	logger.Log(event)

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got: %d", len(events))
	}

	if len(facts) == 0 {
		t.Errorf("Expected facts to be generated")
	}

	// Check metrics
	metrics := logger.GetMetrics()
	if metrics.TotalExecutions != 1 {
		t.Errorf("Expected 1 total execution, got: %d", metrics.TotalExecutions)
	}
}

func TestCompositeExecutor(t *testing.T) {
	composite := NewCompositeExecutor()
	caps := composite.Capabilities()

	if caps.Name != "composite" {
		t.Errorf("Expected name 'composite', got: %s", caps.Name)
	}

	// Should have at least SandboxNone
	found := false
	for _, mode := range caps.SupportedSandboxModes {
		if mode == SandboxNone {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected SandboxNone in supported modes")
	}

	// Execute a simple command
	var cmd Command
	if runtime.GOOS == "windows" {
		cmd = Command{
			Binary:    "cmd",
			Arguments: []string{"/c", "echo", "composite"},
		}
	} else {
		cmd = Command{
			Binary:    "echo",
			Arguments: []string{"composite"},
		}
	}

	result, err := composite.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success")
	}

	if !strings.Contains(result.Output(), "composite") {
		t.Errorf("Expected output to contain 'composite'")
	}
}

func TestExecutorFactory(t *testing.T) {
	factory := NewDefaultFactory()

	// Create direct executor
	direct := factory.CreateDirect()
	if direct == nil {
		t.Errorf("Expected direct executor")
	}

	caps := direct.Capabilities()
	if caps.Name != "direct" {
		t.Errorf("Expected 'direct' executor")
	}

	// Create composite
	composite := factory.CreateComposite()
	if composite == nil {
		t.Errorf("Expected composite executor")
	}

	// Create best
	best := factory.CreateBest()
	if best == nil {
		t.Errorf("Expected best executor")
	}
}

func TestOutputAnalyzer_TestOutput(t *testing.T) {
	analyzer := NewOutputAnalyzer()

	// Go test output
	output := `=== RUN   TestFoo
--- PASS: TestFoo (0.00s)
=== RUN   TestBar
--- FAIL: TestBar (0.01s)
=== RUN   TestBaz
--- SKIP: TestBaz (0.00s)
FAIL
coverage: 75.5% of statements`

	analysis := analyzer.AnalyzeTestOutput(output)

	if analysis.Passed != 1 {
		t.Errorf("Expected 1 passed, got: %d", analysis.Passed)
	}

	if analysis.Failed != 1 {
		t.Errorf("Expected 1 failed, got: %d", analysis.Failed)
	}

	if analysis.Skipped != 1 {
		t.Errorf("Expected 1 skipped, got: %d", analysis.Skipped)
	}

	if analysis.OverallPass {
		t.Errorf("Expected overall fail")
	}

	if len(analysis.FailedTests) != 1 || analysis.FailedTests[0] != "TestBar" {
		t.Errorf("Expected TestBar in failed tests, got: %v", analysis.FailedTests)
	}

	if analysis.Coverage != 75.5 {
		t.Errorf("Expected coverage 75.5, got: %f", analysis.Coverage)
	}
}

func TestOutputAnalyzer_BuildOutput(t *testing.T) {
	analyzer := NewOutputAnalyzer()

	output := `main.go:10:5: undefined: foo
main.go:15:10: cannot use x (type int) as type string`

	analysis := analyzer.AnalyzeBuildOutput(output)

	if analysis.Success {
		t.Errorf("Expected build failure")
	}

	if analysis.Errors != 2 {
		t.Errorf("Expected 2 errors, got: %d", analysis.Errors)
	}

	if len(analysis.Diagnostics) != 2 {
		t.Errorf("Expected 2 diagnostics, got: %d", len(analysis.Diagnostics))
	}

	if analysis.Diagnostics[0].File != "main.go" {
		t.Errorf("Expected file 'main.go', got: %s", analysis.Diagnostics[0].File)
	}
}

func TestPooledExecutor(t *testing.T) {
	config := DefaultExecutorConfig()
	pool := NewPooledExecutor(config, 5)

	// Execute some commands
	var cmd Command
	if runtime.GOOS == "windows" {
		cmd = Command{
			Binary:    "cmd",
			Arguments: []string{"/c", "echo", "pooled"},
		}
	} else {
		cmd = Command{
			Binary:    "echo",
			Arguments: []string{"pooled"},
		}
	}

	result, err := pool.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success")
	}

	// Check stats
	stats := pool.Stats()
	if stats["borrowed"] < 1 {
		t.Errorf("Expected at least 1 borrow")
	}
	if stats["returned"] < 1 {
		t.Errorf("Expected at least 1 return")
	}
}

func TestRetryExecutor(t *testing.T) {
	direct := NewDirectExecutor()
	retry := NewRetryExecutor(direct, 2)

	// Successful command should work first try
	var cmd Command
	if runtime.GOOS == "windows" {
		cmd = Command{
			Binary:    "cmd",
			Arguments: []string{"/c", "echo", "retry"},
		}
	} else {
		cmd = Command{
			Binary:    "echo",
			Arguments: []string{"retry"},
		}
	}

	result, err := retry.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success")
	}
}
