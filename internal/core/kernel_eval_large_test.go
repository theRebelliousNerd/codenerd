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
	t.Parallel()
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

// BenchmarkProductionWorldBatchInsert measures a 2000-fact batch insert over a
// 48K-file production world (file_topology plus one symbol_graph fact per file
// so the derived cone is non-trivial), including the derived answer. Evaluate
// forces the full fixpoint: this is the reference the lazy path
// (BenchmarkProductionWorldInsertPending, kernel_eval_delta_test.go) beats.
func BenchmarkProductionWorldBatchInsert(b *testing.B) {
	k, err := NewRealKernel()
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 48000; i++ {
		path := fmt.Sprintf("src/f%d.go", i)
		k.AssertWithoutEval(Fact{Predicate: "file_topology", Args: []any{path, "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}})
		k.AssertWithoutEval(Fact{Predicate: "symbol_graph", Args: []any{fmt.Sprintf("%s:Fn%d", path, i), MangleAtom("/function"), MangleAtom("/public"), path, "fn()"}})
	}
	if err := k.Evaluate(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 2000; j++ {
			path := fmt.Sprintf("batch/b%d_f%d.go", i, j)
			k.AssertWithoutEval(Fact{Predicate: "file_topology", Args: []any{path, "hash", MangleAtom("/go"), int64(0), MangleAtom("/false")}})
		}
		if err := k.Evaluate(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	facts, err := k.Query("file_exists")
	if err != nil {
		b.Fatal(err)
	}
	present := make(map[string]struct{}, len(facts))
	for _, f := range facts {
		if len(f.Args) > 0 {
			if s, ok := f.Args[0].(string); ok {
				present[s] = struct{}{}
			}
		}
	}
	for i := 0; i < b.N; i++ {
		for j := 0; j < 2000; j++ {
			path := fmt.Sprintf("batch/b%d_f%d.go", i, j)
			if _, ok := present[path]; !ok {
				b.Fatalf("derived world answer lost batch file %s", path)
			}
		}
	}
	stats := k.LastEvaluation()
	b.ReportMetric(float64(stats.Duration.Microseconds()), "last-eval-us")
	b.ReportMetric(float64(stats.EngineDuration.Microseconds()), "last-eval-engine-us")
	b.ReportMetric(float64(stats.PrepareDuration.Microseconds()), "last-eval-prepare-us")
	modeFull := 0.0
	if stats.Mode == "full" {
		modeFull = 1.0
	}
	b.ReportMetric(modeFull, "last-eval-mode-full")
	b.Logf("last evaluation mode=%s duration=%s engine=%s prepare=%s", stats.Mode, stats.Duration, stats.EngineDuration, stats.PrepareDuration)
}
