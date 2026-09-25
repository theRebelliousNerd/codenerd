package campaign

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gitFixture is a repository with one commit on main.
func gitFixture(t *testing.T, files map[string]string) recurseGit {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	writeTree(t, root, files)
	for _, args := range [][]string{
		{"init", "--quiet", "-b", "main"},
		{"add", "--all"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return recurseGit{root: root}
}

func gitOut(t *testing.T, g recurseGit, args ...string) string {
	t.Helper()
	out, err := g.run(context.Background(), nil, args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func readFile(t *testing.T, root, rel string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

func TestRecurseGit_PrepareOwnsABranchAndRefusesADirtyCheckout(t *testing.T) {
	ctx := context.Background()
	g := gitFixture(t, map[string]string{"a.txt": "a\n", ".gitignore": "out/\n"})
	writeTree(t, g.root, map[string]string{"notes.txt": "owner's untracked file\n"})
	if err := g.prepare(ctx, DefaultRecurseBranch); err != nil {
		t.Fatalf("an untracked file does not make the checkout dirty: %v", err)
	}
	if got := gitOut(t, g, "rev-parse", "--abbrev-ref", "HEAD"); got != DefaultRecurseBranch {
		t.Fatalf("branch = %q", got)
	}
	writeTree(t, g.root, map[string]string{"a.txt": "owner's edit\n"})
	if err := g.prepare(ctx, DefaultRecurseBranch); err == nil {
		t.Fatal("a tracked change the loop did not make must stop it from starting")
	}

	bare := recurseGit{root: t.TempDir()}
	if err := bare.prepare(ctx, DefaultRecurseBranch); err == nil {
		t.Fatal("no repository, no ratchet")
	}
}

// changed names exactly what an attempt did: edits, deletions, new files --
// not the owner's untracked files and not ignored output.
func TestRecurseGit_ChangedKeepCommitRevert(t *testing.T) {
	ctx := context.Background()
	g := gitFixture(t, map[string]string{
		"keep.txt": "v1\n", "edit.txt": "v1\n", "gone.txt": "here\n", ".gitignore": "out/\n",
	})
	writeTree(t, g.root, map[string]string{"owner.txt": "untouched\n"})
	before, err := g.untracked(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// The attempt: edit, delete, create, stage one of them, write build output.
	writeTree(t, g.root, map[string]string{"edit.txt": "v2\n", "new/file.txt": "new\n", "out/bin": "artifact\n"})
	if err := os.Remove(filepath.Join(g.root, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	gitOut(t, g, "add", "new/file.txt")

	changed, err := g.changed(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"edit.txt", "gone.txt", "new/file.txt"}; !slices.Equal(changed, want) {
		t.Fatalf("changed = %v, want %v", changed, want)
	}

	if err := g.revert(ctx, changed); err != nil {
		t.Fatal(err)
	}
	if got, _ := readFile(t, g.root, "edit.txt"); got != "v1\n" {
		t.Fatalf("edit.txt = %q after revert", got)
	}
	if _, ok := readFile(t, g.root, "gone.txt"); !ok {
		t.Fatal("a deleted tracked file comes back on revert")
	}
	if _, ok := readFile(t, g.root, "new/file.txt"); ok {
		t.Fatal("a file the attempt created is removed on revert, staged or not")
	}
	if got, ok := readFile(t, g.root, "owner.txt"); !ok || got != "untouched\n" {
		t.Fatal("revert must not touch the owner's untracked files")
	}
	if status := gitOut(t, g, "status", "--porcelain", "--untracked-files=no"); status != "" {
		t.Fatalf("revert leaves no tracked change behind: %q", status)
	}

	// A kept attempt commits exactly its paths, with trailers resume reads.
	writeTree(t, g.root, map[string]string{"edit.txt": "v3\n", "added.txt": "x\n"})
	changed, err = g.changed(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := g.commit(ctx, changed, "recurse: fix edit", 7, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if files := gitOut(t, g, "show", "--name-only", "--format=", hash); files != "added.txt\nedit.txt" {
		t.Fatalf("commit holds %q", files)
	}
	if _, ok := readFile(t, g.root, "owner.txt"); !ok || strings.Contains(gitOut(t, g, "show", "--name-only", "--format=", hash), "owner.txt") {
		t.Fatal("the owner's untracked file is neither committed nor removed")
	}
	kept, err := g.keptCycles(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if kept[7] != hash || len(kept) != 1 {
		t.Fatalf("kept cycles = %v, want 7 -> %s", kept, hash)
	}
}

// A workspace below the repository root sees and touches only its own tree.
func TestRecurseGit_WorkspaceInASubdirectory(t *testing.T) {
	ctx := context.Background()
	repo := gitFixture(t, map[string]string{"svc/a.txt": "a\n", "other/b.txt": "b\n"})
	g := recurseGit{root: filepath.Join(repo.root, "svc")}
	writeTree(t, repo.root, map[string]string{"svc/a.txt": "a2\n", "svc/n.txt": "n\n", "other/new.txt": "not ours\n"})
	changed, err := g.changed(ctx, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.txt", "n.txt"}; !slices.Equal(changed, want) {
		t.Fatalf("changed = %v, want %v (root-relative, workspace only)", changed, want)
	}
	if err := g.revert(ctx, changed); err != nil {
		t.Fatal(err)
	}
	if got, _ := readFile(t, g.root, "a.txt"); got != "a\n" {
		t.Fatalf("svc/a.txt = %q", got)
	}
	if _, ok := readFile(t, repo.root, "other/new.txt"); !ok {
		t.Fatal("a file outside the workspace is not the loop's to remove")
	}
}
