package orient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A nested checkout (a git worktree's .git file, a submodule's .git dir) is
// another repository; its agent files are not this workspace's corpus.
func TestCollectSkipsNestedCheckouts(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".git/HEAD", "ref: refs/heads/main\n")
	write(".claude/agents/own.md", "own agent")
	write(".claude/worktrees/copy/.git", "gitdir: /elsewhere\n")
	write(".claude/worktrees/copy/.claude/agents/own.md", "copied agent")
	write("vendor/sub/.git/HEAD", "ref: refs/heads/main\n")
	write("vendor/sub/AGENTS.md", "submodule agent")

	rels, err := collect(root)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	got := map[string]bool{}
	for _, rel := range rels {
		got[rel] = true
	}
	if !got[".claude/agents/own.md"] {
		t.Fatalf("the workspace's own agent file was not collected: %v", rels)
	}
	for _, rel := range rels {
		if strings.HasPrefix(rel, ".claude/worktrees/") || strings.HasPrefix(rel, "vendor/sub/") {
			t.Errorf("collected %s from a nested checkout", rel)
		}
	}
}
