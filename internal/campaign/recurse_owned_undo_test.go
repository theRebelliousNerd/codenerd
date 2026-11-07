package campaign

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/observation"
)

// A cancelled attempt undoes its own writes and nothing else. The attempt
// fixes store/store.go (and reports that write); while it runs, another
// writer edits web/web.go and, after the attempt wrote it, store/extra.go
// that the attempt also wrote. /recurse stop cancels. The attempt's fix is
// undone; the outside edit to web/web.go survives, and so does the outside
// edit on top of the attempt's store/extra.go (the file no longer holds what
// the attempt left); the journal names both. Before, the cleanup restored
// every file that differed from HEAD (audit
// recurse-compensation-write-ownership).
func TestRecurseCycles_ACancelledAttemptUndoesOnlyItsOwnWrites(t *testing.T) {
	t.Parallel()
	root := recurseFixture(t, map[string]string{"store/extra.go": "package store\n"})
	const outsideWeb = "package web\n\nimport \"example.com/m/store\"\n\n// Page is edited by someone else.\nfunc Page() int { return store.Get() }\n"
	const outsideExtra = "package store\n\n// edited by someone else after the attempt\n"
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempted := false
	execute := func(_ context.Context, a RecurseAttempt) ([]observation.FileWrite, error) {
		if a.Angle != "" || attempted {
			return nil, nil
		}
		attempted = true
		storePath := filepath.Join(root, "store", "store.go")
		extraPath := filepath.Join(root, "store", "extra.go")
		write(t, root, "store/store.go", fixedStore)
		const attemptExtra = "package store\n\n// the attempt's\n"
		write(t, root, "store/extra.go", attemptExtra)
		// Another writer, during the attempt.
		write(t, root, "web/web.go", outsideWeb)
		write(t, root, "store/extra.go", outsideExtra)
		cancel() // /recurse stop
		return []observation.FileWrite{
			{Path: storePath, Before: known(recurseModule["store/store.go"]), After: known(fixedStore)},
			{Path: extraPath, Before: known("package store\n"), After: known(attemptExtra)},
		}, ctx.Err()
	}
	_, err = RunRecurseCycles(ctx, RecurseCycleConfig{Workspace: root, Kernel: k, Execute: execute, Passes: 1})
	if err == nil {
		t.Fatal("a cancelled run returned no error")
	}
	if got := mustRead(t, root, "store/store.go"); got != recurseModule["store/store.go"] {
		t.Fatalf("the attempt's own write survived the cancel:\n%s", got)
	}
	if got := mustRead(t, root, "web/web.go"); got != outsideWeb {
		t.Fatalf("another writer's edit to web/web.go was undone:\n%s", got)
	}
	if got := mustRead(t, root, "store/extra.go"); got != outsideExtra {
		t.Fatalf("another writer's edit on top of the attempt's write was undone:\n%s", got)
	}
	var detail string
	for _, rec := range journalOf(t, root) {
		if rec.Step == stepRatchet && strings.Contains(rec.Detail, "stopped mid-attempt") {
			detail = rec.Detail
		}
	}
	if !strings.Contains(detail, "web/web.go (not this attempt's write)") || !strings.Contains(detail, "store/extra.go (changed since the attempt wrote it)") {
		t.Fatalf("the journal does not name what was left in place: %q", detail)
	}
}

