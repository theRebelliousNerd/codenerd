package system

import (
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
)

// newBootJIT is the JIT compiler a booted session wires: a real kernel and the
// embedded atom corpus the binary ships.
func newBootJIT(t *testing.T) (*prompt.JITPromptCompiler, *core.RealKernel) {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	embedded, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithKernel(NewKernelAdapter(kernel)),
		prompt.WithEmbeddedCorpus(embedded),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	return compiler, kernel
}

// newBootRolePrompts is the role compiler a booted session wires over newBootJIT.
func newBootRolePrompts(t *testing.T) *prompt.RoleCompiler {
	t.Helper()
	compiler, _ := newBootJIT(t)
	roles, err := prompt.NewRoleCompiler(compiler, config.DefaultJITConfig())
	if err != nil {
		t.Fatalf("NewRoleCompiler: %v", err)
	}
	return roles
}
