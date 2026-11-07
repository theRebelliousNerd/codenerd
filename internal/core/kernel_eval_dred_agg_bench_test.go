package core

import (
	"fmt"
	"testing"
	"time"
)

// aggBenchPolicy has the production dependent_count shape (a count over a
// two-premise join) and a single-atom count and sum, over a predicate each.
const aggBenchPolicy = `
	Decl ab_link(Importer, File, Via).
	Decl ab_file(File, Kind).
	Decl ab_dependent_count(File, N).
	Decl ab_item(Owner, Id, V).
	Decl ab_stats(Owner, N, S).

	ab_dependent_count(F, N) :- ab_link(I, F, _), ab_file(F, _) |> do fn:group_by(F), let N = fn:count().
	ab_stats(O, N, S) :- ab_item(O, _, V) |> do fn:group_by(O), let N = fn:count(), let S = fn:sum(V).
`

// BenchmarkDRedAggregateOneChange measures one write to a group of 1, 100 and
// 10,000 members: a member is added and removed again, and each evaluation is
// timed. A maintained aggregate must not grow with the group.
func BenchmarkDRedAggregateOneChange(b *testing.B) {
	for _, shape := range []string{"join-count", "single-count-sum"} {
		for _, size := range []int{1, 100, 10000} {
			b.Run(fmt.Sprintf("%s/members=%d", shape, size), func(b *testing.B) {
				k := newRealKernelBase()
				k.AppendPolicy(aggBenchPolicy)
				member := func(i int) Fact {
					if shape == "join-count" {
						return Fact{Predicate: "ab_link", Args: []any{MangleAtom(fmt.Sprintf("/i%d", i)), MangleAtom("/hot"), "via"}}
					}
					return Fact{Predicate: "ab_item", Args: []any{MangleAtom("/hot"), MangleAtom(fmt.Sprintf("/m%d", i)), int64(i % 7)}}
				}
				if shape == "join-count" {
					_ = k.AssertWithoutEval(Fact{Predicate: "ab_file", Args: []any{MangleAtom("/hot"), "go"}})
				}
				for i := 0; i < size; i++ {
					if err := k.AssertWithoutEval(member(i)); err != nil {
						b.Fatal(err)
					}
				}
				if err := k.Evaluate(); err != nil {
					b.Fatal(err)
				}
				step := func(i int) time.Duration {
					extra := member(size + 1 + i)
					if err := k.AssertWithoutEval(extra); err != nil {
						b.Fatal(err)
					}
					started := time.Now()
					if err := k.EvaluatePending(); err != nil {
						b.Fatal(err)
					}
					d := time.Since(started)
					if err := k.RetractExactFact(extra); err != nil {
						b.Fatal(err)
					}
					started = time.Now()
					if err := k.EvaluatePending(); err != nil {
						b.Fatal(err)
					}
					return d + time.Since(started)
				}
				step(-1) // the first pass drops the engine's scratch relations
				var total time.Duration
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					total += step(i)
				}
				b.StopTimer()
				if ev := k.LastEvaluation(); ev.Mode != "dred" {
					b.Fatalf("evaluated as %q (fallback %q), want dred", ev.Mode, ev.ConeFallback)
				}
				b.ReportMetric(float64(total.Microseconds())/float64(2*b.N), "eval-us/change")
			})
		}
	}
}
