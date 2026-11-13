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
		got, err := k.Query(BoundQuery("dependency_link", 3, map[int]string{c.pos: c.value}))
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
		q := BoundQuery("dependency_link", 3, map[int]string{0: "pkg/f00007.go"})
		for b.Loop() {
			rows, _ := k.Query(q)
			if len(rows) == 0 {
				b.Fatal("no rows")
			}
		}
	})
}
