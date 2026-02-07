package core

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkDreamer_SimulateAction_LargeGraph(b *testing.B) {
	// Setup kernel with 10k defines and 100k calls
	k, err := NewRealKernel()
	if err != nil {
		b.Fatalf("Failed to create kernel: %v", err)
	}
	d := NewDreamer(k)

	// Seed data
	// 100 files, 100 symbols each -> 10k symbols
	// file_0.go ... file_99.go
	// sym_0_0 ... sym_99_99
	// We'll treat file_90.go ... file_99.go as tests

	// Pre-allocate to avoid resize overhead during setup
	// But we use Assert loop anyway.

	for i := 0; i < 100; i++ {
		file := fmt.Sprintf("pkg/file_%d.go", i)
		if i >= 90 {
			file = fmt.Sprintf("pkg/file_%d_test.go", i)
		}

		for j := 0; j < 100; j++ {
			sym := fmt.Sprintf("sym_%d_%d", i, j)
			k.AssertWithoutEval(Fact{
				Predicate: "code_defines",
				Args: []interface{}{file, sym},
			})
		}
	}

	// 100k calls
	// Make "sym_0_0" (in file_0.go) very popular (1000 callers from test files)
	targetSym := "sym_0_0"
	targetFile := "pkg/file_0.go"

	// 1000 calls from tests to targetSym
	for i := 9000; i < 10000; i++ { // symbols from test files (90*100 to 99*100)
		fileIdx := 90 + (i-9000)/100
		symIdx := (i - 9000) % 100
		caller := fmt.Sprintf("sym_%d_%d", fileIdx, symIdx)

		k.AssertWithoutEval(Fact{
			Predicate: "code_calls",
			Args: []interface{}{caller, targetSym},
		})
	}

	// 99k random calls
	for i := 0; i < 99000; i++ {
		caller := fmt.Sprintf("sym_%d_%d", i%100, i%100)
		callee := fmt.Sprintf("sym_%d_%d", (i+1)%100, (i+1)%100)
		k.AssertWithoutEval(Fact{
			Predicate: "code_calls",
			Args: []interface{}{caller, callee},
		})
	}

	req := ActionRequest{
		Type:   ActionEditFile,
		Target: targetFile,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.SimulateAction(context.Background(), req)
	}
}
