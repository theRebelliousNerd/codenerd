package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/experts"
)

// An agent defined mid-session joins the running roster: before activation a
// fix on a policy file needs no one; after the wizard (or autopoiesis) writes
// the agent and the session activates it, its KB is registered with the JIT
// and the same fix needs it -- no reboot.
func TestAgentDefinedMidSessionJoinsTheRoster(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	task := experts.Task{Verb: "/fix", Targets: []string{"internal/x/policy.mg"}, Workspace: ws}
	if sel, err := experts.Select(cortex.Kernel, task); err != nil || len(sel.Needs) != 0 {
		t.Fatalf("before the agent exists the fix needs %v (%v)", sel.Agents(), err)
	}

	dir := filepath.Join(ws, ".nerd", "agents", "PolicyExpert")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yml := "- id: \"policyexpert/domain/policy\"\n  category: \"domain\"\n  subcategory: \"policyexpert\"\n  is_mandatory: false\n  languages: [\"/mangle\"]\n  description: \"Policy files\"\n  content: |\n    Policy files load in order.\n"
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cortex.ActivateAgent(ctx, "PolicyExpert"); err != nil {
		t.Fatalf("ActivateAgent: %v", err)
	}
	if _, ok := cortex.JITCompiler.LookupShardDB("policyexpert"); !ok {
		t.Error("the activated agent's KB is not registered with the JIT compiler")
	}
	sel, err := experts.Select(cortex.Kernel, task)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got := sel.Agents(); len(got) != 1 || got[0] != "policyexpert" {
		t.Errorf("after activation the fix needs %v, want [policyexpert]", got)
	}
}
