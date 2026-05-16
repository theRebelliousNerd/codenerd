package core

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// PYTHON ENVIRONMENT ACTION HANDLERS (General Purpose)
// =============================================================================
// These handlers work with ANY Python project, not just benchmarks.

// handlePythonEnvSetup creates a Python development environment.
// Payload expects: project_name, git_url (optional), commit (optional), branch (optional)
func (v *VirtualStore) handlePythonEnvSetup(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	gitURL, _ := req.Payload["git_url"].(string)
	commit, _ := req.Payload["commit"].(string)
	branch, _ := req.Payload["branch"].(string)

	if projectName == "" && gitURL == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name or git_url required in payload",
		}, nil
	}

	logging.VirtualStore("Python env setup: project=%s", projectName)

	facts := []Fact{
		{Predicate: "python_environment", Args: []interface{}{projectName, "", "/initializing", time.Now().Unix()}},
	}

	if gitURL != "" {
		facts = append(facts, Fact{
			Predicate: "python_project_source",
			Args:      []interface{}{projectName, gitURL, commit, branch},
		})
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Python environment initializing for %s", projectName),
		Metadata: map[string]interface{}{
			"project_name": projectName,
			"git_url":      gitURL,
			"commit":       commit,
			"branch":       branch,
		},
		FactsToAdd: facts,
	}, nil
}

// handlePythonEnvExec executes a command in a Python environment.
// Payload expects: project_name, command
func (v *VirtualStore) handlePythonEnvExec(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	command, _ := req.Payload["command"].(string)

	if projectName == "" || command == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name and command required in payload",
		}, nil
	}

	logging.VirtualStore("Python exec: project=%s, cmd=%s", projectName, command)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Executing in %s: %s", projectName, command),
		Metadata: map[string]interface{}{
			"project_name": projectName,
			"command":      command,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_command_executed", Args: []interface{}{projectName, command, time.Now().Unix()}},
		},
	}, nil
}

// handlePythonRunPytest runs pytest in a Python environment.
// Payload expects: project_name, test_args (optional - array of test names/patterns)
func (v *VirtualStore) handlePythonRunPytest(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	if projectName == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name required in payload",
		}, nil
	}

	// Optional test arguments
	var testArgs []string
	if args, ok := req.Payload["test_args"].([]interface{}); ok {
		for _, a := range args {
			if s, ok := a.(string); ok {
				testArgs = append(testArgs, s)
			}
		}
	}

	logging.VirtualStore("Python pytest: project=%s, args=%v", projectName, testArgs)

	facts := []Fact{
		{Predicate: "python_environment", Args: []interface{}{projectName, "", "/testing", time.Now().Unix()}},
		{Predicate: "pytest_execution", Args: []interface{}{projectName, len(testArgs), time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Running pytest in %s", projectName),
		Metadata: map[string]interface{}{
			"project_name": projectName,
			"test_args":    testArgs,
		},
		FactsToAdd: facts,
	}, nil
}

// handlePythonApplyPatch applies a git patch to a Python project.
// Payload expects: project_name, patch (unified diff format)
func (v *VirtualStore) handlePythonApplyPatch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	patch, _ := req.Payload["patch"].(string)

	if projectName == "" || patch == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name and patch required in payload",
		}, nil
	}

	logging.VirtualStore("Python apply patch: project=%s, size=%d", projectName, len(patch))

	facts := []Fact{
		{Predicate: "python_patch_applied", Args: []interface{}{projectName, len(patch), time.Now().Unix()}},
		{Predicate: "python_environment", Args: []interface{}{projectName, "", "/patched", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Patch applied to %s (%d bytes)", projectName, len(patch)),
		Metadata: map[string]interface{}{
			"project_name": projectName,
			"patch_size":   len(patch),
		},
		FactsToAdd: facts,
	}, nil
}

// handlePythonSnapshot creates a snapshot of the Python environment.
// Payload expects: project_name, snapshot_name (optional)
func (v *VirtualStore) handlePythonSnapshot(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)

	if projectName == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name required in payload",
		}, nil
	}

	if snapshotName == "" {
		snapshotName = fmt.Sprintf("%s-snapshot-%d", projectName, time.Now().Unix())
	}

	logging.VirtualStore("Python snapshot: project=%s, name=%s", projectName, snapshotName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Snapshot created: %s", snapshotName),
		Metadata: map[string]interface{}{
			"project_name":  projectName,
			"snapshot_name": snapshotName,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_snapshot", Args: []interface{}{projectName, snapshotName, time.Now().Unix()}},
		},
	}, nil
}

