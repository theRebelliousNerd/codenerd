package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
	"codenerd/internal/tactile/python"
	"codenerd/internal/tactile/swebench"
	"codenerd/internal/types"
)

// =============================================================================
// PYTHON / SWE-BENCH WORKBENCH
// =============================================================================
//
// These handlers used to return Success with facts claiming work nothing did:
// "Executing in", "Patch applied", "Snapshot created", "Restored" -- no
// container, no command, no patch. python.Environment and swebench.Harness,
// which do the work in a long-lived container, had no production caller. The
// handlers now drive them through one container runtime per VirtualStore, and
// report failure when the runtime or the environment is not there.
//
// Every command runs inside the environment's container (tactile's
// PersistentDockerExecutor), never on the host. The actions keep their
// existing constitutional classification (virtual_store_constitution.go marks
// python_env_exec and python_apply_patch as code-executing).

// pythonWorkbench holds the runtime and the live environments.
type pythonWorkbench struct {
	mu        sync.Mutex
	runtime   tactile.ContainerRuntime
	envs      map[string]*python.Environment
	harnesses map[string]*swebench.Harness
	// snapshots maps an environment key to snapshot name -> snapshot ID.
	snapshots map[string]map[string]string
}

// SetContainerRuntime injects the container runtime the python_* and
// swebench_* actions use. Without it, the first such action creates a
// PersistentDockerExecutor. It must be called before that first action.
func (v *VirtualStore) SetContainerRuntime(runtime tactile.ContainerRuntime) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.containerRuntime = runtime
}

// bench returns the workbench, creating it (and, when none was injected, a
// PersistentDockerExecutor) on first use.
func (v *VirtualStore) bench() *pythonWorkbench {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.pythonBench != nil {
		return v.pythonBench
	}
	runtime := v.containerRuntime
	if runtime == nil {
		docker := tactile.NewPersistentDockerExecutor(tactile.DefaultContainerPoolConfig())
		if docker.IsAvailable() {
			// Health checks and idle reaping for the containers it creates.
			if err := docker.Start(); err != nil {
				logging.Get(logging.CategoryVirtualStore).Warn("persistent docker runtime not started: %v", err)
			}
		}
		runtime = docker
	}
	// Commands run in a container are executions like any other: send their
	// lifecycle events to the store's audit logger so they land in the kernel
	// as execution facts, the same way composite executions do.
	if audited, ok := runtime.(interface {
		SetAuditCallback(func(tactile.AuditEvent))
	}); ok && v.auditLogger != nil {
		audited.SetAuditCallback(v.auditLogger.Log)
	}
	v.pythonBench = &pythonWorkbench{
		runtime:   runtime,
		envs:      make(map[string]*python.Environment),
		harnesses: make(map[string]*swebench.Harness),
		snapshots: make(map[string]map[string]string),
	}
	return v.pythonBench
}

// closePythonBench removes the containers this store created. The caller
// holds v.mu.
func (v *VirtualStore) closePythonBench() {
	b := v.pythonBench
	if b == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b.mu.Lock()
	envs := make([]*python.Environment, 0, len(b.envs)+len(b.harnesses))
	for _, env := range b.envs {
		envs = append(envs, env)
	}
	for _, h := range b.harnesses {
		envs = append(envs, h.Environment())
	}
	b.envs = map[string]*python.Environment{}
	b.harnesses = map[string]*swebench.Harness{}
	b.mu.Unlock()
	for _, env := range envs {
		if err := env.Teardown(ctx); err != nil {
			logging.Get(logging.CategoryVirtualStore).Warn("python environment teardown on close: %v", err)
		}
	}
	if docker, ok := b.runtime.(*tactile.PersistentDockerExecutor); ok {
		_ = docker.Stop()
	}
}

// errRuntimeUnavailable is reported, not hidden, when there is no container
// runtime: nothing was created and nothing ran.
const errRuntimeUnavailable = "container runtime unavailable (Docker not found or not responding); no environment was created and nothing ran"

