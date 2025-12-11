// Package swebench provides SWE-bench evaluation environment management.
// SWE-bench is a benchmark for evaluating AI coding agents on real-world
// software engineering tasks from GitHub issues.
//
// This package handles:
// - Parsing SWE-bench instance specifications (from HuggingFace datasets)
// - Managing containerized environments for each instance
// - Evaluating model patches against test suites
// - Tracking evaluation metrics
package swebench

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// =============================================================================
// SWE-BENCH INSTANCE TYPES
// =============================================================================
// These types mirror the HuggingFace dataset schema exactly.
// Dataset: princeton-nlp/SWE-bench_Lite (300 instances)

// Instance represents a single SWE-bench problem instance.
// This is loaded from the HuggingFace dataset JSON.
type Instance struct {
	// Core identification
	InstanceID string `json:"instance_id"` // e.g., "django__django-11001"
	Repo       string `json:"repo"`        // e.g., "django/django"
	BaseCommit string `json:"base_commit"` // Git commit to start from

	// Problem specification
	ProblemStatement string `json:"problem_statement"` // Issue description (NL)
	HintsText        string `json:"hints_text"`        // Optional hints

	// Version and setup
	Version                string `json:"version"`                  // e.g., "3.0"
	EnvironmentSetupCommit string `json:"environment_setup_commit"` // Commit with env setup info
	CreatedAt              string `json:"created_at"`               // ISO timestamp

	// Gold patch (for validation only - not shown to model)
	Patch     string `json:"patch"`      // Gold fix patch
	TestPatch string `json:"test_patch"` // Test additions

	// Test specifications
	FailToPass []string `json:"FAIL_TO_PASS"` // Tests that should flip fail→pass
	PassToPass []string `json:"PASS_TO_PASS"` // Tests that should remain passing
}

// Prediction represents a model's patch prediction.
type Prediction struct {
	InstanceID      string `json:"instance_id"`
	ModelPatch      string `json:"model_patch"`
	ModelNameOrPath string `json:"model_name_or_path"`
}

// EvaluationResult represents the complete evaluation outcome.
type EvaluationResult struct {
	InstanceID string `json:"instance_id"`

	// Resolution status
	Resolved bool `json:"resolved"` // All FAIL_TO_PASS passed AND all PASS_TO_PASS passed

	// Test results
	FailToPassResults map[string]TestResult `json:"fail_to_pass_results"`
	PassToPassResults map[string]TestResult `json:"pass_to_pass_results"`

	// Summary statistics
	TotalTests   int `json:"total_tests"`
	PassedTests  int `json:"passed_tests"`
	FailedTests  int `json:"failed_tests"`
	ErroredTests int `json:"errored_tests"`

	// Timing
	Duration      time.Duration `json:"duration"`
	SetupDuration time.Duration `json:"setup_duration"`
	TestDuration  time.Duration `json:"test_duration"`

	// Status flags
	PatchApplied   bool `json:"patch_applied"`
	SetupSucceeded bool `json:"setup_succeeded"`

	// Error information
	Error        string `json:"error,omitempty"`
	ErrorPhase   string `json:"error_phase,omitempty"` // "clone", "setup", "patch", "test"
	ErrorDetails string `json:"error_details,omitempty"`

	// Metadata
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// TestResult represents the outcome of running a single test.
type TestResult struct {
	TestName     string        `json:"test_name"`
	Passed       bool          `json:"passed"`
	Duration     time.Duration `json:"duration"`
	Output       string        `json:"output"`
	ErrorMessage string        `json:"error_message,omitempty"`
	ExitCode     int           `json:"exit_code"`
}

// =============================================================================
// INSTANCE PARSING
// =============================================================================

// LoadInstances loads SWE-bench instances from a JSON file.
func LoadInstances(path string) ([]*Instance, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read instances file: %w", err)
	}

	var instances []*Instance
	if err := json.Unmarshal(data, &instances); err != nil {
		// Try loading as JSONL (one JSON per line)
		instances, err = parseJSONL(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse instances: %w", err)
		}
	}

	return instances, nil
}

// parseJSONL parses JSON Lines format (common for HuggingFace datasets).
func parseJSONL(data []byte) ([]*Instance, error) {
	var instances []*Instance
	lines := strings.Split(string(data), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var instance Instance
		if err := json.Unmarshal([]byte(line), &instance); err != nil {
			return nil, fmt.Errorf("failed to parse line %d: %w", i+1, err)
		}
		instances = append(instances, &instance)
	}

	return instances, nil
}

// LoadInstance loads a single instance from a JSON file.
func LoadInstance(path string) (*Instance, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read instance file: %w", err)
	}

	var instance Instance
	if err := json.Unmarshal(data, &instance); err != nil {
		return nil, fmt.Errorf("failed to parse instance: %w", err)
	}

	return &instance, nil
}

// =============================================================================
// INSTANCE METHODS
// =============================================================================

