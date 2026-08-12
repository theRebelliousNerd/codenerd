package session

import (
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/projectdoc"
	"codenerd/internal/types"
)

func TestResponsePresentsTestRunnerOutput(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"go pass line", "--- PASS: TestFoo (0.00s)", true},
		{"go fail line", "--- FAIL: TestFoo (0.00s)", true},
		{"go run line", "=== RUN   TestFoo", true},
		{"ok line tab", "ok  \tgithub.com/example/pkg 0.012s", true},
		{"ok line spaces", "ok  github.com/example/pkg 0.012s", true},
		{"pass arrow", "go test ./internal/world -run TestASTParser -v => PASS for all four", true},
		{"fail arrow", "go test ./... => FAIL", true},
		{"pass standalone", "PASS", true},
		{"fail standalone", "FAIL", true},
		{"pytest passed", "1 passed in 0.02s", true},
		{"pytest failed", "2 failed in 0.05s", true},
		{"jest passed", "Tests: 3 passed, 3 total", true},
		{"prose should pass", "tests should pass", false},
		{"prose make sure", "make sure the tests pass", false},
		{"prose did not run", "I did not run the tests", false},
		{"prose will pass", "the tests will pass", false},
		{"empty", "", false},
		{"ok without path", "ok  ", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := responsePresentsTestRunnerOutput(c.text)
			if got != c.want {
				t.Errorf("responsePresentsTestRunnerOutput(%q)=%v want %v", c.text, got, c.want)
			}
			if got2 := ResponsePresentsTestRunnerOutput(c.text); got2 != c.want {
				t.Errorf("ResponsePresentsTestRunnerOutput(%q)=%v want %v", c.text, got2, c.want)
			}
		})
	}
}

func newUnverifiedExec(t *testing.T) *Executor {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	e := NewExecutor(k, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.kernel = k
	e.config.WorkspaceRoot = t.TempDir()
	return e
}

func unverifiedResult(response string, successfulTestTools int) *ExecutionResult {
	res := &ExecutionResult{
		Response:             response,
		ToolCallsExecuted:    2,
		SuccessfulToolCalls:  2,
		SuccessfulWriteTools: 1,
		SuccessfulTestTools:  successfulTestTools,
	}
	res.Intent.Verb = "/fix"
	res.Intent.Category = "/mutation"
	return res
}

func TestUnverifiedClaimFailsWithoutTool(t *testing.T) {
	e := newUnverifiedExec(t)
	result := unverifiedResult("--- PASS: TestFoo", 0)
	err := e.checkHollowSuccess(result)
	if err == nil {
		t.Fatal("expected hollow success when response presents test output but no test tool ran")
	}
	if !isHollowSuccessError(err) {
		t.Fatalf("expected hollowSuccessError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "test-runner output") {
		t.Fatalf("error must mention test-runner output, got: %v", err)
	}
}

func TestUnverifiedClaimPassesWithTool(t *testing.T) {
	e := newUnverifiedExec(t)
	result := unverifiedResult("--- PASS: TestFoo", 1)
	if err := e.checkHollowSuccess(result); err != nil {
		t.Fatalf("response with test output plus a test tool must pass, got: %v", err)
	}
}

func TestUnverifiedClaimProseDoesNotFail(t *testing.T) {
	e := newUnverifiedExec(t)
	result := unverifiedResult("make sure the tests pass", 0)
	if err := e.checkHollowSuccess(result); err != nil {
		t.Fatalf("prose must NOT fail, got: %v", err)
	}
	for _, prose := range []string{
		"tests should pass",
		"I did not run the tests",
		"the tests will pass",
	} {
		result2 := unverifiedResult(prose, 0)
		if err := e.checkHollowSuccess(result2); err != nil {
			t.Fatalf("prose %q must not fail, got: %v", prose, err)
		}
	}
}

func TestUnverifiedClaimRetracted(t *testing.T) {
	e := newUnverifiedExec(t)
	result := unverifiedResult("--- PASS: TestFoo", 0)
	if err := e.checkHollowSuccess(result); err == nil {
		t.Fatal("first turn must fail")
	}
	if facts, err := e.kernel.Query("claimed_test_output"); err != nil {
		t.Fatalf("query claimed_test_output: %v", err)
	} else if len(facts) != 0 {
		t.Fatalf("claimed_test_output must be retracted, got %v", facts)
	}
	if facts, err := e.kernel.Query("executed_test_tool"); err != nil {
		t.Fatalf("query executed_test_tool: %v", err)
	} else if len(facts) != 0 {
		t.Fatalf("executed_test_tool must be retracted, got %v", facts)
	}
	clean := unverifiedResult("all good, no test output here", 0)
	if err := e.checkHollowSuccess(clean); err != nil {
		t.Fatalf("second turn must be clean, got: %v", err)
	}
}

func TestUnverifiedClaimVariousOutputs(t *testing.T) {
	e := newUnverifiedExec(t)
	markers := []string{
		"--- FAIL: TestBar",
		"=== RUN   TestBaz",
		"ok  \tgithub.com/example/project/pkg 0.123s",
		"go test ./... => PASS",
		"PASS",
		"FAIL",
		"3 passed in 0.12s",
		"2 failed",
	}
	for _, marker := range markers {
		result := unverifiedResult(marker, 0)
		if err := e.checkHollowSuccess(result); err == nil {
			t.Fatalf("marker %q with no test tool must fail", marker)
		}
		result2 := unverifiedResult(marker, 1)
		if err := e.checkHollowSuccess(result2); err != nil {
			t.Fatalf("marker %q with test tool must pass, got: %v", marker, err)
		}
	}
}

func TestUnverifiedClaimMangleString(t *testing.T) {
	e := newUnverifiedExec(t)
	fact := types.Fact{Predicate: "claimed_test_output", Args: []any{types.MangleString("/create")}}
	if err := e.kernel.Assert(fact); err != nil {
		t.Fatalf("assert claimed_test_output: %v", err)
	}
	facts, err := e.kernel.Query("claimed_test_output")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("expected claimed_test_output fact")
	}
	got, ok := facts[0].Args[0].(string)
	if !ok {
		t.Fatalf("arg must be string, got %T", facts[0].Args[0])
	}
	if got != "/create" {
		t.Fatalf("arg = %q want %q", got, "/create")
	}
	_ = e.kernel.RetractFact(fact)
}

func TestIsTestToolRecognises(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		want bool
	}{
		{"run_tests", nil, true},
		{"run_impacted_tests", nil, true},
		{"go test", map[string]any{"command": "go test ./..."}, true},
		{"pytest", map[string]any{"command": "pytest"}, true},
		{"pytest full", map[string]any{"command": "python -m pytest"}, true},
		{"cargo", map[string]any{"command": "cargo test"}, true},
	}
	for _, c := range cases {
		tool := c.name
		if c.name == "go test" || c.name == "pytest" || c.name == "pytest full" || c.name == "cargo" {
			tool = "run_command"
		}
		got := isTestExecutionTool(tool, c.args)
		if got != c.want {
			t.Errorf("isTestExecutionTool(%q, %v)=%v want %v", tool, c.args, got, c.want)
		}
		if got2 := projectdoc.IsTestExecutionTool(tool, c.args); got2 != c.want {
			t.Errorf("projectdoc.IsTestExecutionTool(%q)=%v want %v", tool, got2, c.want)
		}
	}
}

