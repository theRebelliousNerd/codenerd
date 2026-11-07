package world

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A delta scan over a tree with one edited file reads that one file: every
// other stamp is trusted, and the counters say so. The old line printed
// hits=0 misses=0 dirty=<every entry> for any such scan.
func TestIncrementalScan_CacheCountersNameTheWork(t *testing.T) {
	useStableContentStamp(t)
	root := canonicalTempRoot(t)
	old := time.Unix(1_700_000_000, 0)
	for _, name := range []string{"a.go", "b.go", "c.go", "d.go"} {
		writeAt(t, filepath.Join(root, name), []byte("package p\n\nfunc "+strings.ToUpper(name[:1])+"() {}\n"), old)
	}
	db := newScanDB(t)
	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(root, "b.go")
	writeAt(t, edited, []byte("package p\n\nfunc B() { _ = 1 }\n"), old.Add(time.Hour))

	res, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Full || res.Unchanged {
		t.Fatalf("delta scan full=%v unchanged=%v", res.Full, res.Unchanged)
	}
	if len(res.ChangedFiles) != 1 || !pathListed(res.ChangedFiles, edited) {
		t.Fatalf("changed = %v, want only %s", res.ChangedFiles, edited)
	}
	if res.Cache.Hits != 3 || res.Cache.Misses != 1 {
		t.Fatalf("cache = %+v, want 3 trusted stamps and 1 read", res.Cache)
	}
	if !res.Cache.Unsaved {
		t.Fatal("the edited file's new hash was not staged for the manifest")
	}
	if got := res.Cache.String(); !strings.Contains(got, "unsaved=true") || strings.Contains(got, "dirty=") {
		t.Fatalf("stats line %q still reports a per-entry dirty count", got)
	}
}

func TestScanBreakdownSamples_SortedCanonicalAndCapped(t *testing.T) {
	root := canonicalTempRoot(t)
	var changed []string
	for _, n := range []string{"g", "f", "e", "d", "c", "b", "a"} {
		changed = append(changed, filepath.Join(root, "pkg", n+".go"))
	}
	got := scanBreakdownSamples(root, map[string][]string{
		"changed": changed,
		"deleted": {filepath.Join(root, "gone.md")},
	})
	want := "; changed: pkg/a.go pkg/b.go pkg/c.go pkg/d.go pkg/e.go (+2 more); deleted: gone.md"
	if got != want {
		t.Fatalf("samples\n got %q\nwant %q", got, want)
	}
	if scanBreakdownSamples(root, nil) != "" {
		t.Fatal("an empty breakdown named paths")
	}
}
