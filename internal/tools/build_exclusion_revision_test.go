package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The projector skipped its check when a non-empty revision differed from the
// element's, taking the handler to refuse it. The handler reads an omission
// sentinel ("None") as no precondition and a padded revision trimmed, so the
// edit landed unchecked. Both now read a revision the same way.
func TestRefuseAddedBuildExclusion_ARevisionTheHandlerAcceptsIsProjected(t *testing.T) {
	pinExclusionEnv(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n\nfunc A() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rev := range []string{"None", "null", "  "} {
		err := RefuseAddedBuildExclusion("edit_element", map[string]any{
			"path": "a.go", "ref": "header", "old": "package p", "new": "//go:build ignore\n\npackage p", "revision": rev,
		}, dir)
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("revision %q: err = %v, want the header edit refused", rev, err)
		}
		err = RefuseAddedBuildExclusion("replace_element", map[string]any{
			"path": "a.go", "ref": "header", "source": "//go:build ignore\n\npackage p", "revision": rev,
		}, dir)
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("revision %q: err = %v, want the header replacement refused", rev, err)
		}
	}
	// A genuinely stale revision is the handler's to refuse; the projector
	// leaves it.
	if err := RefuseAddedBuildExclusion("edit_element", map[string]any{
		"path": "a.go", "ref": "header", "old": "package p", "new": "//go:build ignore\n\npackage p", "revision": "000000000000",
	}, dir); err != nil {
		t.Fatalf("a stale revision was refused by the projector instead of the handler: %v", err)
	}
	if rev, applies := RevisionPrecondition("  1f11f9cf40f7 "); !applies || rev != "1f11f9cf40f7" {
		t.Fatalf("RevisionPrecondition = %q, %v", rev, applies)
	}
}