func (b *pythonWorkbench) env(project string) (*python.Environment, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	env, ok := b.envs[project]
	return env, ok
}

func (b *pythonWorkbench) harness(instanceID string) (*swebench.Harness, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	h, ok := b.harnesses[instanceID]
	return h, ok
}

func (b *pythonWorkbench) recordSnapshot(key, name, id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.snapshots[key] == nil {
		b.snapshots[key] = make(map[string]string)
	}
	b.snapshots[key][name] = id
}

func (b *pythonWorkbench) snapshotID(key, name string) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id, ok := b.snapshots[key][name]
	return id, ok
}

func payloadStrings(payload map[string]any, key string) []string {
	var out []string
	if items, ok := payload[key].([]any); ok {
		for _, item := range items {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func boolAtom(b bool) string {
	if b {
		return "/true"
	}
	return "/false"
}

func actionFailed(msg string, facts ...Fact) ActionResult {
	return ActionResult{Success: false, Error: msg, FactsToAdd: facts}
}

// =============================================================================
// PYTHON ENVIRONMENT ACTION HANDLERS (General Purpose)
// =============================================================================

// handlePythonEnvSetup creates a containerized Python environment: container,
// clone/checkout (when git_url is given), venv and dependencies.
// Payload: project_name, git_url (optional), commit (optional), branch (optional).
func (v *VirtualStore) handlePythonEnvSetup(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	gitURL, _ := req.Payload["git_url"].(string)
	commit, _ := req.Payload["commit"].(string)
	branch, _ := req.Payload["branch"].(string)

	if projectName == "" && gitURL == "" {
		return ActionResult{Success: false, Error: "project_name or git_url required in payload"}, nil
	}
	project := &python.ProjectInfo{Name: projectName, GitURL: gitURL, Commit: commit, Branch: branch}
	projectName = project.RepoName()
	now := time.Now().Unix()

	b := v.bench()
	if _, exists := b.env(projectName); exists {
		return actionFailed(fmt.Sprintf("python environment %q already exists; tear it down before setting it up again", projectName)), nil
	}
	errorFact := Fact{Predicate: "python_environment", Args: []any{projectName, "", "/error", now}}
	if !b.runtime.IsAvailable() {
		return actionFailed(errRuntimeUnavailable, errorFact), nil
	}

	logging.VirtualStore("Python env setup: project=%s", projectName)
	env := python.NewEnvironment(project, python.DefaultConfig(), b.runtime)
	if err := env.Initialize(ctx); err != nil {
		return actionFailed(fmt.Sprintf("python environment %q: %v", projectName, err), errorFact), nil
	}
	b.mu.Lock()
	b.envs[projectName] = env
	b.mu.Unlock()

	if err := env.Setup(ctx); err != nil {
		errorFact.Args[1] = env.ContainerID()
		return actionFailed(fmt.Sprintf("python environment %q setup: %v", projectName, err), errorFact), nil
	}

	facts := []Fact{
		{Predicate: "python_environment", Args: []any{projectName, env.ContainerID(), "/ready", now}},
	}
	if gitURL != "" {
		facts = append(facts, Fact{Predicate: "python_project_source", Args: []any{projectName, gitURL, commit, branch}})
	}
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Python environment ready for %s (container %s)", projectName, shortContainerID(env.ContainerID())),
		Metadata: map[string]any{
			"project_name": projectName,
			"container_id": env.ContainerID(),
			"git_url":      gitURL,
			"commit":       commit,
			"branch":       branch,
		},
		FactsToAdd: facts,
	}, nil
}

// handlePythonEnvExec runs a command in the environment's repository, inside
// its container, with the venv on PATH.
// Payload: project_name, command.
func (v *VirtualStore) handlePythonEnvExec(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	command, _ := req.Payload["command"].(string)
	if projectName == "" || command == "" {
		return ActionResult{Success: false, Error: "project_name and command required in payload"}, nil
	}

	env, ok := v.bench().env(projectName)
	if !ok {
		return actionFailed(fmt.Sprintf("no python environment %q; run python_env_setup first", projectName)), nil
	}

	logging.VirtualStore("Python exec: project=%s, cmd=%s", projectName, command)
	result, err := env.ExecInRepo(ctx, command)
	if err != nil {
		return actionFailed(fmt.Sprintf("python exec in %q: %v", projectName, err)), nil
	}
	return ActionResult{
		Success: result.ExitCode == 0,
		Output:  result.Output(),
		Error:   nonZeroExit(result.ExitCode),
		Metadata: map[string]any{
			"project_name": projectName,
			"command":      command,
			"exit_code":    result.ExitCode,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_command_executed", Args: []any{projectName, command, int64(result.ExitCode), time.Now().Unix()}},
		},
	}, nil
}

// handlePythonRunPytest runs pytest in the environment. The run itself is the
// action; a failing test is its result, recorded in python_pytest_result.
// Payload: project_name, test_args (optional).
func (v *VirtualStore) handlePythonRunPytest(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	if projectName == "" {
		return ActionResult{Success: false, Error: "project_name required in payload"}, nil
	}
	testArgs := payloadStrings(req.Payload, "test_args")

	env, ok := v.bench().env(projectName)
	if !ok {
		return actionFailed(fmt.Sprintf("no python environment %q; run python_env_setup first", projectName)), nil
	}

	logging.VirtualStore("Python pytest: project=%s, args=%v", projectName, testArgs)
	result, err := env.RunPytest(ctx, testArgs...)
	if err != nil {
		return actionFailed(fmt.Sprintf("pytest in %q: %v", projectName, err)), nil
	}
	now := time.Now().Unix()
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("pytest %s in %s: passed=%v exit=%d\n%s", strings.Join(testArgs, " "), projectName, result.Passed, result.ExitCode, result.Output),
		Metadata: map[string]any{
			"project_name": projectName,
			"test_args":    testArgs,
			"passed":       result.Passed,
			"exit_code":    result.ExitCode,
		},
		FactsToAdd: []Fact{
			{Predicate: "pytest_execution", Args: []any{projectName, int64(len(testArgs)), now}},
			{Predicate: "python_pytest_result", Args: []any{projectName, boolAtom(result.Passed), int64(result.ExitCode), now}},
		},
	}, nil
}

// handlePythonApplyPatch applies a unified diff to the environment's repo.
// Payload: project_name, patch.
func (v *VirtualStore) handlePythonApplyPatch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	patch, _ := req.Payload["patch"].(string)
	if projectName == "" || patch == "" {
		return ActionResult{Success: false, Error: "project_name and patch required in payload"}, nil
	}

	env, ok := v.bench().env(projectName)
	if !ok {
		return actionFailed(fmt.Sprintf("no python environment %q; run python_env_setup first", projectName)), nil
	}

	logging.VirtualStore("Python apply patch: project=%s, size=%d", projectName, len(patch))
	now := time.Now().Unix()
	if err := env.ApplyPatch(ctx, patch); err != nil {
		return actionFailed(fmt.Sprintf("patch not applied to %q: %v", projectName, err),
			Fact{Predicate: "python_environment", Args: []any{projectName, env.ContainerID(), "/error", now}}), nil
	}
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Patch applied to %s (%d bytes)", projectName, len(patch)),
		Metadata: map[string]any{
			"project_name": projectName,
			"patch_size":   len(patch),
		},
		FactsToAdd: []Fact{
			{Predicate: "python_patch_applied", Args: []any{projectName, int64(len(patch)), now}},
			{Predicate: "python_environment", Args: []any{projectName, env.ContainerID(), "/patched", now}},
		},
	}, nil
}

