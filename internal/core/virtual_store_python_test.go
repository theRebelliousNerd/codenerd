package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// These handlers used to return Success with facts for work nothing did (no
// container, no command, no patch). They now drive python.Environment and
// swebench.Harness through the store's container runtime; the tests drive
// them through an in-memory runtime (fake_container_runtime_test.go).

func pythonStore(t *testing.T, rt *fakeContainerRuntime) *VirtualStore {
	t.Helper()
	vs := &VirtualStore{kernel: setupMockKernel(t)}
	vs.SetContainerRuntime(rt)
	return vs
}

func factPredicates(facts []Fact) []string {
	out := make([]string, 0, len(facts))
	for _, f := range facts {
		out = append(out, f.Predicate)
	}
	return out
}

func hasFact(facts []Fact, predicate string, argIndex int, want any) bool {
	for _, f := range facts {
		if f.Predicate == predicate && len(f.Args) > argIndex && f.Args[argIndex] == want {
			return true
		}
	}
	return false
}

type pythonHandler func(*VirtualStore, context.Context, ActionRequest) (ActionResult, error)

func TestPythonHandlers_ValidatePayloadAndContext(t *testing.T) {
	cases := []struct {
		name    string
		handler pythonHandler
		wantErr string
		payload map[string]any
	}{
		{"setup", (*VirtualStore).handlePythonEnvSetup, "project_name or git_url required in payload", map[string]any{"project_name": "p"}},
		{"exec", (*VirtualStore).handlePythonEnvExec, "project_name and command required in payload", map[string]any{"project_name": "p", "command": "true"}},
		{"pytest", (*VirtualStore).handlePythonRunPytest, "project_name required in payload", map[string]any{"project_name": "p"}},
		{"patch", (*VirtualStore).handlePythonApplyPatch, "project_name and patch required in payload", map[string]any{"project_name": "p", "patch": "x"}},
		{"snapshot", (*VirtualStore).handlePythonSnapshot, "project_name required in payload", map[string]any{"project_name": "p"}},
		{"restore", (*VirtualStore).handlePythonRestore, "project_name and snapshot_name required in payload", map[string]any{"project_name": "p", "snapshot_name": "s"}},
		{"teardown", (*VirtualStore).handlePythonTeardown, "project_name required in payload", map[string]any{"project_name": "p"}},
		{"swebench setup", (*VirtualStore).handleSWEBenchSetup, "instance_id required in payload", map[string]any{"instance_id": "i"}},
		{"swebench patch", (*VirtualStore).handleSWEBenchApplyPatch, "instance_id and patch required in payload", map[string]any{"instance_id": "i", "patch": "x"}},
		{"swebench tests", (*VirtualStore).handleSWEBenchRunTests, "instance_id required in payload", map[string]any{"instance_id": "i"}},
		{"swebench snapshot", (*VirtualStore).handleSWEBenchSnapshot, "instance_id required in payload", map[string]any{"instance_id": "i"}},
		{"swebench restore", (*VirtualStore).handleSWEBenchRestore, "instance_id and snapshot_name required in payload", map[string]any{"instance_id": "i", "snapshot_name": "s"}},
		{"swebench evaluate", (*VirtualStore).handleSWEBenchEvaluate, "instance_id required in payload", map[string]any{"instance_id": "i"}},
		{"swebench teardown", (*VirtualStore).handleSWEBenchTeardown, "instance_id required in payload", map[string]any{"instance_id": "i"}},
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vs := pythonStore(t, newFakeContainerRuntime())
			res, err := tc.handler(vs, context.Background(), ActionRequest{Payload: map[string]any{}})
			if err != nil || res.Success || res.Error != tc.wantErr {
				t.Fatalf("empty payload: got %+v, %v; want error %q", res, err, tc.wantErr)
			}
			res, err = tc.handler(vs, canceled, ActionRequest{Payload: tc.payload})
			if err != nil || res.Success || res.Error != context.Canceled.Error() {
				t.Fatalf("canceled context: got %+v, %v", res, err)
			}
		})
	}
}

