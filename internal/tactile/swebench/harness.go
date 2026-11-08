// Package swebench provides SWE-bench specific evaluation harness.
// This is a THIN WRAPPER over the general-purpose python.Environment.
// All the real work is done by the base Python environment.
package swebench

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
	"codenerd/internal/tactile/python"
)

// =============================================================================
// SWE-BENCH HARNESS - Benchmark-specific wrapper
// =============================================================================
// This harness adds SWE-bench-specific functionality on top of the general
// Python environment. It handles:
// - Instance-specific metadata (FAIL_TO_PASS, PASS_TO_PASS tests)
// - Benchmark-specific evaluation metrics
// - Reporting in SWE-bench format
//
// The heavy lifting (containers, git, pytest) is done by python.Environment.

// Harness wraps a Python environment for SWE-bench evaluation.
type Harness struct {
	instance *Instance
	env      *python.Environment
}

// NewHarness creates a SWE-bench harness for an instance.
func NewHarness(
	instance *Instance,
	config python.EnvironmentConfig,
	executor tactile.ContainerRuntime,
) *Harness {
	// NewHarness tolerates a nil instance so callers that wire
	// configuration before data do not panic on RepoName/GitURL.
	if instance == nil {
		instance = &Instance{}
	}
	// Convert SWE-bench instance to general project info
	project := &python.ProjectInfo{
		Name:   instance.RepoName(),
		GitURL: instance.GitURL(),
		Commit: instance.BaseCommit,
	}

	// Override image if instance specifies one
	if img := instance.DockerImage(); img != "" && config.BaseImage == "" {
		config.BaseImage = img
	}

	// Use /testbed as workspace (SWE-bench convention)
	if config.WorkspaceDir == "" || config.WorkspaceDir == "/workspace" {
		config.WorkspaceDir = "/testbed"
	}

	return &Harness{
		instance: instance,
		env:      python.NewEnvironment(project, config, executor),
	}
}

// =============================================================================
// DELEGATION TO BASE ENVIRONMENT
// =============================================================================

// Initialize creates the container.
func (h *Harness) Initialize(ctx context.Context) error {
	return h.env.Initialize(ctx)
}

// Setup performs full environment setup.
func (h *Harness) Setup(ctx context.Context) error {
	return h.env.Setup(ctx)
}

// Teardown removes the container.
func (h *Harness) Teardown(ctx context.Context) error {
	return h.env.Teardown(ctx)
}

// Reset restores to post-setup state.
func (h *Harness) Reset(ctx context.Context) error {
	if h == nil {
		return fmt.Errorf("swebench: Reset called with nil harness")
	}
	if h.env == nil {
		return fmt.Errorf("swebench: Reset called with nil environment")
	}
	return h.env.Reset(ctx)
}

// ApplyPatch applies a model's patch.
func (h *Harness) ApplyPatch(ctx context.Context, patch string) error {
	return h.env.ApplyPatch(ctx, patch)
}

// RevertPatch reverts changes.
func (h *Harness) RevertPatch(ctx context.Context) error {
	return h.env.RevertChanges(ctx)
}

// State returns the current state.
func (h *Harness) State() python.EnvironmentState {
	return h.env.State()
}

// Instance returns the SWE-bench instance.
func (h *Harness) Instance() *Instance {
	return h.instance
}

// Environment returns the underlying Python environment.
func (h *Harness) Environment() *python.Environment {
	return h.env
}

// =============================================================================
// SWE-BENCH SPECIFIC METHODS
// =============================================================================

// RunFailToPassTests runs only the FAIL_TO_PASS tests.
func (h *Harness) RunFailToPassTests(ctx context.Context) (map[string]*python.TestResult, error) {
	if h == nil || h.instance == nil {
		return nil, fmt.Errorf("swebench: RunFailToPassTests called with nil harness or instance")
	}
	if h.env == nil {
		return nil, fmt.Errorf("swebench: RunFailToPassTests called with nil environment")
	}
	return h.env.RunTests(ctx, h.instance.FailToPass)
}

// RunPassToPassTests runs only the PASS_TO_PASS tests.
func (h *Harness) RunPassToPassTests(ctx context.Context) (map[string]*python.TestResult, error) {
	if h == nil || h.instance == nil {
		return nil, fmt.Errorf("swebench: RunPassToPassTests called with nil harness or instance")
	}
	if h.env == nil {
		return nil, fmt.Errorf("swebench: RunPassToPassTests called with nil environment")
	}
	return h.env.RunTests(ctx, h.instance.PassToPass)
}

