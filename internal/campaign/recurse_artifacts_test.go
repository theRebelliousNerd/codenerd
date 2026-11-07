package campaign

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// An attempt's `go build` output is not its change: a new untracked
// executable is deleted and dropped before the gates judge the attempt, so
// it can never be committed. New source files, and executables that were
// already there before the attempt, are left alone.
func TestDiscardBuiltExecutables(t *testing.T) {
	root := recurseFixture(t, nil)
	r := &recurseRun{root: root, git: recurseGit{root: root}}
	ctx := context.Background()

	write := func(rel string, body []byte) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, body, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	elf := []byte{0x7F, 'E', 'L', 'F', 2, 1, 1, 0}
	write("prior_tool", elf) // the owner's, untracked before the attempt
	before, err := r.git.untracked(ctx)
	if err != nil {
		t.Fatal(err)
	}

	write("audit_tool", elf)
	write("pkg/new_test.go", []byte("package pkg\n"))
	write("run.sh", []byte("#!/bin/sh\necho ok\n"))
	changed, err := r.changedByAttempt(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := r.discardBuiltExecutables(ctx, changed, before)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range kept {
		got[p] = true
	}
	if got["audit_tool"] {
		t.Fatal("the attempt's built executable stayed in the change set")
	}
	if _, err := os.Stat(filepath.Join(root, "audit_tool")); !os.IsNotExist(err) {
		t.Fatal("the attempt's built executable is still on disk")
	}
	for _, want := range []string{"pkg/new_test.go", "run.sh"} {
		if !got[want] {
			t.Fatalf("source %s was dropped: %v", want, kept)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "prior_tool")); err != nil {
		t.Fatal("an executable that predates the attempt was removed")
	}

	// An attempt whose only change is a binary changed nothing.
	if err := os.Remove(filepath.Join(root, "pkg/new_test.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "run.sh")); err != nil {
		t.Fatal(err)
	}
	write("only_binary", elf)
	changed, err = r.changedByAttempt(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	if kept, err = r.discardBuiltExecutables(ctx, changed, before); err != nil || len(kept) != 0 {
		t.Fatalf("binary-only attempt kept %v (%v)", kept, err)
	}
}
