package core

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

func TestVirtualStorePytestLiteralArgv_GovernedRoute(test *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		test.Fatalf("NewRealKernel: %v", err)
	}
	kernel.SetWorkspace(test.TempDir())
	recorder := newFakeContainerRuntime()
	store := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	store.SetKernel(kernel)
	store.DisableBootGuard()
	store.SetContainerRuntime(recorder)
	test.Cleanup(func() {
		if err := store.Close(); err != nil {
			test.Errorf("close VirtualStore: %v", err)
		}
	})
	cases := []struct {
		name     string
		selector string
		failed   bool
	}{
		{name: "valid selected test", selector: "tests/test_math.py::test_add"},
		{name: "spaces and quotes", selector: "tests/test spaced.py::test_value[\"two words\" 'quoted']"},
		{name: "shell metacharacters", selector: "tests/test_math.py::test_add; echo ignored && echo injected | cat > /tmp/pytest-route-sentinel"},
		{name: "command substitution", selector: "tests/test_math.py::test_value[$(touch /tmp/pytest-route-sentinel) \x60touch /tmp/pytest-route-sentinel\x60]"},
		{name: "failed literal selector", selector: "tests/test_math.py::test_failure; echo ignored", failed: true},
	}
	for caseIndex, testcase := range cases {
		test.Run(testcase.name, func(test *testing.T) {
			instanceID := fmt.Sprintf("literal-argv-%d", caseIndex)
			recorder.mu.Lock()
			recorder.failing[testcase.selector] = testcase.failed
			recorder.mu.Unlock()
			setupResult := routeSWEBench(test, store, kernel, "swebench_setup", instanceID, map[string]any{
				"instance_id": instanceID, "repo": "org/" + instanceID, "base_commit": "abc123",
				"fail_to_pass": []any{testcase.selector},
			})
			if !setupResult.Success {
				test.Fatalf("governed setup failed: %+v", setupResult)
			}
			previousExecutions := len(recorder.executions())
			evaluationResult := routeSWEBench(test, store, kernel, "swebench_evaluate", instanceID, map[string]any{
				"instance_id": instanceID, "patch": "good diff for " + instanceID,
			})
			if !evaluationResult.Success {
				test.Fatalf("governed evaluation failed: %+v", evaluationResult)
			}
			var pytestInvocations []tactile.ContainerExecOptions
			for _, invocation := range recorder.executions()[previousExecutions:] {
				if invocation.Binary == "/testbed/venv/bin/python" {
					pytestInvocations = append(pytestInvocations, invocation)
				}
			}
			if len(pytestInvocations) != 1 {
				test.Fatalf("governed route executed %d pytest invocations, want one", len(pytestInvocations))
			}
			invocation := pytestInvocations[0]
			expectedArguments := []string{"-c", expectedPytestLauncher, testcase.selector}
			if !reflect.DeepEqual(invocation.Arguments, expectedArguments) {
				test.Errorf("governed pytest argv = %#v, want literal %#v", invocation.Arguments, expectedArguments)
			}
			if invocation.ContainerID == "" || invocation.WorkingDir != "/testbed/"+instanceID || invocation.Timeout != 5*time.Minute || len(invocation.Environment) != 0 {
				test.Errorf("governed pytest execution settings changed: %+v", invocation)
			}
			resultFacts, err := kernel.Query("swebench_test_result")
			if err != nil {
				test.Fatalf("query swebench_test_result: %v", err)
			}
			matchingResults := 0
			for _, resultFact := range resultFacts {
				if len(resultFact.Args) != 4 || types.ExtractString(resultFact.Args[0]) != instanceID {
					continue
				}
				matchingResults++
				expectedOutcome := "/true"
				if testcase.failed {
					expectedOutcome = "/false"
				}
				if types.ExtractString(resultFact.Args[1]) != testcase.selector || types.ExtractString(resultFact.Args[2]) != expectedOutcome {
					test.Errorf("governed kernel result = %v, want exact selector %q and %s", resultFact, testcase.selector, expectedOutcome)
				}
			}
			if matchingResults != 1 {
				test.Fatalf("kernel recorded %d selected-test results, want one", matchingResults)
			}
			resolved, unmetTests, err := SWEBenchVerdict(kernel, instanceID)
			if err != nil || resolved != !testcase.failed {
				test.Fatalf("kernel verdict resolved=%v unmet=%v error=%v, want resolved=%v", resolved, unmetTests, err, !testcase.failed)
			}
			if testcase.failed && !reflect.DeepEqual(unmetTests, []string{testcase.selector}) {
				test.Errorf("failed literal selector was lost from unmet expectations: %#v", unmetTests)
			}
		})
	}
}

func TestFakeContainerRuntimeRejectsUnrecognizedPytestInvocation(test *testing.T) {
	recorder := newFakeContainerRuntime()
	container, err := recorder.CreateContainer(context.Background(), tactile.ContainerCreateOptions{Image: "pytest-contract-fixture"})
	if err != nil {
		test.Fatal(err)
	}
	for _, arguments := range [][]string{
		{"-c", "print('unsupported launcher')", "tests/test_math.py::test_failure"},
		{"-m", "pytest", "-xvs", "tests/test_math.py::test_failure"},
		{"-c"},
	} {
		result, err := recorder.ExecInContainer(context.Background(), tactile.ContainerExecOptions{
			ContainerID: container.ID, Binary: "/testbed/venv/bin/python", Arguments: arguments,
		})
		if err == nil || result != nil {
			test.Fatalf("unknown pytest invocation became a fabricated success: argv=%#v result=%+v error=%v", arguments, result, err)
		}
	}
}
