package swebench

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
)

// =============================================================================
// SWE-BENCH ENVIRONMENT
// =============================================================================
// Manages the lifecycle of a single SWE-bench instance evaluation:
// 1. Create container with appropriate Python version
// 2. Clone repository at base commit
// 3. Set up virtual environment and install dependencies
// 4. Apply model's patch
// 5. Run tests and collect results
// 6. Clean up or snapshot for debugging

// EnvironmentState tracks the state of a SWE-bench environment.
type EnvironmentState string

const (
	EnvStateInitializing EnvironmentState = "initializing"
	EnvStateCloning      EnvironmentState = "cloning"
	EnvStateCheckout     EnvironmentState = "checkout"
	EnvStateSetup        EnvironmentState = "setup"
	EnvStateReady        EnvironmentState = "ready"
	EnvStatePatchApplied EnvironmentState = "patch_applied"
	EnvStateTesting      EnvironmentState = "testing"
	EnvStateError        EnvironmentState = "error"
	EnvStateComplete     EnvironmentState = "complete"
)

// EnvironmentConfig configures environment behavior.
type EnvironmentConfig struct {
	// Container settings
	BaseImage      string        `json:"base_image"`       // Override Docker image
	PythonVersion  string        `json:"python_version"`   // e.g., "3.10"
	MemoryLimit    int64         `json:"memory_limit"`     // Bytes
	CPULimit       float64       `json:"cpu_limit"`        // CPUs
	NetworkEnabled bool          `json:"network_enabled"`  // For pip install
	TestTimeout    time.Duration `json:"test_timeout"`     // Per-test timeout
	SetupTimeout   time.Duration `json:"setup_timeout"`    // Total setup timeout

	// Paths
	WorkspaceDir string `json:"workspace_dir"` // Inside container
	CacheDir     string `json:"cache_dir"`     // pip cache mount

	// Behavior
	EnableSnapshots    bool `json:"enable_snapshots"`
	SnapshotAfterSetup bool `json:"snapshot_after_setup"`
	PreserveOnFailure  bool `json:"preserve_on_failure"`
	Verbose            bool `json:"verbose"`
}

// DefaultEnvironmentConfig returns sensible defaults.
func DefaultEnvironmentConfig() EnvironmentConfig {
	return EnvironmentConfig{
		MemoryLimit:        4 * 1024 * 1024 * 1024, // 4GB
		CPULimit:           2.0,
		NetworkEnabled:     true, // Need network for pip
		TestTimeout:        5 * time.Minute,
		SetupTimeout:       15 * time.Minute,
		WorkspaceDir:       "/testbed",
		EnableSnapshots:    true,
		SnapshotAfterSetup: true,
		PreserveOnFailure:  true,
		Verbose:            false,
	}
}

// Environment manages a single SWE-bench instance evaluation.
type Environment struct {
	mu sync.RWMutex

	instance  *Instance
	config    EnvironmentConfig
	executor  *tactile.PersistentDockerExecutor
	container *tactile.PersistentContainer

	state         EnvironmentState
	setupSnapshot *tactile.ContainerSnapshot
	lastError     error
	repoPath      string
	venvPath      string

	// Timing
	setupStarted   time.Time
	setupCompleted time.Time
}

// NewEnvironment creates a new environment for an instance.
func NewEnvironment(
	instance *Instance,
	config EnvironmentConfig,
	executor *tactile.PersistentDockerExecutor,
) *Environment {
	return &Environment{
		instance: instance,
		config:   config,
		executor: executor,
		state:    EnvStateInitializing,
		repoPath: "/testbed/" + instance.RepoName(),
		venvPath: "/testbed/venv",
	}
}

// =============================================================================
// STATE ACCESSORS
// =============================================================================

// State returns the current environment state.
func (e *Environment) State() EnvironmentState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// Instance returns the SWE-bench instance.
func (e *Environment) Instance() *Instance {
	return e.instance
}

