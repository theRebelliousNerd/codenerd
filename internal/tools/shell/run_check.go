package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
)

// checkExecutor is the tactile executor the run_check tool runs the
// campaign's acceptance command through: the same executor the acceptance
// round uses (the orchestrator's, built from execution.default_timeout at
// boot), so the turn's run and the round's run cannot disagree about
// timeouts, environment or audit. It is bound once at boot; the research
// package binds its browser runtime the same way (research.SetBrowserRuntime,
// wired in internal/system/factory.go).
var (
	checkExecutorMu sync.RWMutex
	checkExecutor   tactile.Executor
)

// SetCheckExecutor binds the tactile executor run_check executes through. It
// is called once at boot with the process's audited executor.
func SetCheckExecutor(exec tactile.Executor) {
	checkExecutorMu.Lock()
	defer checkExecutorMu.Unlock()
	checkExecutor = exec
}

// ClearCheckExecutor removes exec only if it is still the process binding, so
// an older shutdown cannot detach a newer executor. Every tactile.Executor
// implementation is used as a pointer, so the comparison cannot panic.
func ClearCheckExecutor(exec tactile.Executor) {
	checkExecutorMu.Lock()
	defer checkExecutorMu.Unlock()
	if checkExecutor == exec {
		checkExecutor = nil
	}
}

func getCheckExecutor() tactile.Executor {
	checkExecutorMu.RLock()
	defer checkExecutorMu.RUnlock()
	return checkExecutor
}

// RunCheckTool returns the tool that runs a campaign task's own acceptance
// check: the exact argv the campaign will run after the turn, carried on the
// turn's context by the orchestrator. It takes no arguments because there is
// nothing for the model to decide: the campaign declared the command, and any
// model-supplied key is refused.
func RunCheckTool() *tools.Tool {
	return &tools.Tool{
		Name:          "run_check",
		AltCategories: []tools.ToolCategory{tools.CategoryCode},
		Description:   "Run this turn's campaign acceptance check: the exact command the campaign will run after the turn, in the workspace root. Takes no arguments; fails when the turn carries no check.",
		Category:      tools.CategoryTest,
		Priority:      75,
		Execute:       executeRunCheck,
		Timeout: func(_ map[string]any) time.Duration {
			return runCheckTimeout()
		},
		Schema: tools.ToolSchema{
			Required:   []string{},
			Properties: map[string]tools.Property{},
		},
	}
}

func executeRunCheck(ctx context.Context, args map[string]any) (string, error) {
	check, ok := tools.CampaignCheckFrom(ctx)
	if !ok {
		return "", fmt.Errorf("no campaign acceptance check is declared for this turn")
	}
	if len(args) != 0 {
		keys := make([]string, 0, len(args))
		for k := range args {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return "", fmt.Errorf("run_check takes no arguments; the campaign declared the command, so %v was refused", keys)
	}
	exec := getCheckExecutor()
	if exec == nil {
		return "", fmt.Errorf("no check executor is wired; the campaign's acceptance command cannot be run from this turn")
	}
	root, err := tools.WorkspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	argv := append([]string(nil), check.Argv...)
	logging.ToolsDebug("run_check: argv=%v, dir=%s", argv, root)

	// No Limits: the executor applies the user's execution.default_timeout,
	// exactly as the acceptance round does (runAcceptanceRound passes none).
	res, execErr := exec.Execute(ctx, tactile.Command{
		Binary:           argv[0],
		Arguments:        argv[1:],
		WorkingDirectory: root,
	})
	output, exitCode := "", -1
	if res != nil {
		output, exitCode = res.Output(), res.ExitCode
		tools.RecordAcceptanceRun(ctx, tools.AcceptanceRun{Argv: argv, ExitCode: exitCode})
	}

	// The output is returned whole. A check log is exactly the kind of
	// result the working context archives and pages; cutting it here would
	// hand the model the head of a log whose failures are at the tail.
	data, _ := json.Marshal(struct {
		Argv      []string `json:"argv"`
		Directory string   `json:"directory"`
		ExitCode  int      `json:"exit_code"`
		Output    string   `json:"output"`
	}{argv, root, exitCode, output})
	if ctx.Err() != nil {
		return string(data), ctx.Err()
	}
	if execErr != nil {
		// A command that could not run is reported as one: a witness nobody
		// ran must not read as a pass (runAcceptanceRound states it the same
		// way). A command that ran and failed is not an error here: its exit
		// code is the verdict the turn asked for.
		return string(data), fmt.Errorf("the acceptance check could not be run: %w", execErr)
	}
	logging.Tools("run_check completed: %v (exit %d, %d bytes output)", argv, exitCode, len(output))
	return string(data), nil
}

// runCheckTimeout resolves the budget the session must grant a run_check
// call: the bound executor's own default, which boot built from the user's
// execution.default_timeout (executionLayerConfigs), so the session never
// cuts the call before the tactile deadline it runs under fires. With no
// executor bound the tool fails closed anyway; the configured default from
// internal/config keeps the hook total without a hardcoded duration.
func runCheckTimeout() time.Duration {
	if exec := getCheckExecutor(); exec != nil {
		if d := exec.Capabilities().DefaultTimeout; d > 0 {
			return d
		}
	}
	if d, err := time.ParseDuration(config.DefaultExecutionConfig().DefaultTimeout); err == nil && d > 0 {
		return d
	}
	return 0
}
