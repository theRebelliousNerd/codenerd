package world

import (
	"slices"
	"strconv"
	"testing"

	"codenerd/internal/core"
)

func edge(from, to string) Fact {
	return Fact{Predicate: "dependency_link", Args: []any{from, to, "imp"}}
}

// A cycle of three files is one component named by its smallest path; a chain
// into it, an edge out to an external token and a two-file chain are not
// cycles.
func TestImportCycleFacts_MeasuresStronglyConnectedComponents(t *testing.T) {
	got := ImportCycleFacts([]Fact{
		edge("b.py", "c.py"), edge("c.py", "a.py"), edge("a.py", "b.py"), // cycle
		edge("entry.py", "a.py"), // into the cycle, not on it
		edge("a.py", "pkg:os"),   // external token: no way back
		edge("x.py", "y.py"),     // a chain
		edge("z.py", "z.py"),     // a self-import is no cycle of files
		{Predicate: "file_imports", Args: []any{"y.py", "x.py"}}, // not an edge here
	})
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
	if err := ApplyIncrementalResult(kernel, &IncrementalResult{NewFacts: []Fact{edge("a.py", "b.py")}}); err != nil {
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