// ContainerID returns the container ID (if created).
func (e *Environment) ContainerID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.container == nil {
		return ""
	}
	return e.container.ID
}

// RepoPath returns the path to the cloned repo inside the container.
func (e *Environment) RepoPath() string {
	return e.repoPath
}

// VenvPath returns the path to the virtual environment.
func (e *Environment) VenvPath() string {
	return e.venvPath
}

// GetError returns the last error.
func (e *Environment) GetError() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastError
}

func (e *Environment) setState(state EnvironmentState) {
	e.mu.Lock()
	e.state = state
	e.mu.Unlock()
}

func (e *Environment) setError(err error) {
	e.mu.Lock()
	e.lastError = err
	e.state = EnvStateError
	e.mu.Unlock()
}

// =============================================================================
// LIFECYCLE METHODS
// =============================================================================

// Initialize creates the container and prepares the environment.
func (e *Environment) Initialize(ctx context.Context) error {
	logging.Tactile("Initializing environment for %s", e.instance.InstanceID)
	e.setupStarted = time.Now()

	// Determine Docker image
	image := e.config.BaseImage
	if image == "" {
		image = e.instance.DockerImage()
	}

	// Create container
	networkMode := "none"
	if e.config.NetworkEnabled {
		networkMode = "bridge"
	}

	opts := tactile.ContainerCreateOptions{
		Name:        fmt.Sprintf("swebench-%s", e.instance.InstanceID),
		Image:       image,
		WorkingDir:  e.config.WorkspaceDir,
		MemoryLimit: e.config.MemoryLimit,
		CPULimit:    e.config.CPULimit,
		NetworkMode: networkMode,
		Labels: map[string]string{
			"swebench.instance_id": e.instance.InstanceID,
			"swebench.repo":        e.instance.Repo,
		},
	}

	container, err := e.executor.CreateContainer(ctx, opts)
	if err != nil {
		e.setError(fmt.Errorf("failed to create container: %w", err))
		return e.lastError
	}

	e.mu.Lock()
	e.container = container
	e.mu.Unlock()

	// Start container
	if err := e.executor.StartContainer(ctx, container.ID); err != nil {
		e.setError(fmt.Errorf("failed to start container: %w", err))
		return e.lastError
	}

	logging.Tactile("Container created and started: %s", container.ID[:12])
	return nil
}

// Setup performs full environment setup: clone, checkout, venv, deps.
func (e *Environment) Setup(ctx context.Context) error {
	logging.Tactile("Setting up environment for %s", e.instance.InstanceID)

	// Clone repository
	if err := e.CloneRepo(ctx); err != nil {
		return err
	}

	// Checkout base commit
	if err := e.CheckoutBaseCommit(ctx); err != nil {
		return err
	}

	// Set up virtual environment
	if err := e.SetupVirtualEnv(ctx); err != nil {
		return err
	}

	// Install dependencies
	if err := e.InstallDependencies(ctx); err != nil {
		return err
	}

	e.setupCompleted = time.Now()
	e.setState(EnvStateReady)

	// Create post-setup snapshot if enabled
	if e.config.EnableSnapshots && e.config.SnapshotAfterSetup {
		snapshot, err := e.executor.CreateSnapshot(ctx, e.container.ID, "post-setup")
		if err != nil {
			logging.TactileWarn("Failed to create setup snapshot: %v", err)
		} else {
			e.mu.Lock()
			e.setupSnapshot = snapshot
			e.mu.Unlock()
			logging.Tactile("Setup snapshot created: %s", snapshot.ImageTag)
		}
	}

	logging.Tactile("Environment setup complete for %s (took %s)",
		e.instance.InstanceID, e.setupCompleted.Sub(e.setupStarted).Round(time.Second))
	return nil
}

// Teardown removes the container and cleans up resources.
func (e *Environment) Teardown(ctx context.Context) error {
	logging.Tactile("Tearing down environment for %s", e.instance.InstanceID)

	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil
	}

	// Remove container (force)
	if err := e.executor.RemoveContainer(ctx, container.ID, true); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	e.mu.Lock()
	e.container = nil
	e.state = EnvStateComplete
	e.mu.Unlock()

	return nil
}

