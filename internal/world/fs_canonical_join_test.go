package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSymbolGraphDefinedAtMatchesFileTopology verifies that symbol_graph facts
// use the same workspace-relative canonical identity as file_topology facts.
// Measured failure: file_topology was keyed relative ("internal/session/executor.go")
// while symbol_graph carried an absolute Windows path
// ("C:\CodeProjects\codeNERD\internal\session\executor.go") in its DefinedAt
// slot (Args[3]), so no Mangle rule joining the two (notably unwired_function
// at reviewer.mg:427 and layer at reviewer.mg:541) could ever unify.
//
// Root cause is in Scanner.ScanDirectory: canonical := canonicalScanPath(root, path)
// is used for file_topology / file_dir but the TreeSitter parsers were handed the raw
// absolute path. The fix passes canonical as the identity to each parser call.
// This test fails before that fix and passes after.
func TestSymbolGraphDefinedAtMatchesFileTopology(t *testing.T) {
	tmpDir := t.TempDir()

	// One Go file with an exported function — enough to emit at least two
	// symbol_graph facts (package + function) and exercise the Go parser path.
	content := "package hello\n\nfunc ExportedFunc() string { return \"hello\" }\n"
	filePath := filepath.Join(tmpDir, "hello.go")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile hello.go: %v", err)
	}

	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}

	// Collect file_topology path for the file we created.
	var topoPath string
	var topoFound bool
	for _, f := range result.Facts {
		if f.Predicate != "file_topology" {
			continue
		}
		if len(f.Args) < 1 {
			continue
		}
		p, ok := f.Args[0].(string)
		if !ok {
			continue
		}
		// Canonical for a file directly under root is just the base name.
		if strings.HasSuffix(filepath.ToSlash(p), "hello.go") {
			topoPath = p
			topoFound = true
			break
		}
	}
	if !topoFound {
		t.Fatalf("no file_topology fact for hello.go; facts: %v", result.Facts)
	}

	// Also sanity-check that file_topology itself is canonical (no backslash,
	// no drive letter, not absolute).
	if strings.Contains(topoPath, "\\") {
		t.Errorf("file_topology path contains backslash: %q", topoPath)
	}
	if len(topoPath) >= 2 && topoPath[1] == ':' {
		t.Errorf("file_topology path begins with drive letter: %q", topoPath)
	}
	if filepath.IsAbs(topoPath) {
		t.Errorf("file_topology path is absolute, want workspace-relative canonical: %q", topoPath)
	}

	// Collect all symbol_graph facts.
	var symbolGraphs []Fact
	// Also try core.Fact compatibility: facts are stored as world.Fact (alias to types.Fact)
	for _, f := range result.Facts {
		if f.Predicate == "symbol_graph" {
			symbolGraphs = append(symbolGraphs, f)
		}
	}
	if len(symbolGraphs) == 0 {
		t.Fatalf("no symbol_graph facts emitted for hello.go (parser did not run or file treated as test); result facts: %v", result.Facts)
	}

	for _, sg := range symbolGraphs {
		if len(sg.Args) < 4 {
			t.Errorf("symbol_graph fact has too few args (%d): %+v", len(sg.Args), sg)
			continue
		}
		definedAt, ok := sg.Args[3].(string)
		if !ok {
			t.Errorf("symbol_graph DefinedAt slot Args[3] is not a string: %+v", sg)
			continue
		}

		// Core assertion: must equal the file_topology path exactly — same
		// string so a Mangle join on File can succeed.
		if definedAt != topoPath {
			t.Errorf("symbol_graph DefinedAt %q != file_topology path %q (join would fail); fact: %+v", definedAt, topoPath, sg)
		}

		// Must not contain a backslash (Windows separator) — canonical is POSIX.
		if strings.Contains(definedAt, "\\") {
			t.Errorf("symbol_graph DefinedAt contains backslash: %q (fact %+v)", definedAt, sg)
		}

		// Must not begin with a drive letter (e.g. "C:") — absolute Windows path.
		if len(definedAt) >= 2 && definedAt[1] == ':' {
			t.Errorf("symbol_graph DefinedAt begins with drive letter: %q (fact %+v)", definedAt, sg)
		}
		// Must not be absolute — canonical is workspace-relative.
		if filepath.IsAbs(definedAt) {
			t.Errorf("symbol_graph DefinedAt is absolute, want workspace-relative canonical: %q (fact %+v)", definedAt, sg)
		}
	}
}
