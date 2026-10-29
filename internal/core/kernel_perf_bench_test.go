package core

import (
	"fmt"
	"testing"
)

// The synthetic workload behind the KERNEL-PERF benchmarks
// (Docs/audits/mangle-kernel-performance.md): a large edge relation joined on
// its first argument, a large derived relation nothing small writes into, and
// a one-row probe predicate whose cone is a single join rule.
const perfProgram = `
Decl perf_edge(X, Y) bound [/string, /string].
Decl perf_label(X, L) bound [/string, /string].
Decl perf_bulk(X, Y) bound [/string, /number].
Decl perf_probe(X) bound [/string].
Decl perf_hit(X, L) bound [/string, /string].
Decl perf_labelled(X, L) bound [/string, /string].
Decl perf_quiet(X) bound [/string].
Decl perf_quiet_seen(X) bound [/string].

perf_hit(X, L) :- perf_probe(X), perf_edge(X, Y), perf_label(Y, L).
perf_labelled(X, L) :- perf_edge(X, Y), perf_label(Y, L).
perf_quiet_seen(X) :- perf_quiet(X), !perf_probe(X).
`

// newPerfKernel boots a kernel holding only perfProgram and n edges (plus a
// label per edge target and n bulk rows), evaluated to fixpoint.
func newPerfKernel(tb testing.TB, n int) *RealKernel {
	tb.Helper()
	k := newRealKernelBase()
	k.AppendPolicy(perfProgram)
	facts := make([]Fact, 0, 3*n)
	for i := 0; i < n; i++ {
		facts = append(facts,
			Fact{Predicate: "perf_edge", Args: []any{fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", (i*7+1)%n)}},
			Fact{Predicate: "perf_label", Args: []any{fmt.Sprintf("n%d", i), fmt.Sprintf("label%d", i%13)}},
			Fact{Predicate: "perf_bulk", Args: []any{fmt.Sprintf("b%d", i), int64(i)}},
		)
	}
	if err := k.LoadFacts(facts); err != nil {
		tb.Fatal(err)
	}
	if err := k.Evaluate(); err != nil {
		tb.Fatal(err)
	}
	return k
}

// BenchmarkDreamCloneConeEvaluation is the dream safety check's shape: clone
// an evaluated kernel, stage one projected fact, evaluate its cone, read the
// answer. The copy and index build are inside the timed loop on purpose: they
// are paid on every speculative request.
func BenchmarkDreamCloneConeEvaluation(b *testing.B) {
	for _, n := range []int{512, 20000} {
		b.Run(fmt.Sprintf("edges=%d", n), func(b *testing.B) {
			k := newPerfKernel(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c := k.Clone()
				if err := c.AssertWithoutEval(Fact{Predicate: "perf_probe", Args: []any{fmt.Sprintf("n%d", i%n)}}); err != nil {
					b.Fatal(err)
				}
				if err := c.EvaluatePending(); err != nil {
					b.Fatal(err)
				}
				rows, err := c.Query("perf_hit")
				if err != nil || len(rows) != 1 {
					b.Fatalf("perf_hit = %d rows, %v", len(rows), err)
				}
			}
		})
	}
}

// BenchmarkDreamCheck is a speculative check end to end on two shapes --
// clone (an overlay over the parent's store, kernel_overlay_store.go), stage
// the projected facts, evaluate their cone, read the answer -- on the
// synthetic join and on the production corpus with a dream projection.
func BenchmarkDreamCheck(b *testing.B) {
	run := func(b *testing.B, k *RealKernel, stage func(i int) []Fact, read string) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c := k.Clone()
			for _, f := range stage(i) {
				if err := c.AssertWithoutEval(f); err != nil {
					b.Fatal(err)
				}
			}
			if err := c.EvaluatePending(); err != nil {
				b.Fatal(err)
			}
			if _, err := c.Query(read); err != nil {
				b.Fatal(err)
			}
		}
	}
	for _, n := range []int{512, 20000} {
		b.Run(fmt.Sprintf("synthetic/edges=%d", n), func(b *testing.B) {
			k := newPerfKernel(b, n)
			run(b, k, func(i int) []Fact {
				return []Fact{{Predicate: "perf_probe", Args: []any{fmt.Sprintf("n%d", i%n)}}}
			}, "perf_hit")
		})
	}
	b.Run("production/dream", func(b *testing.B) {
		d, k := setupTestDreamer(b)
		var seed []Fact
		for i := 0; i < 4000; i++ {
			seed = append(seed,
				Fact{Predicate: "code_defines", Args: []any{fmt.Sprintf("internal/core/f%d.go", i%400), fmt.Sprintf("Sym%d", i), MangleAtom("/function"), int64(i), int64(i + 5)}},
				Fact{Predicate: "code_calls", Args: []any{fmt.Sprintf("Sym%d", i), fmt.Sprintf("Sym%d", (i*7+1)%4000)}},
			)
		}
		if err := k.LoadFacts(seed); err != nil {
			b.Fatal(err)
		}
		if _, err := k.Query("panic_state"); err != nil {
			b.Fatal(err)
		}
		run(b, k, func(i int) []Fact {
			return d.projectEffects(k, "dream:bench", ActionRequest{Type: ActionEditFile, Target: fmt.Sprintf("internal/core/f%d.go", i%400)})
		}, "panic_state")
	})
}

// BenchmarkConeSmallWrite measures a one-row write whose cone is one rule,
// on a kernel whose EDB and materialized store are large and untouched.
func BenchmarkConeSmallWrite(b *testing.B) {
	for _, n := range []int{512, 20000} {
		b.Run(fmt.Sprintf("edges=%d", n), func(b *testing.B) {
			k := newPerfKernel(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := k.Assert(Fact{Predicate: "perf_quiet", Args: []any{fmt.Sprintf("q%d", i)}}); err != nil {
					b.Fatal(err)
				}
				if _, err := k.Query("perf_quiet_seen"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkTransactionMixedRetractions commits one transaction holding many
// exact, first-argument and whole-predicate retractions over a large EDB: the
// orientation refresh shape (one exact retraction per removed row).
func BenchmarkTransactionMixedRetractions(b *testing.B) {
	const n = 20000
	for _, exact := range []int{50, 1000} {
		b.Run(fmt.Sprintf("edges=%d/exact=%d", n, exact), func(b *testing.B) {
			k := newPerfKernel(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				c := k.Clone()
				tx := c.Transaction()
				for j := 0; j < exact; j++ {
					m := (j * 17) % n
					tx.RetractExactFact(Fact{Predicate: "perf_edge", Args: []any{fmt.Sprintf("n%d", m), fmt.Sprintf("n%d", (m*7+1)%n)}})
				}
				for j := 0; j < 20; j++ {
					tx.RetractFact(Fact{Predicate: "perf_label", Args: []any{fmt.Sprintf("n%d", j*31)}})
				}
				tx.Retract("perf_quiet")
				tx.Assert(Fact{Predicate: "perf_probe", Args: []any{"n1"}})
				b.StartTimer()
				if err := tx.Commit(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
