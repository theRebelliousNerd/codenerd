package session

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// A failed catalog derivation fail-closes both consumers: the subagent gets
// no tools, and its prompt compiled against none (so no tool-gated guidance
// reaches a model that cannot act on it).
func TestSpawnerCatalogFailureFailClosesPromptAndConfig(t *testing.T) {
	var compiledTools []string
	jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		compiledTools = slices.Clone(cc.AvailableTools)
		return &prompt.CompilationResult{Prompt: "system"}, nil
	}}
	spawner := NewSpawner(&MockKernel{QueryError: errors.New("unavailable")},
		&MockVirtualStore{}, &MockLLMClient{}, jit,
		&MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name: "coder", Task: "fix the thing", Type: SubAgentTypeEphemeral, IntentVerb: "/fix",
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if len(compiledTools) != 0 {
		t.Errorf("failed derivation compiled the prompt against %v; fail-closed means none", compiledTools)
	}
	if allowed := agent.config.EffectiveAgentRuntimeConfig.AllowedTools; len(allowed) != 0 {
		t.Errorf("failed derivation granted %v; fail-closed means no tools", allowed)
	}
}

// countingKernel delegates to a real kernel and counts turn-catalog reads.
type countingKernel struct {
	types.Kernel
	catalogQueries int
}

func (k *countingKernel) Query(q string) ([]types.Fact, error) {
	if strings.HasPrefix(q, "turn_tool_allowed(") {
		k.catalogQueries++
	}
	return k.Kernel.Query(q)
}

// One derivation feeds both the prompt and the allowlist: a second query
// could answer differently and drift the two apart.
func TestSpawnerDerivesTurnCatalogOnce(t *testing.T) {
	real, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	k := &countingKernel{Kernel: real}
	var compiledTools []string
	jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		compiledTools = slices.Clone(cc.AvailableTools)
		return &prompt.CompilationResult{Prompt: "system"}, nil
	}}
	spawner := NewSpawner(k, &MockVirtualStore{}, &MockLLMClient{}, jit,
		&MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name: "coder", Task: "fix the thing", Type: SubAgentTypeEphemeral, IntentVerb: "/fix",
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if k.catalogQueries != 1 {
		t.Errorf("spawn issued %d turn_tool_allowed queries, want 1: one derivation feeds prompt and allowlist", k.catalogQueries)
	}
	allowed := agent.config.EffectiveAgentRuntimeConfig.AllowedTools
	if len(compiledTools) == 0 || len(allowed) == 0 {
		t.Fatalf("empty catalog proves nothing: compiled=%v allowed=%v", compiledTools, allowed)
	}
	for _, tool := range allowed {
		if !slices.Contains(compiledTools, tool) {
			t.Errorf("AllowedTools carries %q the prompt never saw; the two drifted", tool)
		}
	}
}