// handlePythonSnapshot commits the environment's container under a name.
// Payload: project_name, snapshot_name (optional).
func (v *VirtualStore) handlePythonSnapshot(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)
	if projectName == "" {
		return ActionResult{Success: false, Error: "project_name required in payload"}, nil
	}
	if snapshotName == "" {
		snapshotName = fmt.Sprintf("%s-snapshot-%d", projectName, time.Now().Unix())
	}

	b := v.bench()
	env, ok := b.env(projectName)
	if !ok {
		return actionFailed(fmt.Sprintf("no python environment %q; run python_env_setup first", projectName)), nil
	}

	logging.VirtualStore("Python snapshot: project=%s, name=%s", projectName, snapshotName)
	snapshot, err := env.Snapshot(ctx, snapshotName)
	if err != nil {
		return actionFailed(fmt.Sprintf("python snapshot of %q: %v", projectName, err)), nil
	}
	b.recordSnapshot("python:"+projectName, snapshotName, snapshot.ID)
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Snapshot created: %s (%s)", snapshotName, snapshot.ImageTag),
		Metadata: map[string]any{
			"project_name":  projectName,
			"snapshot_name": snapshotName,
			"snapshot_id":   snapshot.ID,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_snapshot", Args: []any{projectName, snapshotName, time.Now().Unix()}},
		},
	}, nil
}