// Reset restores the environment to post-setup state.
func (e *Environment) Reset(ctx context.Context) error {
	e.mu.RLock()
	snapshot := e.setupSnapshot
	e.mu.RUnlock()

	if snapshot == nil {
		return fmt.Errorf("no setup snapshot available")
	}

	logging.Tactile("Resetting environment to post-setup state")

	// Stop current container
	if err := e.executor.StopContainer(ctx, e.container.ID, 10*time.Second); err != nil {
		logging.TactileWarn("Failed to stop container during reset: %v", err)
	}

	// Remove current container
	if err := e.executor.RemoveContainer(ctx, e.container.ID, true); err != nil {
		return fmt.Errorf("failed to remove container during reset: %w", err)
	}

	// Restore from snapshot
	newContainer, err := e.executor.RestoreSnapshot(ctx, snapshot.ID)
	if err != nil {
		e.setError(fmt.Errorf("failed to restore from snapshot: %w", err))
		return e.lastError
	}

	// Start restored container
	if err := e.executor.StartContainer(ctx, newContainer.ID); err != nil {
		e.setError(fmt.Errorf("failed to start restored container: %w", err))
		return e.lastError
	}

	e.mu.Lock()
	e.container = newContainer
	e.state = EnvStateReady
	e.mu.Unlock()

	logging.Tactile("Environment reset complete")
	return nil
}

// =============================================================================
// REPOSITORY OPERATIONS
// =============================================================================

// CloneRepo clones the repository into the container.
func (e *Environment) CloneRepo(ctx context.Context) error {
	e.setState(EnvStateCloning)
	logging.Tactile("Cloning repository: %s", e.instance.GitURL())

	// Create workspace directory
	_, err := e.Exec(ctx, "mkdir", []string{"-p", e.config.WorkspaceDir})
	if err != nil {
		e.setError(fmt.Errorf("failed to create workspace: %w", err))
		return e.lastError
	}

	// Clone repository (shallow for speed)
	result, err := e.execWithTimeout(ctx, e.config.SetupTimeout,
		"git", "clone", "--depth", "1000", e.instance.GitURL(), e.repoPath)
	if err != nil {
		e.setError(fmt.Errorf("failed to clone repository: %w", err))
		return e.lastError
	}
	if result.ExitCode != 0 {
		e.setError(fmt.Errorf("git clone failed: %s", result.Stderr))
		return e.lastError
	}

	logging.Tactile("Repository cloned to %s", e.repoPath)
	return nil
}

// CheckoutBaseCommit checks out the base commit.
func (e *Environment) CheckoutBaseCommit(ctx context.Context) error {
	e.setState(EnvStateCheckout)
	logging.Tactile("Checking out base commit: %s", e.instance.BaseCommit)

	// Fetch the specific commit if needed
	result, err := e.execInRepo(ctx, 2*time.Minute, "git", "fetch", "origin", e.instance.BaseCommit)
	if err != nil {
		logging.TactileDebug("Fetch failed (may be okay): %v", err)
	}

	// Checkout the commit
	result, err = e.execInRepo(ctx, 1*time.Minute, "git", "checkout", e.instance.BaseCommit)
	if err != nil {
		e.setError(fmt.Errorf("failed to checkout: %w", err))
		return e.lastError
	}
	if result.ExitCode != 0 {
		e.setError(fmt.Errorf("git checkout failed: %s", result.Stderr))
		return e.lastError
	}

	logging.Tactile("Checked out commit: %s", e.instance.BaseCommit[:12])
	return nil
}

// =============================================================================
// ENVIRONMENT SETUP
// =============================================================================

