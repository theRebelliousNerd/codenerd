package core

import (
	"context"
	"fmt"
	"testing"
)

// BenchmarkDreamer_CodeGraphProjections benchmarks the performance of SimulateAction
// which calls codeGraphProjections and Kernel.Clone.
func BenchmarkDreamer_CodeGraphProjections(b *testing.B) {
	// 1. Setup Kernel
	k, err := NewRealKernel()
	if err != nil {
		b.Fatalf("Failed to create kernel: %v", err)
	}

	// 2. Inject Facts
	// We want enough facts to make O(N) scan noticeable.
	// 10k defines, 10k calls.
	numFiles := 1000
	numSymsPerFile := 10
	totalSyms := numFiles * numSymsPerFile

	facts := make([]Fact, 0, totalSyms*2)

	// code_defines
	for i := 0; i < totalSyms; i++ {
		fileIdx := i / numSymsPerFile
		facts = append(facts, Fact{
			Predicate: "code_defines",
			Args: []interface{}{
				fmt.Sprintf("internal/pkg/file_%d.go", fileIdx),
				fmt.Sprintf("Sym%d", i),
			},
		})
	}

	// code_calls (chain)
	for i := 0; i < totalSyms-1; i++ {
		facts = append(facts, Fact{
			Predicate: "code_calls",
			Args: []interface{}{
				fmt.Sprintf("Sym%d", i),
				fmt.Sprintf("Sym%d", i+1),
			},
		})
	}

	// Add some test files
	for i := 0; i < 100; i++ {
		facts = append(facts, Fact{
			Predicate: "code_defines",
			Args: []interface{}{
				fmt.Sprintf("internal/pkg/file_%d_test.go", i),
				fmt.Sprintf("TestSym%d", i),
			},
		})
	}

	// Add calls from tests to syms
	for i := 0; i < 100; i++ {
		facts = append(facts, Fact{
			Predicate: "code_calls",
			Args: []interface{}{
				fmt.Sprintf("TestSym%d", i),
				fmt.Sprintf("Sym%d", i*10), // Call into source
			},
		})
	}

	if err := k.LoadFacts(facts); err != nil {
		b.Fatalf("Failed to load facts: %v", err)
	}

	d := NewDreamer(k)
	req := ActionRequest{
		Type:   ActionEditFile, // triggers codeGraphProjections
		Target: "internal/pkg/file_0.go",
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// SimulateAction calls projectEffects -> codeGraphProjections
		_ = d.SimulateAction(ctx, req)
	}
}
