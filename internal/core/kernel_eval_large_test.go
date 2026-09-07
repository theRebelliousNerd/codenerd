package core

import (
	"fmt"
	"testing"
)

func TestLargeWorldDeltaUsesFullEvaluatorAndPreservesResults(t *testing.T) {
	t.Setenv("CODENERD_DIFF_EVAL", "1")
	k := setupMockKernel(t)
	for i := 0; i < differentialFactCeiling+1; i++ {
		k.AssertWithoutEval(Fact{Predicate: "file_topology", Args: []any{fmt.Sprintf("src/f%d.go", i), "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}})
	}
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	if stats := k.LastEvaluation(); stats.Mode != "full" || stats.DemotionReason != "large fact set" {
		t.Fatalf("unsafe evaluator selection: %+v", stats)
	}
	k.AssertWithoutEval(Fact{Predicate: "file_topology", Args: []any{"src/delta.go", "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}})
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	facts, err := k.Query("file_topology")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != differentialFactCeiling+2 {
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
func BenchmarkProductionWorldDelta(b *testing.B) {
	for _, mode := range []string{"full", "bounded_differential"} {
		b.Run(mode, func(b *testing.B) {
			flag := "0"
			if mode == "bounded_differential" {
				flag = "1"
			}
			b.Setenv("CODENERD_DIFF_EVAL", flag)
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
		})
	}
}