// SetupVirtualEnv creates a Python virtual environment.
func (e *Environment) SetupVirtualEnv(ctx context.Context) error {
	e.setState(EnvStateSetup)
	logging.Tactile("Setting up virtual environment")

	// Create venv
	result, err := e.Exec(ctx, "python", []string{"-m", "venv", e.venvPath})
	if err != nil {
		e.setError(fmt.Errorf("failed to create venv: %w", err))
		return e.lastError
	}
	if result.ExitCode != 0 {
		e.setError(fmt.Errorf("venv creation failed: %s", result.Stderr))
		return e.lastError
	}

	// Upgrade pip
	result, err = e.ExecInVenv(ctx, "pip install --upgrade pip setuptools wheel")
	if err != nil {
		logging.TactileWarn("Failed to upgrade pip: %v", err)
	}

	logging.Tactile("Virtual environment created at %s", e.venvPath)
	return nil
}

// InstallDependencies installs project dependencies.
func (e *Environment) InstallDependencies(ctx context.Context) error {
	logging.Tactile("Installing dependencies for %s", e.instance.Repo)

	// Try different dependency files in order
	depFiles := []struct {
		file    string
		command string
	}{
		{"pyproject.toml", "pip install -e .[dev,test]"},
		{"setup.py", "pip install -e .[dev,test]"},
		{"requirements.txt", "pip install -r requirements.txt"},
		{"requirements-dev.txt", "pip install -r requirements-dev.txt"},
	}

	for _, dep := range depFiles {
		// Check if file exists
		checkResult, _ := e.execInRepo(ctx, 10*time.Second, "test", "-f", dep.file)
		if checkResult != nil && checkResult.ExitCode == 0 {
			logging.Tactile("Installing from %s", dep.file)

			// Install with timeout
			result, err := e.execInRepoVenv(ctx, e.config.SetupTimeout, dep.command)
			if err != nil {
				logging.TactileWarn("Install command failed: %v", err)
				continue
			}
			if result.ExitCode != 0 {
				logging.TactileWarn("Install exited non-zero: %s", result.Stderr)
				// Continue trying other files
				continue
			}

			logging.Tactile("Dependencies installed successfully")
			return nil
		}
	}

	// Fallback: try installing the package directly
	logging.Tactile("Attempting fallback: pip install -e .")
	result, err := e.execInRepoVenv(ctx, e.config.SetupTimeout, "pip install -e .")
	if err != nil || result.ExitCode != 0 {
		logging.TactileWarn("Fallback install failed, continuing anyway")
	}

	return nil
}

// =============================================================================
// PATCH OPERATIONS
// =============================================================================

// ApplyPatch applies a model's patch to the repository.
func (e *Environment) ApplyPatch(ctx context.Context, patch string) error {
	logging.Tactile("Applying patch (%d bytes)", len(patch))

	// Write patch to temp file
	patchPath := "/tmp/model.patch"
	writeResult, err := e.Exec(ctx, "sh", []string{"-c", fmt.Sprintf("cat > %s << 'PATCH_EOF'\n%s\nPATCH_EOF", patchPath, patch)})
	if err != nil {
		e.setError(fmt.Errorf("failed to write patch file: %w", err))
		return e.lastError
	}
	if writeResult.ExitCode != 0 {
		e.setError(fmt.Errorf("failed to write patch: %s", writeResult.Stderr))
		return e.lastError
	}

	// Apply patch
	result, err := e.execInRepo(ctx, 1*time.Minute, "git", "apply", "--verbose", patchPath)
	if err != nil {
		e.setError(fmt.Errorf("failed to apply patch: %w", err))
		return e.lastError
	}
	if result.ExitCode != 0 {
		e.setError(fmt.Errorf("patch apply failed: %s", result.Combined))
		return e.lastError
	}

	e.setState(EnvStatePatchApplied)
	logging.Tactile("Patch applied successfully")
	return nil
}

