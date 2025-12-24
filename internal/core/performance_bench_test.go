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
	for i := 0; i < 1000; i++ {
		facts[i] = Fact{
			Predicate: "test_fact",
			Args:      []interface{}{fmt.Sprintf("arg_%d", i), i},
		}
	}
	kernel.LoadFacts(facts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kernel.Assert(Fact{
			Predicate: "dynamic_fact",
			Args:      []interface{}{fmt.Sprintf("bench_%d", i)},
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
	for i := 0; i < 5000; i++ {
		facts[i] = Fact{
			Predicate: fmt.Sprintf("pred_%d", i%10),
			Args:      []interface{}{i},
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
	for i := 0; i < 10000; i++ {
		facts[i] = Fact{
			Predicate: "test_data",
			Args:      []interface{}{fmt.Sprintf("key_%d", i), i, float64(i) * 1.5},
		}
	}
	kernel.LoadFacts(facts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kernel.Query("test_data")
	}
}

// Benchmark: CRITICAL - LoadFacts with ToAtom conversion
func BenchmarkKernelLoadFacts(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		kernel, err := NewRealKernel()
		if err != nil {
			b.Fatalf("Failed to create kernel: %v", err)
		}

		facts := make([]Fact, 1000)
		for j := 0; j < 1000; j++ {
			facts[j] = Fact{
				Predicate: "load_test",
				Args:      []interface{}{fmt.Sprintf("arg_%d", j), j},
			}
		}

		b.StartTimer()
		kernel.LoadFacts(facts)
		b.StopTimer()
		kernel.Reset()
	}
}

// Benchmark: HIGH - Pattern matching with reflect.DeepEqual
func BenchmarkFactMatching(b *testing.B) {
	fact := Fact{
		Predicate: "test",
		Args:      []interface{}{"string", 123, 45.6, MangleAtom("/atom")},
	}
	pattern := Fact{
		Predicate: "test",
		Args:      []interface{}{"string", 123, 45.6, MangleAtom("/atom")},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	for i := 0; i < 1000; i++ {
		facts[i] = Fact{
			Predicate: "dup_test",
			Args:      []interface{}{i % 100}, // 10% duplication rate
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kernel.LoadFacts(facts)
		kernel.Clear()
	}
}

// Benchmark: Batch assertion (to compare with single Assert)
func BenchmarkKernelAssertBatch(b *testing.B) {
	kernel, err := NewRealKernel()
	if err != nil {
		b.Fatalf("Failed to create kernel: %v", err)
	}
	defer kernel.Reset()

	// Initial load
	initial := make([]Fact, 1000)
	for i := 0; i < 1000; i++ {
		initial[i] = Fact{
			Predicate: "base_fact",
			Args:      []interface{}{i},
		}
	}
	kernel.LoadFacts(initial)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := make([]Fact, 100)
		for j := 0; j < 100; j++ {
			batch[j] = Fact{
				Predicate: "batch_fact",
				Args:      []interface{}{i*100 + j},
			}
		}
		// This will currently be slow - we'll add AssertBatch later
		for _, f := range batch {
			kernel.Assert(f)
		}
	}
}

// Benchmark baseline for comparison after optimizations
func BenchmarkTypicalWorkflow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		kernel, err := NewRealKernel()
		if err != nil {
			b.Fatalf("Failed to create kernel: %v", err)
		}

		b.StartTimer()

		// Simulate typical workflow
		// 1. Load initial facts
		facts := make([]Fact, 500)
		for j := 0; j < 500; j++ {
			facts[j] = Fact{
				Predicate: "initial",
				Args:      []interface{}{j},
			}
		}
		kernel.LoadFacts(facts)

		// 2. Query some data
		kernel.Query("initial")

		// 3. Assert new facts
		for j := 0; j < 50; j++ {
			kernel.Assert(Fact{
				Predicate: "dynamic",
				Args:      []interface{}{j},
			})
		}

		// 4. Retract something
		kernel.Retract("initial")

		// 5. Final query
		kernel.Query("dynamic")

		b.StopTimer()
		kernel.Reset()
	}
}