// handlePythonRestore restores the environment from a named snapshot taken by
// python_snapshot.
// Payload: project_name, snapshot_name.
func (v *VirtualStore) handlePythonRestore(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)
	if projectName == "" || snapshotName == "" {
		return ActionResult{Success: false, Error: "project_name and snapshot_name required in payload"}, nil
	}

	b := v.bench()
	env, ok := b.env(projectName)
	if !ok {
		return actionFailed(fmt.Sprintf("no python environment %q; run python_env_setup first", projectName)), nil
	}
	snapshotID, ok := b.snapshotID("python:"+projectName, snapshotName)
	if !ok {
		return actionFailed(fmt.Sprintf("no snapshot %q for python environment %q", snapshotName, projectName)), nil
	}

	logging.VirtualStore("Python restore: project=%s, snapshot=%s", projectName, snapshotName)
	now := time.Now().Unix()
	if err := env.RestoreSnapshot(ctx, snapshotID); err != nil {
		return actionFailed(fmt.Sprintf("python restore of %q from %q: %v", projectName, snapshotName, err),
			Fact{Predicate: "python_environment", Args: []any{projectName, env.ContainerID(), "/error", now}}), nil
	}
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Restored %s from snapshot %s", projectName, snapshotName),
		Metadata: map[string]any{
			"project_name":  projectName,
			"snapshot_name": snapshotName,
			"container_id":  env.ContainerID(),
		},
		FactsToAdd: []Fact{
			{Predicate: "python_restored", Args: []any{projectName, snapshotName, now}},
			{Predicate: "python_environment", Args: []any{projectName, env.ContainerID(), "/ready", now}},
		},
	}, nil
}

// handlePythonTeardown removes the environment's container.
// Payload: project_name.
func (v *VirtualStore) handlePythonTeardown(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	if projectName == "" {
		return ActionResult{Success: false, Error: "project_name required in payload"}, nil
	}

	b := v.bench()
	env, ok := b.env(projectName)
	if !ok {
		return actionFailed(fmt.Sprintf("no python environment %q to tear down", projectName)), nil
	}

	logging.VirtualStore("Python teardown: project=%s", projectName)
	if err := env.Teardown(ctx); err != nil {
		return actionFailed(fmt.Sprintf("python teardown of %q: %v", projectName, err)), nil
	}
	b.mu.Lock()
	delete(b.envs, projectName)
	delete(b.snapshots, "python:"+projectName)
	b.mu.Unlock()

	now := time.Now().Unix()
	return ActionResult{
		Success:  true,
		Output:   fmt.Sprintf("Python environment torn down for %s", projectName),
		Metadata: map[string]any{"project_name": projectName},
		FactsToAdd: []Fact{
			{Predicate: "python_environment", Args: []any{projectName, "", "/terminated", now}},
			{Predicate: "python_teardown_complete", Args: []any{projectName, now}},
		},
	}, nil
}

// =============================================================================
// SWE-BENCH ACTION HANDLERS (Benchmark-specific)
// =============================================================================

