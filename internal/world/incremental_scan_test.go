package world

import (
	"codenerd/internal/core"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIncrementalScan_Basics(t *testing.T) {
	scanner := NewScanner()
	ctx := context.Background()

	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "incremental-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy file
	f1Path := filepath.Join(tmpDir, "test1.go")
	if err := os.WriteFile(f1Path, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make sure cache dir is correctly formed and permissions are good
	if err := os.MkdirAll(filepath.Join(tmpDir, ".nerd", "cache"), 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	opts := IncrementalOptions{}

	// Pass 1: First scan, should be Full scan
	res1, err := scanner.ScanWorkspaceIncremental(ctx, tmpDir, nil, opts)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if !res1.Full {
		t.Errorf("Expected full scan on empty cache")
	}
	// Note: We're not doing exact counts because of potential subdirs, just ensuring > 0
	if res1.FileCount == 0 {
		t.Errorf("Expected > 0 files, got %d", res1.FileCount)
	}

	// To simulate the cache persisting correctly between scans in the same test process,
	// the `cache.Save()` happens in a `defer` inside `ScanWorkspaceIncremental`.

	// Pass 2: Second scan, no changes
	res2, err := scanner.ScanWorkspaceIncremental(ctx, tmpDir, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if res2.Full {
		t.Errorf("Expected incremental scan, got full")
	}
	if !res2.Unchanged {
		t.Errorf("Expected unchanged flag to be true")
	}

	// Pass 3: Modify the file
	if err := os.WriteFile(f1Path, []byte("package main\nfunc main() {}\n// added\n"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	res3, err := scanner.ScanWorkspaceIncremental(ctx, tmpDir, nil, opts)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(res3.ChangedFiles) != 1 || res3.ChangedFiles[0] != f1Path {
		t.Errorf("Expected %s in changed files, got %v", f1Path, res3.ChangedFiles)
	}

	// Pass 4: Add a new file
	f2Path := filepath.Join(tmpDir, "test2.js")
	if err := os.WriteFile(f2Path, []byte("console.log('hi');\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	res4, err := scanner.ScanWorkspaceIncremental(ctx, tmpDir, nil, opts)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(res4.NewFiles) != 1 || res4.NewFiles[0] != f2Path {
		t.Errorf("Expected %s in new files, got %v", f2Path, res4.NewFiles)
	}

	// Pass 5: Delete a file
	if err := os.Remove(f1Path); err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	res5, err := scanner.ScanWorkspaceIncremental(ctx, tmpDir, nil, opts)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(res5.DeletedFiles) != 1 || res5.DeletedFiles[0] != f1Path {
		t.Errorf("Expected %s in deleted files, got %v", f1Path, res5.DeletedFiles)
	}
}

func TestGroupFactsByPath(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "file_topology", Args: []interface{}{"/a/b/c.go"}},
		{Predicate: "symbol_graph", Args: []interface{}{"foo", "bar", "baz", "/a/b/c.go"}},
		{Predicate: "global_thing", Args: []interface{}{"xyz"}},
	}
	out := groupFactsByPath(facts)

	if len(out["/a/b/c.go"]) != 2 {
		t.Errorf("Expected 2 facts for c.go, got %d", len(out["/a/b/c.go"]))
	}
	if len(out[globalWorldFactsPath]) != 1 {
		t.Errorf("Expected 1 global fact, got %d", len(out[globalWorldFactsPath]))
	}
}

func TestExtractHashFromFacts(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "file_topology", Args: []interface{}{"/a/b/c.go", "hash123"}},
	}
	hash := extractHashFromFacts(facts)
	if hash != "hash123" {
		t.Errorf("Expected hash123, got %s", hash)
	}
}

func TestDetectProjectLanguage(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "file_topology", Args: []interface{}{"1.go", "hash", core.MangleAtom("/go")}},
		{Predicate: "file_topology", Args: []interface{}{"2.go", "hash", core.MangleAtom("/go")}},
		{Predicate: "file_topology", Args: []interface{}{"3.js", "hash", core.MangleAtom("/javascript")}},
	}
	lang := detectProjectLanguage(facts)
	if lang != "go" {
		t.Errorf("Expected go, got %s", lang)
	}
}

func TestDetectEntryPoints(t *testing.T) {
	facts := []core.Fact{
		{Predicate: "file_topology", Args: []interface{}{"/path/to/main.go"}},
		{Predicate: "file_topology", Args: []interface{}{"/path/to/lib.go"}},
		{Predicate: "symbol_graph", Args: []interface{}{"func:main", "function", "public", "/path/to/lib.go", ""}},
	}
	eps := detectEntryPoints(facts)

	if len(eps) != 2 {
		t.Errorf("Expected 2 entry points, got %d", len(eps))
	}

	foundMainGo := false
	foundLibGo := false
	for _, ep := range eps {
		if path, ok := ep.Args[0].(string); ok {
			if path == "/path/to/main.go" {
				foundMainGo = true
			}
			if path == "/path/to/lib.go" {
				foundLibGo = true
			}
		}
	}

	if !foundMainGo || !foundLibGo {
		t.Errorf("Missing expected entry points")
	}
}
