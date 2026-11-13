package world

import (
	"context"
	"fmt"
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

// dependencyEdges is "from depends on the file to" over a set of facts: the
// join import_unit(from, U), file_unit(to, U) the rules write (schemas_world.mg).
// Imports that leave the workspace appear in their token form, as
// dependency_link records them.
func dependencyEdges(facts []Fact) map[string][]string {
	members := make(map[string][]string)
	for _, f := range facts {
		if f.Predicate == "file_unit" && len(f.Args) >= 2 {
			file, _ := f.Args[0].(string)
			unit, _ := f.Args[1].(string)
			members[unit] = append(members[unit], file)
		}
	}
	out := make(map[string][]string)
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		from, _ := f.Args[0].(string)
		switch f.Predicate {
		case "import_unit":
			unit, _ := f.Args[1].(string)
			for _, to := range members[unit] {
				if to != from {
					out[from] = append(out[from], to)
				}
			}
		case "dependency_link":
			to, _ := f.Args[1].(string)
			out[from] = append(out[from], to)
		}
	}
	return out
}

// importsUnit reports whether facts hold import_unit(from, unit).
func importsUnit(facts []Fact, from, unit string) bool {
	for _, f := range facts {
		if f.Predicate == "import_unit" && len(f.Args) >= 2 && f.Args[0] == from && f.Args[1] == unit {
			return true
		}
	}
	return false
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
	// The delta carries the changed file's own rows: the unit it imports. The
	// untouched package's file_unit rows are already in the kernel.
	if !importsUnit(delta.NewFacts, "cmd/app/main.go", "example.com/app/internal/core") {
		t.Errorf("incremental scan lost the in-repo import; facts=%v", delta.NewFacts)
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
	if !importsUnit(delta.RetractFacts, "cmd/app/main.go", "example.com/app/internal/core") {
		t.Errorf("the resolved import was not retracted when its import was deleted; retractions=%v", delta.RetractFacts)
	}
	if importsUnit(delta.NewFacts, "cmd/app/main.go", "example.com/app/internal/core") {
		t.Errorf("the import was re-asserted although it is gone")
	}
}

// TestDependencyLink_WhenResolved_ShouldBeDeduplicatedAndSelfFree — the same
// package imported twice is one row, and a file that names its own package
// imports nothing: joined with its own file_unit row it would depend on
// itself, and impact cascades would cycle.
func TestDependencyLink_WhenResolved_ShouldBeDeduplicatedAndSelfFree(t *testing.T) {
	idx := &repoFileIndex{
		goFilesByDir: map[string][]string{"pkg": {"pkg/a.go", "pkg/b.go"}, "other": {"other/o.go"}},
		all:          map[string]struct{}{"pkg/a.go": {}, "pkg/b.go": {}, "other/o.go": {}},
		goModule:     "example.com/app",
	}
	raw := []Fact{
		{Predicate: "dependency_link", Args: []any{"pkg/a.go", "pkg:example.com/app/pkg", "example.com/app/pkg"}},
		{Predicate: "dependency_link", Args: []any{"pkg/a.go", "pkg:example.com/app/pkg", "example.com/app/pkg"}},
	}
	if got := resolveDependencyLinksWithIndex(idx, raw); len(got) != 0 {
		t.Fatalf("a file naming its own package resolved to %v, want nothing", got)
	}
	raw = []Fact{
		{Predicate: "dependency_link", Args: []any{"other/o.go", "pkg:example.com/app/pkg", "example.com/app/pkg"}},
		{Predicate: "dependency_link", Args: []any{"other/o.go", "pkg:example.com/app/pkg", "example.com/app/pkg"}},
	}
	got := resolveDependencyLinksWithIndex(idx, raw)
	if len(got) != 2 {
		t.Fatalf("expected one import_unit row and one imports_project_file for a package imported twice, got %d: %v", len(got), got)
	}
	if !importsUnit(got, "other/o.go", "example.com/app/pkg") {
		t.Fatalf("missing import_unit(other/o.go, example.com/app/pkg): %v", got)
	}
	for _, f := range got {
		if f.Predicate == "dependency_link" || f.Predicate == "file_imports" {
			t.Fatalf("the resolver emitted a product row: %s", f.String())
		}
	}
}

// The product of an importer and a large package was one row per file, and an
// early cap of 50,000 dropped the sorted tail. In factors the import is one
// row whatever the package's size, and the last file is reached through its
// own file_unit row.
func TestResolveDependencyLinks_PastFormerCap_KeepsEveryEdge(t *testing.T) {
	const n = 50001
	files := make([]string, n)
	for i := range files {
		files[i] = fmt.Sprintf("pkg/f%05d.go", i)
	}
	idx := &repoFileIndex{
		goFilesByDir: map[string][]string{"pkg": files},
		all:          map[string]struct{}{},
		goModule:     "example.com/app",
	}
	raw := []Fact{{
		Predicate: "dependency_link",
		Args:      []any{"cmd/main.go", "pkg:example.com/app/pkg", "example.com/app/pkg"},
	}}
	got := resolveDependencyLinksWithIndex(idx, raw)
	if len(got) != 2 || !importsUnit(got, "cmd/main.go", "example.com/app/pkg") {
		t.Fatalf("resolved facts = %v, want one import_unit row and one imports_project_file", got)
	}
	last, ok := fileUnitFact(idx, files[n-1])
	if !ok || last.Args[1] != "example.com/app/pkg" {
		t.Fatalf("the sorted tail %s is not a member of the imported unit: %v", files[n-1], last)
	}
	all := append(got, last)
	if !hasEdge(dependencyEdges(all), "cmd/main.go", files[n-1]) {
		t.Fatalf("the importer does not reach the sorted tail %s", files[n-1])
	}
}
