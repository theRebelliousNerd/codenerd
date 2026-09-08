package init

import (
	"context"
	"testing"
)

// TestIsExpertInitPhase verifies expert-phase classification.
func TestIsExpertInitPhase(t *testing.T) {
	expertPhases := []string{"agents", "/agents", "kb_agent", "/kb_agent", "kb_complete"}
	for _, phase := range expertPhases {
		if !isExpertInitPhase(phase) {
			t.Errorf("expected phase %q to be expert phase", phase)
		}
	}
	nonExpert := []string{"analysis", "profile", "facts", "setup", ""}
	for _, phase := range nonExpert {
		if isExpertInitPhase(phase) {
			t.Errorf("expected phase %q NOT to be expert phase", phase)
		}
	}
}

// TestVerifyJITKernelLoaded_DuringExpertInit verifies the kernel loads during
// expert initialization: NewInitializer must produce a booted kernel and the
// probe query must succeed for every expert phase.
func TestVerifyJITKernelLoaded_DuringExpertInit(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultInitConfig(tmpDir)
	cfg.LLMClient = &MockLLMClient{}
	cfg.Interactive = false

	init, err := NewInitializer(cfg)
	if err != nil {
		t.Fatalf("NewInitializer failed: %v", err)
	}
	defer init.Close()

	if init.kernel == nil {
		t.Fatal("expected non-nil init kernel")
	}
	if !init.kernel.IsInitialized() {
		t.Fatal("expected init kernel to be initialized after NewInitializer")
	}

	for _, phase := range []string{"agents", "kb_agent", "kb_complete"} {
		if err := init.verifyJITKernelLoaded(phase); err != nil {
			t.Errorf("verifyJITKernelLoaded(%q) failed: %v", phase, err)
		}
	}
}

// TestCreateJITCompiler_WiresKernel restores the missing JIT kernel in the
// init path: the compiler must be created with a kernel-backed scope and the
// adapter must round-trip facts using Declared predicates.
func TestCreateJITCompiler_WiresKernel(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultInitConfig(tmpDir)
	cfg.LLMClient = &MockLLMClient{}
	cfg.Interactive = false

	init, err := NewInitializer(cfg)
	if err != nil {
		t.Fatalf("NewInitializer failed: %v", err)
	}
	defer init.Close()

	compiler, err := init.createJITCompiler()
	if err != nil {
		t.Fatalf("createJITCompiler failed: %v", err)
	}
	if compiler == nil {
		t.Fatal("expected non-nil JIT compiler")
	}

	// Adapter round-trip on Declared predicates:
	// project_language(Language) bound [/name] and
	// compile_context(Dimension, Value) bound [/name, /name].
	adapter := newInitJITKernelAdapter(init.kernel)
	if err := adapter.AssertBatch([]any{`project_language(/go)`}); err != nil {
		t.Fatalf("adapter AssertBatch failed: %v", err)
	}
	facts, err := adapter.Query("project_language")
	if err != nil {
		t.Fatalf("adapter Query failed: %v", err)
	}
	if len(facts) == 0 {
		t.Error("expected project_language fact to be queryable after AssertBatch")
	}

	// Compilation scope isolation: facts asserted in a scope must be writable.
	scope, err := adapter.NewCompilationScope()
	if err != nil {
		t.Fatalf("NewCompilationScope failed: %v", err)
	}
	defer scope.Close()
	if err := scope.AssertBatch([]any{`compile_context(/test_dim, /test_val)`}); err != nil {
		t.Fatalf("scope AssertBatch failed: %v", err)
	}

	// Full compile smoke test on the expert phase: must not fall back to error.
	cc := BuildInitCompilationContext("agents", "recommend experts", nil)
	if _, err := compiler.Compile(context.Background(), cc); err != nil {
		t.Fatalf("JIT compile for expert phase failed: %v", err)
	}
}

// TestVerifyJITKernelLoaded_NilKernelFails documents the fail-closed behavior:
// a nil kernel must produce an error, not a silent fallback.
func TestVerifyJITKernelLoaded_NilKernelFails(t *testing.T) {
	var nilInit *Initializer
	if err := nilInit.verifyJITKernelLoaded("agents"); err == nil {
		t.Error("expected error for nil Initializer, got nil")
	}
	empty := &Initializer{}
	if err := empty.verifyJITKernelLoaded("agents"); err == nil {
		t.Error("expected error for nil kernel, got nil")
	}
}