// handleSWEBenchSetup records an instance and its test expectations, then
// builds its environment: container at the instance image, clone at the base
// commit, venv, dependencies. The instance and expectation facts land even
// when the environment cannot be built -- they describe the instance, not the
// environment.
// Payload: instance_id, repo, base_commit, problem_statement, fail_to_pass,
// pass_to_pass, version (optional).
func (v *VirtualStore) handleSWEBenchSetup(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	if instanceID == "" {
		return ActionResult{Success: false, Error: "instance_id required in payload"}, nil
	}

	logging.VirtualStore("SWE-bench setup: instance=%s", instanceID)

	repo, _ := req.Payload["repo"].(string)
	baseCommit, _ := req.Payload["base_commit"].(string)
	problemStatement, _ := req.Payload["problem_statement"].(string)
	version, _ := req.Payload["version"].(string)
	instance := &swebench.Instance{
		InstanceID:       instanceID,
		Repo:             repo,
		BaseCommit:       baseCommit,
		ProblemStatement: problemStatement,
		Version:          version,
		FailToPass:       payloadStrings(req.Payload, "fail_to_pass"),
		PassToPass:       payloadStrings(req.Payload, "pass_to_pass"),
	}
	now := time.Now().Unix()

	facts := []Fact{{Predicate: "swebench_instance", Args: []any{instanceID, repo, baseCommit, version}}}
	for _, test := range instance.FailToPass {
		facts = append(facts, Fact{Predicate: "swebench_expected_fail_to_pass", Args: []any{instanceID, test}})
	}
	for _, test := range instance.PassToPass {
		facts = append(facts, Fact{Predicate: "swebench_expected_pass_to_pass", Args: []any{instanceID, test}})
	}
	metadata := map[string]any{
		"instance_id":       instanceID,
		"repo":              repo,
		"base_commit":       baseCommit,
		"problem_statement": problemStatement,
		"fail_to_pass":      instance.FailToPass,
		"pass_to_pass":      instance.PassToPass,
	}

	b := v.bench()
	if _, exists := b.harness(instanceID); exists {
		return actionFailed(fmt.Sprintf("swebench instance %q already set up; tear it down first", instanceID)), nil
	}
	if !b.runtime.IsAvailable() {
		facts = append(facts, Fact{Predicate: "swebench_environment", Args: []any{instanceID, "", "/error", now}})
		return ActionResult{Success: false, Error: errRuntimeUnavailable, Metadata: metadata, FactsToAdd: facts}, nil
	}

	harness := swebench.NewHarness(instance, python.DefaultConfig(), b.runtime)
	if err := harness.Initialize(ctx); err != nil {
		facts = append(facts, Fact{Predicate: "swebench_environment", Args: []any{instanceID, "", "/error", now}})
		return ActionResult{Success: false, Error: fmt.Sprintf("swebench %s: %v", instanceID, err), Metadata: metadata, FactsToAdd: facts}, nil
	}
	b.mu.Lock()
	b.harnesses[instanceID] = harness
	b.mu.Unlock()
	containerID := harness.Environment().ContainerID()
	if err := harness.Setup(ctx); err != nil {
		facts = append(facts, Fact{Predicate: "swebench_environment", Args: []any{instanceID, containerID, "/error", now}})
		return ActionResult{Success: false, Error: fmt.Sprintf("swebench %s setup: %v", instanceID, err), Metadata: metadata, FactsToAdd: facts}, nil
	}

	facts = append(facts, Fact{Predicate: "swebench_environment", Args: []any{instanceID, containerID, "/ready", now}})
	metadata["container_id"] = containerID
	return ActionResult{
		Success:    true,
		Output:     fmt.Sprintf("SWE-bench environment ready for %s (%s@%s)", instanceID, repo, shortContainerID(baseCommit)),
		Metadata:   metadata,
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchApplyPatch applies a model-generated patch to the instance repo.
// Payload: instance_id, patch.
func (v *VirtualStore) handleSWEBenchApplyPatch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	patch, _ := req.Payload["patch"].(string)
	if instanceID == "" || patch == "" {
		return ActionResult{Success: false, Error: "instance_id and patch required in payload"}, nil
	}

	harness, ok := v.bench().harness(instanceID)
	if !ok {
		return actionFailed(fmt.Sprintf("no swebench environment for %q; run swebench_setup first", instanceID)), nil
	}

	logging.VirtualStore("SWE-bench apply patch: instance=%s, patch_size=%d", instanceID, len(patch))
	now := time.Now().Unix()
	containerID := harness.Environment().ContainerID()
	if err := harness.ApplyPatch(ctx, patch); err != nil {
		return actionFailed(fmt.Sprintf("patch not applied to %s: %v", instanceID, err),
			Fact{Predicate: "swebench_environment", Args: []any{instanceID, containerID, "/error", now}}), nil
	}
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Patch applied to instance %s (%d bytes)", instanceID, len(patch)),
		Metadata: map[string]any{
			"instance_id": instanceID,
			"patch_size":  len(patch),
		},
		FactsToAdd: []Fact{
			{Predicate: "swebench_patch_applied", Args: []any{instanceID, int64(len(patch)), now}},
			{Predicate: "swebench_environment", Args: []any{instanceID, containerID, "/patched", now}},
		},
	}, nil
}

