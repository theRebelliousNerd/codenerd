package world

import (
	"slices"
	"strconv"
	"testing"

	"codenerd/internal/core"
)

// edge is "from imports the file to" for a language whose import names one
// file: the file is its own unit.
func edge(from, to string) Fact {
	return Fact{Predicate: "import_unit", Args: []any{from, to}}
}

func member(file, unit string) Fact {
	return Fact{Predicate: "file_unit", Args: []any{file, unit}}
}

// withOwnUnits adds, for every file an edge names, the row that makes it its
// own unit.
func withOwnUnits(facts ...Fact) []Fact {
	seen := map[string]bool{}
	out := append([]Fact(nil), facts...)
	for _, f := range facts {
		if f.Predicate != "import_unit" {
			continue
		}
		for _, a := range f.Args {
			if file := a.(string); !seen[file] {
				seen[file] = true
				out = append(out, member(file, file))
			}
		}
	}
	return out
}

// A cycle of three files is one component named by its smallest path; a chain
// into it, an edge out to an external token and a two-file chain are not
// cycles.
func TestImportCycleFacts_MeasuresStronglyConnectedComponents(t *testing.T) {
	got := ImportCycleFacts(append(withOwnUnits(
		edge("b.py", "c.py"), edge("c.py", "a.py"), edge("a.py", "b.py"), // cycle
		edge("entry.py", "a.py"), // into the cycle, not on it
		edge("x.py", "y.py"),     // a chain
		edge("z.py", "z.py"),     // a self-import is no cycle of files
	),
		// An import that leaves the workspace is only its token: no unit, no way back.
		Fact{Predicate: "dependency_link", Args: []any{"a.py", "mod:os", "os"}},
		Fact{Predicate: "dependency_link", Args: []any{"y.py", "mod:x", "x"}},
	))
	var rows []string
	for _, f := range got {
		if f.Predicate != "import_cycle_member" {
			t.Fatalf("unexpected fact %v", f)
		}
		rows = append(rows, f.Args[0].(string)+" "+f.Args[1].(string))
	}
	slices.Sort(rows)
	want := []string{"a.py a.py", "b.py a.py", "c.py a.py"}
	if !slices.Equal(rows, want) {
		t.Fatalf("import_cycle_member = %v, want %v", rows, want)
	}
}

// Two disjoint cycles get their own roots, and a long chain does not exhaust
// the (iterative) traversal.
func TestImportCycleFacts_DisjointCyclesAndDeepChains(t *testing.T) {
	facts := []Fact{edge("m.go", "n.go"), edge("n.go", "m.go"), edge("q.go", "p.go"), edge("p.go", "q.go")}
	for i := 0; i < 200000; i++ {
		facts = append(facts, edge(chainFile(i), chainFile(i+1)))
	}
	facts = withOwnUnits(facts...)
	roots := map[string]string{}
	for _, f := range ImportCycleFacts(facts) {
		roots[f.Args[0].(string)] = f.Args[1].(string)
	}
	if len(roots) != 4 || roots["n.go"] != "m.go" || roots["q.go"] != "p.go" {
		t.Fatalf("roots = %v, want m.go/n.go -> m.go and p.go/q.go -> p.go", roots)
	}
}

func chainFile(i int) string {
	return "chain/" + strconv.Itoa(i) + ".go"
}

// A delta scan that closes a cycle puts its members in the kernel; one that
// breaks it takes them out. The rows follow the kernel's edges, not the
// delta's.
func TestApplyIncrementalResult_RemeasuresImportCycles(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyIncrementalResult(kernel, &IncrementalResult{NewFacts: withOwnUnits(edge("a.py", "b.py"))}); err != nil {
		t.Fatal(err)
	}
	members := func() int {
		rows, err := kernel.Query("import_cycle_member")
		if err != nil {
			t.Fatal(err)
		}
		return len(rows)
	}
	if n := members(); n != 0 {
		t.Fatalf("a single edge measured %d cycle members", n)
	}
	if err := ApplyIncrementalResult(kernel, &IncrementalResult{NewFacts: []Fact{edge("b.py", "a.py")}}); err != nil {
		t.Fatal(err)
	}
	if n := members(); n != 2 {
		t.Fatalf("closing the cycle measured %d members, want 2", n)
	}
	if err := ApplyIncrementalResult(kernel, &IncrementalResult{RetractFacts: []Fact{edge("b.py", "a.py")}}); err != nil {
		t.Fatal(err)
	}
	if n := members(); n != 0 {
		t.Fatalf("breaking the cycle left %d members", n)
	}
}

// Two Go packages that import each other are a cycle of the files that hold
// the imports. A file of one of them that imports nothing is reached by the
// cycle but is not on it, exactly as when every file-to-file edge was stored.
func TestImportCycleFacts_GoPackagesThatImportEachOther(t *testing.T) {
	got := ImportCycleFacts([]Fact{
		member("a/a1.go", "m/a"), member("a/a2.go", "m/a"), member("b/b1.go", "m/b"), member("c/c1.go", "m/c"),
		edge("a/a1.go", "m/b"), // a imports b
		edge("b/b1.go", "m/a"), // b imports a: the cycle
		edge("c/c1.go", "m/a"), // into the cycle, not on it
	})
	var rows []string
	for _, f := range got {
		rows = append(rows, f.Args[0].(string)+" "+f.Args[1].(string))
	}
	slices.Sort(rows)
	want := []string{"a/a1.go a/a1.go", "b/b1.go a/a1.go"}
	if !slices.Equal(rows, want) {
		t.Fatalf("import_cycle_member = %v, want %v", rows, want)
	}
}
