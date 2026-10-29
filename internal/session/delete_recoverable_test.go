package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"codenerd/internal/types"
	"testing"
)

// These pin the narrowing, and the narrowing is the whole point: the gate on
// /delete_file is not removed, it is made to stop claiming that a delete git
// can undo is irreversible. Every case that is NOT recoverable must keep
// answering false, because that is what still requires a human.

func gitInit(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	return dir
}

func writeAndCommit(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", "seed"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	return p
}

func TestGitRestoresExactly(t *testing.T) {
	dir := gitInit(t)
	writeAndCommit(t, dir, "docs/stub.md", "# Redirect\n\nSee elsewhere.\n")
	ctx := context.Background()

	t.Run("tracked and clean is recoverable", func(t *testing.T) {
		if !gitRestoresExactly(ctx, dir, "docs/stub.md") {
			t.Error("a committed, unmodified file must be recoverable: git checkout restores it exactly")
		}
	})

	t.Run("absolute path to the same file is recoverable", func(t *testing.T) {
		if !gitRestoresExactly(ctx, dir, filepath.Join(dir, "docs/stub.md")) {
			t.Error("the tool passes absolute paths; they must resolve the same way")
		}
	})

	t.Run("untracked is not recoverable", func(t *testing.T) {
		p := filepath.Join(dir, "docs", "scratch.md")
		if err := os.WriteFile(p, []byte("not committed"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if gitRestoresExactly(ctx, dir, "docs/scratch.md") {
			t.Error("an untracked file is gone for good; it must still require approval")
		}
	})

	t.Run("tracked but modified is not recoverable", func(t *testing.T) {
		p := filepath.Join(dir, "docs", "stub.md")
		if err := os.WriteFile(p, []byte("# Redirect\n\nedited, not committed\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if gitRestoresExactly(ctx, dir, "docs/stub.md") {
			t.Error("uncommitted work would be destroyed; it must still require approval")
		}
		// Put it back so later subtests see a clean tree.
		writeAndCommit(t, dir, "docs/stub.md", "# Redirect\n\nSee elsewhere.\n")
	})

	t.Run("missing file is not recoverable", func(t *testing.T) {
		if gitRestoresExactly(ctx, dir, "docs/never-existed.md") {
			t.Error("a path git does not track must answer false")
		}
	})

	t.Run("path escaping the workspace is refused", func(t *testing.T) {
		if gitRestoresExactly(ctx, dir, "../outside.md") {
			t.Error("a target that climbs out of the workspace must never be recoverable")
		}
		if gitRestoresExactly(ctx, dir, filepath.Join(filepath.Dir(dir), "outside.md")) {
			t.Error("an absolute path outside the workspace must never be recoverable")
		}
	})

	t.Run("empty inputs are refused", func(t *testing.T) {
		if gitRestoresExactly(ctx, "", "docs/stub.md") {
			t.Error("no workspace means the check cannot be made; it must fail closed")
		}
		if gitRestoresExactly(ctx, dir, "") {
			t.Error("no target means the check cannot be made; it must fail closed")
		}
	})

	t.Run("a non-repository is refused", func(t *testing.T) {
		plain := t.TempDir()
		if err := os.WriteFile(filepath.Join(plain, "f.md"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if gitRestoresExactly(ctx, plain, "f.md") {
			t.Error("outside a repository nothing can be restored; it must fail closed")
		}
	})
}

// The fact is asserted for /delete_file and for nothing else. A gate that
// leaked this onto other actions would be granting them a permission route
// they were never meant to have.
func TestRecoverableDeleteFactOnlyForDeleteFile(t *testing.T) {
	dir := gitInit(t)
	writeAndCommit(t, dir, "keep.md", "body\n")

	e := &Executor{}
	e.SetConfig(ExecutorConfig{WorkspaceRoot: dir})
	ctx := context.Background()

	fact, ok := e.recoverableDeleteFact(ctx, "/delete_file", "keep.md")
	if !ok {
		t.Fatal("a tracked, clean target of /delete_file must produce the fact")
	}
	if fact.Predicate != deleteRecoverablePredicate {
		t.Errorf("predicate = %q, want %q", fact.Predicate, deleteRecoverablePredicate)
	}
	if len(fact.Args) != 1 || fact.Args[0] != "keep.md" {
		t.Errorf("args = %v, want the target only", fact.Args)
	}

	for _, action := range []types.MangleAtom{"/write_file", "/edit_file", "/run_command", "/git_push"} {
		if _, ok := e.recoverableDeleteFact(ctx, action, "keep.md"); ok {
			t.Errorf("%s must not produce file_recoverable: it is not a delete", action)
		}
	}
}