// handleSWEBenchRunTests runs the named tests (default: every expected test)
// and records one swebench_test_result per test.
// Payload: instance_id, test_names (optional).
func (v *VirtualStore) handleSWEBenchRunTests(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	if instanceID == "" {
		return ActionResult{Success: false, Error: "instance_id required in payload"}, nil
	}

	harness, ok := v.bench().harness(instanceID)
	if !ok {
		return actionFailed(fmt.Sprintf("no swebench environment for %q; run swebench_setup first", instanceID)), nil
	}
	testNames := payloadStrings(req.Payload, "test_names")
	if len(testNames) == 0 {
		testNames = harness.Instance().AllTests()
	}

	logging.VirtualStore("SWE-bench run tests: instance=%s, tests=%d", instanceID, len(testNames))
	results, err := harness.Environment().RunTests(ctx, testNames)
	if err != nil {
		return actionFailed(fmt.Sprintf("swebench tests for %s: %v", instanceID, err)), nil
	}
	facts := make([]Fact, 0, len(results))
	passed := 0
	for _, name := range sortedKeys(results) {
		r := results[name]
		if r.Passed {
			passed++
		}
		facts = append(facts, Fact{Predicate: "swebench_test_result", Args: []any{instanceID, name, boolAtom(r.Passed), r.Duration.Milliseconds()}})
	}
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Ran %d tests for instance %s: %d passed, %d failed", len(results), instanceID, passed, len(results)-passed),
		Metadata: map[string]any{
			"instance_id": instanceID,
			"test_count":  len(results),
			"passed":      passed,
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchSnapshot commits the instance container under a name.
// Payload: instance_id, snapshot_name (optional).
func (v *VirtualStore) handleSWEBenchSnapshot(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)
	if instanceID == "" {
		return ActionResult{Success: false, Error: "instance_id required in payload"}, nil
	}
	if snapshotName == "" {
		snapshotName = fmt.Sprintf("%s-snapshot-%d", instanceID, time.Now().Unix())
	}

	b := v.bench()
	harness, ok := b.harness(instanceID)
	if !ok {
		return actionFailed(fmt.Sprintf("no swebench environment for %q; run swebench_setup first", instanceID)), nil
	}

	logging.VirtualStore("SWE-bench snapshot: instance=%s, name=%s", instanceID, snapshotName)
	snapshot, err := harness.Environment().Snapshot(ctx, snapshotName)
	if err != nil {
		return actionFailed(fmt.Sprintf("swebench snapshot of %s: %v", instanceID, err)), nil
	}
	b.recordSnapshot("swebench:"+instanceID, snapshotName, snapshot.ID)
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Snapshot created: %s (%s)", snapshotName, snapshot.ImageTag),
		Metadata: map[string]any{
			"instance_id":   instanceID,
			"snapshot_name": snapshotName,
			"snapshot_id":   snapshot.ID,
		},
		FactsToAdd: []Fact{
			{Predicate: "swebench_snapshot", Args: []any{instanceID, snapshotName, time.Now().Unix()}},
		},
	}, nil
}