// Without a runtime nothing can be created, and without an environment
// nothing can run: both are reported, never claimed.
func TestPythonHandlers_RefuseInsteadOfClaimingWork(t *testing.T) {
	ctx := context.Background()
	rt := newFakeContainerRuntime()
	rt.available = false
	vs := pythonStore(t, rt)

	res, err := vs.handlePythonEnvSetup(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj"}})
	if err != nil || res.Success || res.Error != errRuntimeUnavailable {
		t.Fatalf("setup without runtime: %+v, %v", res, err)
	}
	if !hasFact(res.FactsToAdd, "python_environment", 2, "/error") {
		t.Fatalf("setup without runtime must park the environment in /error, got %v", res.FactsToAdd)
	}

	for name, handler := range map[string]pythonHandler{
		"exec":     (*VirtualStore).handlePythonEnvExec,
		"pytest":   (*VirtualStore).handlePythonRunPytest,
		"patch":    (*VirtualStore).handlePythonApplyPatch,
		"snapshot": (*VirtualStore).handlePythonSnapshot,
		"teardown": (*VirtualStore).handlePythonTeardown,
	} {
		res, err := handler(vs, ctx, ActionRequest{Payload: map[string]any{"project_name": "proj", "command": "true", "patch": "p"}})
		if err != nil || res.Success || !strings.Contains(res.Error, "proj") {
			t.Fatalf("%s without an environment should fail naming it, got %+v, %v", name, res, err)
		}
		if len(res.FactsToAdd) != 0 {
			t.Fatalf("%s without an environment asserted %v", name, factPredicates(res.FactsToAdd))
		}
	}
	if rt.opsContaining("exec") != 0 {
		t.Fatalf("no command may run without an environment, runtime saw %v", rt.ops)
	}
}

func TestPythonEnvironmentLifecycleRunsInTheContainer(t *testing.T) {
	ctx := context.Background()
	rt := newFakeContainerRuntime()
	vs := pythonStore(t, rt)

	res, err := vs.handlePythonEnvSetup(ctx, ActionRequest{Payload: map[string]any{
		"project_name": "proj", "git_url": "https://example.invalid/org/proj.git", "commit": "abc123",
	}})
	if err != nil || !res.Success {
		t.Fatalf("setup: %+v, %v", res, err)
	}
	if !hasFact(res.FactsToAdd, "python_environment", 2, "/ready") || !hasFact(res.FactsToAdd, "python_project_source", 1, "https://example.invalid/org/proj.git") {
		t.Fatalf("setup facts: %v", res.FactsToAdd)
	}
	if rt.opsContaining("exec git clone") != 1 || rt.opsContaining("exec git checkout abc123") != 1 || rt.opsContaining("-m venv") != 1 {
		t.Fatalf("setup did not clone, check out and build a venv in the container: %v", rt.ops)
	}

	res, _ = vs.handlePythonEnvExec(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj", "command": "python -c 'print(1)'"}})
	if !res.Success || rt.opsContaining("python -c 'print(1)'") != 1 {
		t.Fatalf("exec did not run in the container: %+v ops=%v", res, rt.ops)
	}

	rt.failing["tests/test_b.py::test_b"] = true
	res, _ = vs.handlePythonRunPytest(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj", "test_args": []any{"tests/test_b.py::test_b"}}})
	if !res.Success || !hasFact(res.FactsToAdd, "python_pytest_result", 1, "/false") {
		t.Fatalf("a failing test is the run's result, recorded as /false: %+v", res)
	}

	res, _ = vs.handlePythonApplyPatch(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj", "patch": "BROKEN diff"}})
	if res.Success || !hasFact(res.FactsToAdd, "python_environment", 2, "/error") {
		t.Fatalf("a patch that does not apply must fail, got %+v", res)
	}
	res, _ = vs.handlePythonApplyPatch(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj", "patch": "good diff"}})
	if !res.Success || !hasFact(res.FactsToAdd, "python_patch_applied", 0, "proj") {
		t.Fatalf("apply patch: %+v", res)
	}

	res, _ = vs.handlePythonSnapshot(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj", "snapshot_name": "after-patch"}})
	if !res.Success || rt.opsContaining("snapshot ") < 1 {
		t.Fatalf("snapshot: %+v ops=%v", res, rt.ops)
	}
	res, _ = vs.handlePythonRestore(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj", "snapshot_name": "missing"}})
	if res.Success {
		t.Fatal("restore of an unknown snapshot must fail")
	}
	res, _ = vs.handlePythonRestore(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj", "snapshot_name": "after-patch"}})
	if !res.Success || rt.opsContaining("restore ") != 1 {
		t.Fatalf("restore: %+v ops=%v", res, rt.ops)
	}

	res, _ = vs.handlePythonEnvSetup(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj"}})
	if res.Success {
		t.Fatal("setting up an existing environment again must fail")
	}

	res, _ = vs.handlePythonTeardown(ctx, ActionRequest{Payload: map[string]any{"project_name": "proj"}})
	if !res.Success || rt.liveCount() != 0 {
		t.Fatalf("teardown left %d containers: %+v", rt.liveCount(), res)
	}
}

// Container executions are executions: the store attaches its audit logger to
// the runtime, so a command run in an environment lands as execution facts.
func TestContainerExecutionsReachTheKernelThroughTheAuditLogger(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	rt := newFakeContainerRuntime()
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetKernel(kernel)
	vs.SetContainerRuntime(rt)
	vs.bench()

	rt.mu.Lock()
	sink := rt.audit
	rt.mu.Unlock()
	if sink == nil {
		t.Fatal("the store did not attach its audit logger to the container runtime")
	}
	cmd := tactile.Command{Binary: "pytest", Arguments: []string{"-x"}, RequestID: "container-audit-probe"}
	sink(tactile.AuditEvent{Type: tactile.AuditEventStart, Timestamp: time.Now(), Command: cmd, ExecutorName: "persistent-docker"})
	sink(tactile.AuditEvent{Type: tactile.AuditEventComplete, Timestamp: time.Now(), Command: cmd, ExecutorName: "persistent-docker",
		Result: &tactile.ExecutionResult{Success: true, ExitCode: 0, Duration: time.Second}})

	facts, err := kernel.Query("execution_completed")
	if err != nil {
		t.Fatalf("query execution_completed: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("a container execution left no execution_completed fact")
	}
}

// routeSWEBench files the pending_action the CLI files and routes the action,
// so the facts land in the kernel the way production lands them.
func routeSWEBench(t *testing.T, vs *VirtualStore, kernel *RealKernel, actionType, target string, payload map[string]any) ActionResult {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	actionID := "swebench-" + actionType + "-" + target
	pending := Fact{Predicate: "pending_action", Args: []any{actionID, types.MangleAtom("/" + actionType), target, string(encoded), time.Now().Unix()}}
	if err := kernel.Assert(pending); err != nil {
		t.Fatalf("assert pending_action: %v", err)
	}
	defer func() { _ = kernel.RetractExactFactsBatch([]Fact{pending}) }()
	res, err := vs.RouteActionResult(context.Background(), Fact{Predicate: "next_action", Args: []any{actionID, "/" + actionType, target, payload}})
	if err != nil {
		t.Fatalf("route %s: %v", actionType, err)
	}
	return res
}

// End to end: setup and evaluate route through the VirtualStore into a real
// kernel, and the kernel -- not the harness -- derives whether the instance
// is resolved from the per-test results and the expectations.
func TestSWEBenchResolutionIsDerivedByTheKernel(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	rt := newFakeContainerRuntime()
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetKernel(kernel)
	vs.DisableBootGuard()
	vs.SetContainerRuntime(rt)

	setup := func(id string) {
		res := routeSWEBench(t, vs, kernel, "swebench_setup", id, map[string]any{
			"instance_id": id, "repo": "org/" + id, "base_commit": "abc123",
			"fail_to_pass": []any{id + "::test_fix"}, "pass_to_pass": []any{id + "::test_keep"},
		})
		if !res.Success {
			t.Fatalf("setup %s: %+v", id, res)
		}
	}
	setup("good")
	setup("bad")
	rt.failing["bad::test_fix"] = true

	for _, id := range []string{"good", "bad"} {
		res := routeSWEBench(t, vs, kernel, "swebench_evaluate", id, map[string]any{"instance_id": id, "patch": "diff for " + id})
		if !res.Success {
			t.Fatalf("evaluate %s: %+v", id, res)
		}
	}

	resolved, err := kernel.Query("swebench_resolved")
	if err != nil {
		t.Fatalf("query swebench_resolved: %v", err)
	}
	if len(resolved) != 1 || types.ExtractString(resolved[0].Args[0]) != "good" {
		t.Fatalf("swebench_resolved = %v, want exactly the instance whose expected tests all passed", resolved)
	}
	unmet, err := kernel.Query("swebench_unmet_expectation")
	if err != nil {
		t.Fatalf("query swebench_unmet_expectation: %v", err)
	}
	if len(unmet) != 1 || types.ExtractString(unmet[0].Args[1]) != "bad::test_fix" {
		t.Fatalf("swebench_unmet_expectation = %v, want bad::test_fix", unmet)
	}

	// The Go consumer reports the kernel's verdict as derived.
	for id, want := range map[string]bool{"good": true, "bad": false} {
		resolved, unmetTests, err := SWEBenchVerdict(kernel, id)
		if err != nil {
			t.Fatalf("SWEBenchVerdict(%s): %v", id, err)
		}
		if resolved != want {
			t.Fatalf("SWEBenchVerdict(%s) resolved = %v, want %v (unmet %v)", id, resolved, want, unmetTests)
		}
		if !want && (len(unmetTests) != 1 || unmetTests[0] != "bad::test_fix") {
			t.Fatalf("SWEBenchVerdict(bad) unmet = %v", unmetTests)
		}
	}
}

// The verdict is the kernel's: a summary that claims resolution is not
// enough when an expected test has no passing result.
func TestSWEBenchResolvedRequiresPassingResultsNotAClaim(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	facts := []Fact{
		{Predicate: "swebench_expected_fail_to_pass", Args: []any{"claimed", "claimed::t"}},
		{Predicate: "swebench_evaluation_result", Args: []any{"claimed", "/true", int64(1), int64(0)}},
	}
	for _, f := range facts {
		if err := kernel.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	resolved, err := kernel.Query("swebench_resolved")
	if err != nil {
		t.Fatalf("query swebench_resolved: %v", err)
	}
	if len(resolved) != 0 {
		t.Fatalf("resolution was taken from the harness claim, not derived from test results: %v", resolved)
	}
	if err := kernel.Assert(Fact{Predicate: "swebench_test_result", Args: []any{"claimed", "claimed::t", "/true", int64(5)}}); err != nil {
		t.Fatalf("assert test result: %v", err)
	}
	resolved, err = kernel.Query("swebench_resolved")
	if err != nil {
		t.Fatalf("query swebench_resolved: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("with every expected test passing the instance must be resolved, got %v", resolved)
	}
}

// An instance whose patch does not apply records no test results, so it is
// evaluated but not resolved.
func TestSWEBenchUnappliedPatchIsNotResolved(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	rt := newFakeContainerRuntime()
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetKernel(kernel)
	vs.DisableBootGuard()
	vs.SetContainerRuntime(rt)

	routeSWEBench(t, vs, kernel, "swebench_setup", "inst", map[string]any{
		"instance_id": "inst", "repo": "org/inst", "base_commit": "abc123", "fail_to_pass": []any{"inst::t"},
	})
	res := routeSWEBench(t, vs, kernel, "swebench_evaluate", "inst", map[string]any{"instance_id": "inst", "patch": "BROKEN"})
	if res.Success {
		t.Fatalf("an evaluation whose patch failed must report failure, got %+v", res)
	}
	resolved, err := kernel.Query("swebench_resolved")
	if err != nil {
		t.Fatalf("query swebench_resolved: %v", err)
	}
	if len(resolved) != 0 {
		t.Fatalf("an unapplied patch was resolved: %v", resolved)
	}
}
