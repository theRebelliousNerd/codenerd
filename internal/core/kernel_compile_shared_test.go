package core

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Kernels that boot at once from the same program text share one compile:
// every one gets the same analyzed program, however their compiles overlap.
func TestConcurrentKernelsShareOneCompile(t *testing.T) {
	policy := fmt.Sprintf("Decl shared_compile_probe_%d(X) bound [/string].\n", time.Now().UnixNano())
	const n = 7
	kernels := make([]*RealKernel, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range kernels {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			k := newRealKernelBase()
			k.AppendPolicy(policy)
			<-start
			if err := k.Evaluate(); err != nil {
				t.Error(err)
			}
			kernels[i] = k
		}(i)
	}
	close(start)
	wg.Wait()
	for i, k := range kernels[1:] {
		if k.programInfo != kernels[0].programInfo {
			t.Fatalf("kernel %d compiled its own copy of the program", i+1)
		}
	}
}

// A sandbox trial compile is never shared or cached.
func TestSandboxCompileIsNotShared(t *testing.T) {
	policy := fmt.Sprintf("Decl sandbox_compile_probe_%d(X) bound [/string].\n", time.Now().UnixNano())
	a, b := newRealKernelBase(), newRealKernelBase()
	a.sandbox, b.sandbox = true, true
	for _, k := range []*RealKernel{a, b} {
		k.AppendPolicy(policy)
		if err := k.Evaluate(); err != nil {
			t.Fatal(err)
		}
	}
	if a.programInfo == b.programInfo {
		t.Fatal("two sandbox kernels share one compiled program")
	}
}
