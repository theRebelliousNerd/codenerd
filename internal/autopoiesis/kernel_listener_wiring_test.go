package autopoiesis

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// TODO P1: "Confirm StartKernelListener started on all interactive boot paths;
// document poll interval."
//
// The audit: two interactive boot paths exist (cmd/nerd/chat/session_boot.go
// and session_shared_boot.go) and both start the listener at 2s. That fact is
// only worth recording if something notices when it stops being true, because
// the failure is invisible — the session boots fine, the kernel keeps
// asserting delegate_task(/tool_generator, …, /pending), and nothing ever
// turns those facts into a tool. So the audit is encoded here rather than
// written down: a file that hands an Orchestrator to a chat session must also
// start the listener, at the documented cadence.

// listenerlessAutopoiesisConsumers maps a repo-relative file that wires an
// Orchestrator into a session to the reason it does not need the poller.
var listenerlessAutopoiesisConsumers = map[string]string{}

func TestKernelListener_WhenSessionWiresOrchestrator_ShouldStartListener(t *testing.T) {
	root := autopoiesisRepoRoot(t)
	chatDir := filepath.Join(root, "cmd", "nerd", "chat")
	if _, err := os.Stat(chatDir); err != nil {
		t.Skipf("chat package not present: %v", err)
	}

	wiresOrchestrator := map[string]bool{}
	startsListener := map[string]bool{}
	intervals := map[string]string{}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(chatDir)
	if err != nil {
		t.Fatalf("read chat dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(chatDir, name)
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			continue
		}
		rel := "cmd/nerd/chat/" + name

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.KeyValueExpr:
				// SessionConfig{ ..., Autopoiesis: orch, ... } is the wire that
				// makes an Orchestrator reachable from an interactive session.
				if key, ok := node.Key.(*ast.Ident); ok && key.Name == "Autopoiesis" {
					if !isNilIdent(node.Value) {
						wiresOrchestrator[rel] = true
					}
				}
			case *ast.CallExpr:
				sel, ok := node.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "StartKernelListener" {
					return true
				}
				startsListener[rel] = true
				if len(node.Args) == 2 {
					intervals[rel+":"+strconv.Itoa(fset.Position(sel.Sel.Pos()).Line)] = exprText(fset, node.Args[1])
				}
			}
			return true
		})
	}

	if len(wiresOrchestrator) == 0 {
		t.Fatal("scanner found no chat file wiring an Orchestrator into a session — the scanner is broken, not the repo")
	}

	var missing []string
	for file := range wiresOrchestrator {
		if startsListener[file] {
			continue
		}
		if reason, ok := listenerlessAutopoiesisConsumers[file]; ok {
			t.Logf("%-40s no listener [%s]", file, reason)
			continue
		}
		missing = append(missing, file)
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("interactive boot path wires an autopoiesis Orchestrator but never calls StartKernelListener:\n  %s\n\n"+
			"Kernel-derived delegate_task(/tool_generator, …, /pending) facts would never be processed in that session. "+
			"Start the listener with autopoiesis.DefaultKernelPollInterval, or record why it is not needed in "+
			"listenerlessAutopoiesisConsumers.",
			strings.Join(missing, "\n  "))
	}

	for file := range listenerlessAutopoiesisConsumers {
		if !wiresOrchestrator[file] || startsListener[file] {
			t.Errorf("stale entry in listenerlessAutopoiesisConsumers for %q", file)
		}
	}

	// Poll cadence: documented as DefaultKernelPollInterval. Accept either the
	// constant or the literal it is defined as, so this does not become a
	// style check.
	if len(intervals) == 0 {
		t.Fatal("no StartKernelListener call sites found in the chat package")
	}
	for site, arg := range intervals {
		normalized := strings.ReplaceAll(arg, " ", "")
		switch normalized {
		case "2*time.Second", "autopoiesis.DefaultKernelPollInterval", "DefaultKernelPollInterval":
			t.Logf("%-60s poll interval %s", site, arg)
		default:
			t.Errorf("%s starts the kernel listener at %q; the documented interactive cadence is "+
				"autopoiesis.DefaultKernelPollInterval (%v)", site, arg, DefaultKernelPollInterval)
		}
	}
}

func isNilIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

func TestDefaultKernelPollInterval_ShouldMatchInteractiveBootCadence(t *testing.T) {
	if DefaultKernelPollInterval != 2*time.Second {
		t.Errorf("DefaultKernelPollInterval = %v; the interactive boot paths use 2s and the constant documents them",
			DefaultKernelPollInterval)
	}
}

// The listener is the only thing that turns a pending delegation fact into a
// generated tool, so prove it actually polls rather than merely starting.
func TestStartKernelListener_WhenDelegationPending_ShouldProcessIt(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	mock := replaceOuroborosWithMock(orch)

	generated := make(chan string, 1)
	mock.ExecuteFunc = func(ctx context.Context, need *ToolNeed) *LoopResult {
		select {
		case generated <- need.Name:
		default:
		}
		return &LoopResult{
			Success:    true,
			ToolName:   need.Name,
			Stage:      StageComplete,
			ToolHandle: runtimeToolFixture(need.Name),
		}
	}

	kernel := &MockKernelInterface{}
	kernel.QueryPredicateFunc = func(predicate string) ([]types.KernelFact, error) {
		if predicate != "delegate_task" {
			return nil, nil
		}
		return []types.KernelFact{{
			Predicate: "delegate_task",
			Args:      []any{"/tool_generator", "csv_summarizer", "/pending"},
		}}, nil
	}
	orch.SetKernel(kernel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := orch.StartKernelListener(ctx, 10*time.Millisecond)

	select {
	case name := <-generated:
		if name != "csv_summarizer" {
			t.Errorf("listener generated %q, want csv_summarizer", name)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("listener never processed the pending delegation")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not stop after context cancellation")
	}
}
