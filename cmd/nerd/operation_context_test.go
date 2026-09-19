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
