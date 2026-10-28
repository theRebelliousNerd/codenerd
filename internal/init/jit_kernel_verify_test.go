package init

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

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
	cc := initCompilationContext(llmRoute{slot: slotPlanner}, "agents", "recommend experts", nil)
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

// Without the JIT kernel no init prompt can be compiled, and the call fails
// before any provider attempt: there is no static prompt to send instead.
func TestInitPrompt_NoKernelIsAnErrorNotAFallback(t *testing.T) {
	cfg := DefaultInitConfig(t.TempDir())
	cfg.LLMClient = &MockLLMClient{}
	cfg.Interactive = false
	init, err := NewInitializer(cfg)
	if err != nil {
		t.Fatalf("NewInitializer failed: %v", err)
	}
	defer init.Close()
	live := init.kernel
	init.kernel = nil // the regression the check exists for
	defer func() { init.kernel = live }()

	for _, phase := range []string{"analysis", "agents"} {
		if _, err := init.compileInitSystemPrompt(t.Context(), init.route(slotPlanner), phase, "build the team", nil); !errors.Is(err, types.ErrRolePromptUnavailable) {
			t.Fatalf("phase %s: err = %v, want ErrRolePromptUnavailable", phase, err)
		}
	}
	_, err = init.withJITPrompt(t.Context(), initCall{slot: slotPlanner, phase: "agents", task: "build the team"},
		func(context.Context, perception.LLMClient, string) (string, error) {
			t.Fatal("the model was asked without a compiled prompt")
			return "", nil
		})
	if !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Fatalf("withJITPrompt err = %v, want ErrRolePromptUnavailable", err)
	}
	if metrics := init.snapshotLLMMetrics(); metrics.Attempts != 0 {
		t.Fatalf("attempts = %d, want 0: no provider was called", metrics.Attempts)
	}
}

// Every phase init sends a compiled prompt for is compiled from the
// init/researcher/* atoms: the persona, the phase's own guidance where it has
// one, no shard-turn protocol, and never the task, which is the user message.
func TestInitPrompt_IsCompiledFromTheInitAtoms(t *testing.T) {
	cfg := DefaultInitConfig(t.TempDir())
	cfg.LLMClient = &MockLLMClient{}
	cfg.Interactive = false
	init, err := NewInitializer(cfg)
	if err != nil {
		t.Fatalf("NewInitializer failed: %v", err)
	}
	defer init.Close()

	const task = "TASK-SENTINEL build the team"
	phaseGuidance := map[string]string{
		"analysis":       "When analyzing codebases during initialization",
		"profile":        "When generating project profiles",
		"facts":          "When generating Mangle facts",
		"agents":         "When recommending agents",
		"kb_complete":    "You are synthesizing extracted knowledge into strategic understanding.",
		"domain_experts": "Propose the domain experts this project needs",
		"project_tools":  "Choose which of this repository's own commands",
		// The three agent-knowledge calls share the knowledge-base guidance
		// and each carries its own instruction.
		"agent_methodology": "Write the methodology atoms for the specialist",
		"agent_domain":      "Write the domain atoms for the specialist",
		"doc_extraction":    "Extract key knowledge atoms from the documentation chunk",
	}
	const kbGuidance = "When creating agent knowledge bases"
	for phase, guidance := range phaseGuidance {
		t.Run(phase, func(t *testing.T) {
			got, err := init.compileInitSystemPrompt(t.Context(), init.route(slotPlanner), phase, task, nil)
			if err != nil {
				t.Fatalf("compileInitSystemPrompt(%s): %v", phase, err)
			}
			if !strings.Contains(got, "You are the ResearcherShard, a deep research specialist.") {
				t.Errorf("the %s prompt lacks the init persona", phase)
			}
			if !strings.Contains(got, guidance) {
				t.Errorf("the %s prompt lacks its phase guidance %q", phase, guidance)
			}
			if wantKB := strings.HasPrefix(phase, "agent_") || phase == "doc_extraction"; strings.Contains(got, kbGuidance) != wantKB {
				t.Errorf("the %s prompt carries the knowledge-base guidance = %v, want %v", phase, !wantKB, wantKB)
			}
			for other, otherGuidance := range phaseGuidance {
				if other != phase && strings.Contains(got, otherGuidance) {
					t.Errorf("the %s prompt carries the %s guidance", phase, other)
				}
			}
			for _, leaked := range []string{"TASK-SENTINEL", "OUTPUT PROTOCOL: PIGGYBACK ENVELOPE", "CONTROL-PACKET FIELD ORDER"} {
				if strings.Contains(got, leaked) {
					t.Errorf("the %s prompt carries %q", phase, leaked)
				}
			}
		})
	}
	// A phase no init atom is gated to has no persona: an error, not a
	// prompt without one.
	if _, err := init.compileInitSystemPrompt(t.Context(), init.route(slotPlanner), "not_a_phase", task, nil); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Fatalf("unknown phase err = %v, want ErrRolePromptUnavailable", err)
	}
}

// withInitKernel gives a hand-built Initializer the booted kernel its JIT
// prompt compile needs, as NewInitializer does.
func withInitKernel(t *testing.T, i *Initializer) *Initializer {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	i.kernel = kernel
	return i
}
