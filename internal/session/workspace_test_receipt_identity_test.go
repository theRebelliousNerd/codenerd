package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/gates"
	"codenerd/internal/processutil"
	"codenerd/internal/tools"
)

// A receipt satisfies a planned check only when it is the same invocation:
// the same argv, run in the same directory, under the same environment. The
// matcher compared argv alone, so `cargo test` in an independent nested
// project discharged the root's `cargo test` without running it (audit
// workspace-test-receipt-invocation-identity). Here the planned check would
// fail if run; each receipt below exited 0 with the planned argv.

// plannedIdentity is the planned check's own invocation, built by the
// resolver gates.Run executes with.
func plannedIdentity(t *testing.T, ws, bin string) tools.TestRun {
	t.Helper()
	g := gates.Gate{ID: "check", Kind: gates.Test, Scope: gates.ScopeNode, Argv: []string{filepath.ToSlash(bin), "{node}"}}
	cmd, argv, err := gates.Command(context.Background(), ws, g, "svc")
	if err != nil {
		t.Fatal(err)
	}
	dir, env := processutil.Identity(cmd)
	return tools.TestRun{Argv: argv, ExitCode: 0, Dir: dir, EnvDigest: env}
}

func TestWorkspaceTestRun_AReceiptFromAnotherDirectoryIsNotTheCheck(t *testing.T) {
	t.Parallel()
	bin := checkGate(t)
	e, result, ws := planTurn(t, nodeGate("check", bin))
	nested := filepath.Join(ws, "fixtures", "independent")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	run := plannedIdentity(t, ws, bin)
	run.Dir = nested
	e.noteTestRun(result, run)
	if ran, _ := e.workspaceTestRun(context.Background(), result); !ran {
		t.Fatal("the workspace declares a check for the written file")
	}
	if len(result.testPlan) != 1 || result.testPlan[0].Reused {
		t.Fatalf("a passing run in %s discharged the root's check: %+v", nested, result.testPlan)
	}
	if got := testRunGate(t, e, result); got != VerifyFailed {
		t.Fatalf("/test_run = %v, want failed: the check itself ran and is red", got)
	}
}

func TestWorkspaceTestRun_AReceiptUnderAnotherEnvironmentIsNotTheCheck(t *testing.T) {
	t.Parallel()
	bin := checkGate(t)
	e, result, ws := planTurn(t, nodeGate("check", bin))
	run := plannedIdentity(t, ws, bin)
	run.EnvDigest = "another environment"
	e.noteTestRun(result, run)
	if ran, _ := e.workspaceTestRun(context.Background(), result); !ran {
		t.Fatal("the workspace declares a check for the written file")
	}
	if len(result.testPlan) != 1 || result.testPlan[0].Reused {
		t.Fatalf("a run under another environment discharged the check: %+v", result.testPlan)
	}
}

// A receipt that does not say where or how it ran is unknown, not the root.
func TestWorkspaceTestRun_AReceiptWithoutIdentityIsNotTheCheck(t *testing.T) {
	t.Parallel()
	bin := checkGate(t)
	e, result, _ := planTurn(t, nodeGate("check", bin))
	e.noteTestRun(result, tools.TestRun{Argv: []string{filepath.ToSlash(bin), "svc"}, ExitCode: 0})
	if ran, _ := e.workspaceTestRun(context.Background(), result); !ran {
		t.Fatal("the workspace declares a check for the written file")
	}
	if len(result.testPlan) != 1 || result.testPlan[0].Reused {
		t.Fatalf("a receipt with no directory or environment discharged the check: %+v", result.testPlan)
	}
}
