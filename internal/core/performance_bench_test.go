// Performance benchmarks for critical paths identified in optimization audit
package core

import (
	"fmt"
	"testing"
)

// Benchmark: CRITICAL - Assert triggering full re-evaluation
func BenchmarkKernelAssert(b *testing.B) {
	kernel, err := NewRealKernel()
	if err != nil {
		b.Fatalf("Failed to create kernel: %v", err)
	}
	defer kernel.Reset()

	// Load initial facts
	facts := make([]Fact, 1000)
	for i := range 1000 {
		facts[i] = Fact{
			Predicate: "test_fact",
			Args:      []any{fmt.Sprintf("arg_%d", i), i},
		}
	}
	kernel.LoadFacts(facts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kernel.Assert(Fact{
			Predicate: "dynamic_fact",
			Args:      []any{fmt.Sprintf("bench_%d", i)},
		})
	}
}

// Benchmark: CRITICAL - Retract triggering index rebuild
func BenchmarkKernelRetract(b *testing.B) {
	kernel, err := NewRealKernel()
	if err != nil {
		b.Fatalf("Failed to create kernel: %v", err)
	}
	defer kernel.Reset()

	// Load facts
	facts := make([]Fact, 5000)
	for i := range 5000 {
		facts[i] = Fact{
			Predicate: fmt.Sprintf("pred_%d", i%10),
			Args:      []any{i},
		}
	}
	kernel.LoadFacts(facts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kernel.Retract(fmt.Sprintf("pred_%d", i%10))
	}
}

// Benchmark: CRITICAL - Query with fmt.Sprintf comparison
func BenchmarkKernelQuery(b *testing.B) {
	kernel, err := NewRealKernel()
	if err != nil {
		b.Fatalf("Failed to create kernel: %v", err)
	}
	defer kernel.Reset()

	// Load diverse facts
	facts := make([]Fact, 10000)
	for i := range 10000 {
		facts[i] = Fact{
			Predicate: "test_data",
			Args:      []any{fmt.Sprintf("key_%d", i), i, float64(i) * 1.5},
		}
	}
	kernel.LoadFacts(facts)

	b.ResetTimer()
	for b.Loop() {
		kernel.Query("test_data")
	}
}

// BenchmarkKernelLoadFacts measures the pure EDB write path (internFact,
// canonFact/canonical key, ToAtom/admission, dedupe index) for 150k facts
// shaped like the spec-pairing corpus (see specGenFacts in
// kernel_eval_spec_pairing_bench_test.go). It drives LoadFacts on a bare
// kernel so no policy evaluation dominates the profile.
func BenchmarkKernelLoadFacts(b *testing.B) {
	links, elements, sections, confirmed := specGenFacts(0, 37500, 37500, 37500)
	facts := make([]Fact, 0, len(links)+len(elements)+len(sections)+len(confirmed))
	facts = append(facts, links...)
	facts = append(facts, elements...)
	facts = append(facts, sections...)
	facts = append(facts, confirmed...)
	if len(facts) != 150000 {
		b.Fatalf("expected 150000 facts, got %d", len(facts))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := newRealKernelBase()
		if err := k.LoadFacts(facts); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark: HIGH - Pattern matching with reflect.DeepEqual
func BenchmarkFactMatching(b *testing.B) {
	fact := Fact{
		Predicate: "test",
		Args:      []any{"string", 123, 45.6, MangleAtom("/atom")},
	}
	pattern := Fact{
		Predicate: "test",
		Args:      []any{"string", 123, 45.6, MangleAtom("/atom")},
	}

	b.ResetTimer()
	for b.Loop() {
		argsSliceEqual(fact.Args, pattern.Args)
	}
}

// Benchmark: MEDIUM - Fact deduplication with string canonicalization
func BenchmarkFactDeduplication(b *testing.B) {
	kernel, err := NewRealKernel()
	if err != nil {
		b.Fatalf("Failed to create kernel: %v", err)
	}
	defer kernel.Reset()

	facts := make([]Fact, 1000)
	for i := range 1000 {
		facts[i] = Fact{
			Predicate: "dup_test",
			Args:      []any{i % 100}, // 10% duplication rate
		}
	}

	b.ResetTimer()
	for b.Loop() {
		kernel.LoadFacts(facts)
		kernel.Clear()
	}
}

// Benchmark: Batch assertion using AssertBatch (OPTIMIZED)
func BenchmarkKernelAssertBatch(b *testing.B) {
	kernel, err := NewRealKernel()
	if err != nil {
		b.Fatalf("Failed to create kernel: %v", err)
	}
	defer kernel.Reset()

	// Initial load
	initial := make([]Fact, 1000)
	for i := range 1000 {
		initial[i] = Fact{
			Predicate: "base_fact",
			Args:      []any{i},
		}
	}
	kernel.LoadFacts(initial)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := make([]Fact, 100)
		for j := range 100 {
			batch[j] = Fact{
				Predicate: "batch_fact",
				Args:      []any{i*100 + j},
			}
		}
		// OPTIMIZATION: Single evaluate() call for all 100 facts
		kernel.AssertBatch(batch)
	}
}

// Benchmark: Batch assertion using Assert loop (SLOW - for comparison)
func BenchmarkKernelAssertLoop(b *testing.B) {
	kernel, err := NewRealKernel()
	if err != nil {
		b.Fatalf("Failed to create kernel: %v", err)
	}
	defer kernel.Reset()

	// Initial load
	initial := make([]Fact, 1000)
	for i := range 1000 {
		initial[i] = Fact{
			Predicate: "base_fact",
			Args:      []any{i},
		}
	}
	kernel.LoadFacts(initial)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := make([]Fact, 100)
		for j := range 100 {
			batch[j] = Fact{
				Predicate: "batch_fact",
				Args:      []any{i*100 + j},
			}
		}
		// SLOW PATH: evaluate() called 100 times
		for _, f := range batch {
			kernel.Assert(f)
		}
	}
}

// Benchmark baseline for comparison after optimizations
func BenchmarkTypicalWorkflow(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		kernel, err := NewRealKernel()
		if err != nil {
			b.Fatalf("Failed to create kernel: %v", err)
		}

		b.StartTimer()

		// Simulate typical workflow
		// 1. Load initial facts
		facts := make([]Fact, 500)
		for j := range 500 {
			facts[j] = Fact{
				Predicate: "initial",
				Args:      []any{j},
			}
		}
		kernel.LoadFacts(facts)

		// 2. Query some data
		kernel.Query("initial")

		// 3. Assert new facts
		for j := range 50 {
			kernel.Assert(Fact{
				Predicate: "dynamic",
				Args:      []any{j},
			})
		}

		// 4. Retract something
		kernel.Retract("initial")

		// 5. Final query
		kernel.Query("dynamic")

		b.StopTimer()
		kernel.Reset()
		b.StartTimer() // b.Loop() requires timer running at loop boundary
	}
}
