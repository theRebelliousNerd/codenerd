package prompt

import (
	"context"
	"testing"
)

// mcpControlPlaneVerbs is the fixed model-facing MCP surface. The count is the
// point: it does not change when a server with two hundred tools connects,
// which is what makes it affordable to grant on every verb.
var mcpControlPlaneVerbs = []string{
	"mcp_map", "mcp_probe", "mcp_call", "mcp_expand", "mcp_context",
}

// TestGenerate_EveryVerbGrantsTheMCPControlPlane is the assertion that MCP
// capability actually reaches an LLM prompt.
//
// Both prompt paths — the piggyback tool catalog and native function-calling
// definitions — resolve cfg.AllowedTools through tools.Global(). A verb whose
// generated config omits these names cannot see MCP at all, and the symptom is
// indistinguishable from having no servers configured.
func TestGenerate_EveryVerbGrantsTheMCPControlPlane(t *testing.T) {
	provider := NewDefaultConfigAtomProvider()
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	result := &CompilationResult{Prompt: "You are codeNERD."}

	intents := provider.RegisteredIntents()
	if len(intents) == 0 {
		t.Fatal("RegisteredIntents returned no verbs")
	}

	for _, intent := range intents {
		t.Run(intent, func(t *testing.T) {
			cfg, err := factory.Generate(ctx, result, intent)
			if err != nil {
				t.Fatalf("Generate(%q) error = %v", intent, err)
			}
			granted := make(map[string]bool, len(cfg.AllowedTools))
			for _, name := range cfg.AllowedTools {
				granted[name] = true
			}
			for _, verb := range mcpControlPlaneVerbs {
				if !granted[verb] {
					t.Errorf("verb %q does not grant %s; MCP is unreachable from this intent",
						intent, verb)
				}
			}
		})
	}
}

// TestGenerate_MCPSurfaceStaysFixed pins the size of the grant.
//
// The control plane is affordable on every verb precisely because it is five
// tools rather than the connected catalog. If this count starts growing, the
// standing per-turn cost has started scaling again and the design has quietly
// reverted to the thing it replaced.
func TestGenerate_MCPSurfaceStaysFixed(t *testing.T) {
	provider := NewDefaultConfigAtomProvider()
	factory := NewConfigFactory(provider)

	cfg, err := factory.Generate(context.Background(),
		&CompilationResult{Prompt: "You are codeNERD."}, "/fix")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	count := 0
	for _, name := range cfg.AllowedTools {
		if len(name) > 4 && name[:4] == "mcp_" {
			count++
		}
	}
	if count != len(mcpControlPlaneVerbs) {
		t.Errorf("granted %d mcp_* tools, want exactly %d; the facade must stay fixed-size",
			count, len(mcpControlPlaneVerbs))
	}
}
