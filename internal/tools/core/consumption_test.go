package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// The tools that scan report what they scanned, so the ledger can compare it
// with what they returned.

func seedScanTree(t *testing.T) (root string, totalBytes int64) {
	t.Helper()
	root = t.TempDir()
	for name, body := range map[string]string{
		"a.go":     "package a\n// needle one\n" + strings.Repeat("filler line\n", 50),
		"b.go":     "package b\n" + strings.Repeat("other line\n", 80),
		"sub/c.go": "package c\n// needle two\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		totalBytes += int64(len(body))
	}
	return root, totalBytes
}

func TestGrep_ReportsTheBytesItReadNotTheBytesItReturned(t *testing.T) {
	root, total := seedScanTree(t)
	ctx, consumed := tools.WithConsumptionLog(wsCtx(root))
	out, err := executeGrep(ctx, map[string]any{"pattern": "needle", "path": root})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := consumed()
	if !ok {
		t.Fatal("grep reported no consumption")
	}
	if got.Items != 3 || got.Bytes != total {
		t.Errorf("consumed %+v, want all %d bytes of 3 files read", got, total)
	}
	if int64(len(out)) >= got.Bytes/4 {
		t.Errorf("grep returned %d bytes for %d scanned: not a condensing result", len(out), got.Bytes)
	}
}

func TestGrep_ACappedSearchReportsOnlyWhatItRead(t *testing.T) {
	root, total := seedScanTree(t)
	ctx, consumed := tools.WithConsumptionLog(wsCtx(root))
	if _, err := executeGrep(ctx, map[string]any{"pattern": "needle", "path": root, "max_results": 1}); err != nil {
		t.Fatal(err)
	}
	got, _ := consumed()
	if got.Bytes >= total || got.Items >= 3 {
		t.Errorf("a search that stopped at its cap reported the whole tree: %+v of %d bytes", got, total)
	}
}

func TestSearchCode_ReportsItsScan(t *testing.T) {
	root, _ := seedScanTree(t)
	ctx, consumed := tools.WithConsumptionLog(wsCtx(root))
	if _, err := executeSearchCode(ctx, map[string]any{"pattern": "needle", "path": root}); err != nil {
		t.Fatal(err)
	}
	if got, ok := consumed(); !ok || got.Bytes == 0 {
		t.Errorf("search_code consumption %+v reported=%v", got, ok)
	}
}

func TestReadFile_ConsumesTheWholeFileWhateverItShows(t *testing.T) {
	root, _ := seedScanTree(t)
	body, _ := os.ReadFile(filepath.Join(root, "b.go"))
	ctx, consumed := tools.WithConsumptionLog(wsCtx(root))
	if _, err := executeReadFile(ctx, map[string]any{"path": "b.go", "start_line": 1, "end_line": 3}); err != nil {
		t.Fatal(err)
	}
	if got, ok := consumed(); !ok || got.Bytes != int64(len(body)) || got.Items != 1 {
		t.Errorf("consumed %+v, want the file's %d bytes", got, len(body))
	}
}

func TestListFilesAndGlob_ReportTheEntriesTheyVisited(t *testing.T) {
	root, _ := seedScanTree(t)
	ctx, consumed := tools.WithConsumptionLog(wsCtx(root))
	if _, err := executeListFiles(ctx, map[string]any{"path": ".", "recursive": true}); err != nil {
		t.Fatal(err)
	}
	if got, ok := consumed(); !ok || got.Items < 4 {
		t.Errorf("list_files visited %+v, want at least the 3 files and a directory", got)
	}
	ctx, consumed = tools.WithConsumptionLog(wsCtx(root))
	if _, err := executeGlob(ctx, map[string]any{"pattern": "**/*.go", "base_path": root}); err != nil {
		t.Fatal(err)
	}
	if got, ok := consumed(); !ok || got.Items < 3 {
		t.Errorf("recursive glob visited %+v", got)
	}
}

// A non-recursive glob is matched inside the standard library and cannot say
// what it scanned. It reports nothing, which the ledger reads as unknown.
func TestGlob_ASimplePatternReportsNothing(t *testing.T) {
	root, _ := seedScanTree(t)
	ctx, consumed := tools.WithConsumptionLog(wsCtx(root))
	if _, err := executeGlob(ctx, map[string]any{"pattern": "*.go", "base_path": root}); err != nil {
		t.Fatal(err)
	}
	if got, ok := consumed(); ok {
		t.Errorf("a simple glob guessed its scan: %+v", got)
	}
}
