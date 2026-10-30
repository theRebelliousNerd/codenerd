package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

// gitFixture is a real checkout with files a.txt .. (n-1).txt committed.
func gitFixture(t *testing.T, n int) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")
	for i := 0; i < n; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d.txt", i)), []byte("seed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", ".")
	run("-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "seed")
	return dir
}

func gitStateValue(t *testing.T, k *RealKernel, attr string) (string, bool) {
	t.Helper()
	if err := k.UpdateSystemFacts(); err != nil {
		t.Fatalf("UpdateSystemFacts: %v", err)
	}
	rows, err := k.Query("git_state")
	if err != nil {
		t.Fatalf("Query git_state: %v", err)
	}
	for _, f := range rows {
		if len(f.Args) == 2 && types.ExtractName(f.Args[0]) == attr {
			return types.ExtractString(f.Args[1]), true
		}
	}
	return "", false
}

// The porcelain parser keeps each row's leading status column, but gitCmd
// trimmed the whole output first, so the first row " M file" reached it as
// "M file", a staged change: one worktree edit published unstaged_count "0",
// and 21 published "20", one short of high churn (audit
// git-sysfacts-porcelain). Status output now reaches the parser as git wrote
// it.
func TestUpdateSystemFacts_CountsTheFirstWorktreeEdit(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 21} {
		dir := gitFixture(t, n)
		for i := 0; i < n; i++ {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d.txt", i)), []byte("edited\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		k, err := NewRealKernelWithWorkspace(dir)
		if err != nil {
			t.Fatalf("boot: %v", err)
		}
		got, ok := gitStateValue(t, k, "/unstaged_count")
		if !ok || got != fmt.Sprint(n) {
			t.Errorf("%d worktree edits published unstaged_count %q, want %q", n, got, fmt.Sprint(n))
		}
	}
}

// A staged first row stays staged: nothing is inferred about lost columns.
func TestUpdateSystemFacts_AStagedFirstRowIsNotUnstaged(t *testing.T) {
	t.Parallel()
	dir := gitFixture(t, 1)
	if err := os.WriteFile(filepath.Join(dir, "00.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "00.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	k, err := NewRealKernelWithWorkspace(dir)
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	if got, _ := gitStateValue(t, k, "/unstaged_count"); got != "0" {
		t.Errorf("a staged-only change published unstaged_count %q, want \"0\"", got)
	}
}

// gitCmd read stdout and stderr together, so a warning git wrote to stderr
// was parsed as a porcelain row: "warning: could not open directory ..."
// became a changed path with status "wa", counted unstaged. Only stdout is
// status; stderr belongs to the error.
func TestUpdateSystemFacts_AGitWarningIsNotAStatusRow(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads unreadable directories")
	}
	dir := gitFixture(t, 1)
	locked := filepath.Join(dir, "locked")
	if err := os.MkdirAll(filepath.Join(locked, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	stderr, _ := exec.Command("git", "-C", dir, "status", "--porcelain").CombinedOutput()
	if len(stderr) == 0 {
		t.Skip("this git writes no warning for an unreadable directory")
	}

	k, err := NewRealKernelWithWorkspace(dir)
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	if got, _ := gitStateValue(t, k, "/unstaged_count"); got != "0" {
		t.Errorf("a clean tree with an unreadable directory published unstaged_count %q, want \"0\"", got)
	}
	if files, ok := gitStateValue(t, k, "/modified_files"); ok {
		t.Errorf("git's stderr warning was published as modified files: %q", files)
	}
}
