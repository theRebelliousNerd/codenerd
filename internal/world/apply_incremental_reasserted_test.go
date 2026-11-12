package world

import (
	"fmt"
	"sort"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// A rescan re-emits most of a file's facts unchanged. Applying it must leave
// the kernel exactly as retract-all-then-load-all would, while touching only
// the facts that changed: the evaluation that follows is incremental, not the
// full fixpoint a whole-world retract and re-insert forced.
func TestApplyIncrementalResult_UnchangedFactsAreNotRetracted(t *testing.T) {
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := func(from, to string) core.Fact {
		return core.Fact{Predicate: "dependency_link", Args: []any{from, to, "import"}}
	}
	topo := func(path string) core.Fact {
		return core.Fact{Predicate: "file_topology", Args: []any{path, "h", types.MangleAtom("/go"), int64(1), types.MangleAtom("/false")}}
	}
	var old []core.Fact
	for i := 0; i < 2000; i++ {
		old = append(old, topo(fmt.Sprintf("f%d.go", i)), link(fmt.Sprintf("f%d.go", i), "core.go"))
	}
	old = append(old, topo("core.go"), link("f0.go", "gone.go"))
	if err := kernel.LoadFacts(old); err != nil {
		t.Fatal(err)
	}
	if err := kernel.Evaluate(); err != nil {
		t.Fatal(err)
	}

	// Every importer was rescanned: all its facts are retracted and re-emitted,
	// plus one new edge each to the package's new file; one edge disappeared.
	res := &IncrementalResult{RetractFacts: old[:len(old)-1:len(old)-1]}
	res.RetractFacts = append(res.RetractFacts, link("f0.go", "gone.go"))
	for _, f := range old[:len(old)-1] {
		res.NewFacts = append(res.NewFacts, f)
	}
	res.NewFacts = append(res.NewFacts, topo("new.go"))
	for i := 0; i < 2000; i++ {
		res.NewFacts = append(res.NewFacts, link(fmt.Sprintf("f%d.go", i), "new.go"))
	}
	if err := ApplyIncrementalResult(kernel, res); err != nil {
		t.Fatal(err)
	}

	got, err := kernel.Query("dependency_link")
	if err != nil {
		t.Fatal(err)
	}
	if ev := kernel.LastEvaluation(); ev.Mode == "full" {
		t.Fatalf("applying a rescan of unchanged facts ran a full evaluation (fallback %q)", ev.ConeFallback)
	}
	var edges []string
	for _, f := range got {
		edges = append(edges, fmt.Sprint(f.Args[0], "->", f.Args[1]))
	}
	sort.Strings(edges)
	if len(edges) != 4000 {
		t.Fatalf("kernel holds %d dependency_link facts, want 4000 (2000 to core.go, 2000 to new.go)", len(edges))
	}
	for _, e := range edges {
		if e == "f0.go->gone.go" {
			t.Fatal("the edge the rescan no longer emits was not retracted")
		}
	}
	counts, err := kernel.Query("dependent_count")
	if err != nil {
		t.Fatal(err)
	}
	byFile := map[string]any{}
	for _, f := range counts {
		byFile[fmt.Sprint(f.Args[0])] = f.Args[1]
	}
	if fmt.Sprint(byFile["core.go"]) != "2000" || fmt.Sprint(byFile["new.go"]) != "2000" {
		t.Fatalf("dependent_count = %v, want 2000 importers each for core.go and new.go", byFile)
	}
}

func TestNotReasserted(t *testing.T) {
	a := core.Fact{Predicate: "p", Args: []any{"a", int64(1)}}
	a2 := core.Fact{Predicate: "p", Args: []any{"a", int64(1)}}
	b := core.Fact{Predicate: "p", Args: []any{"b", int64(1)}}
	atom := core.Fact{Predicate: "p", Args: []any{types.MangleAtom("a"), int64(1)}}
	got := notReasserted([]core.Fact{a, b, atom}, []core.Fact{a2})
	if len(got) != 2 || got[0].Args[0] != "b" || got[1].Args[0] != types.MangleAtom("a") {
		t.Fatalf("notReasserted = %v, want b and the /name-typed fact (a string and a name are different facts)", got)
	}
	if got := notReasserted([]core.Fact{a, a}, []core.Fact{a2}); len(got) != 1 {
		t.Fatalf("a fact retracted twice and asserted once must still be retracted once, got %v", got)
	}
}
