//go:build integration

package e2e_test

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// This kernel scripts an inert mechanics envelope. It does not derive policy.
// The factory's explicit ceiling is copied before any generation can mutate it.
type sclCatalogKernel struct {
	*mockKernel
	ceiling []string
}

func (k *sclCatalogKernel) Query(query string) ([]types.Fact, error) {
	if !strings.HasPrefix(query, "turn_catalog(") || !strings.HasSuffix(query, ", Tool)") {
		return k.mockKernel.Query(query)
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	if rows, ok := k.queryResults["turn_catalog"]; ok {
		return rows, nil
	}
	if rows, ok := k.queryResults[query]; ok {
		return rows, nil
	}
	turn := strings.TrimSuffix(strings.TrimPrefix(query, "turn_catalog("), ", Tool)")
	if !strings.HasPrefix(turn, "/turn_") {
		return nil, nil
	}
	for _, fact := range k.facts {
		if fact.Predicate != "turn_verb" || len(fact.Args) != 2 || types.ExtractString(fact.Args[0]) != turn {
			continue
		}
		rows := make([]types.Fact, 0, len(k.ceiling))
		for _, name := range k.ceiling {
			rows = append(rows, types.Fact{Predicate: "turn_catalog", Args: []any{types.MangleAtom(turn), types.MangleAtom("/" + name)}})
		}
		return rows, nil
	}
	return nil, nil
}

type sclFixture struct {
	executor *session.Executor
	kernel   *sclCatalogKernel
	registry *tools.Registry
}

func newSCLFixture(t *testing.T, transducer *sclMockTransducer, compiler session.JITCompiler, factory session.ConfigFactory, model types.LLMClient, ceiling []string) *sclFixture {
	t.Helper()
	kernel := &sclCatalogKernel{mockKernel: newMockKernel(), ceiling: append([]string(nil), ceiling...)}
	registry := tools.NewRegistry()
	for _, tool := range []*tools.Tool{
		{Name: "dummy_tool", Effect: tools.EffectRead, Execute: func(context.Context, map[string]any) (string, error) { return "dummy result", nil }},
		{Name: "hanging_tool", Effect: tools.EffectRead, Execute: func(ctx context.Context, _ map[string]any) (string, error) { <-ctx.Done(); return "", ctx.Err() }},
	} {
		if err := registry.Register(tool); err != nil {
			t.Fatal(err)
		}
	}
	cfg := session.DefaultExecutorConfig()
	cfg.WorkspaceRoot = t.TempDir()
	// Preserve this existing inert fixture's scope. Real-kernel negative
	// controls keep the production safety gate enabled.
	cfg.EnableSafetyGate = false
	registry.SetWorkspaceRoot(cfg.WorkspaceRoot)
	registry.SetAllowlist(&tools.Allowlist{Enforced: true, Names: append([]string(nil), ceiling...)})
	executor := session.NewExecutor(kernel, nil, model, compiler, factory, transducer)
	executor.SetConfig(cfg)
	executor.SetModularTools(registry)
	return &sclFixture{executor: executor, kernel: kernel, registry: registry}
}

// sclResponseSnapshot mirrors call-owned native responses. Copy the scripted
// containers without JSON round-tripping: opaque Go tool values retain identity.
func sclResponseSnapshot(template *types.LLMToolResponse) *types.LLMToolResponse {
	if template == nil {
		return nil
	}
	response := *template
	response.ToolCalls = append([]types.ToolCall(nil), template.ToolCalls...)
	for i := range response.ToolCalls {
		response.ToolCalls[i].Input = sclInputSnapshot(template.ToolCalls[i].Input)
	}
	response.Blocks = append([]types.ContentBlock(nil), template.Blocks...)
	for i := range response.Blocks {
		response.Blocks[i].Input = sclInputSnapshot(template.Blocks[i].Input)
		response.Blocks[i].Raw = append([]byte(nil), template.Blocks[i].Raw...)
	}
	return &response
}

func sclInputSnapshot(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = sclValueSnapshot(value)
	}
	return out
}

func sclValueSnapshot(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return sclInputSnapshot(value)
	case []any:
		if value == nil {
			return []any(nil)
		}
		out := make([]any, len(value))
		for i := range value {
			out[i] = sclValueSnapshot(value[i])
		}
		return out
	case []byte:
		return append([]byte(nil), value...)
	default:
		return value
	}
}
