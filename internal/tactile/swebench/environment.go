// Package swebench provides SWE-bench evaluation infrastructure.
// The core Python environment management is delegated to python.Environment.
// This file contains SWE-bench specific types and the legacy Environment type.
package swebench

import (
	"time"
)

// =============================================================================
// DEPRECATED: Legacy Environment type
// =============================================================================
// Use Harness instead for new code. This is kept for backward compatibility.
// The Harness wraps python.Environment and adds SWE-bench specific methods.

// EnvironmentState is now an alias to python.EnvironmentState
// Kept for backward compatibility.
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

// EnvironmentConfig is kept for backward compatibility.
// Prefer python.EnvironmentConfig for new code.
type EnvironmentConfig struct {
	BaseImage          string        `json:"base_image"`
	PythonVersion      string        `json:"python_version"`
	MemoryLimit        int64         `json:"memory_limit"`
	CPULimit           float64       `json:"cpu_limit"`
	NetworkEnabled     bool          `json:"network_enabled"`
	TestTimeout        time.Duration `json:"test_timeout"`
	SetupTimeout       time.Duration `json:"setup_timeout"`
	WorkspaceDir       string        `json:"workspace_dir"`
	CacheDir           string        `json:"cache_dir"`
	EnableSnapshots    bool          `json:"enable_snapshots"`
	SnapshotAfterSetup bool          `json:"snapshot_after_setup"`
	PreserveOnFailure  bool          `json:"preserve_on_failure"`
	Verbose            bool          `json:"verbose"`
}

// DefaultEnvironmentConfig returns sensible defaults.
func DefaultEnvironmentConfig() EnvironmentConfig {
	return EnvironmentConfig{
		MemoryLimit:        4 * 1024 * 1024 * 1024,
		CPULimit:           2.0,
		NetworkEnabled:     true,
		TestTimeout:        5 * time.Minute,
		SetupTimeout:       15 * time.Minute,
		WorkspaceDir:       "/testbed",
		EnableSnapshots:    true,
		SnapshotAfterSetup: true,
		PreserveOnFailure:  true,
		Verbose:            false,
	}
}

// =============================================================================
// TEST RESULT TYPES
// =============================================================================

// TestResult holds the result of running a single test.
// This is the SWE-bench specific version with additional metadata.
type TestResult struct {
	TestName     string        `json:"test_name"`
	Passed       bool          `json:"passed"`
	Duration     time.Duration `json:"duration"`
	Output       string        `json:"output"`
	ErrorMessage string        `json:"error_message,omitempty"`
	ExitCode     int           `json:"exit_code"`
}

// =============================================================================
// EVALUATION RESULT
// =============================================================================

// EvaluationResult contains the complete result of evaluating a prediction.
type EvaluationResult struct {
	InstanceID   string `json:"instance_id"`
	PatchApplied bool   `json:"patch_applied"`
	Resolved     bool   `json:"resolved"`

	// Test results
	FailToPassResults map[string]TestResult `json:"fail_to_pass_results"`
	PassToPassResults map[string]TestResult `json:"pass_to_pass_results"`

	// Counts
	TotalTests  int `json:"total_tests"`
	PassedTests int `json:"passed_tests"`
	FailedTests int `json:"failed_tests"`

	// Timing
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  time.Time     `json:"completed_at"`
	Duration     time.Duration `json:"duration"`
	SetupTime    time.Duration `json:"setup_time"`
	TestDuration time.Duration `json:"test_duration"`

	// Error tracking
	Error      string `json:"error,omitempty"`
	ErrorPhase string `json:"error_phase,omitempty"`
}

// FailToPassRate returns the percentage of FAIL_TO_PASS tests that now pass.
func (e *EvaluationResult) FailToPassRate() float64 {
	if len(e.FailToPassResults) == 0 {
		return 0.0
	}
	passed := 0
	for _, r := range e.FailToPassResults {
		if r.Passed {
			passed++
		}
	}
	return float64(passed) / float64(len(e.FailToPassResults)) * 100.0
}

// PassToPassRate returns the percentage of PASS_TO_PASS tests that still pass.
func (e *EvaluationResult) PassToPassRate() float64 {
	if len(e.PassToPassResults) == 0 {
		return 100.0 // No regression tests = all pass
	}
	passed := 0
	for _, r := range e.PassToPassResults {
		if r.Passed {
			passed++
		}
	}
	return float64(passed) / float64(len(e.PassToPassResults)) * 100.0
}

// Summary returns a human-readable summary of the evaluation.
func (e *EvaluationResult) Summary() string {
	status := "FAIL"
	if e.Resolved {
		status = "RESOLVED"
	}
	return status + " | " +
		"fail_to_pass=" + formatPercent(e.FailToPassRate()) + "% " +
		"pass_to_pass=" + formatPercent(e.PassToPassRate()) + "%"
}

func formatPercent(p float64) string {
	if p == 100.0 {
		return "100"
	}
	return string(rune(int(p)/10+'0')) + string(rune(int(p)%10+'0'))
}

// =============================================================================
// PREDICTION
// =============================================================================

// Prediction represents a model's prediction for a SWE-bench instance.
type Prediction struct {
	InstanceID string `json:"instance_id"`
	ModelName  string `json:"model_name_or_path"`
	ModelPatch string `json:"model_patch"`
}