// RepoOwner returns the repository owner (e.g., "django" from "django/django").
func (i *Instance) RepoOwner() string {
	parts := strings.Split(i.Repo, "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

// RepoName returns the repository name (e.g., "django" from "django/django").
func (i *Instance) RepoName() string {
	parts := strings.Split(i.Repo, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return i.Repo
}

// GitURL returns the full Git URL for cloning.
func (i *Instance) GitURL() string {
	return fmt.Sprintf("https://github.com/%s.git", i.Repo)
}

// AllTests returns all test names (FAIL_TO_PASS + PASS_TO_PASS).
func (i *Instance) AllTests() []string {
	all := make([]string, 0, len(i.FailToPass)+len(i.PassToPass))
	all = append(all, i.FailToPass...)
	all = append(all, i.PassToPass...)
	return all
}

// TestCount returns the total number of tests.
func (i *Instance) TestCount() int {
	return len(i.FailToPass) + len(i.PassToPass)
}

// String returns a human-readable representation.
func (i *Instance) String() string {
	return fmt.Sprintf("Instance{ID: %s, Repo: %s, Version: %s, Tests: %d}",
		i.InstanceID, i.Repo, i.Version, i.TestCount())
}

// =============================================================================
// EVALUATION RESULT METHODS
// =============================================================================

// IsResolved returns true if the patch fully resolved the issue.
func (r *EvaluationResult) IsResolved() bool {
	return r.Resolved
}

// FailToPassRate returns the percentage of FAIL_TO_PASS tests that passed.
func (r *EvaluationResult) FailToPassRate() float64 {
	if len(r.FailToPassResults) == 0 {
		return 0.0
	}
	passed := 0
	for _, result := range r.FailToPassResults {
		if result.Passed {
			passed++
		}
	}
	return float64(passed) / float64(len(r.FailToPassResults)) * 100.0
}

// PassToPassRate returns the percentage of PASS_TO_PASS tests that passed.
func (r *EvaluationResult) PassToPassRate() float64 {
	if len(r.PassToPassResults) == 0 {
		return 100.0 // No tests to maintain = 100%
	}
	passed := 0
	for _, result := range r.PassToPassResults {
		if result.Passed {
			passed++
		}
	}
	return float64(passed) / float64(len(r.PassToPassResults)) * 100.0
}

// Summary returns a human-readable summary.
func (r *EvaluationResult) Summary() string {
	status := "FAILED"
	if r.Resolved {
		status = "RESOLVED"
	}
	return fmt.Sprintf("[%s] %s: F2P=%.0f%% (%d/%d), P2P=%.0f%% (%d/%d), Duration=%s",
		status,
		r.InstanceID,
		r.FailToPassRate(),
		r.countPassed(r.FailToPassResults),
		len(r.FailToPassResults),
		r.PassToPassRate(),
		r.countPassed(r.PassToPassResults),
		len(r.PassToPassResults),
		r.Duration.Round(time.Second),
	)
}

func (r *EvaluationResult) countPassed(results map[string]TestResult) int {
	count := 0
	for _, result := range results {
		if result.Passed {
			count++
		}
	}
	return count
}

// =============================================================================
// PYTHON VERSION DETECTION
// =============================================================================

// PythonVersionHints returns likely Python versions based on the instance.
// SWE-bench instances are from specific repos with known Python requirements.
func (i *Instance) PythonVersionHints() []string {
	// Known Python versions for SWE-bench Lite repos
	repoVersions := map[string][]string{
		"django/django":              {"3.9", "3.10", "3.11"},
		"pallets/flask":              {"3.8", "3.9", "3.10"},
		"psf/requests":               {"3.8", "3.9", "3.10"},
		"matplotlib/matplotlib":      {"3.9", "3.10", "3.11"},
		"sympy/sympy":                {"3.9", "3.10", "3.11"},
		"scikit-learn/scikit-learn":  {"3.9", "3.10", "3.11"},
		"astropy/astropy":            {"3.9", "3.10", "3.11"},
		"sphinx-doc/sphinx":          {"3.9", "3.10", "3.11"},
		"pylint-dev/pylint":          {"3.8", "3.9", "3.10"},
		"pydata/xarray":              {"3.9", "3.10", "3.11"},
		"pydicom/pydicom":            {"3.8", "3.9", "3.10"},
		"pytest-dev/pytest":          {"3.8", "3.9", "3.10", "3.11"},
	}

	if versions, ok := repoVersions[i.Repo]; ok {
		return versions
	}

	// Default to common Python 3 versions
	return []string{"3.10", "3.11"}
}

// PreferredPythonVersion returns the most likely Python version to use.
func (i *Instance) PreferredPythonVersion() string {
	hints := i.PythonVersionHints()
	if len(hints) > 0 {
		// Prefer the middle version (usually most stable)
		return hints[len(hints)/2]
	}
	return "3.10"
}

// DockerImage returns the recommended Docker image for this instance.
func (i *Instance) DockerImage() string {
	return fmt.Sprintf("python:%s-slim", i.PreferredPythonVersion())
}
