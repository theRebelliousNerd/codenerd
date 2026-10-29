package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

// Producer pins for the test-impact file facts (test_impact_facts.go): the
// fast scanner asserts file_package, is_test_file and file_imports for Go
// files, spelled exactly as test_impact.mg joins them. Same-directory
// grouping joins the file_dir companion (pinned in fs_test.go), not a
// materialised pair relation. The chain tests prove the rules fire; these
// prove the facts exist.

func factsByPredicate(facts []Fact) map[string][]Fact {
	out := make(map[string][]Fact)
	for _, f := range facts {
		out[f.Predicate] = append(out[f.Predicate], f)
	}
	return out
}

func hasFact(facts []Fact, pred string, args ...string) bool {
	for _, f := range facts {
		if f.Predicate != pred || len(f.Args) != len(args) {
			continue
		}
		match := true
		for i, want := range args {
			s, ok := f.Args[i].(string)
			if !ok || s != want {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestScanner_WhenGoWorkspace_ShouldEmitTestImpactFileFacts is the producer
// pin for all three predicates on one workspace: package rows for every Go
// file (test files included), the test-file mark, and the resolved import
// edge as file_imports.
func TestScanner_WhenGoWorkspace_ShouldEmitTestImpactFileFacts(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "p/app.go", "package p\n\nfunc Target() int { return 1 }\n")
	writeWorkspaceFile(t, root, "p/app_test.go", "package p\n\nimport \"testing\"\n\nfunc TestTarget(t *testing.T) {}\n")
	writeWorkspaceFile(t, root, "q/lib.go", "package q\n\nfunc Q() {}\n")
	writeWorkspaceFile(t, root, "cmd/main.go", "package main\n\nimport \"example.com/app/q\"\n\nfunc main() { q.Q() }\n")

	facts, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	byPred := factsByPredicate(facts)

	for _, tc := range [][2]string{
		{"p/app.go", "p"}, {"p/app_test.go", "p"}, {"q/lib.go", "q"}, {"cmd/main.go", "main"},
	} {
		if !hasFact(byPred["file_package"], "file_package", tc[0], tc[1]) {
			t.Errorf("missing file_package(%s, %s); got %v", tc[0], tc[1], byPred["file_package"])
		}
	}
	if len(byPred["file_package"]) != 4 {
		t.Errorf("file_package has %d rows, want 4: %v", len(byPred["file_package"]), byPred["file_package"])
	}

	if len(byPred["is_test_file"]) != 1 || !hasFact(byPred["is_test_file"], "is_test_file", "p/app_test.go") {
		t.Errorf("is_test_file should mark exactly p/app_test.go, got %v", byPred["is_test_file"])
	}

	if !hasFact(byPred["file_imports"], "file_imports", "cmd/main.go", "q/lib.go") {
		t.Errorf("missing file_imports(cmd/main.go, q/lib.go); got %v", byPred["file_imports"])
	}
	for _, f := range byPred["file_imports"] {
		from, _ := f.Args[0].(string)
		to, _ := f.Args[1].(string)
		if from == to {
			t.Errorf("file_imports self-edge %v", f.Args)
		}
		if from == "p/app_test.go" {
			t.Errorf("stdlib-only test file gained a file_imports edge %v", f.Args)
		}
	}
}

// TestScanner_WhenExternalTestPackage_ShouldEmitTestFileHeaderFacts pins the
// header-only parse for an external test file (package p_test): the clause
// truth in file_package, the is_test_file mark, and the shared file_dir the
// same-directory joins group on. A package-name grouping would strand it.
func TestScanner_WhenExternalTestPackage_ShouldEmitTestFileHeaderFacts(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "p/app.go", "package p\n\nfunc Target() int { return 1 }\n")
	writeWorkspaceFile(t, root, "p/app_test.go", "package p_test\n\nimport \"testing\"\n\nfunc TestTarget(t *testing.T) {}\n")

	facts, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	byPred := factsByPredicate(facts)
	// file_package still reports the clause truth, not the directory's.
	if !hasFact(byPred["file_package"], "file_package", "p/app_test.go", "p_test") {
		t.Errorf("file_package should report the p_test clause, got %v", byPred["file_package"])
	}
	if !hasFact(byPred["is_test_file"], "is_test_file", "p/app_test.go") {
		t.Errorf("external test file lost its is_test_file mark, got %v", byPred["is_test_file"])
	}
	if !hasFact(byPred["file_dir"], "file_dir", "p/app_test.go", "p") ||
		!hasFact(byPred["file_dir"], "file_dir", "p/app.go", "p") {
		t.Errorf("external test file shares no file_dir with its sources; got %v", byPred["file_dir"])
	}
}

// TestScanner_WhenIncremental_ShouldMatchFullScanFileFacts proves the delta
// path emits the same file facts for the files it touches: a changed test
// file's rows match the full scan exactly, and untouched files re-emit
// nothing.
func TestScanner_WhenIncremental_ShouldMatchFullScanFileFacts(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "p/app.go", "package p\n\nfunc Target() int { return 1 }\n")
	testFile := writeWorkspaceFile(t, root, "p/app_test.go", "package p\n\nimport \"testing\"\n\nfunc TestTarget(t *testing.T) {}\n")

	scanner := NewScanner()
	ctx := context.Background()
	full, err := scanner.ScanWorkspaceCtx(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	want := factsByPredicate(full)

	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, []byte("package p\n\nimport \"testing\"\n\n// touched\nfunc TestTarget(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	delta, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if delta.Full {
		t.Fatal("expected a delta scan")
	}
	got := factsByPredicate(delta.NewFacts)
	for _, pred := range []string{"file_package", "is_test_file"} {
		for _, f := range want[pred] {
			first, _ := f.Args[0].(string)
			if first != "p/app_test.go" {
				continue
			}
			args := make([]string, len(f.Args))
			for i, a := range f.Args {
				args[i], _ = a.(string)
			}
			if !hasFact(got[pred], pred, args...) {
				t.Errorf("delta lost %s%v from the changed test file", pred, args)
			}
		}
	}
}

// TestFileImports_WhenImportRemoved_ShouldRetract mirrors the dependency_link
// retraction pin: file_imports rows live in the importer's per-file fact set,
// so deleting the import retracts the edge instead of leaving it to accuse
// the next edit forever.
func TestFileImports_WhenImportRemoved_ShouldRetract(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer db.Close()

	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "q/lib.go", "package q\n\nfunc Q() {}\n")
	main := writeWorkspaceFile(t, root, "cmd/main.go", "package main\n\nimport \"example.com/app/q\"\n\nfunc main() { q.Q() }\n")

	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	delta, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	retracted := factsByPredicate(delta.RetractFacts)["file_imports"]
	if !hasFact(retracted, "file_imports", "cmd/main.go", "q/lib.go") {
		t.Errorf("file_imports edge not retracted when its import was deleted; retractions=%v", retracted)
	}
	if fresh := factsByPredicate(delta.NewFacts)["file_imports"]; len(fresh) != 0 {
		t.Errorf("file_imports re-asserted although the import is gone: %v", fresh)
	}
}

// TestParseGoHeader unit-pins the header parse the fast scans use for test
// files: clause name plus every import path, grouped or single. ImportsOnly
// ignores a broken function body, and a broken header returns an error that
// callers drop rather than failing the scan.
func TestParseGoHeader(t *testing.T) {
	for _, tc := range []struct {
		name    string
		src     string
		pkg     string
		imports []string
		wantErr bool
	}{
		{"grouped", "package p\n\nimport (\n\t\"testing\"\n\t\"example.com/app/q\"\n)\n", "p",
			[]string{"testing", "example.com/app/q"}, false},
		{"single", "package main\n\nimport \"fmt\"\n", "main", []string{"fmt"}, false},
		{"external test package", "package p_test\n\nimport \"testing\"\n", "p_test", []string{"testing"}, false},
		{"no imports", "package q\n\nfunc Q() {}\n", "q", nil, false},
		// ImportsOnly stops at the first declaration, so a broken function
		// body is not an error and the package clause still comes back.
		{"broken body", "package p\n\nfunc (\n", "p", nil, false},
		// An unclosed import is a header error. Callers drop the file.
		{"broken header", "package p\n\nimport (\n", "", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg, imports, err := parseGoHeader([]byte(tc.src))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected a parse error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGoHeader: %v", err)
			}
			if pkg != tc.pkg {
				t.Errorf("pkg = %q, want %q", pkg, tc.pkg)
			}
			if len(imports) != len(tc.imports) {
				t.Fatalf("imports = %v, want %v", imports, tc.imports)
			}
			for i := range imports {
				if imports[i] != tc.imports[i] {
					t.Errorf("imports[%d] = %q, want %q", i, imports[i], tc.imports[i])
				}
			}
		})
	}
}

