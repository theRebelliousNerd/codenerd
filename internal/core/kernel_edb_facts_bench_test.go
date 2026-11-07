package core

import (
	"fmt"
	"testing"
	"time"
)

// worldTopology is the file_topology row worldBatch asserts for path.
func worldTopology(path string) Fact {
	return Fact{Predicate: "file_topology", Args: []any{path, "h", MangleAtom("/go"), int64(1), MangleAtom("/false")}}
}

// evaluatedWorld is a production-policy kernel holding a files-file world,
// evaluated, as chat holds one after the world scan.
func evaluatedWorld(tb testing.TB, files int) *RealKernel {
	tb.Helper()
	k, err := NewRealKernel()
	if err != nil {
		tb.Fatal(err)
	}
	worldBatch(tb, k, "base", files)
	if err := k.Evaluate(); err != nil {
		tb.Fatal(err)
	}
	return k
}

// BenchmarkEDBRetractOne measures RetractExactFact of one row of a large EDB.
// The row is asserted back, outside the measurement, so the world stays the
// same size; the metric is the retract alone. It must not grow with the world.
func BenchmarkEDBRetractOne(b *testing.B) {
	for _, files := range []int{4000, 48000} {
		b.Run(fmt.Sprintf("files=%d", files), func(b *testing.B) {
			k := evaluatedWorld(b, files)
			b.ReportAllocs()
			var total time.Duration
			var prev Fact
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if prev.Predicate != "" {
					if err := k.AssertWithoutEval(prev); err != nil {
						b.Fatal(err)
					}
				}
				prev = worldTopology(fmt.Sprintf("base/f%d.go", 100+i%(files-200)))
				started := time.Now()
				if err := k.RetractExactFact(prev); err != nil {
					b.Fatal(err)
				}
				total += time.Since(started)
			}
			b.StopTimer()
			b.ReportMetric(float64(total.Nanoseconds())/float64(b.N), "retract-ns/op")
			b.ReportMetric(float64(k.FactCount()), "edb-rows")
		})
	}
}

// BenchmarkKernelCloneAndStage is a dream check's storage shape: clone an
// evaluated kernel, stage one fact, evaluate what is pending. "small" stages a
// fact of a predicate the world holds no rows of; "world" stages one more row
// of the world's biggest predicate. Neither may grow with the number of files.
func BenchmarkKernelCloneAndStage(b *testing.B) {
	for _, files := range []int{4000, 48000} {
		for _, shape := range []string{"small", "world"} {
			b.Run(fmt.Sprintf("files=%d/stage=%s", files, shape), func(b *testing.B) {
				k := evaluatedWorld(b, files)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					c := k.Clone()
					f := Fact{Predicate: "pending_action", Args: []any{fmt.Sprintf("/probe%d", i), MangleAtom("/read_file"), "x", int64(i)}}
					if shape == "world" {
						f = worldTopology(fmt.Sprintf("probe/f%d.go", i))
					}
					if err := c.AssertWithoutEval(f); err != nil {
						b.Fatal(err)
					}
					if err := c.EvaluatePending(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
