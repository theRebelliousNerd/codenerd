package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/prompt"
)

// The agent generator writes methodology that depends on the agent's
// identity atom (internal/init/agents.go). Identity is never lent, so a lent
// methodology arrived with a prerequisite no source supplied and the resolver
// pruned it: an ordinary turn that pushed the expert got none of its
// guidance. A lent atom now drops its edges to the unlent identity, and the
// compile carries the methodology, still without the expert's identity.
func TestBootLentExpertGuidanceSurvivesItsIdentityDependency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	const yaml = `- id: "lendexpert/identity"
  category: "identity"
  subcategory: "lendexpert"
  is_mandatory: true
  description: "Identity for LendExpert"
  content: |
    LEND_IDENTITY_MARKER You are LendExpert.
- id: "lendexpert/methodology"
  category: "methodology"
  subcategory: "lendexpert"
  is_mandatory: true
  depends_on: ["lendexpert/identity"]
  description: "Methodology for LendExpert"
  content: |
    LEND_METHOD_MARKER Read the policy table before changing a rule.
`
	ws := t.TempDir()
	agentDir := filepath.Join(ws, ".nerd", "agents", "LendExpert")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAgentsJSON(t, ws, []map[string]any{{"name": "LendExpert", "type": "persistent", "status": "ready", "domains": []string{"mangle"}}})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	result, err := cortex.JITCompiler.Compile(ctx, &prompt.CompilationContext{
		IntentVerb: "/fix", IntentTarget: "internal/x/policy.mg", OperationalMode: "/active",
		ShardType: "/coder", ShardID: "coder", Language: "/mangle", TokenBudget: 64000,
		Experts: []string{"lendexpert"},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(result.Prompt, "LEND_METHOD_MARKER") {
		t.Fatal("the lent methodology was pruned for its dependency on the expert's identity")
	}
	if strings.Contains(result.Prompt, "LEND_IDENTITY_MARKER") {
		t.Fatal("the expert's identity was lent with its methodology")
	}
}
