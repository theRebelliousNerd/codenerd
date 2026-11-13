package core

import (
	"fmt"
	"testing"
)

func boundQueryKernel(tb testing.TB, edges int) *RealKernel {
	tb.Helper()
	k, err := NewRealKernel()
	if err != nil {
		tb.Fatal(err)
	}
	facts := make([]Fact, 0, edges)
	for i := range edges {
		facts = append(facts, Fact{Predicate: "dependency_link", Args: []any{
			fmt.Sprintf("pkg/f%05d.go", i%2000), fmt.Sprintf("pkg:mod/p%06d", i/3), fmt.Sprintf("mod/p%06d", i/3),
		}})
	}
	if err := k.LoadFacts(facts); err != nil {
		tb.Fatal(err)
	}
	return k
}

// A bound query returns exactly the rows a scan of the whole relation would
// keep, for a value with odd characters too.
func TestBoundQuery_MatchesTheScanItReplaces(t *testing.T) {
	k := boundQueryKernel(t, 6000)
	odd := `dir with "quotes"\and a backslash.go`
	if err := k.Assert(Fact{Predicate: "dependency_link", Args: []any{odd, "pkg:mod/p000001", "mod/p000001"}}); err != nil {
		t.Fatal(err)
	}
	all, err := k.Query("dependency_link")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		pos   int
		value string
	}{{0, "pkg/f00007.go"}, {1, "pkg:mod/p000001"}, {0, odd}, {0, "no/such/file.go"}} {
		want := 0
		for _, f := range all {
			if s, _ := f.Args[c.pos].(string); s == c.value {
				want++
			}
		}
		got, err := k.QueryBound("dependency_link", 3, map[int]any{c.pos: c.value})
		if err != nil {
			t.Fatalf("%q at %d: %v", c.value, c.pos, err)
		}
		if len(got) != want {
			t.Fatalf("%q at %d: bound query returned %d rows, the scan keeps %d", c.value, c.pos, len(got), want)
		}
		for _, f := range got {
			if s, _ := f.Args[c.pos].(string); s != c.value {
				t.Fatalf("bound query returned a row for %q", s)
			}
		}
	}
}

func BenchmarkDependencyRowsOfOneFile(b *testing.B) {
	k := boundQueryKernel(b, 250000)
	if _, err := k.Query("dependency_link"); err != nil {
		b.Fatal(err)
	}
	b.Run("scan", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			rows, _ := k.Query("dependency_link")
			n := 0
			for _, f := range rows {
				if s, _ := f.Args[0].(string); s == "pkg/f00007.go" {
					n++
				}
			}
			if n == 0 {
				b.Fatal("no rows")
			}
		}
	})
	b.Run("bound", func(b *testing.B) {
		b.ReportAllocs()
		q := BoundQuery("dependency_link", 3, map[int]any{0: "pkg/f00007.go"})
		for b.Loop() {
			rows, _ := k.Query(q)
			if len(rows) == 0 {
				b.Fatal("no rows")
			}
		}
	})
}

// Both kernels answer bound queries from their index. If either stopped
// satisfying this, BoundRows would quietly fall back to copying the relation.
var (
	_ interface {
		QueryBound(pred string, arity int, bound map[int]any) ([]Fact, error)
	} = (*RealKernel)(nil)
	_ interface {
		QueryBound(pred string, arity int, bound map[int]any) ([]Fact, error)
	} = (*CortexKernel)(nil)
)

type barePredicateQuerier struct{ rows map[string][]Fact }

func (b barePredicateQuerier) Query(pred string) ([]Fact, error) { return b.rows[pred], nil }

// A querier with no index is asked for the predicate and filtered here, to the
// same rows the index returns: strings by value, a name constant by its name,
// rows of another arity never.
func TestBoundRows_FallsBackToAnExactFilter(t *testing.T) {
	k := boundQueryKernel(t, 3000)
	if err := k.Assert(Fact{Predicate: "symbol_graph", Args: []any{"func:A", MangleAtom("/function"), MangleAtom("/public"), "a.go", "func A()"}}); err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(Fact{Predicate: "symbol_graph", Args: []any{"func:b", MangleAtom("/function"), MangleAtom("/private"), "a.go", "func b()"}}); err != nil {
		t.Fatal(err)
	}
	bare := barePredicateQuerier{rows: map[string][]Fact{}}
	for _, pred := range []string{"dependency_link", "symbol_graph"} {
		rows, err := k.Query(pred)
		if err != nil {
			t.Fatal(err)
		}
		bare.rows[pred] = rows
	}
	bare.rows["dependency_link"] = append(bare.rows["dependency_link"], Fact{Predicate: "dependency_link", Args: []any{"pkg/f00007.go"}})
	for _, c := range []struct {
		pred  string
		arity int
		bound map[int]any
	}{
		{"dependency_link", 3, map[int]any{0: "pkg/f00007.go"}},
		{"dependency_link", 3, map[int]any{1: "pkg:mod/p000001"}},
		{"dependency_link", 3, map[int]any{0: "none.go"}},
		{"symbol_graph", 5, map[int]any{2: MangleAtom("/public")}},
		{"symbol_graph", 5, map[int]any{2: MangleAtom("/private"), 0: "func:b"}},
	} {
		indexed, err := BoundRows(k, c.pred, c.arity, c.bound)
		if err != nil {
			t.Fatal(err)
		}
		filtered, err := BoundRows(bare, c.pred, c.arity, c.bound)
		if err != nil {
			t.Fatal(err)
		}
		if len(indexed) != len(filtered) {
			t.Fatalf("%s %v: the index returns %d rows, the filter %d", c.pred, c.bound, len(indexed), len(filtered))
		}
	}
	public, _ := BoundRows(k, "symbol_graph", 5, map[int]any{2: MangleAtom("/public")})
	if len(public) != 1 {
		t.Fatalf("one public symbol was asserted, the index returns %d", len(public))
	}
}
