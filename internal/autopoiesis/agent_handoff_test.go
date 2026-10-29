package autopoiesis

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TODO P2: "Agent spec → runtime scheduler ownership decision (shards vs
// autopoiesis)."
//
// Decision: shards own scheduling. Chat boot runs
// system.SyncAgentRegistryFromDisk → DiscoverAgentsOnDisk →
// shardMgr.DefineProfile, so an agent directory on disk becomes a spawnable
// shard profile. Autopoiesis is the author, not a second scheduler.
//
// DiscoverAgentsOnDisk keys off prompts.yaml (internal/system/
// agent_registry.go:49 — a directory without one is skipped outright), and
// writeAgentSpec never wrote one. Every persistent agent autopoiesis created
// was unreachable from the runtime that was supposed to run it. These tests
// pin the handoff artifact.

func createAgentSpecOnDisk(t *testing.T, orch *Orchestrator, spec *AgentSpec) string {
	t.Helper()
	if err := orch.ExecuteAction(context.Background(), AutopoiesisAction{
		Type:    ActionCreateAgent,
		Payload: spec,
	}); err != nil {
		t.Fatalf("agent creation failed: %v", err)
	}
	return filepath.Join(orch.config.AgentsDir, spec.Name)
}

// definitionCall is one call the installed writer received.
type definitionCall struct {
	workspace, name, role, topics, systemPrompt string
}

// installRecordingWriter installs a writer that records its call and writes a
// minimal prompts.yaml, standing in for internal/system.WriteAgentDefinition.
func installRecordingWriter(t *testing.T, orch *Orchestrator) *[]definitionCall {
	t.Helper()
	var calls []definitionCall
	orch.SetAgentDefinitionWriter(func(_ context.Context, workspace, name, role, topics, systemPrompt string) (string, error) {
		calls = append(calls, definitionCall{workspace, name, role, topics, systemPrompt})
		dir := filepath.Join(orch.config.AgentsDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		path := filepath.Join(dir, "prompts.yaml")
		return path, os.WriteFile(path, []byte("# canonical template\n- id: \""+name+"/identity\"\n"), 0o644)
	})
	return &calls
}

// The installed writer owns prompts.yaml and receives the agent's
// model-written system prompt, which it stores as admitted methodology atoms.
func TestWriteAgentSpec_WhenAgentCreated_ShouldEmitDiscoverablePromptsYAML(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	calls := installRecordingWriter(t, orch)

	agentDir := createAgentSpecOnDisk(t, orch, &AgentSpec{
		Name:         "dep_watcher",
		Type:         "monitor",
		Purpose:      "watch go.mod for risky upgrades",
		SystemPrompt: "You monitor dependency changes and flag risk.",
	})

	if len(*calls) != 1 {
		t.Fatalf("writer called %d times, want 1", len(*calls))
	}
	got := (*calls)[0]
	want := definitionCall{orch.config.WorkspaceRoot, "dep_watcher", "monitor", "watch go.mod for risky upgrades", "You monitor dependency changes and flag risk."}
	if got != want {
		t.Errorf("writer called with %+v, want %+v", got, want)
	}
	// The file DiscoverAgentsOnDisk requires.
	if _, err := os.Stat(filepath.Join(agentDir, "prompts.yaml")); err != nil {
		t.Fatalf("no prompts.yaml: the shard runtime skips this directory entirely and the agent can never be spawned: %v", err)
	}
	// The pre-existing artifacts must survive.
	for _, name := range []string{"agent.json", "system_prompt.md"} {
		if _, err := os.Stat(filepath.Join(agentDir, name)); err != nil {
			t.Errorf("%s missing: %v", name, err)
		}
	}
}

// Without the canonical writer, creation fails and says so rather than
// writing a second, uncapped template.
func TestWriteAgentSpec_WithoutAWriter_FailsAndSaysWhy(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	err := orch.ExecuteAction(context.Background(), AutopoiesisAction{
		Type:    ActionCreateAgent,
		Payload: &AgentSpec{Name: "orphan", Type: "monitor", Purpose: "x"},
	})
	if err == nil || !strings.Contains(err.Error(), "no agent definition writer installed") {
		t.Fatalf("creation without a writer: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(orch.config.AgentsDir, "orphan", "prompts.yaml")); statErr == nil {
		t.Fatal("a prompts.yaml was written without the canonical writer")
	}
}

// An operator who edits prompts.yaml must not have it overwritten the next
// time the same agent name is created.
func TestWriteAgentSpec_WhenPromptsYAMLExists_ShouldNotOverwriteIt(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	installRecordingWriter(t, orch)
	spec := &AgentSpec{Name: "curated", Type: "reviewer", Purpose: "review diffs"}

	agentDir := createAgentSpecOnDisk(t, orch, spec)
	promptsPath := filepath.Join(agentDir, "prompts.yaml")

	const handEdited = "# hand written, do not clobber\n- id: \"curated/identity\"\n"
	if err := os.WriteFile(promptsPath, []byte(handEdited), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	createAgentSpecOnDisk(t, orch, spec)

	data, err := os.ReadFile(promptsPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != handEdited {
		t.Error("regenerating the agent clobbered an operator-edited prompts.yaml")
	}
}
