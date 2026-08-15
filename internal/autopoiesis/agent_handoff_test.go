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

func TestWriteAgentSpec_WhenAgentCreated_ShouldEmitDiscoverablePromptsYAML(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)

	agentDir := createAgentSpecOnDisk(t, orch, &AgentSpec{
		Name:         "dep_watcher",
		Type:         "monitor",
		Purpose:      "watch go.mod for risky upgrades",
		SystemPrompt: "You monitor dependency changes and flag risk.",
	})

	// The file DiscoverAgentsOnDisk requires.
	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	data, err := os.ReadFile(promptsPath)
	if err != nil {
		t.Fatalf("no prompts.yaml: the shard runtime skips this directory entirely and the agent can never be spawned: %v", err)
	}
	content := string(data)

	// The fields cmd_advanced.go:parseDreamAgentMetaContent scans for when
	// scoring agents for dream-state relevance.
	if !strings.Contains(content, "Role: monitor") {
		t.Errorf("prompts.yaml has no Role line; dream-agent scoring reads it:\n%s", content)
	}
	if !strings.Contains(content, "Topics: watch go.mod for risky upgrades") {
		t.Errorf("prompts.yaml has no Topics line:\n%s", content)
	}
	if !strings.Contains(content, "dep_watcher/identity") {
		t.Errorf("prompts.yaml has no identity atom:\n%s", content)
	}

	// The pre-existing artifacts must survive.
	for _, name := range []string{"agent.json", "system_prompt.md"} {
		if _, err := os.Stat(filepath.Join(agentDir, name)); err != nil {
			t.Errorf("%s missing: %v", name, err)
		}
	}
}

// An operator who edits prompts.yaml must not have it overwritten the next
// time the same agent name is created.
func TestWriteAgentSpec_WhenPromptsYAMLExists_ShouldNotOverwriteIt(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
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

// Boot installs the canonical writer from internal/system, which cannot be
// imported here without a cycle. Prove the seam is actually used.
func TestSetAgentDefinitionWriter_WhenInstalled_ShouldOwnPromptsYAML(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)

	var gotWorkspace, gotName, gotRole, gotTopics string
	orch.SetAgentDefinitionWriter(func(workspace, name, role, topics string) (string, error) {
		gotWorkspace, gotName, gotRole, gotTopics = workspace, name, role, topics
		dir := filepath.Join(orch.config.AgentsDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		path := filepath.Join(dir, "prompts.yaml")
		return path, os.WriteFile(path, []byte("# canonical template\n"), 0o644)
	})

	agentDir := createAgentSpecOnDisk(t, orch, &AgentSpec{
		Name:    "canonical_agent",
		Type:    "researcher",
		Purpose: "gather docs",
	})

	if gotName != "canonical_agent" || gotRole != "researcher" || gotTopics != "gather docs" {
		t.Errorf("writer called with name=%q role=%q topics=%q", gotName, gotRole, gotTopics)
	}
	if gotWorkspace != orch.config.WorkspaceRoot {
		t.Errorf("writer got workspace %q, want %q", gotWorkspace, orch.config.WorkspaceRoot)
	}
	data, err := os.ReadFile(filepath.Join(agentDir, "prompts.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "canonical template") {
		t.Error("the installed writer did not own the file; the fallback ran instead")
	}
}