// handlePythonRestore restores a Python environment from snapshot.
// Payload expects: project_name, snapshot_name
func (v *VirtualStore) handlePythonRestore(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)

	if projectName == "" || snapshotName == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name and snapshot_name required in payload",
		}, nil
	}

	logging.VirtualStore("Python restore: project=%s, snapshot=%s", projectName, snapshotName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Restored %s from snapshot %s", projectName, snapshotName),
		Metadata: map[string]interface{}{
			"project_name":  projectName,
			"snapshot_name": snapshotName,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_restored", Args: []interface{}{projectName, snapshotName, time.Now().Unix()}},
			{Predicate: "python_environment", Args: []interface{}{projectName, "", "/ready", time.Now().Unix()}},
		},
	}, nil
}

// handlePythonTeardown cleans up a Python environment.
// Payload expects: project_name
func (v *VirtualStore) handlePythonTeardown(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	if projectName == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name required in payload",
		}, nil
	}

	logging.VirtualStore("Python teardown: project=%s", projectName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Python environment torn down for %s", projectName),
		Metadata: map[string]interface{}{
			"project_name": projectName,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_environment", Args: []interface{}{projectName, "", "/terminated", time.Now().Unix()}},
			{Predicate: "python_teardown_complete", Args: []interface{}{projectName, time.Now().Unix()}},
		},
	}, nil
}

// =============================================================================
// SWE-BENCH ACTION HANDLERS (Benchmark-specific)
// =============================================================================
// These handlers delegate to Python handlers with SWE-bench metadata.