// Evaluate runs the full SWE-bench evaluation: apply patch, run tests, compute metrics.
func (h *Harness) Evaluate(ctx context.Context, prediction *Prediction) (*EvaluationResult, error) {
	if h == nil || h.instance == nil {
		return nil, fmt.Errorf("swebench: Evaluate called with nil harness or instance")
	}
	if h.env == nil {
		return nil, fmt.Errorf("swebench: Evaluate called with nil environment")
	}
	logging.Tactile("Evaluating prediction for %s", h.instance.InstanceID)
	startTime := time.Now()

	evalResult := &EvaluationResult{
		InstanceID:        h.instance.InstanceID,
		FailToPassResults: make(map[string]TestResult),
		PassToPassResults: make(map[string]TestResult),
		StartedAt:         startTime,
	}
	finish := func() {
		evalResult.CompletedAt = time.Now()
		evalResult.Duration = evalResult.CompletedAt.Sub(startTime)
		evalResult.TestDuration = evalResult.Duration
	}

	if prediction == nil {
		evalResult.Error = "nil prediction"
		evalResult.ErrorPhase = "patch"
		evalResult.PatchApplied = false
		finish()
		return evalResult, fmt.Errorf("swebench: nil prediction")
	}

	// Apply patch
	if err := h.ApplyPatch(ctx, prediction.ModelPatch); err != nil {
		evalResult.Error = err.Error()
		evalResult.ErrorPhase = "patch"
		evalResult.PatchApplied = false
		finish()
		return evalResult, nil
	}
	evalResult.PatchApplied = true

	// Run FAIL_TO_PASS tests
	logging.Tactile("Running FAIL_TO_PASS tests (%d)", len(h.instance.FailToPass))
	ftpResults, err := h.RunFailToPassTests(ctx)
	if err != nil {
		evalResult.Error = err.Error()
		evalResult.ErrorPhase = "test"
		finish()
		return evalResult, nil
	}
	for name, result := range ftpResults {
		if result == nil {
			evalResult.FailToPassResults[name] = TestResult{
				TestName:     name,
				Passed:       false,
				ErrorMessage: "nil test result",
			}
			evalResult.FailedTests++
			continue
		}
		evalResult.FailToPassResults[name] = TestResult{
			TestName:     result.TestName,
			Passed:       result.Passed,
			Duration:     result.Duration,
			Output:       result.Output,
			ErrorMessage: result.ErrorMessage,
			ExitCode:     result.ExitCode,
		}
		if result.Passed {
			evalResult.PassedTests++
		} else {
			evalResult.FailedTests++
		}
	}

	// Run PASS_TO_PASS tests
	logging.Tactile("Running PASS_TO_PASS tests (%d)", len(h.instance.PassToPass))
	ptpResults, err := h.RunPassToPassTests(ctx)
	if err != nil {
		evalResult.Error = err.Error()
		evalResult.ErrorPhase = "test"
		finish()
		return evalResult, nil
	}
	for name, result := range ptpResults {
		if result == nil {
			evalResult.PassToPassResults[name] = TestResult{
				TestName:     name,
				Passed:       false,
				ErrorMessage: "nil test result",
			}
			evalResult.FailedTests++
			continue
		}
		evalResult.PassToPassResults[name] = TestResult{
			TestName:     result.TestName,
			Passed:       result.Passed,
			Duration:     result.Duration,
			Output:       result.Output,
			ErrorMessage: result.ErrorMessage,
			ExitCode:     result.ExitCode,
		}
		if result.Passed {
			evalResult.PassedTests++
		} else {
			evalResult.FailedTests++
		}
	}

	// Calculate final metrics
	evalResult.TotalTests = len(h.instance.FailToPass) + len(h.instance.PassToPass)
	evalResult.Resolved = evalResult.FailToPassRate() == 100.0 && evalResult.PassToPassRate() == 100.0

	finish()

	logging.Tactile("Evaluation complete: %s", evalResult.Summary())
	return evalResult, nil
}

// EvaluateWithReset evaluates and then resets the environment for next attempt.
func (h *Harness) EvaluateWithReset(ctx context.Context, prediction *Prediction) (*EvaluationResult, error) {
	if h == nil || h.instance == nil {
		return nil, fmt.Errorf("swebench: EvaluateWithReset called with nil harness or instance")
	}
	result, err := h.Evaluate(ctx, prediction)

	// Reset for next evaluation attempt
	if resetErr := h.Reset(ctx); resetErr != nil {
		logging.TactileWarn("Failed to reset after evaluation: %v", resetErr)
	}

	return result, err
}