// TestGroupFactsByPath_WhenTestImpactFacts_ShouldFileUnderArgZero pins the
// per-file ownership the incremental retraction depends on: every test-impact
// file fact names its producing file first, so the file_imports edge files
// under its importer and retracts with it.
func TestGroupFactsByPath_WhenTestImpactFacts_ShouldFileUnderArgZero(t *testing.T) {
	facts := []Fact{
		{Predicate: "file_topology", Args: []any{"p/a.go", "h1", "/go", int64(1), "/false"}},
		{Predicate: "file_topology", Args: []any{"p/b_test.go", "h2", "/go", int64(2), "/true"}},
		{Predicate: "file_package", Args: []any{"p/a.go", "p"}},
		{Predicate: "is_test_file", Args: []any{"p/b_test.go"}},
		{Predicate: "file_imports", Args: []any{"p/b_test.go", "p/a.go"}},
	}
	out := groupFactsByPath(facts)
	if got := len(out["p/a.go"]); got != 2 {
		t.Errorf("p/a.go owns %d facts, want 2 (topology, file_package): %v", got, out["p/a.go"])
	}
	if got := len(out["p/b_test.go"]); got != 3 {
		t.Errorf("p/b_test.go owns %d facts, want 3 (topology, is_test_file, file_imports): %v",
			got, out["p/b_test.go"])
	}
}
