package shell

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
)

// stubCheckExecutor is a tactile.Executor that records the commands it ran
// and answers every one with a scripted result.
type stubCheckExecutor struct {
	mu       sync.Mutex
	commands []tactile.Command
	res      *tactile.ExecutionResult
	err      error
	timeout  time.Duration
}

func (s *stubCheckExecutor) Execute(_ context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, cmd)
	return s.res, s.err
}

func (s *stubCheckExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{Name: "stub-check", DefaultTimeout: s.timeout}
}

func (s *stubCheckExecutor) Validate(tactile.Command) error { return nil }

func (s *stubCheckExecutor) recorded() []tactile.Command {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]tactile.Command(nil), s.commands...)
}

// withCheckExecutor binds exec for the test and restores the previous
// binding afterwards. These tests are sequential on purpose: the binding is
// process-wide, like the browser runtime's.
func withCheckExecutor(t *testing.T, exec tactile.Executor) {
	t.Helper()
	SetCheckExecutor(exec)
	t.Cleanup(func() { ClearCheckExecutor(exec) })
}

// checkCtx returns a context carrying a campaign check for argv in a fresh
// workspace, with an acceptance log attached.
func checkCtx(t *testing.T, argv ...string) (context.Context, func() []tools.AcceptanceRun) {
	t.Helper()
	ctx := tools.WithWorkspaceRoot(context.Background(), t.TempDir())
	ctx, runs := tools.WithAcceptanceRunLog(ctx)
	return tools.WithCampaignCheck(ctx, tools.CampaignCheck{
		CampaignID: "/campaign_test",
		TaskID:     "/task_1",
		Argv:       argv,
	}), runs
}

type checkResult struct {
	Argv      []string `json:"argv"`
	Directory string   `json:"directory"`
	ExitCode  int      `json:"exit_code"`
	Output    string   `json:"output"`
}

func decodeCheckResult(t *testing.T, raw string) checkResult {
	t.Helper()
	var res checkResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatalf("run_check result is not the argv/directory/exit_code/output envelope: %v\n%s", err, raw)
	}
	return res
}

func TestRunCheckTool_Definition(t *testing.T) {
	tool := RunCheckTool()
	if tool.Name != "run_check" {
		t.Fatalf("Name = %q, want run_check", tool.Name)
	}
	if len(tool.Schema.Properties) != 0 || len(tool.Schema.Required) != 0 {
		t.Fatalf("schema = %+v, want no properties: the campaign declared the command, not the model", tool.Schema)
	}
	if tool.Timeout == nil {
		t.Fatal("Timeout hook is nil: the session would cut the check under its own budget")
	}
	if effect, err := tool.DeclaredEffect(); err != nil || effect != tools.EffectExecute {
		t.Fatalf("DeclaredEffect = %v, %v; want execute", effect, err)
	}
}

func TestExecuteRunCheck_RefusesWithoutCheck(t *testing.T) {
	withCheckExecutor(t, &stubCheckExecutor{res: &tactile.ExecutionResult{Success: true, ExitCode: 0}})
	ctx := tools.WithWorkspaceRoot(context.Background(), t.TempDir())
	ctx, runs := tools.WithAcceptanceRunLog(ctx)
	if _, err := RunCheckTool().Execute(ctx, map[string]any{}); err == nil ||
		!strings.Contains(err.Error(), "no campaign acceptance check") {
		t.Fatalf("without a check on the context the tool must fail closed naming it, got %v", err)
	}
	if got := runs(); len(got) != 0 {
		t.Fatalf("a refused check recorded %v", got)
	}
}

func TestExecuteRunCheck_RefusesModelArgs(t *testing.T) {
	stub := &stubCheckExecutor{res: &tactile.ExecutionResult{Success: true, ExitCode: 0}}
	withCheckExecutor(t, stub)
	ctx, runs := checkCtx(t, "go", "version")
	if _, err := RunCheckTool().Execute(ctx, map[string]any{"command": "rm -rf /"}); err == nil ||
		!strings.Contains(err.Error(), "takes no arguments") {
		t.Fatalf("model-supplied keys must be refused, got %v", err)
	}
	if got := stub.recorded(); len(got) != 0 {
		t.Fatalf("a refused call ran %v", got)
	}
	if got := runs(); len(got) != 0 {
		t.Fatalf("a refused call recorded %v", got)
	}
}

