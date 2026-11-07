//go:build !windows

package main

import (
	"bufio"
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Lift the command's actual context setup into an offline process fixture.
// Running RunE itself would boot Cortex/providers. The setup and operationContext
// are read from production source; the only instrumentation records the owner
// argument before delegating to the real ownerstop.With.
func buildRecurseSignalFixture(t *testing.T) string {
	t.Helper()
	runSource, err := os.ReadFile("cmd_campaign_recurse.go")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(runSource), "\t// The run honors --timeout")
	end := strings.Index(string(runSource), "\n\tkey := resolveAPIKey")
	if start < 0 || end <= start {
		t.Fatal("recurse pre-boot context section not found")
	}
	opSource, err := os.ReadFile("operation_context.go")
	if err != nil {
		t.Fatal(err)
	}
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "operation_context.go", opSource, 0)
	if err != nil {
		t.Fatal(err)
	}
	operation := ""
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "operationContext" {
			operation = string(opSource[fs.Position(fn.Pos()).Offset:fs.Position(fn.End()).Offset])
		}
	}
	if operation == "" || !strings.Contains(operation, "ownerstop.With") {
		t.Fatal("production operation context no longer records its owner")
	}
	operation = strings.ReplaceAll(operation, "ownerstop.With", "recordOwner")
	fixture := `package main
import (
 "context"
 "errors"
 "fmt"
 "os"
 "os/signal"
 "syscall"
 "time"
 "codenerd/internal/ownerstop"
 "github.com/spf13/cobra"
)
var timeout time.Duration
var recordedOwner, recordedWork context.Context
var parent context.Context
var stopParent context.CancelFunc
var _ = signal.NotifyContext
var _ = syscall.SIGINT
func recordOwner(ctx, owner context.Context) context.Context {
 recordedOwner = owner
 return ownerstop.With(ctx, owner)
}
` + operation + `
func scope(scenario string) error {
 cmd := &cobra.Command{}
 cmd.SetContext(parent)
` + string(runSource[start:end]) + `
 recordedWork = ctx
 if recordedOwner == nil { return fmt.Errorf("no owner recorded") }
 if scenario == "normal" || scenario == "cleanup" {
  if _, ok := ctx.Deadline(); ok { return fmt.Errorf("unset timeout gained a deadline") }
  if ctx.Err() != nil { return fmt.Errorf("ordinary run started canceled: %v", ctx.Err()) }
  return nil
 }
 select {
 case <-ctx.Done():
 case <-time.After(time.Second): return fmt.Errorf("requested child deadline did not fire")
 }
 if !errors.Is(ctx.Err(), context.DeadlineExceeded) { return fmt.Errorf("child error = %v", ctx.Err()) }
 if recordedOwner.Err() != nil { return fmt.Errorf("soft child deadline stopped its owner") }
 if scenario == "timeout_only" { return nil }
 if scenario == "parent_after_deadline" { stopParent() } else { fmt.Println("deadline-ready") }
 select {
 case <-recordedOwner.Done(): return nil
 case <-time.After(2*time.Second): return fmt.Errorf("explicit stop missed after child deadline")
 }
}
func main() {
 parent, stopParent = context.WithCancel(context.Background())
 defer stopParent()
 scenario := os.Args[1]
 if scenario != "normal" && scenario != "cleanup" { timeout = time.Nanosecond }
 if err := scope(scenario); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
 if scenario == "cleanup" {
  if !errors.Is(recordedOwner.Err(), context.Canceled) { fmt.Fprintln(os.Stderr,"normal return did not release signal owner"); os.Exit(1) }
  if !errors.Is(recordedWork.Err(), context.Canceled) || parent.Err() != nil { fmt.Fprintln(os.Stderr,"cleanup lost child or canceled caller"); os.Exit(1) }
 }
 fmt.Println("ok")
}
`
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "fixture.go"), filepath.Join(dir, "recurse-signal-fixture")
	if err := os.WriteFile(source, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	// A named child module keeps the real internal/ownerstop import legal while
	// leaving generated source outside the checkout and preserving readonly deps.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	module := strings.Replace(string(mod), "module codenerd", "module codenerd/recurseownerfixture", 1)
	module += "\nrequire codenerd v0.0.0\nreplace codenerd => " + strconv.Quote(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(module), 0o600); err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	// Give the standalone fixture its own real VCS identity. Stamping stays
	// enabled, and no checkout or user Git configuration is changed.
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "fixture.go", "go.mod", "go.sum"},
		{"-c", "user.name=CodeNERD Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-m", "offline signal fixture"},
	} {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("initialize owned fixture repository: %v\n%s", err, out)
		}
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-race", "-o", binary, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build offline production-prefix fixture: %v\n%s", err, out)
	}
	return binary
}

func TestRecurseSignalOwner_ProductionContextLifetime(t *testing.T) {
	t.Parallel()
	binary := buildRecurseSignalFixture(t)
	for _, scenario := range []struct {
		name   string
		signal os.Signal
	}{
		{"sigint_after_deadline", syscall.SIGINT},
		{"sigterm_after_deadline", syscall.SIGTERM},
		{"parent_after_deadline", nil},
		{"normal", nil},
		{"timeout_only", nil},
		{"cleanup", nil},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			// The race-instrumented fixture (cobra and the owner-stop leaf) can be cold under
			// concurrent builds. The fixture's post-ready stop assertion stays 2s.
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, scenario.name)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			if scenario.signal != nil {
				scanner := bufio.NewScanner(stdout)
				ready := false
				for scanner.Scan() {
					if scanner.Text() == "deadline-ready" {
						ready = true
						break
					}
				}
				if !ready {
					_ = cmd.Wait()
					t.Fatalf("fixture did not reach deadline barrier: %v / %s", scanner.Err(), stderr.String())
				}
				if err := cmd.Process.Signal(scenario.signal); err != nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
					t.Fatal(err)
				}
			}
			_, _ = io.Copy(io.Discard, stdout)
			if err := cmd.Wait(); err != nil {
				t.Errorf("production lifetime %s: %v / %s", scenario.name, err, stderr.String())
			}
		})
	}
}
