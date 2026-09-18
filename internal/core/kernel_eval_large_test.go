package core

import (
	"fmt"
	"testing"
)

// largeWorldFacts is above the old differentialFactCeiling (10000): this
// store size is the one the deleted differential path used to demote at,
// and it is where a lost delta or a surviving retracted fact would show up
// first. See Docs/journeys/impl/S23-differential-path.md.
const largeWorldFacts = 10001

func TestLargeWorldDeltaPreservesResults(t *testing.T) {
	k := setupMockKernel(t)
	for i := 0; i < largeWorldFacts; i++ {
		k.AssertWithoutEval(Fact{Predicate: "file_topology", Args: []any{fmt.Sprintf("src/f%d.go", i), "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}})
	}
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	k.AssertWithoutEval(Fact{Predicate: "file_topology", Args: []any{"src/delta.go", "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}})
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	facts, err := k.Query("file_topology")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != largeWorldFacts+1 {
		t.Fatalf("delta lost: %d facts", len(facts))
	}
	if err := k.Retract("file_topology"); err != nil {
		t.Fatal(err)
	}
	facts, err = k.Query("file_topology")
	if err != nil || len(facts) != 0 {
		t.Fatalf("retraction lost: %d %v", len(facts), err)
	}
}

// BenchmarkProductionWorldDelta measures a one-fact change over 48K world
// inputs with the embedded production corpus, including the derived answer.
// This is the price of rebuilding the store on every evaluate; it is the
// number to beat if a sound incremental evaluator is ever attempted.
func BenchmarkProductionWorldDelta(b *testing.B) {
	k, err := NewRealKernel()
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 48000; i++ {
		k.AssertWithoutEval(Fact{Predicate: "file_topology", Args: []any{fmt.Sprintf("src/f%d.go", i), "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}})
	}
	if err := k.Evaluate(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k.AssertWithoutEval(Fact{Predicate: "file_topology", Args: []any{fmt.Sprintf("delta/f%d.go", i), "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}})
		if err := k.Evaluate(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	facts, err := k.Query("file_exists")
	if err != nil || len(facts) < 48000+b.N {
		b.Fatalf("derived world answer lost: %d %v", len(facts), err)
	}
	b.ReportMetric(float64(k.LastEvaluation().Duration.Microseconds()), "last-eval-us")
}