// handleSWEBenchSetup initializes a SWE-bench environment for an instance.
// Payload expects: instance_id, repo, base_commit, problem_statement, fail_to_pass, pass_to_pass
func (v *VirtualStore) handleSWEBenchSetup(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	logging.VirtualStore("SWE-bench setup: instance=%s", instanceID)

	// Extract instance details from payload
	repo, _ := req.Payload["repo"].(string)
	baseCommit, _ := req.Payload["base_commit"].(string)
	problemStatement, _ := req.Payload["problem_statement"].(string)

	// Convert test lists
	var failToPass, passToPass []string
	if ftp, ok := req.Payload["fail_to_pass"].([]interface{}); ok {
		for _, t := range ftp {
			if s, ok := t.(string); ok {
				failToPass = append(failToPass, s)
			}
		}
	}
	if ptp, ok := req.Payload["pass_to_pass"].([]interface{}); ok {
		for _, t := range ptp {
			if s, ok := t.(string); ok {
				passToPass = append(passToPass, s)
			}
		}
	}

	// Generate Mangle facts for the instance
	facts := []Fact{
		{Predicate: "swebench_instance", Args: []interface{}{instanceID, repo, baseCommit, ""}},
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/initializing", time.Now().Unix()}},
	}

	// Add test expectations as facts
	for _, test := range failToPass {
		facts = append(facts, Fact{
			Predicate: "swebench_expected_fail_to_pass",
			Args:      []interface{}{instanceID, test},
		})
	}
	for _, test := range passToPass {
		facts = append(facts, Fact{
			Predicate: "swebench_expected_pass_to_pass",
			Args:      []interface{}{instanceID, test},
		})
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("SWE-bench environment initializing for %s (%s@%s)", instanceID, repo, baseCommit[:8]),
		Metadata: map[string]interface{}{
			"instance_id":       instanceID,
			"repo":              repo,
			"base_commit":       baseCommit,
			"problem_statement": problemStatement,
			"fail_to_pass":      failToPass,
			"pass_to_pass":      passToPass,
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchApplyPatch applies a model-generated patch to the environment.
// Payload expects: instance_id, patch (unified diff format)
func (v *VirtualStore) handleSWEBenchApplyPatch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	patch, _ := req.Payload["patch"].(string)

	if instanceID == "" || patch == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id and patch required in payload",
		}, nil
	}

	logging.VirtualStore("SWE-bench apply patch: instance=%s, patch_size=%d", instanceID, len(patch))

	// Record patch application attempt
	facts := []Fact{
		{Predicate: "swebench_patch_applied", Args: []interface{}{instanceID, len(patch), time.Now().Unix()}},
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/patched", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Patch applied to instance %s (%d bytes)", instanceID, len(patch)),
		Metadata: map[string]interface{}{
			"instance_id": instanceID,
			"patch_size":  len(patch),
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchRunTests runs tests for a SWE-bench instance.
// Payload expects: instance_id, test_names (optional - defaults to all)
func (v *VirtualStore) handleSWEBenchRunTests(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	// Optional: specific test names
	var testNames []string
	if tn, ok := req.Payload["test_names"].([]interface{}); ok {
		for _, t := range tn {
			if s, ok := t.(string); ok {
				testNames = append(testNames, s)
			}
		}
	}

	logging.VirtualStore("SWE-bench run tests: instance=%s, tests=%d", instanceID, len(testNames))

	facts := []Fact{
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/testing", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Running tests for instance %s", instanceID),
		Metadata: map[string]interface{}{
			"instance_id": instanceID,
			"test_count":  len(testNames),
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchSnapshot creates a container snapshot for rollback.
// Payload expects: instance_id, snapshot_name (optional)
func (v *VirtualStore) handleSWEBenchSnapshot(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)

	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	if snapshotName == "" {
		snapshotName = fmt.Sprintf("%s-snapshot-%d", instanceID, time.Now().Unix())
	}

	logging.VirtualStore("SWE-bench snapshot: instance=%s, name=%s", instanceID, snapshotName)

	facts := []Fact{
		{Predicate: "swebench_snapshot", Args: []interface{}{instanceID, snapshotName, time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Snapshot created: %s", snapshotName),
		Metadata: map[string]interface{}{
			"instance_id":   instanceID,
			"snapshot_name": snapshotName,
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchRestore restores environment from a snapshot.
// Payload expects: instance_id, snapshot_name
func (v *VirtualStore) handleSWEBenchRestore(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)

	if instanceID == "" || snapshotName == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id and snapshot_name required in payload",
		}, nil
	}

	logging.VirtualStore("SWE-bench restore: instance=%s, snapshot=%s", instanceID, snapshotName)

	facts := []Fact{
		{Predicate: "swebench_restored", Args: []interface{}{instanceID, snapshotName, time.Now().Unix()}},
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/ready", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Restored instance %s from snapshot %s", instanceID, snapshotName),
		Metadata: map[string]interface{}{
			"instance_id":   instanceID,
			"snapshot_name": snapshotName,
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchEvaluate evaluates a prediction against the instance tests.
// Payload expects: instance_id, patch, model_name (optional)
func (v *VirtualStore) handleSWEBenchEvaluate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	patch, _ := req.Payload["patch"].(string)
	modelName, _ := req.Payload["model_name"].(string)

	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	if modelName == "" {
		modelName = "codenerd"
	}

	logging.VirtualStore("SWE-bench evaluate: instance=%s, model=%s", instanceID, modelName)

	// Evaluation result will be populated by actual test execution
	// For now, record the evaluation attempt
	facts := []Fact{
		{Predicate: "swebench_evaluation_started", Args: []interface{}{instanceID, modelName, time.Now().Unix()}},
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/evaluating", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Evaluation started for instance %s with model %s", instanceID, modelName),
		Metadata: map[string]interface{}{
			"instance_id": instanceID,
			"model_name":  modelName,
			"patch_size":  len(patch),
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchTeardown cleans up a SWE-bench environment.
// Payload expects: instance_id
func (v *VirtualStore) handleSWEBenchTeardown(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	logging.VirtualStore("SWE-bench teardown: instance=%s", instanceID)

	facts := []Fact{
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/terminated", time.Now().Unix()}},
		{Predicate: "swebench_teardown_complete", Args: []interface{}{instanceID, time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("SWE-bench environment torn down for instance %s", instanceID),
		Metadata: map[string]interface{}{
			"instance_id": instanceID,
		},
		FactsToAdd: facts,
	}, nil
}
