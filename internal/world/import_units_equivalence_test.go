package world

import (
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// productEdges is the import graph as it used to be stored: one edge from each
// importer to every workspace file its import resolves to (every non-test file
// of a Go package; the one file of a Python, JS/TS or Rust import), never to
// itself. It is the oracle for the factored form.
func productEdges(idx *repoFileIndex, facts []core.Fact) []string {
	seen := make(map[string]struct{})
	for _, f := range facts {
		if f.Predicate != "dependency_link" || len(f.Args) < 3 {
			continue
		}
		from, _ := f.Args[0].(string)
		token, _ := f.Args[1].(string)
		imp, _ := f.Args[2].(string)
		for _, target := range idx.resolve(from, token, imp) {
			if target != from {
				seen[from+" -> "+target] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// joinedEdges is "X depends on the file Y" as the rules now derive it:
// import_unit(X, U), file_unit(Y, U).
func joinedEdges(rows []core.Fact) []string {
	members := make(map[string][]string)
	for _, f := range rows {
		if f.Predicate == "file_unit" {
			members[f.Args[1].(string)] = append(members[f.Args[1].(string)], f.Args[0].(string))
		}
	}
	seen := make(map[string]struct{})
	for _, f := range rows {
		if f.Predicate != "import_unit" {
			continue
		}
		from := f.Args[0].(string)
		for _, target := range members[f.Args[1].(string)] {
			if target != from {
				seen[from+" -> "+target] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

func rawImport(from, token, imp string) core.Fact {
	return core.Fact{Predicate: "dependency_link", Args: []any{from, token, imp}}
}

// The factors carry exactly the edges the product held, for every language the
// resolver reads, and in far fewer rows.
func TestImportUnits_JoinEqualsTheStoredProduct(t *testing.T) {
	files := []string{
		"go.mod",
		"cmd/app/main.go", "cmd/app/flags.go", "cmd/app/main_test.go",
		"internal/core/kernel.go", "internal/core/query.go", "internal/core/eval.go", "internal/core/kernel_test.go",
		"internal/store/store.go", "internal/store/store_test.go",
		"internal/leaf/leaf.go",
		"internal/empty/only_test.go",
		"tools/gen.py", "tools/lib/__init__.py", "tools/lib/util.py",
		"web/app.ts", "web/widget.tsx", "web/lib/index.ts",
		"native/src/main.rs", "native/src/parse.rs",
		"Docs/guide.md",
	}
	idx := newRepoFileIndex(t.TempDir(), files)
	idx.goModule = "example.test/m"
	var facts []core.Fact
	for _, f := range files {
		facts = append(facts, core.Fact{Predicate: "file_topology", Args: []any{f, "h", "/x", int64(1), false}})
	}
	facts = append(facts,
		rawImport("cmd/app/main.go", "pkg:example.test/m/internal/core", "example.test/m/internal/core"),
		rawImport("cmd/app/main.go", "pkg:example.test/m/internal/store", "example.test/m/internal/store"),
		rawImport("cmd/app/main.go", "pkg:fmt", "fmt"),
		rawImport("cmd/app/flags.go", "pkg:example.test/m/internal/core", "example.test/m/internal/core"),
		rawImport("cmd/app/main_test.go", "pkg:example.test/m/cmd/app", "example.test/m/cmd/app"),
		rawImport("internal/core/kernel.go", "pkg:example.test/m/internal/store", "example.test/m/internal/store"),
		rawImport("internal/core/kernel_test.go", "pkg:example.test/m/internal/core", "example.test/m/internal/core"),
		rawImport("internal/store/store.go", "pkg:example.test/m/internal/leaf", "example.test/m/internal/leaf"),
		rawImport("internal/store/store.go", "pkg:example.test/m/internal/empty", "example.test/m/internal/empty"),
		rawImport("internal/store/store.go", "pkg:github.com/else/where", "github.com/else/where"),
		rawImport("tools/gen.py", "mod:tools.lib.util", "tools.lib.util"),
		rawImport("tools/gen.py", "mod:tools.lib", "tools.lib"),
		rawImport("tools/gen.py", "mod:os", "os"),
		rawImport("web/app.ts", "mod:./widget", "./widget"),
		rawImport("web/app.ts", "mod:./lib", "./lib"),
		rawImport("web/app.ts", "mod:react", "react"),
		rawImport("native/src/main.rs", "crate:crate::parse::Thing", "crate::parse::Thing"),
	)

	want := productEdges(idx, facts)
	rows := append(resolveDependencyLinksWithIndex(idx, facts), fileUnitFacts(idx, facts)...)
	got := joinedEdges(rows)
	if len(want) < 12 {
		t.Fatalf("the fixture resolves only %d edges: %v", len(want), want)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the join of the factors differs from the product\n got %v\nwant %v", got, want)
	}

	counts := make(map[string]int)
	for _, f := range rows {
		counts[f.Predicate]++
		if f.Predicate == "dependency_link" || f.Predicate == "file_imports" {
			t.Fatalf("the resolver still emits a product row: %s", f.String())
		}
	}
	// A test file, a document and a package of only test files are no one's
	// unit; every other source file is exactly one.
	if counts["file_unit"] != 15 {
		t.Fatalf("file_unit rows = %d, want one per importable source file (15): %v", counts["file_unit"], rows)
	}
	if counts["imports_project_file"] != 9 {
		t.Fatalf("imports_project_file rows = %d, want one per file with an in-workspace import (9)", counts["imports_project_file"])
	}
}

// The rows scale with the factors. A package of many files imported by many
// files is members plus importers, not their product.
func TestImportUnits_RowsAreTheFactorsNotTheProduct(t *testing.T) {
	var files []string
	var facts []core.Fact
	const members, importers = 300, 600
	for i := range members {
		files = append(files, "internal/core/f"+itoa(i)+".go")
	}
	for i := range importers {
		f := "pkg/p" + itoa(i) + "/x.go"
		files = append(files, f)
		facts = append(facts, rawImport(f, "pkg:example.test/m/internal/core", "example.test/m/internal/core"))
	}
	idx := newRepoFileIndex(t.TempDir(), files)
	idx.goModule = "example.test/m"
	for _, f := range files {
		facts = append(facts, core.Fact{Predicate: "file_topology", Args: []any{f, "h", "/go", int64(1), false}})
	}
	rows := append(resolveDependencyLinksWithIndex(idx, facts), fileUnitFacts(idx, facts)...)
	if product := len(productEdges(idx, facts)); product != members*importers {
		t.Fatalf("the product has %d edges, want %d", product, members*importers)
	}
	if len(joinedEdges(rows)) != members*importers {
		t.Fatal("the factors lost edges")
	}
	// import_unit per importer, imports_project_file per importer, file_unit per file.
	if want := importers + importers + members + importers; len(rows) != want {
		t.Fatalf("%d rows for %d members and %d importers, want %d (the product is %d)", len(rows), members, importers, want, members*importers)
	}
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{digits[i%10]}, b...)
	}
	return string(b)
}