// RevertPatch reverts any applied patches.
func (e *Environment) RevertPatch(ctx context.Context) error {
	logging.Tactile("Reverting patch")

	result, err := e.execInRepo(ctx, 1*time.Minute, "git", "checkout", "--", ".")
	if err != nil {
		return fmt.Errorf("failed to revert: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git checkout failed: %s", result.Stderr)
	}

	e.setState(EnvStateReady)
	return nil
}

// GetCurrentDiff returns the current diff in the repository.
func (e *Environment) GetCurrentDiff(ctx context.Context) (string, error) {
	result, err := e.execInRepo(ctx, 30*time.Second, "git", "diff")
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

// =============================================================================
// TEST EXECUTION
// =============================================================================

// RunTest runs a single test and returns the result.
func (e *Environment) RunTest(ctx context.Context, testName string) (*TestResult, error) {
	logging.Tactile("Running test: %s", testName)
	e.setState(EnvStateTesting)

	startTime := time.Now()

	// Build pytest command
	// Most SWE-bench tests use pytest format: path/to/test.py::TestClass::test_method
	result, err := e.execInRepoVenv(ctx, e.config.TestTimeout, fmt.Sprintf("pytest -xvs %s", testName))

	duration := time.Since(startTime)

	testResult := &TestResult{
		TestName: testName,
		Duration: duration,
	}

	if err != nil {
		testResult.Passed = false
		testResult.ErrorMessage = err.Error()
		testResult.ExitCode = -1
		return testResult, nil
	}

	testResult.Output = result.Combined
	testResult.ExitCode = result.ExitCode
	testResult.Passed = result.ExitCode == 0

	if !testResult.Passed {
		testResult.ErrorMessage = extractPytestError(result.Combined)
	}

	logging.Tactile("Test %s: passed=%v, duration=%s", testName, testResult.Passed, duration.Round(time.Millisecond))
	return testResult, nil
}

// RunTests runs multiple tests and returns all results.
func (e *Environment) RunTests(ctx context.Context, testNames []string) (map[string]*TestResult, error) {
	results := make(map[string]*TestResult)

	for _, testName := range testNames {
		result, err := e.RunTest(ctx, testName)
		if err != nil {
			return results, err
		}
		results[testName] = result
	}

	return results, nil
}

// RunAllTests runs all FAIL_TO_PASS and PASS_TO_PASS tests.
func (e *Environment) RunAllTests(ctx context.Context) (*EvaluationResult, error) {
	startTime := time.Now()

	evalResult := &EvaluationResult{
		InstanceID:        e.instance.InstanceID,
		FailToPassResults: make(map[string]TestResult),
		PassToPassResults: make(map[string]TestResult),
		StartedAt:         startTime,
	}

	// Run FAIL_TO_PASS tests
	logging.Tactile("Running FAIL_TO_PASS tests (%d)", len(e.instance.FailToPass))
	for _, testName := range e.instance.FailToPass {
		result, err := e.RunTest(ctx, testName)
		if err != nil {
			evalResult.Error = err.Error()
			evalResult.ErrorPhase = "test"
			break
		}
		evalResult.FailToPassResults[testName] = *result
		if result.Passed {
			evalResult.PassedTests++
		} else {
			evalResult.FailedTests++
		}
	}

	// Run PASS_TO_PASS tests
	logging.Tactile("Running PASS_TO_PASS tests (%d)", len(e.instance.PassToPass))
	for _, testName := range e.instance.PassToPass {
		result, err := e.RunTest(ctx, testName)
		if err != nil {
			evalResult.Error = err.Error()
			evalResult.ErrorPhase = "test"
			break
		}
		evalResult.PassToPassResults[testName] = *result
		if result.Passed {
			evalResult.PassedTests++
		} else {
			evalResult.FailedTests++
		}
	}

	// Calculate resolution
	evalResult.TotalTests = len(e.instance.FailToPass) + len(e.instance.PassToPass)
	evalResult.Resolved = evalResult.FailToPassRate() == 100.0 && evalResult.PassToPassRate() == 100.0

	evalResult.CompletedAt = time.Now()
	evalResult.Duration = evalResult.CompletedAt.Sub(startTime)
	evalResult.TestDuration = evalResult.Duration

	logging.Tactile("Evaluation complete: %s", evalResult.Summary())
	return evalResult, nil
}

// =============================================================================
// EVALUATION
// =============================================================================

// Evaluate applies a prediction patch and runs tests.
func (e *Environment) Evaluate(ctx context.Context, prediction *Prediction) (*EvaluationResult, error) {
	logging.Tactile("Evaluating prediction for %s", e.instance.InstanceID)

	// Apply patch
	if err := e.ApplyPatch(ctx, prediction.ModelPatch); err != nil {
		return &EvaluationResult{
			InstanceID:   e.instance.InstanceID,
			PatchApplied: false,
			Error:        err.Error(),
			ErrorPhase:   "patch",
		}, nil
	}

	// Run tests
	evalResult, err := e.RunAllTests(ctx)
	if err != nil {
		return evalResult, err
	}

	evalResult.PatchApplied = true
	return evalResult, nil
}

// =============================================================================
// COMMAND EXECUTION HELPERS
// =============================================================================

// Exec executes a command in the container.
func (e *Environment) Exec(ctx context.Context, binary string, args []string) (*tactile.ExecutionResult, error) {
	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container not initialized")
	}

	return e.executor.ExecInContainer(ctx, tactile.ContainerExecOptions{
		ContainerID: container.ID,
		Binary:      binary,
		Arguments:   args,
		WorkingDir:  e.config.WorkspaceDir,
		Timeout:     2 * time.Minute,
	})
}

