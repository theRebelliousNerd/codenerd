package session

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A canceled verification command takes its whole process tree with it.
// The runners used to call exec.Cmd.CombinedOutput directly: a cancel killed
// only the direct child, and a grandchild (the test binary `go test` starts,
// here a backgrounded sleep) kept the output pipe open, so the runner blocked
// until it exited and then left it running (audit
// verification-process-cleanup-parity).
func TestVerificationRunners_CancelKillsTheProcessTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture is a POSIX shell script")
	}
	cases := []struct {
		name string
		run  func(ctx context.Context, dir, script string) ([]byte, error)
	}{
		{"build", func(ctx context.Context, dir, script string) ([]byte, error) {
			return runVerificationProcess(ctx, dir, os.Environ(), "sh", []string{script})
		}},
		{"test", func(ctx context.Context, dir, script string) ([]byte, error) {
			return runVerificationProcess(ctx, dir, os.Environ(), "sh", []string{script})
		}},
		{"gopls", func(ctx context.Context, dir, script string) ([]byte, error) {
			// goplsCheckProcess runs `<bin> check main.go`. With bin = sh and the
			// fixture saved as dir/check, sh reads the script instead of the
			// kernel executing it: executing a file this process just wrote
			// fails with ETXTBSY whenever a parallel subtest forks while the
			// write is open (the child inherits the descriptor), and the
			// fixture then never starts.
			if err := os.Link(script, filepath.Join(dir, "check")); err != nil {
				return nil, err
			}
			return goplsCheckProcess(ctx, "sh", dir, []string{"main.go"})
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			pidFile := filepath.Join(dir, "grandchild.pid")
			script := filepath.Join(dir, "tool.sh")
			body := "#!/bin/sh\nsleep 60 &\necho $! > " + strconv.Quote(pidFile) + "\nwait\n"
			if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			returned := make(chan struct{})
			go func() {
				defer close(returned)
				_, _ = tc.run(ctx, dir, script)
			}()
			pid := waitForPID(t, pidFile)
			cancel()

			select {
			case <-returned:
			case <-time.After(15 * time.Second):
				t.Fatal("the runner stayed blocked after cancel: a grandchild kept its output pipe open")
			}
			deadline := time.Now().Add(5 * time.Second)
			for processLive(pid) {
				if time.Now().After(deadline) {
					_ = syscall.Kill(pid, syscall.SIGKILL)
					t.Fatalf("grandchild %d outlived the canceled command", pid)
				}
				time.Sleep(20 * time.Millisecond)
			}
		})
	}
}

func waitForPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the fixture never started its grandchild")
	return 0
}

// processLive reports whether pid is a running (not zombie) process.
func processLive(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return runtime.GOOS != "linux" // no /proc: kill(0) succeeding is all we know
	}
	// The state follows the parenthesised command name.
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 && i+2 < len(s) {
		return s[i+2] != 'Z'
	}
	return true
}

// Every subprocess the session package starts goes through processutil's
// wrappers. The rule is processutil's (internal/processutil/agents.md): a
// raw Run, Start, Output or CombinedOutput on a CommandContext command skips
// the kill scope and the bounded pipe wait.
func TestSessionSubprocesses_UseProcessutilWrappers(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	raw := map[string]bool{"Run": true, "Start": true, "Output": true, "CombinedOutput": true}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Names bound to exec.Command / exec.CommandContext in this file.
		cmds := map[string]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, rhs := range as.Rhs {
				if isExecCommand(rhs) && i < len(as.Lhs) {
					if id, ok := as.Lhs[i].(*ast.Ident); ok {
						cmds[id.Name] = true
					}
				}
			}
			return true
		})
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !raw[sel.Sel.Name] {
				return true
			}
			switch x := sel.X.(type) {
			case *ast.Ident:
				if cmds[x.Name] {
					t.Errorf("%s: raw %s.%s(); use processutil.%s", fset.Position(call.Pos()), x.Name, sel.Sel.Name, wrapperFor(sel.Sel.Name))
				}
			case *ast.CallExpr:
				// processutil.NonInteractive(cmd).CombinedOutput() is still the raw method.
				if isExecCommand(x) || isSelector(x.Fun, "processutil", "NonInteractive") {
					t.Errorf("%s: raw .%s() on a command; use processutil.%s", fset.Position(call.Pos()), sel.Sel.Name, wrapperFor(sel.Sel.Name))
				}
			}
			return true
		})
	}
}

func isExecCommand(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	return ok && (isSelector(call.Fun, "exec", "Command") || isSelector(call.Fun, "exec", "CommandContext"))
}

func isSelector(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

func wrapperFor(method string) string {
	if method == "Output" {
		return "CombinedOutput (or Run with a buffer)"
	}
	return method
}
