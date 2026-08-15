package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/store"
)

func writeWorkspaceFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

func dependencyEdges(facts []Fact) map[string][]string {
	out := make(map[string][]string)
	for _, f := range facts {
		if f.Predicate != "dependency_link" || len(f.Args) < 2 {
			continue
		}
		from, _ := f.Args[0].(string)
		to, _ := f.Args[1].(string)
		out[from] = append(out[from], to)
	}
	return out
}

func hasEdge(edges map[string][]string, from, to string) bool {
	for _, t := range edges[from] {
		if t == to {
			return true
		}
	}
	return false
}

// TestDependencyLink_WhenGoImportIsInRepo_ShouldEmitFileToFileEdge — the raw
// import fact names a synthetic "pkg:..." token, which no rule can join against
// modified()/pending_edit(); every impact and activation rule keyed on
// dependency_link was therefore dormant.
func TestDependencyLink_WhenGoImportIsInRepo_ShouldEmitFileToFileEdge(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "internal/core/core.go", "package core\n\nfunc Do() {}\n")
	writeWorkspaceFile(t, root, "internal/core/helper.go", "package core\n\nfunc Help() {}\n")
	writeWorkspaceFile(t, root, "internal/core/core_test.go", "package core\n\nfunc TestDo() {}\n")
	writeWorkspaceFile(t, root, "cmd/app/main.go", "package main\n\nimport (\n\t\"fmt\"\n\t\"example.com/app/internal/core\"\n)\n\nfunc main() { fmt.Println(core.Do) }\n")

	facts, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	edges := dependencyEdges(facts)

	if !hasEdge(edges, "cmd/app/main.go", "internal/core/core.go") {
		t.Errorf("missing in-repo import edge cmd/app/main.go -> internal/core/core.go; edges=%v", edges)
	}
	if !hasEdge(edges, "cmd/app/main.go", "internal/core/helper.go") {
		t.Errorf("package import must reach every file of the imported package; edges=%v", edges)
	}
	if hasEdge(edges, "cmd/app/main.go", "internal/core/core_test.go") {
		t.Errorf("test files must not be import targets; edges=%v", edges)
	}
	// Standard library imports stay unresolved (token form), never invented as
	// workspace files.
	for _, to := range edges["cmd/app/main.go"] {
		if to == "fmt" || strings.HasSuffix(to, "/fmt") {
			t.Errorf("stdlib import resolved to a workspace file: %q", to)
		}
	}
}

// TestDependencyLink_WhenRelativeImport_ShouldResolveExactFile covers the
// languages whose imports name one file rather than a package.
func TestDependencyLink_WhenRelativeImport_ShouldResolveExactFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "web/app.ts", "import { thing } from \"./lib/util\";\nexport function run() { return thing(); }\n")
	writeWorkspaceFile(t, root, "web/lib/util.ts", "export function thing() { return 1; }\n")
	writeWorkspaceFile(t, root, "svc/main.py", "import svc.helpers\n\ndef run():\n    return svc.helpers.go()\n")
	writeWorkspaceFile(t, root, "svc/helpers.py", "def go():\n    return 2\n")

	facts, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	edges := dependencyEdges(facts)

	if !hasEdge(edges, "web/app.ts", "web/lib/util.ts") {
		t.Errorf("relative TS import did not resolve; edges=%v", edges)
	}
	if !hasEdge(edges, "svc/main.py", "svc/helpers.py") {
		t.Errorf("python module import did not resolve; edges=%v", edges)
	}
}

// TestDependencyLink_WhenExternalImport_ShouldNotFabricateFile — resolution must
// never invent a workspace file for a third-party dependency, or impact
// analysis starts blocking writes on files that do not exist.
func TestDependencyLink_WhenExternalImport_ShouldNotFabricateFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "a/a.go", "package a\n\nimport \"github.com/other/dep\"\n\nfunc F() { _ = dep.X }\n")

	facts, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range dependencyEdges(facts)["a/a.go"] {
		if !strings.Contains(to, ":") {
			t.Errorf("external import produced a file-shaped edge target %q", to)
		}
	}
}

// TestDependencyLink_WhenIncremental_ShouldMatchFullScanEdges — a delta scan
// resolves against the whole workspace, not just the changed files, so an edge
// into an untouched package still lands.
func TestDependencyLink_WhenIncremental_ShouldMatchFullScanEdges(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "internal/core/core.go", "package core\n\nfunc Do() {}\n")
	main := writeWorkspaceFile(t, root, "cmd/app/main.go", "package main\n\nimport \"example.com/app/internal/core\"\n\nfunc main() { core.Do() }\n")

	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte("package main\n\nimport \"example.com/app/internal/core\"\n\nfunc main() { core.Do(); core.Do() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	delta, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if delta.Full {
		t.Fatal("expected delta scan")
	}
	if !hasEdge(dependencyEdges(delta.NewFacts), "cmd/app/main.go", "internal/core/core.go") {
		t.Errorf("incremental scan lost the in-repo import edge; edges=%v", dependencyEdges(delta.NewFacts))
	}
}

// TestDependencyLink_WhenImportRemoved_ShouldRetractTheResolvedEdge — resolved
// edges have to live in the same per-file fact set as the raw import fact, or
// the store never learns about them and a deleted import keeps its edge in the
// kernel forever, permanently over-reporting impact.
func TestDependencyLink_WhenImportRemoved_ShouldRetractTheResolvedEdge(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer db.Close()

	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "internal/core/core.go", "package core\n\nfunc Do() {}\n")
	main := writeWorkspaceFile(t, root, "cmd/app/main.go", "package main\n\nimport \"example.com/app/internal/core\"\n\nfunc main() { core.Do() }\n")

	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}

	// Drop the import entirely.
	if err := os.WriteFile(main, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	delta, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(dependencyEdges(delta.RetractFacts), "cmd/app/main.go", "internal/core/core.go") {
		t.Errorf("the resolved edge was not retracted when its import was deleted; retractions=%v", dependencyEdges(delta.RetractFacts))
	}
	if hasEdge(dependencyEdges(delta.NewFacts), "cmd/app/main.go", "internal/core/core.go") {
		t.Errorf("the edge was re-asserted although the import is gone")
	}
}

// TestDependencyLink_WhenResolved_ShouldBeDeduplicatedAndSelfFree — a file that
// imports its own package (or the same package twice) must not produce a
// self-edge or duplicates; both make impact cascades cycle on themselves.
func TestDependencyLink_WhenResolved_ShouldBeDeduplicatedAndSelfFree(t *testing.T) {
	idx := &repoFileIndex{
		goFilesByDir: map[string][]string{"pkg": {"pkg/a.go", "pkg/b.go"}},
		all:          map[string]struct{}{"pkg/a.go": {}, "pkg/b.go": {}},
		goModule:     "example.com/app",
	}
	raw := []Fact{
		{Predicate: "dependency_link", Args: []any{"pkg/a.go", "pkg:example.com/app/pkg", "example.com/app/pkg"}},
		{Predicate: "dependency_link", Args: []any{"pkg/a.go", "pkg:example.com/app/pkg", "example.com/app/pkg"}},
	}
	got := resolveDependencyLinksWithIndex(idx, raw)
	if len(got) != 1 {
		t.Fatalf("expected exactly one resolved edge (dedup, no self-edge), got %d: %v", len(got), got)
	}
	if to, _ := got[0].Args[1].(string); to != "pkg/b.go" {
		t.Errorf("resolved edge target = %q, want pkg/b.go", to)
	}
}