// ExecInVenv executes a command using the venv's Python.
func (e *Environment) ExecInVenv(ctx context.Context, command string) (*tactile.ExecutionResult, error) {
	// Use the venv's pip/python via full path
	venvBin := e.venvPath + "/bin"
	fullCommand := fmt.Sprintf("PATH=%s:$PATH %s", venvBin, command)

	return e.Exec(ctx, "sh", []string{"-c", fullCommand})
}

func (e *Environment) execWithTimeout(ctx context.Context, timeout time.Duration, binary string, args ...string) (*tactile.ExecutionResult, error) {
	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container not initialized")
	}

	return e.executor.ExecInContainer(ctx, tactile.ContainerExecOptions{
		ContainerID: container.ID,
		Binary:      binary,
		Arguments:   args,
		WorkingDir:  e.config.WorkspaceDir,
		Timeout:     timeout,
	})
}

func (e *Environment) execInRepo(ctx context.Context, timeout time.Duration, binary string, args ...string) (*tactile.ExecutionResult, error) {
	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container not initialized")
	}

	return e.executor.ExecInContainer(ctx, tactile.ContainerExecOptions{
		ContainerID: container.ID,
		Binary:      binary,
		Arguments:   args,
		WorkingDir:  e.repoPath,
		Timeout:     timeout,
	})
}

func (e *Environment) execInRepoVenv(ctx context.Context, timeout time.Duration, command string) (*tactile.ExecutionResult, error) {
	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container not initialized")
	}

	// Activate venv and run command
	venvBin := e.venvPath + "/bin"
	fullCommand := fmt.Sprintf("PATH=%s:$PATH %s", venvBin, command)

	return e.executor.ExecInContainer(ctx, tactile.ContainerExecOptions{
		ContainerID: container.ID,
		Binary:      "sh",
		Arguments:   []string{"-c", fullCommand},
		WorkingDir:  e.repoPath,
		Timeout:     timeout,
	})
}

// extractPytestError extracts a concise error message from pytest output.
func extractPytestError(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for assertion errors or exception lines
		if strings.Contains(line, "AssertionError") ||
			strings.Contains(line, "Error:") ||
			strings.Contains(line, "FAILED") {
			return strings.TrimSpace(line)
		}
	}
	// Return last non-empty line
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimSpace(lines[i])
		}
	}
	return "unknown error"
}
