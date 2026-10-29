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