// handleSWEBenchRestore restores the instance from a named snapshot.
// Payload: instance_id, snapshot_name.
func (v *VirtualStore) handleSWEBenchRestore(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)
	if instanceID == "" || snapshotName == "" {
		return ActionResult{Success: false, Error: "instance_id and snapshot_name required in payload"}, nil
	}

	b := v.bench()
	harness, ok := b.harness(instanceID)
	if !ok {
		return actionFailed(fmt.Sprintf("no swebench environment for %q; run swebench_setup first", instanceID)), nil
	}
	snapshotID, ok := b.snapshotID("swebench:"+instanceID, snapshotName)
	if !ok {
		return actionFailed(fmt.Sprintf("no snapshot %q for swebench instance %q", snapshotName, instanceID)), nil
	}

	logging.VirtualStore("SWE-bench restore: instance=%s, snapshot=%s", instanceID, snapshotName)
	env := harness.Environment()
	now := time.Now().Unix()
	if err := env.RestoreSnapshot(ctx, snapshotID); err != nil {
		return actionFailed(fmt.Sprintf("swebench restore of %s from %q: %v", instanceID, snapshotName, err),
			Fact{Predicate: "swebench_environment", Args: []any{instanceID, env.ContainerID(), "/error", now}}), nil
	}
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Restored instance %s from snapshot %s", instanceID, snapshotName),
		Metadata: map[string]any{
			"instance_id":   instanceID,
			"snapshot_name": snapshotName,
		},
		FactsToAdd: []Fact{
			{Predicate: "swebench_restored", Args: []any{instanceID, snapshotName, now}},
			{Predicate: "swebench_environment", Args: []any{instanceID, env.ContainerID(), "/ready", now}},
		},
	}, nil
}

// handleSWEBenchEvaluate applies a prediction and runs every FAIL_TO_PASS and
// PASS_TO_PASS test. It records the per-test results and the harness summary;
// whether the instance is resolved is derived by the kernel from those results
// and the expectations (benchmarks.mg swebench_resolved), not decided here.
// Payload: instance_id, patch, model_name (optional).
func (v *VirtualStore) handleSWEBenchEvaluate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	patch, _ := req.Payload["patch"].(string)
	modelName, _ := req.Payload["model_name"].(string)
	if instanceID == "" {
		return ActionResult{Success: false, Error: "instance_id required in payload"}, nil
	}
	if modelName == "" {
		modelName = "codenerd"
	}

	harness, ok := v.bench().harness(instanceID)
	if !ok {
		// Scoring needs the instance's container; without it nothing was
		// run and no verdict exists. Park the environment in /error rather
		// than let a missing harness read as an evaluation.
		return actionFailed(fmt.Sprintf("swebench evaluation for %q not run: no environment; run swebench_setup first", instanceID),
			Fact{Predicate: "swebench_environment", Args: []any{instanceID, "", "/error", time.Now().Unix()}}), nil
	}

	logging.VirtualStore("SWE-bench evaluate: instance=%s, model=%s", instanceID, modelName)
	now := time.Now().Unix()
	result, err := harness.Evaluate(ctx, &swebench.Prediction{InstanceID: instanceID, ModelPatch: patch, ModelNameOrPath: modelName})
	containerID := harness.Environment().ContainerID()
	facts := []Fact{{Predicate: "swebench_evaluation_started", Args: []any{instanceID, modelName, now}}}
	if err != nil {
		facts = append(facts, Fact{Predicate: "swebench_environment", Args: []any{instanceID, containerID, "/error", now}})
		return ActionResult{Success: false, Error: fmt.Sprintf("swebench evaluation of %s: %v", instanceID, err), FactsToAdd: facts}, nil
	}
	for _, group := range []map[string]swebench.TestResult{result.FailToPassResults, result.PassToPassResults} {
		for _, name := range sortedKeys(group) {
			r := group[name]
			facts = append(facts, Fact{Predicate: "swebench_test_result", Args: []any{instanceID, name, boolAtom(r.Passed), r.Duration.Milliseconds()}})
		}
	}
	facts = append(facts, Fact{Predicate: "swebench_evaluation_result",
		Args: []any{instanceID, boolAtom(result.Resolved), int64(result.PassedTests), int64(result.FailedTests)}})
	state := "/ready"
	if result.Error != "" {
		state = "/error"
	}
	facts = append(facts, Fact{Predicate: "swebench_environment", Args: []any{instanceID, containerID, state, now}})

	out := ActionResult{
		Success: result.Error == "",
		Output:  result.Summary(),
		Metadata: map[string]any{
			"instance_id":  instanceID,
			"model_name":   modelName,
			"patch_size":   len(patch),
			"passed_tests": result.PassedTests,
			"failed_tests": result.FailedTests,
		},
		FactsToAdd: facts,
	}
	if result.Error != "" {
		out.Error = fmt.Sprintf("swebench evaluation of %s failed in %s phase: %s", instanceID, result.ErrorPhase, result.Error)
	}
	return out, nil
}

