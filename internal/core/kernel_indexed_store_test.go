package core

import (
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestIndexedEvaluationPreservesProductionClosure(t *testing.T) {
	t.Setenv("CODENERD_DIFF_EVAL", "0")
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 1200; n++ {
		k.AssertWithoutEval(Fact{Predicate: "file_topology", Args: []any{fmt.Sprintf("src/f%d.go", n), "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}})
	}
	compare := func() {
		t.Helper()
		if err := k.Evaluate(); err != nil {
			t.Fatal(err)
		}
		reference := factstore.NewSimpleInMemoryStore()
		for _, a := range k.cachedAtoms {
			reference.Add(a)
		}
		if _, err := engine.EvalStratifiedProgramWithStats(k.programInfo, k.strata, k.predToStratum, reference); err != nil {
			t.Fatal(err)
		}
		collect := func(s factstore.FactStore) []string {
			var rows []string
			if err := factstore.GetAllFacts(s, func(a ast.Atom) error { rows = append(rows, a.String()); return nil }); err != nil {
				t.Fatal(err)
			}
			sort.Strings(rows)
			return rows
		}
		want, got := collect(reference), collect(k.store)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("closure differs: reference=%d indexed=%d", len(want), len(got))
		}
	}
	compare()
	removed := Fact{Predicate: "file_topology", Args: []any{"src/f32.go", "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}}
	if err := k.RetractFact(removed); err != nil {
		t.Fatal(err)
	}
	compare()
	if facts, err := k.Query(`file_exists("src/f32.go")`); err != nil || len(facts) != 0 {
		t.Fatalf("stale derived answer survived retraction: %v %v", facts, err)
	}
}
func BenchmarkBoundFactLookup(b *testing.B) {
	for _, mode := range []string{"scan", "indexed"} {
		b.Run(mode, func(b *testing.B) {
			var s factstore.FactStore = factstore.NewSimpleInMemoryStore()
			if mode == "indexed" {
				s = newEvaluationFactStore(12000)
			}
			pred := ast.PredicateSym{Symbol: "entry", Arity: 2}
			for n := 0; n < 12000; n++ {
				s.Add(ast.Atom{Predicate: pred, Args: []ast.BaseTerm{ast.String(fmt.Sprint(n)), ast.String(fmt.Sprint(n % 1200))}})
			}
			query := ast.Atom{Predicate: pred, Args: []ast.BaseTerm{ast.Variable{Symbol: "X"}, ast.String("31")}}
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				count := 0
				if err := s.GetFacts(query, func(ast.Atom) error { count++; return nil }); err != nil {
					b.Fatal(err)
				}
				if count != 10 {
					b.Fatalf("lookup lost facts: %d", count)
				}
			}
		})
	}
}
