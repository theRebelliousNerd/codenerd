package core

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The world-model rules were reshaped for cost (2026-11-12): dependent_count
// reuses intelligence_dependent_count instead of aggregating the same import
// join again, and doc_exists_for finds the directories from their markdown
// files instead of pairing every two files of a directory. These tests hold
// the production rules to the formulations they replaced, written here under
// other names, on a world with the edge cases each one has.
const replacedWorldRules = `
Decl old_dependent_count(File, N) bound [/string, /number].
old_dependent_count(File, N) :-
    dependency_link(Importer, File, _),
    file_topology(File, _, _, _, _)
    |> do fn:group_by(File), let N = fn:count().

Decl old_doc_exists_for(File) bound [/string].
old_doc_exists_for(File) :-
    file_dir(File, Dir),
    file_dir(Doc, Dir),
    file_extension(Doc, ".md").
`

func worldShapeKernel(t *testing.T) *RealKernel {
	t.Helper()
	k := setupMockKernel(t)
	k.AppendPolicy(replacedWorldRules)
	topo := func(path string) Fact {
		return Fact{Predicate: "file_topology", Args: []any{path, "h", MangleAtom("/go"), int64(1), MangleAtom("/false")}}
	}
	facts := []Fact{
		topo("a.go"), topo("b.go"), topo("c.go"), topo("pkg/d.go"),
		// Two kinds of edge between the same pair count once; an edge to a
		// file the world does not hold counts nowhere.
		{Predicate: "dependency_link", Args: []any{"a.go", "b.go", "import"}},
		{Predicate: "dependency_link", Args: []any{"a.go", "b.go", "call"}},
		{Predicate: "dependency_link", Args: []any{"c.go", "b.go", "import"}},
		{Predicate: "dependency_link", Args: []any{"b.go", "pkg/d.go", "import"}},
		{Predicate: "dependency_link", Args: []any{"a.go", "missing.go", "import"}},
		{Predicate: "dependency_link", Args: []any{"pkg/d.go", "a.go", "import"}},
	}
	// Directory "doc" holds a markdown file (which documents itself), "src"
	// holds none, "mixed" holds two markdown files.
	dirs := map[string][]string{
		"doc":   {"doc/x.go", "doc/README.md"},
		"src":   {"src/y.go", "src/z.go"},
		"mixed": {"mixed/w.go", "mixed/A.md", "mixed/B.md"},
	}
	for dir, files := range dirs {
		for _, f := range files {
			ext := ".go"
			if len(f) > 3 && f[len(f)-3:] == ".md" {
				ext = ".md"
			}
			facts = append(facts,
				Fact{Predicate: "file_dir", Args: []any{f, dir}},
				Fact{Predicate: "file_extension", Args: []any{f, ext}},
			)
		}
	}
	require.NoError(t, k.LoadFacts(facts))
	require.NoError(t, k.Evaluate())
	return k
}

func sortedRows(t *testing.T, k *RealKernel, pred string) []string {
	t.Helper()
	rows, err := k.Query(pred)
	require.NoError(t, err)
	out := make([]string, 0, len(rows))
	for _, f := range rows {
		out = append(out, strings.TrimSuffix(fmt.Sprintln(f.Args...), "\n"))
	}
	sort.Strings(out)
	return out
}

func TestWorldRuleShapes_DependentCountMatchesTheImportAggregate(t *testing.T) {
	k := worldShapeKernel(t)
	got := sortedRows(t, k, "dependent_count")
	require.Equal(t, sortedRows(t, k, "old_dependent_count"), got)
	require.Equal(t, []string{"a.go 1", "b.go 2", "pkg/d.go 1"}, got)
}

func TestWorldRuleShapes_DocExistsForMatchesTheDirectoryPairing(t *testing.T) {
	k := worldShapeKernel(t)
	got := sortedRows(t, k, "doc_exists_for")
	require.Equal(t, sortedRows(t, k, "old_doc_exists_for"), got)
	require.Equal(t, []string{"doc/README.md", "doc/x.go", "mixed/A.md", "mixed/B.md", "mixed/w.go"}, got)
}