func TestExecuteRunCheck_RunsArgvInTheWorkspaceRoot(t *testing.T) {
	stub := &stubCheckExecutor{res: &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "ok"}}
	withCheckExecutor(t, stub)
	ctx, runs := checkCtx(t, "checker", "--strict")
	root, err := tools.WorkspaceRoot(ctx)
	if err != nil {
		t.Fatalf("WorkspaceRoot: %v", err)
	}
	raw, err := RunCheckTool().Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	res := decodeCheckResult(t, raw)
	if len(res.Argv) != 2 || res.Argv[0] != "checker" || res.Argv[1] != "--strict" {
		t.Fatalf("argv = %v, want the declared command verbatim", res.Argv)
	}
	if res.Directory != root || res.ExitCode != 0 || res.Output != "ok" {
		t.Fatalf("result = %+v, want the workspace root, exit 0 and the whole output", res)
	}
	cmds := stub.recorded()
	if len(cmds) != 1 {
		t.Fatalf("ran %d commands, want 1", len(cmds))
	}
	got := cmds[0]
	if got.Binary != "checker" || len(got.Arguments) != 1 || got.Arguments[0] != "--strict" {
		t.Fatalf("command = %+v, want exactly argv[0] with argv[1:]", got)
	}
	if got.WorkingDirectory != root {
		t.Fatalf("working directory = %q, want the workspace root %q", got.WorkingDirectory, root)
	}
	if got.Limits != nil {
		t.Fatalf("limits = %+v, want none: the executor applies execution.default_timeout", got.Limits)
	}
	recorded := runs()
	if len(recorded) != 1 || recorded[0].ExitCode != 0 || len(recorded[0].Argv) != 2 {
		t.Fatalf("receipt = %+v, want the one run with its exit code", recorded)
	}
}

func TestExecuteRunCheck_ReturnsLongOutputWhole(t *testing.T) {
	// A check log's failures are at the tail; the tool must not cut it.
	want := strings.Repeat("failure detail line\n", 500)
	stub := &stubCheckExecutor{res: &tactile.ExecutionResult{Success: true, ExitCode: 1, Stdout: want}}
	withCheckExecutor(t, stub)
	ctx, _ := checkCtx(t, "checker")
	raw, err := RunCheckTool().Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("a check that ran and failed is a verdict, not a tool error: %v", err)
	}
	if res := decodeCheckResult(t, raw); res.ExitCode != 1 || res.Output != want {
		t.Fatalf("exit/output = %d/%d bytes, want 1/%d bytes verbatim", res.ExitCode, len(res.Output), len(want))
	}
}

func TestExecuteRunCheck_CouldNotRunIsAnErrorWithNoReceipt(t *testing.T) {
	stub := &stubCheckExecutor{res: nil, err: context.DeadlineExceeded}
	withCheckExecutor(t, stub)
	ctx, runs := checkCtx(t, "checker")
	if _, err := RunCheckTool().Execute(ctx, map[string]any{}); err == nil ||
		!strings.Contains(err.Error(), "could not be run") {
		t.Fatalf("a check that never started must error naming it, got %v", err)
	}
	if got := runs(); len(got) != 0 {
		t.Fatalf("a check that never started recorded %v", got)
	}
}

func TestExecuteRunCheck_RunsARealCommand(t *testing.T) {
	// The argv runs through the real tactile executor as a direct exec, never
	// a shell: `go version` names a binary on PATH with one argument.
	exec := tactile.NewDirectExecutor()
	withCheckExecutor(t, exec)
	ctx, runs := checkCtx(t, "go", "version")
	raw, err := RunCheckTool().Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute go version: %v", err)
	}
	res := decodeCheckResult(t, raw)
	if res.ExitCode != 0 || !strings.Contains(res.Output, "go version") {
		t.Fatalf("result = %+v, want exit 0 and the go version line", res)
	}
	if got := runs(); len(got) != 1 || got[0].ExitCode != 0 {
		t.Fatalf("receipt = %+v, want the one passing run", got)
	}
}

func TestRunCheckTimeout_ResolvesTheBoundExecutorDefault(t *testing.T) {
	exec := tactile.NewDirectExecutorWithConfig(tactile.ExecutorConfig{DefaultTimeout: 42 * time.Second})
	withCheckExecutor(t, exec)
	if got := RunCheckTool().Timeout(map[string]any{}); got != 42*time.Second {
		t.Fatalf("Timeout = %v, want the bound executor's 42s default", got)
	}
}

func TestRunCheckTimeout_FallsBackToTheConfiguredDefault(t *testing.T) {
	ClearCheckExecutor(getCheckExecutor())
	want, err := time.ParseDuration(config.DefaultExecutionConfig().DefaultTimeout)
	if err != nil {
		t.Fatalf("configured default %q does not parse: %v", config.DefaultExecutionConfig().DefaultTimeout, err)
	}
	if got := RunCheckTool().Timeout(map[string]any{}); got != want {
		t.Fatalf("Timeout = %v, want the configured execution.default_timeout %v", got, want)
	}
}
