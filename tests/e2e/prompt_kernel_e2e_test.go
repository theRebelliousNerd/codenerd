//go:build integration

package e2e_test

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
)

// promptKernelAdapter bridges a real core.RealKernel to prompt.KernelQuerier
// for e2e prompt-compiler tests. Production uses system.KernelAdapter; this
// minimal mirror keeps the e2e binary from importing the whole system factory
// tree while preserving semantics: string facts parse via core.ParseFactString
// and batch into one AssertBatch, and queries convert core.Fact to prompt.Fact.
type promptKernelAdapter struct {
	kernel *core.RealKernel
}

var (
	_ prompt.KernelQuerier   = (*promptKernelAdapter)(nil)
	_ prompt.KernelRetracter = (*promptKernelAdapter)(nil)
)

func (a *promptKernelAdapter) Query(predicate string) ([]prompt.Fact, error) {
	facts, err := a.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}
	result := make([]prompt.Fact, len(facts))
	for i, f := range facts {
		result[i] = prompt.Fact{Predicate: f.Predicate, Args: f.Args}
	}
	return result, nil
}

func (a *promptKernelAdapter) AssertBatch(facts []any) error {
	batch := make([]core.Fact, 0, len(facts))
	for _, f := range facts {
		switch v := f.(type) {
		case string:
			parsed, err := core.ParseFactString(v)
			if err != nil {
				return fmt.Errorf("parse fact string %q: %w", v, err)
			}
			batch = append(batch, parsed)
		case core.Fact:
			batch = append(batch, v)
		case prompt.Fact:
			batch = append(batch, core.Fact{Predicate: v.Predicate, Args: v.Args})
		default:
			return fmt.Errorf("unsupported fact type: %T", f)
		}
	}
	return a.kernel.AssertBatch(batch)
}

func (a *promptKernelAdapter) Retract(predicate string) error {
	return a.kernel.Retract(predicate)
}

// newE2EPromptCompiler builds a JIT compiler wired the way production wires
// it: the embedded corpus (now the constructor default) plus a real Mangle
// kernel booted with the policy constitution for atom selection.
func newE2EPromptCompiler(t *testing.T) *prompt.JITPromptCompiler {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("failed to boot real kernel: %v", err)
	}
	compiler, err := prompt.NewJITPromptCompiler(prompt.WithKernel(&promptKernelAdapter{kernel: kernel}))
	if err != nil {
		t.Fatalf("failed to create compiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	return compiler
}
