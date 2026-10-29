package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"

	"github.com/spf13/cobra"
)

// A command runs with no deadline unless the user sets one. Until 2026-09-19
// every command ran under a 25-minute default, and --timeout 0 was a deadline
// that had already passed.
func TestOperationContext_NoDeadlineUnlessTheUserSetsOne(t *testing.T) {
	saved := timeout
	t.Cleanup(func() { timeout = saved })

	for _, unset := range []time.Duration{0, -time.Second} {
		timeout = unset
		ctx, cancel := operationContext(context.Background())
		if deadline, ok := ctx.Deadline(); ok {
			t.Errorf("--timeout %s: deadline %v, want none", unset, deadline)
		}
		if ctx.Err() != nil {
			t.Errorf("--timeout %s: context already done (%v)", unset, ctx.Err())
		}
		cancel()
	}

	timeout = 2 * time.Hour
	ctx, cancel := operationContext(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("--timeout 2h: no deadline, want the user's limit")
	}
	if left := time.Until(deadline); left < 119*time.Minute || left > 2*time.Hour {
		t.Errorf("--timeout 2h: %s left, want about two hours", left)
	}
}

func TestTimeoutFlagHasNoDefault(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("timeout")
	if flag == nil {
		t.Fatal("no --timeout flag")
	}
	if flag.DefValue != "0s" {
		t.Errorf("--timeout default = %s, want 0s (none): an agentic run can take hours", flag.DefValue)
	}
}

// Every command applies --timeout through operationContext, so the rule
// "unset means none" lives in one place. A command that hands the flag to
// context.WithTimeout itself is how --timeout 0 became an expired deadline
// and how two commands grew their own 25-minute fallback.
func TestNoCommandWrapsItsRunInTheTimeoutFlagDirectly(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var offenders []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || path == "operation_context.go" {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "WithTimeout" && sel.Sel.Name != "WithDeadline") {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "context" {
				return true
			}
			for _, arg := range call.Args {
				if id, ok := arg.(*ast.Ident); ok && id.Name == "timeout" {
					offenders = append(offenders, fset.Position(call.Pos()).String())
				}
			}
			return true
		})
	}
	if len(offenders) > 0 {
		t.Errorf("--timeout applied outside operationContext:\n  %s", strings.Join(offenders, "\n  "))
	}
}

// A subcommand's run context has no deadline unless the user set --timeout,
// and it still ends when the cobra command's context is cancelled.
func TestCommandContext_NoDeadlineUnlessTheUserSetsOne(t *testing.T) {
	saved := timeout
	t.Cleanup(func() { timeout = saved })

	parent, stop := context.WithCancel(context.Background())
	cmd := &cobra.Command{}
	cmd.SetContext(parent)

	timeout = 0
	ctx, cancel := commandContext(cmd)
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("no --timeout: command has a deadline")
	}
	stop()
	if ctx.Err() == nil {
		t.Fatal("no --timeout: cancelling the command context did not cancel the run")
	}
	cancel()

	timeout = -time.Second
	ctx, cancel = commandContext(nil)
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("--timeout < 0: command has a deadline")
	}
	if ctx.Err() != nil {
		t.Fatalf("--timeout < 0: context already done (%v)", ctx.Err())
	}
	cancel()

	timeout = 2 * time.Hour
	ctx, cancel = commandContext(&cobra.Command{})
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("--timeout 2h: command has no deadline")
	}
	if left := time.Until(deadline); left < 119*time.Minute || left > 2*time.Hour {
		t.Errorf("--timeout 2h: %s left, want about two hours", left)
	}
}

// One LLM call takes llm_timeouts.per_call_timeout. The command's --timeout,
// when set, is shorter and wins; a zero per-call value adds no second clock.
func TestLLMCallContext_RequestBoundFromConfig(t *testing.T) {
	prev := config.GetLLMTimeouts()
	t.Cleanup(func() { config.SetLLMTimeouts(prev) })
	saved := timeout
	t.Cleanup(func() { timeout = saved })

	config.SetLLMTimeouts(config.LLMTimeouts{PerCallTimeout: 0})
	ctx, cancel := llmCallContext(context.Background())
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("per_call_timeout 0 added a deadline")
	}
	cancel()

	config.SetLLMTimeouts(config.LLMTimeouts{PerCallTimeout: 2 * time.Hour})
	ctx, cancel = llmCallContext(context.Background())
	deadline, ok := ctx.Deadline()
	cancel()
	if !ok {
		t.Fatal("per_call_timeout 2h: call has no deadline")
	}
	if left := time.Until(deadline); left < 119*time.Minute || left > 2*time.Hour {
		t.Errorf("per_call_timeout 2h: %s left, want about two hours", left)
	}

	timeout = 30 * time.Second
	cmdCtx, cmdCancel := commandContext(&cobra.Command{})
	defer cmdCancel()
	callCtx, callCancel := llmCallContext(cmdCtx)
	defer callCancel()
	callDeadline, ok := callCtx.Deadline()
	if !ok {
		t.Fatal("call context lost the command deadline")
	}
	if left := time.Until(callDeadline); left > 45*time.Second {
		t.Errorf("call deadline has %s left, want the 30s --timeout rather than the 2h per-call bound", left)
	}
}

// These commands used to start their own wall clock. They now call
// commandContext, and a duration literal inside context.WithTimeout is a
// run clock that came back.
func TestScopedCommandsHaveNoHardcodedRunClock(t *testing.T) {
	wantCalls := map[string]int{
		"dom_replace_cmd.go":    1,
		"dom_apply_cmd.go":      1,
		"dom_cmd.go":            4,
		"cmd_spawn.go":          2,
		"cmd_test_context.go":   1,
		"cmd_direct_actions.go": 1,
		"cmd_advanced.go":       5,
		"cmd_mcp_select.go":     2,
		"cmd_auth.go":           4,
		"cmd_knowledge.go":      2,
		"cmd_transparency.go":   2,
		"cmd_systems.go":        7,
	}
	fset := token.NewFileSet()
	var offenders []string
	for path, want := range wantCalls {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(src), "commandContext("); got != want {
			t.Errorf("%s: commandContext calls = %d, want %d", path, got, want)
		}
		if path == "cmd_spawn.go" && strings.Contains(string(src), "time.Minute") {
			t.Errorf("%s still contains a time.Minute run clock", path)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "WithTimeout" && sel.Sel.Name != "WithDeadline") {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "context" {
				return true
			}
			if durationLiteral(call) {
				offenders = append(offenders, fset.Position(call.Pos()).String())
			}
			return true
		})
	}
	if len(offenders) > 0 {
		t.Errorf("hardcoded duration passed to context.WithTimeout:\n  %s", strings.Join(offenders, "\n  "))
	}
}

// durationLiteral reports a numeric literal multiplied by time.Second,
// time.Minute, or time.Hour anywhere in the call. time.Duration(cfg)*time.Second
// is a config value, not a literal.
func durationLiteral(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(node ast.Node) bool {
		bin, ok := node.(*ast.BinaryExpr)
		if !ok || bin.Op != token.MUL {
			return true
		}
		if (isTimeUnit(bin.X) && isNumber(bin.Y)) || (isTimeUnit(bin.Y) && isNumber(bin.X)) {
			found = true
		}
		return true
	})
	return found
}

func isTimeUnit(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "time" {
		return false
	}
	switch sel.Sel.Name {
	case "Second", "Minute", "Hour":
		return true
	default:
		return false
	}
}

func isNumber(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.INT
}
