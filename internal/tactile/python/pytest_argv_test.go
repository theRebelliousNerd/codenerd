package python

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"codenerd/internal/processutil"
	"codenerd/internal/tactile"
)

type pytestRecordingRuntime struct {
	calls        []tactile.ContainerExecOptions
	contexts     []context.Context
	result       *tactile.ExecutionResult
	executionErr error
	onExecute    func()
	execute      func(context.Context, tactile.ContainerExecOptions) (*tactile.ExecutionResult, error)
}

type pytestCancellationOutput struct {
	buffer bytes.Buffer
	cancel context.CancelFunc
	ready  bool
}

func (output *pytestCancellationOutput) Write(chunk []byte) (int, error) {
	written, err := output.buffer.Write(chunk)
	if !output.ready && bytes.Contains(output.buffer.Bytes(), []byte("\n")) {
		output.ready = true
		output.cancel()
	}
	return written, err
}

var _ tactile.ContainerRuntime = (*pytestRecordingRuntime)(nil)

func (recorder *pytestRecordingRuntime) ExecInContainer(ctx context.Context, options tactile.ContainerExecOptions) (*tactile.ExecutionResult, error) {
	options.Arguments = append([]string(nil), options.Arguments...)
	recorder.calls = append(recorder.calls, options)
	recorder.contexts = append(recorder.contexts, ctx)
	if recorder.onExecute != nil {
		recorder.onExecute()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if recorder.execute != nil {
		return recorder.execute(ctx, options)
	}
	return recorder.result, recorder.executionErr
}

func (*pytestRecordingRuntime) IsAvailable() bool {
	return true
}

func (*pytestRecordingRuntime) CreateContainer(context.Context, tactile.ContainerCreateOptions) (*tactile.PersistentContainer, error) {
	return nil, fmt.Errorf("unexpected CreateContainer during pytest execution")
}

func (*pytestRecordingRuntime) StartContainer(context.Context, string) error {
	return fmt.Errorf("unexpected StartContainer during pytest execution")
}

func (*pytestRecordingRuntime) StopContainer(context.Context, string, time.Duration) error {
	return fmt.Errorf("unexpected StopContainer during pytest execution")
}

func (*pytestRecordingRuntime) RemoveContainer(context.Context, string, bool) error {
	return fmt.Errorf("unexpected RemoveContainer during pytest execution")
}

func (*pytestRecordingRuntime) CreateSnapshot(context.Context, string, string) (*tactile.ContainerSnapshot, error) {
	return nil, fmt.Errorf("unexpected CreateSnapshot during pytest execution")
}

func (*pytestRecordingRuntime) RestoreSnapshot(context.Context, string) (*tactile.PersistentContainer, error) {
	return nil, fmt.Errorf("unexpected RestoreSnapshot during pytest execution")
}

func newPytestArgvEnvironment(recorder *pytestRecordingRuntime) *Environment {
	config := DefaultConfig()
	config.WorkspaceDir = "/workspace with spaces"
	config.TestTimeout = 37 * time.Second
	environment := NewEnvironment(&ProjectInfo{Name: "selected project"}, config, recorder)
	environment.container = &tactile.PersistentContainer{ID: "pytest-container"}
	return environment
}

func TestRunPytest_LiteralArgv(test *testing.T) {
	cases := []struct {
		name      string
		arguments []string
	}{
		{name: "default flags"},
		{name: "selected valid test", arguments: []string{"tests/test_math.py::test_add"}},
		{name: "multiple selected tests", arguments: []string{"tests/test_math.py::test_add", "tests/test_math.py::test_subtract"}},
		{name: "shell metacharacters", arguments: []string{"tests/test_math.py::test_add; touch /tmp/pytest-sentinel", "ignored && echo injected | cat > /tmp/pytest-sentinel"}},
		{name: "command substitution", arguments: []string{"$(touch /tmp/pytest-sentinel)", "\x60touch /tmp/pytest-sentinel\x60"}},
		{name: "spaces and quotes", arguments: []string{"tests/test spaced.py::test_value[\"two words\"]", "tests/test_math.py::test_value['quoted']"}},
		{name: "filter arguments", arguments: []string{"-k", "test_add or test_subtract"}},
	}
	for _, testcase := range cases {
		test.Run(testcase.name, func(test *testing.T) {
			output := "collected 1 item\n\n1 passed in 0.01s\n"
			recorder := &pytestRecordingRuntime{result: &tactile.ExecutionResult{Success: true, ExitCode: 0, Combined: output}}
			environment := newPytestArgvEnvironment(recorder)
			callerContext, cancel := context.WithCancel(context.Background())
			defer cancel()
			originalArguments := append([]string(nil), testcase.arguments...)

			result, err := environment.RunPytest(callerContext, testcase.arguments...)
			if err != nil {
				test.Fatalf("RunPytest returned an unexpected error: %v", err)
			}
			if len(recorder.calls) != 1 {
				test.Fatalf("runtime calls = %d, want one pytest execution", len(recorder.calls))
			}
			invocation := recorder.calls[0]
			if invocation.Binary != "/workspace with spaces/venv/bin/python" {
				test.Errorf("binary = %q, want the virtualenv interpreter without a shell", invocation.Binary)
			}
			expectedArguments := append([]string{"-c", pytestLauncher}, testcase.arguments...)
			if !reflect.DeepEqual(invocation.Arguments, expectedArguments) {
				test.Errorf("argv = %#v, want literal arguments %#v", invocation.Arguments, expectedArguments)
			}
			if invocation.ContainerID != "pytest-container" || invocation.WorkingDir != "/workspace with spaces/selected project" {
				test.Errorf("execution target changed: %+v", invocation)
			}
			if invocation.Timeout != 37*time.Second || recorder.contexts[0] != callerContext {
				test.Errorf("timeout or caller context changed: timeout=%v context=%v", invocation.Timeout, recorder.contexts[0])
			}
			if len(invocation.Environment) != 0 {
				test.Errorf("pytest must inherit the container environment, got overrides: %#v", invocation.Environment)
			}
			if !reflect.DeepEqual(testcase.arguments, originalArguments) {
				test.Errorf("caller arguments mutated: %#v, originally %#v", testcase.arguments, originalArguments)
			}
			if result == nil || !result.Passed || result.ExitCode != 0 || result.Output != output || result.ErrorMessage != "" {
				test.Fatalf("successful test result was not preserved: %+v", result)
			}
			if result.TestName != strings.Join(testcase.arguments, " ") || environment.State() != StateTesting {
				test.Errorf("test name or state changed: %+v state=%s", result, environment.State())
			}
		})
	}
}

func TestRunPytest_NonzeroExitPreservesFailureAndOutput(test *testing.T) {
	cases := []struct {
		name         string
		exitCode     int
		output       string
		errorMessage string
	}{
		{name: "assertion failure", exitCode: 1, output: "collected 1 item\nFAILED tests/test_math.py::test_add - assert 1 == 2\n", errorMessage: "FAILED tests/test_math.py::test_add - assert 1 == 2"},
		{name: "collection failure", exitCode: 2, output: "Error: cannot import tests\n", errorMessage: "Error: cannot import tests"},
		{name: "no tests collected", exitCode: 5, output: "no tests ran in 0.01s\n", errorMessage: "no tests ran in 0.01s"},
	}
	for _, testcase := range cases {
		test.Run(testcase.name, func(test *testing.T) {
			recorder := &pytestRecordingRuntime{result: &tactile.ExecutionResult{Success: true, ExitCode: testcase.exitCode, Combined: testcase.output}}
			result, err := newPytestArgvEnvironment(recorder).RunPytest(context.Background(), "tests/test_math.py::test_add")
			if err != nil {
				test.Fatalf("a test verdict should remain in TestResult, got error: %v", err)
			}
			if result == nil || result.Passed || result.ExitCode != testcase.exitCode || result.Output != testcase.output || result.ErrorMessage != testcase.errorMessage {
				test.Fatalf("failed test result was not preserved: %+v", result)
			}
		})
	}
}

func TestRunPytest_CancellationReachesRuntime(test *testing.T) {
	for _, cancelBeforeExecution := range []bool{true, false} {
		test.Run(fmt.Sprintf("cancel_before_execution=%t", cancelBeforeExecution), func(test *testing.T) {
			callerContext, cancel := context.WithCancel(context.Background())
			defer cancel()
			recorder := &pytestRecordingRuntime{result: &tactile.ExecutionResult{Success: true, ExitCode: 0}}
			if cancelBeforeExecution {
				cancel()
			} else {
				recorder.onExecute = cancel
			}
			result, err := newPytestArgvEnvironment(recorder).RunPytest(callerContext, "tests/test_math.py::test_add")
			if err != nil {
				test.Fatalf("cancellation should preserve the failed TestResult contract, got error: %v", err)
			}
			if len(recorder.contexts) != 1 || recorder.contexts[0] != callerContext || !errors.Is(recorder.contexts[0].Err(), context.Canceled) {
				test.Fatalf("caller cancellation did not reach the runtime: %#v", recorder.contexts)
			}
			if result == nil || result.Passed || result.ExitCode != -1 || result.ErrorMessage != context.Canceled.Error() {
				test.Fatalf("canceled execution became a passing result: %+v", result)
			}
		})
	}
}

func TestRunPytest_RuntimeErrorIsFailedResult(test *testing.T) {
	runtimeError := errors.New("container runtime unavailable")
	recorder := &pytestRecordingRuntime{executionErr: runtimeError}
	result, err := newPytestArgvEnvironment(recorder).RunPytest(context.Background(), "tests/test_math.py::test_add")
	if err != nil {
		test.Fatalf("runtime failure should preserve the failed TestResult contract, got error: %v", err)
	}
	if len(recorder.calls) != 1 || result == nil || result.Passed || result.ExitCode != -1 || result.ErrorMessage != runtimeError.Error() {
		test.Fatalf("runtime error was not preserved: calls=%d result=%+v", len(recorder.calls), result)
	}
}

func TestRunPytest_UninitializedContainerDoesNotExecute(test *testing.T) {
	recorder := &pytestRecordingRuntime{result: &tactile.ExecutionResult{Success: true, ExitCode: 0}}
	environment := NewEnvironment(&ProjectInfo{Name: "project"}, DefaultConfig(), recorder)
	result, err := environment.RunPytest(context.Background(), "tests/test_math.py::test_add")
	if err != nil {
		test.Fatalf("missing container should preserve the failed TestResult contract, got error: %v", err)
	}
	if len(recorder.calls) != 0 {
		test.Fatalf("pytest executed without an initialized container: %+v", recorder.calls)
	}
	if result == nil || result.Passed || result.ExitCode != -1 || result.ErrorMessage != "container not initialized" {
		test.Fatalf("missing container was not reported as a failed result: %+v", result)
	}
}

func TestRunPytest_LauncherPreservesSubprocessEnvironment(test *testing.T) {
	pythonBinary := os.Getenv("NERD_TEST_PYTHON_BIN")
	if pythonBinary == "" {
		candidates := []string{"python3", "python"}
		if runtime.GOOS == "windows" {
			candidates = []string{"python", "python3"}
		}
		for _, candidate := range candidates {
			if resolved, err := exec.LookPath(candidate); err == nil {
				pythonBinary = resolved
				break
			}
		}
	}
	if pythonBinary == "" {
		test.Fatal("launcher subprocess gate requires Python 3 with stdlib venv; set NERD_TEST_PYTHON_BIN")
	}
	fixtureRoot := test.TempDir()
	venvDirectory := filepath.Join(fixtureRoot, "virtualenv with spaces")
	setupContext, cancelSetup := context.WithTimeout(context.Background(), time.Minute)
	defer cancelSetup()
	setupCommand := exec.CommandContext(setupContext, pythonBinary, "-m", "venv", "--without-pip", venvDirectory)
	if output, err := processutil.CombinedOutput(setupCommand); err != nil {
		test.Fatalf("create real virtualenv: %v\n%s", err, output)
	}
	venvBin := filepath.Join(venvDirectory, "bin")
	venvPython := filepath.Join(venvBin, "python")
	helperName := "pytest-inherited-helper"
	if runtime.GOOS == "windows" {
		venvBin = filepath.Join(venvDirectory, "Scripts")
		venvPython = filepath.Join(venvBin, "python.exe")
		helperName += ".exe"
	}
	inheritedBin := filepath.Join(fixtureRoot, "inherited bin with spaces")
	if err := os.MkdirAll(inheritedBin, 0o700); err != nil {
		test.Fatal(err)
	}
	currentExecutable, err := os.Executable()
	if err != nil {
		test.Fatal(err)
	}
	helperExecutable, err := os.ReadFile(currentExecutable)
	if err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inheritedBin, helperName), helperExecutable, 0o700); err != nil {
		test.Fatal(err)
	}
	moduleSource := `import json
import os
import shutil
import subprocess
import sys
python_lookup = shutil.which("python", path=os.environ["PATH"])
python_command = "python"
if os.name == "nt":
    if python_lookup is None or os.path.normcase(os.path.abspath(python_lookup)) != os.path.normcase(os.path.abspath(sys.executable)):
        raise RuntimeError("inherited PATH lookup did not select the virtualenv interpreter")
    python_command = python_lookup
receipt = {
    "module_name": __name__,
    "arguments": sys.argv[1:],
    "path": os.environ["PATH"],
    "marker": os.environ.get("NERD_PYTEST_INHERITED_MARKER"),
    "python_lookup": python_lookup,
    "python_command": python_command,
    "python_helper": subprocess.check_output([python_command, "-c", "import sys; print(sys.executable)"], text=True).strip(),
    "inherited_helper": subprocess.check_output([os.environ["NERD_PYTEST_HELPER_NAME"], "-test.run=^TestPytestInheritedPathHelperProcess$"], text=True).strip(),
}
print(json.dumps(receipt), flush=True)
if "tests/test_math.py::test_cancel" in sys.argv[1:]:
    import threading
    threading.Event().wait()
sys.exit(1 if "tests/test_math.py::test_failure" in sys.argv[1:] else 0)
`
	if err := os.WriteFile(filepath.Join(fixtureRoot, "pytest.py"), []byte(moduleSource), 0o600); err != nil {
		test.Fatal(err)
	}
	inheritedPath := inheritedBin + string(os.PathListSeparator) + os.Getenv("PATH")
	inheritedEnvironment := make([]string, 0, len(os.Environ())+5)
	for _, entry := range os.Environ() {
		environmentKey, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(environmentKey, "PATH") || strings.EqualFold(environmentKey, "PYTHONPATH") || strings.EqualFold(environmentKey, "PYTHONHOME") {
			continue
		}
		inheritedEnvironment = append(inheritedEnvironment, entry)
	}
	inheritedEnvironment = append(inheritedEnvironment,
		"PATH="+inheritedPath,
		"PYTHONPATH="+fixtureRoot,
		"NERD_PYTEST_INHERITED_MARKER=container-environment-marker",
		"NERD_PYTEST_HELPER_PROCESS=1",
		"NERD_PYTEST_HELPER_NAME="+helperName,
	)
	sentinelPath := filepath.Join(fixtureRoot, "shell-sentinel")
	cases := []struct {
		name      string
		arguments []string
		exitCode  int
		cancel    bool
	}{
		{name: "default flags"},
		{name: "valid selector", arguments: []string{"tests/test_math.py::test_add"}},
		{name: "literal selectors", arguments: []string{
			"tests/test spaced.py::test_value[\"two words\"]",
			"tests/test_math.py::test_value['quoted']",
			"tests/test_math.py::test_add; echo injected > \"" + sentinelPath + "\"",
			"$(echo injected > \"" + sentinelPath + "\")",
			"\x60echo injected > \"" + sentinelPath + "\"\x60",
			"ignored && echo injected | cat > \"" + sentinelPath + "\"",
		}},
		{name: "selected failure", arguments: []string{"tests/test_math.py::test_failure"}, exitCode: 1},
		{name: "cancel running pytest", arguments: []string{"tests/test_math.py::test_cancel"}, exitCode: -1, cancel: true},
	}
	for _, testcase := range cases {
		test.Run(testcase.name, func(test *testing.T) {
			callerContext, cancelCaller := context.WithCancel(context.Background())
			defer cancelCaller()
			var subprocessOutput []byte
			recorder := &pytestRecordingRuntime{}
			recorder.execute = func(ctx context.Context, options tactile.ContainerExecOptions) (*tactile.ExecutionResult, error) {
				executionContext, cancelExecution := context.WithTimeout(ctx, options.Timeout)
				defer cancelExecution()
				command := exec.CommandContext(executionContext, venvPython, options.Arguments...)
				command.Dir = fixtureRoot
				command.Env = inheritedEnvironment
				var output []byte
				var executionError error
				if testcase.cancel {
					cancellationOutput := &pytestCancellationOutput{cancel: cancelCaller}
					var stderr bytes.Buffer
					command.Stdout = cancellationOutput
					command.Stderr = &stderr
					executionError = processutil.Run(command)
					output = append([]byte(nil), cancellationOutput.buffer.Bytes()...)
					if stderr.Len() != 0 {
						test.Errorf("canceled launcher stderr: %s", stderr.String())
					}
				} else {
					output, executionError = processutil.CombinedOutput(command)
				}
				subprocessOutput = output
				if executionContext.Err() != nil {
					return nil, executionContext.Err()
				}
				exitCode := 0
				if executionError != nil {
					var exitError *exec.ExitError
					if !errors.As(executionError, &exitError) {
						return nil, executionError
					}
					exitCode = exitError.ExitCode()
				}
				return &tactile.ExecutionResult{Success: true, ExitCode: exitCode, Combined: string(output)}, nil
			}
			result, err := newPytestArgvEnvironment(recorder).RunPytest(callerContext, testcase.arguments...)
			if err != nil || result == nil || result.ExitCode != testcase.exitCode || result.Passed != (testcase.exitCode == 0) {
				test.Fatalf("actual launcher result = %+v, error = %v, want exit %d", result, err, testcase.exitCode)
			}
			var receipt struct {
				Arguments       []string `json:"arguments"`
				ModuleName      string   `json:"module_name"`
				Path            string   `json:"path"`
				Marker          string   `json:"marker"`
				PythonLookup    string   `json:"python_lookup"`
				PythonCommand   string   `json:"python_command"`
				PythonHelper    string   `json:"python_helper"`
				InheritedHelper string   `json:"inherited_helper"`
			}
			if err := json.Unmarshal(subprocessOutput, &receipt); err != nil {
				test.Fatalf("actual pytest module receipt: %v\n%s", err, subprocessOutput)
			}
			if receipt.ModuleName != "__main__" {
				test.Errorf("pytest module name = %q, want __main__", receipt.ModuleName)
			}
			if testcase.cancel {
				if !errors.Is(callerContext.Err(), context.Canceled) || result.ErrorMessage != context.Canceled.Error() {
					test.Errorf("running subprocess cancellation was lost: context=%v result=%+v", callerContext.Err(), result)
				}
			} else if result.Output != string(subprocessOutput) {
				test.Errorf("launcher output changed: result=%q subprocess=%q", result.Output, subprocessOutput)
			}
			expectedArguments := append([]string{"-xvs"}, testcase.arguments...)
			if !reflect.DeepEqual(receipt.Arguments, expectedArguments) {
				test.Errorf("actual pytest argv = %#v, want %#v", receipt.Arguments, expectedArguments)
			}
			if receipt.Path != venvBin+string(os.PathListSeparator)+inheritedPath {
				test.Errorf("actual PATH = %q, want venv prefix plus exact inherited PATH %q", receipt.Path, inheritedPath)
			}
			sameExecutablePath := func(actualPath string) bool {
				if runtime.GOOS == "windows" {
					return strings.EqualFold(filepath.Clean(actualPath), filepath.Clean(venvPython))
				}
				return filepath.Clean(actualPath) == filepath.Clean(venvPython)
			}
			if !sameExecutablePath(receipt.PythonLookup) {
				test.Errorf("inherited PATH lookup = %q, want virtualenv interpreter %q", receipt.PythonLookup, venvPython)
			}
			expectedPythonCommand := "python"
			if runtime.GOOS == "windows" {
				expectedPythonCommand = receipt.PythonLookup
			}
			if receipt.PythonCommand != expectedPythonCommand {
				test.Errorf("subprocess executable argument = %q, want %q", receipt.PythonCommand, expectedPythonCommand)
			}
			if !sameExecutablePath(receipt.PythonHelper) {
				test.Errorf("subprocess python resolved to %q, want virtualenv interpreter %q", receipt.PythonHelper, venvPython)
			}
			if receipt.Marker != "container-environment-marker" || receipt.InheritedHelper != "inherited-helper-ok" {
				test.Errorf("inherited variables or inherited PATH helper lost: %+v", receipt)
			}
			if _, err := os.Stat(sentinelPath); !errors.Is(err, os.ErrNotExist) {
				test.Errorf("literal caller arguments created a sentinel or inspection failed: %v", err)
			}
		})
	}
}

func TestPytestInheritedPathHelperProcess(test *testing.T) {
	if os.Getenv("NERD_PYTEST_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Print("inherited-helper-ok")
	os.Exit(0)
}
