package system

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"codenerd/internal/prompt"
)

// The specialist roster a compile advertises is the booted workspace's,
// whatever directory the process runs in. The test process runs in
// internal/system, which has no .nerd/agents.json; the workspace's registry
// names a ready agent no other source names. Before, the compiler resolved a
// blank roster workspace through os.Getwd: a session booted on its project
// root but started from a subdirectory advertised only the built-in shards
// (audit specialist-roster-workspace-identity).
func TestCompiledRosterIsTheBootedWorkspaces(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	writeAgentsJSON(t, ws, []map[string]any{{
		"name": "UniqueRosterExpert", "type": "persistent", "status": "ready",
	}})
	cwd, err := os.Getwd()
	if err != nil || strings.HasPrefix(cwd, ws) {
		t.Fatalf("the process must not run in the workspace: cwd %q (%v)", cwd, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	// The chat articulation's consumer need serves the stock
	// knowledge-discovery atom, which interpolates the roster.
	result, err := cortex.JITCompiler.Compile(ctx, &prompt.CompilationContext{
		IntentVerb:      "/explain",
		OperationalMode: "/active",
		TokenBudget:     64000,
		DerivedNeeds:    []string{"envelope_knowledge_requests"},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(result.Prompt, "Available Specialists") {
		t.Fatalf("the knowledge-discovery atom was not compiled; prompt:\n%.400s", result.Prompt)
	}
	if !strings.Contains(result.Prompt, "UniqueRosterExpert") {
		t.Errorf("the compiled roster omits the workspace's ready agent; it read the process directory's registry")
	}
}
