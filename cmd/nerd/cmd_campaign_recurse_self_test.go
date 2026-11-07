package main

import (
	"errors"
	"strings"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
)

// The command's loop configuration is what switches the run's currency on:
// the base branch from the flag, and the run's own rebuild-and-restart. A
// bounded mission gets neither (its evidence is bound to the tree it measured).
func TestRecurseCycleConfig_WiresTheBaseBranchAndSelfUpdate(t *testing.T) {
	cfg, err := resolveRecurseConfig(0, nil, 0, "trunk")
	if err != nil {
		t.Fatal(err)
	}
	var kernel core.Kernel // wiring only; the loop is not started
	got := recurseCycleConfig(cfg, nil, t.TempDir(), kernel, nil, nil, nil)
	if got.BaseBranch != "trunk" {
		t.Fatalf("BaseBranch = %q, want the flag's value", got.BaseBranch)
	}
	if got.SelfUpdate == nil {
		t.Fatal("`nerd campaign recurse` does not switch the run's self-update on")
	}
	mission := recurseCycleConfig(cfg, &campaign.RecurseMission{}, t.TempDir(), kernel, nil, nil, nil)
	if mission.SelfUpdate != nil {
		t.Fatal("a mission run must not restart onto a rebuilt binary")
	}
}

// The hand-over is the same command line the run was started with, on the
// rebuilt binary; the process image replacement itself is injected.
func TestRestartRecurse_HandsOverToTheRebuiltBinaryWithTheSameArguments(t *testing.T) {
	restart := &campaign.RecurseRestart{Binary: "/work/.nerd/recurse/bin/nerd-abc123", Commit: "abc123def456"}
	args := []string{"campaign", "recurse", "--waves", "0", "--base-branch", "main"}
	var gotArgv0 string
	var gotArgv, gotEnv []string
	var out strings.Builder
	err := restartRecurse(&out, restart, args, func(argv0 string, argv, envv []string) error {
		gotArgv0, gotArgv, gotEnv = argv0, argv, envv
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotArgv0 != restart.Binary {
		t.Fatalf("exec'd %q, want the rebuilt binary", gotArgv0)
	}
	if want := append([]string{restart.Binary}, args...); strings.Join(gotArgv, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv = %v, want %v", gotArgv, want)
	}
	if len(gotEnv) == 0 {
		t.Fatal("the process environment is not carried over")
	}
	if !strings.Contains(out.String(), "resuming from the ledger") {
		t.Fatalf("the hand-over is not announced: %q", out.String())
	}

	// A hand-over that fails returns, and says how to resume by hand.
	err = restartRecurse(&out, restart, args, func(string, []string, []string) error { return errors.New("exec format error") })
	if err == nil || !strings.Contains(err.Error(), "exec format error") || !strings.Contains(err.Error(), restart.Binary+" campaign recurse --waves 0") {
		t.Fatalf("error = %v; want the cause and the command to resume with", err)
	}
}
