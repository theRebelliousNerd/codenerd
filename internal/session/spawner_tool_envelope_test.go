package session

import (
	"context"
	"slices"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
)

// codedomCoreRequires loads the shipped capability/codedom_core atom and
// returns the tools its guidance is gated on. The test fails if the atom is
// missing or ungated: an unpinned fixture would prove nothing about the
// prompt the spawner compiles.
func codedomCoreRequires(t *testing.T) []string {
	t.Helper()
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	atom, ok := corpus.Get("capability/codedom_core")
	if !ok {
		t.Fatal("capability/codedom_core is not in the embedded corpus")
	}
	if len(atom.RequiresTools) == 0 {
		t.Fatal("capability/codedom_core lost its requires_tools gate; the fixture no longer exercises tool gating")
	}
	return slices.Clone(atom.RequiresTools)
}

// The spawner's compile must see the tools the subagent is handed.
// generateConfig derives the turn catalog from the kernel AFTER compiling,
// so compilationCtx.AvailableTools is empty at selection time and the
// requires_tools gate strips every tool-gated atom (all the CodeDOM
// guidance) from a subagent that is then handed exactly those tools.
func TestSpawnerFixPromptCompiledAgainstItsTools(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	requires := codedomCoreRequires(t)

	var mu sync.Mutex
	var compiledTools []string
	var included []string
	jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		// The real selector's rule (atomToolSatisfied, selector.go): a
		// tool-gated atom is included only when the compile-time catalog
		// holds every required tool. The stub applies the same rule so
		// the test shows what the real compile would select.
		available := make(map[string]struct{}, len(cc.AvailableTools))
		for _, tool := range cc.AvailableTools {
			available[tool] = struct{}{}
		}
		satisfied := true
		for _, tool := range requires {
			if _, ok := available[tool]; !ok {
				satisfied = false
				break
			}
		}
		mu.Lock()
		compiledTools = slices.Clone(cc.AvailableTools)
		included = nil
		atoms := []*prompt.PromptAtom{{ID: "methodology/editing_discipline"}}
		if satisfied {
			included = []string{"capability/codedom_core"}
			atoms = append(atoms, &prompt.PromptAtom{ID: "capability/codedom_core"})
		}
		mu.Unlock()
		return &prompt.CompilationResult{Prompt: "system", IncludedAtoms: atoms}, nil
	}}

	spawner := NewSpawner(k, &MockVirtualStore{}, &MockLLMClient{}, jit,
		&MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name: "coder", Task: "fix the thing", Type: SubAgentTypeEphemeral, IntentVerb: "/fix",
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	allowed := agent.config.EffectiveAgentRuntimeConfig.AllowedTools

	for _, tool := range requires {
		if !slices.Contains(allowed, tool) {
			t.Fatalf("AllowedTools = %v, want %q: the subagent was not handed the atom's tools", allowed, tool)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, tool := range requires {
		if !slices.Contains(compiledTools, tool) {
			t.Errorf("the /fix prompt compiled without %q (compiled catalog: %v) while AllowedTools=%v carries it: capability/codedom_core was gated out of a prompt for tools the subagent holds",
				tool, compiledTools, allowed)
		}
	}
	if !slices.Contains(included, "capability/codedom_core") {
		t.Errorf("compiled prompt lacks capability/codedom_core although AllowedTools=%v holds %v", allowed, requires)
	}
	if len(compiledTools) != len(allowed) {
		t.Errorf("compiled catalog %v drifted from AllowedTools %v: one derivation must feed both", compiledTools, allowed)
	}
}