func TestIsTestToolRejects(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
	}{
		{"write_file", map[string]any{"path": "a.go", "content": "package a"}},
		{"read_file", map[string]any{"path": "a.go"}},
		{"edit_file", map[string]any{"path": "a.go"}},
	}
	for _, c := range cases {
		if isTestExecutionTool(c.name, c.args) {
			t.Errorf("isTestExecutionTool(%q) must be false", c.name)
		}
	}
}

func simulateTestIncrement(result *ExecutionResult, call types.ToolCall) {
	result.ToolCallsExecuted++
	result.SuccessfulToolCalls++
	if isWriteMutationTool(call.Name) {
		result.SuccessfulWriteTools++
	}
	if isTestExecutionTool(call.Name, call.Input) {
		result.SuccessfulTestTools++
	}
}

func TestSuccessfulTestToolsCounting(t *testing.T) {
	result := &ExecutionResult{}
	simulateTestIncrement(result, types.ToolCall{Name: "run_tests", Input: map[string]any{}})
	if result.SuccessfulTestTools != 1 || result.SuccessfulWriteTools != 0 {
		t.Fatalf("after run_tests got test=%d write=%d want 1,0", result.SuccessfulTestTools, result.SuccessfulWriteTools)
	}
	simulateTestIncrement(result, types.ToolCall{Name: "run_command", Input: map[string]any{"command": "go test ./..."}})
	if result.SuccessfulTestTools != 2 {
		t.Fatalf("after go test got test=%d want 2", result.SuccessfulTestTools)
	}
	simulateTestIncrement(result, types.ToolCall{Name: "write_file", Input: map[string]any{"path": "a.go"}})
	if result.SuccessfulWriteTools != 1 || result.SuccessfulTestTools != 2 {
		t.Fatalf("after write_file got test=%d write=%d want 2,1", result.SuccessfulTestTools, result.SuccessfulWriteTools)
	}
	simulateTestIncrement(result, types.ToolCall{Name: "read_file", Input: map[string]any{"path": "a.go"}})
	if result.SuccessfulTestTools != 2 || result.SuccessfulWriteTools != 1 {
		t.Fatalf("read_file must not increment, got test=%d write=%d", result.SuccessfulTestTools, result.SuccessfulWriteTools)
	}
}
