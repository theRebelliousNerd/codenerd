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

// landOnBase commits files onto branch without touching any checkout or the
// repository's index: the way a fix lands on main from another checkout while
// recurse runs. It returns the new commit.
func landOnBase(t *testing.T, g recurseGit, branch string, files map[string]string, msg string) string {
	t.Helper()
	idx := filepath.Join(t.TempDir(), "index")
	env := append(os.Environ(), "GIT_INDEX_FILE="+idx,
		"GIT_AUTHOR_NAME=base", "GIT_AUTHOR_EMAIL=base@example.com",
		"GIT_COMMITTER_NAME=base", "GIT_COMMITTER_EMAIL=base@example.com")
	run := func(stdin string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = g.root, env
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	parent := run("", "rev-parse", "refs/heads/"+branch)
	run("", "read-tree", parent)
	for rel, body := range files {
		blob := run(body, "hash-object", "-w", "--stdin")
		run("", "update-index", "--add", "--cacheinfo", "100644,"+blob+","+rel)
	}
	tree := run("", "write-tree")
	commit := run("", "commit-tree", tree, "-p", parent, "-m", msg)
	run("", "update-ref", "refs/heads/"+branch, commit, parent)
	return commit
}

func headOf(t *testing.T, g recurseGit) string { return gitOut(t, g, "rev-parse", "HEAD") }

func TestRecurseBase_ResolutionOrder(t *testing.T) {
	ctx := context.Background()
	g := gitFixture(t, map[string]string{"a.txt": "a\n"})
	gitOut(t, g, "branch", "trunk")
	gitOut(t, g, "branch", "release")
	// The recurse branch is created from trunk: its creation record says so.
	gitOut(t, g, "branch", DefaultRecurseBranch, "trunk")

	pick := func(flag, recorded, start string) (string, string) {
		t.Helper()
		name, source, err := g.resolveBase(ctx, DefaultRecurseBranch, flag, recorded, start)
		if err != nil {
			t.Fatal(err)
		}
		return name, source
	}
	if name, src := pick("release", "trunk", "main"); name != "release" || src != baseFromFlag {
		t.Fatalf("the flag wins: %s from %s", name, src)
	}
	if name, src := pick("", "release", "main"); name != "release" || src != baseFromLedger {
		t.Fatalf("the ledger's recorded base beats the start branch: %s from %s", name, src)
	}
	if name, src := pick("", "gone-branch", "main"); name != "main" || src != baseFromStart {
		t.Fatalf("a recorded base that no longer exists falls through to the start branch: %s from %s", name, src)
	}
	if name, src := pick("", "", DefaultRecurseBranch); name != "trunk" || src != baseFromReflog {
		t.Fatalf("started on the recurse branch itself: the branch it was created from, not itself: %s from %s", name, src)
	}
	if _, _, err := g.resolveBase(ctx, DefaultRecurseBranch, "nowhere", "", ""); err == nil {
		t.Fatal("a flag naming no commit must fail loudly")
	}
	if _, _, err := g.resolveBase(ctx, DefaultRecurseBranch, DefaultRecurseBranch, "", ""); err == nil {
		t.Fatal("the recurse branch cannot be its own base")
	}

	// No record, no start branch, no creation record: a local main.
	h := gitFixture(t, map[string]string{"a.txt": "a\n"})
	gitOut(t, h, "branch", DefaultRecurseBranch)
	if err := os.Remove(filepath.Join(h.root, ".git", "logs", "refs", "heads", filepath.FromSlash(DefaultRecurseBranch))); err != nil {
		t.Fatal(err)
	}
	if name, src, _ := h.resolveBase(ctx, DefaultRecurseBranch, "", "", ""); name != "main" || src != baseFromMain {
		t.Fatalf("fallback: %s from %s", name, src)
	}
	// Nothing to absorb from: absorbing is off, said by the caller.
	solo := gitFixture(t, map[string]string{"a.txt": "a\n"})
	gitOut(t, solo, "branch", "-m", "main", DefaultRecurseBranch)
	if name, _, err := solo.resolveBase(ctx, DefaultRecurseBranch, "", "", ""); err != nil || name != "" {
		t.Fatalf("no base exists: got %q, %v", name, err)
	}
}

// A base commit after the start is merged; the loop's own kept commit keeps
// its hash and its place in the ledger's source of truth, and a cycle trailer
// that arrives from the base is never read as the loop's own.
func TestRecurseGit_AbsorbMergesAndKeepsKeptCommitIdentity(t *testing.T) {
	ctx := context.Background()
	g := gitFixture(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	if err := g.prepare(ctx, DefaultRecurseBranch); err != nil {
		t.Fatal(err)
	}
	writeTree(t, g.root, map[string]string{"a.txt": "a fixed\n"})
	kept, err := g.commit(ctx, []string{"a.txt"}, "recurse: fix a", 1, "finding-a")
	if err != nil {
		t.Fatal(err)
	}
	tip := landOnBase(t, g, "main", map[string]string{"b.txt": "b from base\n", "new/c.txt": "c\n"},
		"base fix\n\nRecurse-Cycle: 1\nRecurse-Finding: foreign\n")

	gotTip, behind, err := g.behindBase(ctx, "main")
	if err != nil || gotTip != tip || behind != 1 {
		t.Fatalf("behindBase = %s, %d, %v; want %s, 1", gotTip, behind, err, tip)
	}
	res, err := g.absorb(ctx, "main", tip, behind)
	if err != nil || res.Outcome != absorbMerged {
		t.Fatalf("absorb = %+v, %v", res, err)
	}
	if want := []string{"b.txt", "new/c.txt"}; !slices.Equal(res.Changed, want) {
		t.Fatalf("changed = %v, want %v", res.Changed, want)
	}
	if got, _ := readFile(t, g.root, "b.txt"); got != "b from base\n" {
		t.Fatalf("b.txt = %q: the base's change is in the tree", got)
	}
	if got, _ := readFile(t, g.root, "a.txt"); got != "a fixed\n" {
		t.Fatalf("a.txt = %q: the kept fix survives the merge", got)
	}
	if parents := strings.Fields(gitOut(t, g, "rev-list", "--parents", "-n", "1", "HEAD")); len(parents) != 3 {
		t.Fatalf("the absorb is a merge commit with two parents, got %v", parents)
	}
	if msg := gitOut(t, g, "log", "-1", "--format=%B"); !strings.Contains(msg, recurseAbsorbTrailer+": "+tip) {
		t.Fatalf("the merge names the base commit it absorbed: %q", msg)
	}
	cycles, err := g.keptCycles(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 1 || cycles[1] != kept {
		t.Fatalf("kept cycles = %v; want only cycle 1 -> %s (the base's own trailer is not ours)", cycles, kept)
	}
	if _, behind, _ := g.behindBase(ctx, "main"); behind != 0 {
		t.Fatalf("still %d commits behind after the absorb", behind)
	}
}

// A conflict is aborted and the tree is exactly as it was.
func TestRecurseGit_AbsorbConflictLeavesTheTreeUntouched(t *testing.T) {
	ctx := context.Background()
	g := gitFixture(t, map[string]string{"a.txt": "line\n", "keep.txt": "k\n"})
	if err := g.prepare(ctx, DefaultRecurseBranch); err != nil {
		t.Fatal(err)
	}
	writeTree(t, g.root, map[string]string{"a.txt": "ours\n"})
	if _, err := g.commit(ctx, []string{"a.txt"}, "recurse: fix a", 1, "f"); err != nil {
		t.Fatal(err)
	}
	writeTree(t, g.root, map[string]string{"owner.txt": "the owner's untracked file\n"})
	before := headOf(t, g)
	tip := landOnBase(t, g, "main", map[string]string{"a.txt": "theirs\n", "keep.txt": "base keep\n"}, "base edits a")

	res, err := g.absorb(ctx, "main", tip, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != absorbConflict || !slices.Equal(res.Conflicts, []string{"a.txt"}) {
		t.Fatalf("absorb = %+v; want a conflict on a.txt", res)
	}
	if headOf(t, g) != before {
		t.Fatal("HEAD moved on a conflict")
	}
	if status := gitOut(t, g, "status", "--porcelain", "--untracked-files=no"); status != "" {
		t.Fatalf("the tree is not clean after the abort: %q", status)
	}
	if _, err := os.Stat(filepath.Join(g.root, ".git", "MERGE_HEAD")); err == nil {
		t.Fatal("a merge is still in progress")
	}
	if got, _ := readFile(t, g.root, "a.txt"); got != "ours\n" {
		t.Fatalf("a.txt = %q", got)
	}
	if got, _ := readFile(t, g.root, "keep.txt"); got != "k\n" {
		t.Fatalf("keep.txt = %q: a clean part of the base must not leak in", got)
	}
	if got, ok := readFile(t, g.root, "owner.txt"); !ok || got != "the owner's untracked file\n" {
		t.Fatal("the owner's untracked file was touched")
	}
}

// A merge git refuses to start (an owner's untracked file in the way) and a
// tree with uncommitted tracked changes are both left as they were.
func TestRecurseGit_AbsorbRefusedAndSkippedLeaveTheTreeAlone(t *testing.T) {
	ctx := context.Background()
	g := gitFixture(t, map[string]string{"a.txt": "a\n"})
	if err := g.prepare(ctx, DefaultRecurseBranch); err != nil {
		t.Fatal(err)
	}
	tip := landOnBase(t, g, "main", map[string]string{"notes.txt": "from base\n"}, "base adds notes")

	writeTree(t, g.root, map[string]string{"notes.txt": "owner's untracked notes\n"})
	before := headOf(t, g)
	res, err := g.absorb(ctx, "main", tip, 1)
	if err != nil || res.Outcome != absorbConflict || res.Reason == "" {
		t.Fatalf("a merge git will not start is a conflict with its reason: %+v, %v", res, err)
	}
	if got, _ := readFile(t, g.root, "notes.txt"); got != "owner's untracked notes\n" || headOf(t, g) != before {
		t.Fatal("the refused merge touched the tree")
	}

	if err := os.Remove(filepath.Join(g.root, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, g.root, map[string]string{"a.txt": "owner edit\n"})
	res, err = g.absorb(ctx, "main", tip, 1)
	if err != nil || res.Outcome != absorbSkipped {
		t.Fatalf("a dirty tracked tree is skipped: %+v, %v", res, err)
	}
	if got, _ := readFile(t, g.root, "a.txt"); got != "owner edit\n" || headOf(t, g) != before {
		t.Fatal("the skipped absorb touched the tree")
	}
}

// The creation record's start point is a base only when git has a branch by
// that name: a branch named like a hash counts, a bare commit does not.
func TestRecurseBase_CreatedFromAsksGitForABranch(t *testing.T) {
	ctx := context.Background()
	g := gitFixture(t, map[string]string{"a.txt": "a\n"})
	gitOut(t, g, "branch", "abcdef1") // a branch whose name looks like a hash
	gitOut(t, g, "branch", "from-hashy", "abcdef1")
	if got := g.createdFrom(ctx, "from-hashy"); got != "abcdef1" {
		t.Fatalf("created from a hash-shaped branch: got %q, want abcdef1", got)
	}
	gitOut(t, g, "branch", "from-commit", headOf(t, g))
	if got := g.createdFrom(ctx, "from-commit"); got != "" {
		t.Fatalf("created from a bare commit named a base: %q", got)
	}
}