func TestRecurseCycles_KeptAttemptPreservesConcurrentExternalEditsUncommitted(t *testing.T) {
	t.Parallel()
	const baseNerdMD = "---\nschema: nerd/v1\nforbid:\n  - match: nerd.md\n    reason: protected\n---\n# Test\n"
	root := recurseFixture(t, map[string]string{"nerd.md": baseNerdMD})
	const outsideWeb = "package web\n\nimport \"example.com/m/store\"\n\n// Page is concurrently edited by a human.\nfunc Page() int { return store.Get() }\n"
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	origNerdMD := mustRead(t, root, "nerd.md")
	humanNerdMD := origNerdMD + "\n<!-- human note -->\n"

	fixed := false
	execute := func(_ context.Context, a RecurseAttempt) ([]observation.FileWrite, error) {
		if a.Finding.ID == "" || fixed {
			return nil, nil
		}
		fixed = true
		storePath := filepath.Join(root, "store", "store.go")
		write(t, root, "store/store.go", fixedStore)
		// Human concurrently edits web/web.go and forbidden nerd.md while the attempt is running.
		write(t, root, "web/web.go", outsideWeb)
		write(t, root, "nerd.md", humanNerdMD)
		return []observation.FileWrite{
			{Path: storePath, Before: known(recurseModule["store/store.go"]), After: known(fixedStore)},
		}, nil
	}
	res, err := RunRecurseCycles(context.Background(), RecurseCycleConfig{Workspace: root, Kernel: k, Execute: execute, Passes: 2})
	if err != nil {
		t.Fatalf("RunRecurseCycles failed: %v", err)
	}
	if res.Kept != 1 || res.Refused != 0 {
		t.Fatalf("expected Kept=1 Refused=0, got %+v", res)
	}
	// Also verify resuming the run (e.g. after a self-restart) succeeds with the human's edits still in the working tree.
	k2, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RunRecurseCycles(context.Background(), RecurseCycleConfig{Workspace: root, Kernel: k2, Execute: execute, Passes: 1}); err != nil {
		t.Fatalf("resuming RunRecurseCycles with concurrent human edits in tree failed: %v", err)
	}
	if got := mustRead(t, root, "store/store.go"); got != fixedStore {
		t.Fatalf("expected store/store.go fix to be kept, got:\n%s", got)
	}
	if got := mustRead(t, root, "web/web.go"); got != outsideWeb {
		t.Fatalf("concurrent human edit to web/web.go was lost:\n%s", got)
	}
	if got := mustRead(t, root, "nerd.md"); got != humanNerdMD {
		t.Fatalf("concurrent human edit to nerd.md was lost:\n%s", got)
	}
	// Verify git status still shows web/web.go and nerd.md as uncommitted working-tree changes
	// (they were not swept into the attempt's commit).
	g := recurseGit{root: root}
	untracked, err := g.untracked(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	stillDirty, err := g.changed(context.Background(), untracked)
	if err != nil {
		t.Fatal(err)
	}
	dirtyStr := strings.Join(stillDirty, ",")
	if !strings.Contains(dirtyStr, "web/web.go") || !strings.Contains(dirtyStr, "nerd.md") {
		t.Fatalf("expected concurrent human edits to remain uncommitted in working tree, got dirty=%v", stillDirty)
	}
}

func TestFilterSnapshotForConcurrentWrites_PreservesHumanEditsInsideWriteSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	storeFile := filepath.Join(dir, "store.go")
	extraFile := filepath.Join(dir, "extra.go")
	otherFile := filepath.Join(dir, "other.go")

	write(t, dir, "store.go", "package store\nvar a = 1\n")
	write(t, dir, "extra.go", "package store\nvar b = 1\n")
	write(t, dir, "other.go", "package store\nvar c = 1\n")

	snap := taskExecutionSnapshot{
		fileMutations: []fileMutationSnapshot{
			{Path: storeFile, Exists: true, Content: []byte("package store\nvar a = 1\n"), Mode: 0o644},
			{Path: extraFile, Exists: true, Content: []byte("package store\nvar b = 1\n"), Mode: 0o644},
			{Path: otherFile, Exists: true, Content: []byte("package store\nvar c = 1\n"), Mode: 0o644},
		},
	}

	// Attempt writes store.go and extra.go.
	write(t, dir, "store.go", "package store\nvar a = 2\n")
	write(t, dir, "extra.go", "package store\nvar b = 2\n")
	// Human concurrently edits other.go (not touched by attempt) and extra.go (after attempt wrote it).
	write(t, dir, "other.go", "package store\nvar c = 99 // human\n")
	write(t, dir, "extra.go", "package store\nvar b = 99 // human on top\n")

	inside := []observation.FileWrite{
		{Path: storeFile, Before: known("package store\nvar a = 1\n"), After: known("package store\nvar a = 2\n")},
		{Path: extraFile, Before: known("package store\nvar b = 1\n"), After: known("package store\nvar b = 2\n")},
	}
	filtered, left := filterSnapshotForConcurrentWrites(snap, inside)
	if len(filtered.fileMutations) != 1 || filtered.fileMutations[0].Path != storeFile {
		t.Fatalf("expected only store.go to remain in rollback mutations, got %+v", filtered.fileMutations)
	}
	leftJoined := strings.Join(left, " | ")
	if !strings.Contains(leftJoined, "extra.go (changed since the attempt wrote it)") ||
		!strings.Contains(leftJoined, "other.go (changed by another writer during the attempt)") {
		t.Fatalf("unexpected left list: %v", left)
	}
}
