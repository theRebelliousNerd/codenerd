//go:build integration

package e2e_test

import (
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// newPerceptionTransducer is perception.NewUnderstandingTransducer wired the
// way a booted session wires it (internal/system/factory.go): the
// classification prompt is compiled from the perception atoms through the JIT
// assembler. There is no embedded prompt to fall back to.
func newPerceptionTransducer(t *testing.T, client perception.LLMClient) perception.Transducer {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("failed to boot the prompt kernel: %v", err)
	}
	assembler, err := articulation.NewPromptAssemblerWithJIT(kernel, newE2EPromptCompiler(t))
	if err != nil {
		t.Fatalf("NewPromptAssemblerWithJIT: %v", err)
	}
	tr := perception.NewUnderstandingTransducer(client)
	tr.SetPromptAssembler(articulation.NewPromptAssemblerAdapter(assembler))
	return tr
}