// handleSWEBenchTeardown removes the instance's container.
// Payload: instance_id.
func (v *VirtualStore) handleSWEBenchTeardown(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	if instanceID == "" {
		return ActionResult{Success: false, Error: "instance_id required in payload"}, nil
	}

	b := v.bench()
	harness, ok := b.harness(instanceID)
	if !ok {
		return actionFailed(fmt.Sprintf("no swebench environment for %q to tear down", instanceID)), nil
	}

	logging.VirtualStore("SWE-bench teardown: instance=%s", instanceID)
	if err := harness.Teardown(ctx); err != nil {
		return actionFailed(fmt.Sprintf("swebench teardown of %s: %v", instanceID, err)), nil
	}
	b.mu.Lock()
	delete(b.harnesses, instanceID)
	delete(b.snapshots, "swebench:"+instanceID)
	b.mu.Unlock()

	now := time.Now().Unix()
	return ActionResult{
		Success:  true,
		Output:   fmt.Sprintf("SWE-bench environment torn down for instance %s", instanceID),
		Metadata: map[string]any{"instance_id": instanceID},
		FactsToAdd: []Fact{
			{Predicate: "swebench_environment", Args: []any{instanceID, "", "/terminated", now}},
			{Predicate: "swebench_teardown_complete", Args: []any{instanceID, now}},
		},
	}, nil
}

// SWEBenchVerdict reads the kernel's verdict for an instance: whether
// swebench_resolved holds and which expected tests have no passing result
// (swebench_unmet_expectation). It reports; the decision is benchmarks.mg's.
func SWEBenchVerdict(kernel Kernel, instanceID string) (resolved bool, unmet []string, err error) {
	if kernel == nil {
		return false, nil, fmt.Errorf("no kernel")
	}
	resolvedFacts, err := kernel.Query("swebench_resolved")
	if err != nil {
		return false, nil, fmt.Errorf("query swebench_resolved: %w", err)
	}
	for _, f := range resolvedFacts {
		if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == instanceID {
			resolved = true
		}
	}
	unmetFacts, err := kernel.Query("swebench_unmet_expectation")
	if err != nil {
		return false, nil, fmt.Errorf("query swebench_unmet_expectation: %w", err)
	}
	for _, f := range unmetFacts {
		if len(f.Args) > 1 && types.ExtractString(f.Args[0]) == instanceID {
			unmet = append(unmet, types.ExtractString(f.Args[1]))
		}
	}
	sort.Strings(unmet)
	return resolved, unmet, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func shortContainerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func nonZeroExit(code int) string {
	if code == 0 {
		return ""
	}
	return fmt.Sprintf("command exited %d", code)
}
