package prompt

import (
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
// definitions — resolve the turn's allowlist through tools.Global(). That
// allowlist is DeriveTurnTools. A verb whose envelope omits these names
// cannot see MCP at all, and the symptom is indistinguishable from having
// no servers configured.
func TestGenerate_EveryVerbGrantsTheMCPControlPlane(t *testing.T) {
	provider := NewDefaultConfigAtomProvider()
	intents := provider.RegisteredIntents()
	if len(intents) == 0 {
		t.Fatal("RegisteredIntents returned no verbs")
	}
	k := testTurnKernel(t)

	for _, intent := range intents {
		t.Run(intent, func(t *testing.T) {
			grantedList, err := DeriveTurnTools(k, intent)
			if err != nil {
				t.Fatalf("DeriveTurnTools(%q) error = %v", intent, err)
			}
			granted := make(map[string]bool, len(grantedList))
			for _, name := range grantedList {
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
	tools, err := DeriveTurnTools(testTurnKernel(t), "/fix")
	if err != nil {
		t.Fatalf("DeriveTurnTools: %v", err)
	}

	count := 0
	for _, name := range tools {
		if len(name) > 4 && name[:4] == "mcp_" {
			count++
		}
	}
	if count != len(mcpControlPlaneVerbs) {
		t.Errorf("granted %d mcp_* tools, want exactly %d; the facade must stay fixed-size",
			count, len(mcpControlPlaneVerbs))
	}
}
