package autopoiesis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestOrchestrator_AgentLifecycle exercises the file-backed agent store end to
// end: write a spec (with memory + triggers), list it, fetch it, append a
// learning to its memory, then delete it.
func TestOrchestrator_AgentLifecycle(t *testing.T) {
	dir := t.TempDir()
	o := &Orchestrator{config: Config{AgentsDir: dir}}

	spec := &AgentSpec{
		Name:         "reviewer",
		Type:         "code_reviewer",
		Purpose:      "review diffs",
		SystemPrompt: "You are a careful reviewer.",
		Triggers:     []TriggerSpec{{Type: "git_event", Pattern: "push"}},
		Memory:       MemorySpec{Enabled: true},
	}

	if err := o.writeAgentSpec(spec); err != nil {
		t.Fatalf("writeAgentSpec: %v", err)
	}

	// The spec, prompt, memory and triggers files should all exist on disk.
	for _, rel := range []string{
		"reviewer/agent.json",
		"reviewer/system_prompt.md",
		"reviewer/memory/memory.json",
		"reviewer/triggers.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}

	// ListAgents finds the written agent.
	agents, err := o.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 || agents[0].Name != "reviewer" {
		t.Fatalf("ListAgents=%+v, want one 'reviewer'", agents)
	}

	// GetAgent round-trips the spec.
	got, err := o.GetAgent("reviewer")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.Type != "code_reviewer" || got.SystemPrompt != spec.SystemPrompt {
		t.Errorf("GetAgent mismatch: %+v", got)
	}
	if _, err := o.GetAgent("missing"); err == nil {
		t.Error("GetAgent(missing) should error")
	}

	// UpdateAgentMemory appends a learning to the persisted memory.
	if err := o.UpdateAgentMemory("reviewer", Learning{ID: "L1", Type: "feedback", Content: "prefer table tests"}); err != nil {
		t.Fatalf("UpdateAgentMemory: %v", err)
	}
	memData, err := os.ReadFile(filepath.Join(dir, "reviewer", "memory", "memory.json"))
	if err != nil {
		t.Fatal(err)
	}
	var mem AgentMemory
	if err := json.Unmarshal(memData, &mem); err != nil {
		t.Fatal(err)
	}
	if len(mem.Learnings) != 1 || mem.Learnings[0].ID != "L1" {
		t.Errorf("memory learnings=%+v, want one with ID L1", mem.Learnings)
	}

	// DeleteAgent removes the agent directory; a subsequent list is empty.
	if err := o.DeleteAgent("reviewer"); err != nil {
		t.Fatalf("DeleteAgent: %v", err)
	}
	after, _ := o.ListAgents()
	if len(after) != 0 {
		t.Errorf("after delete ListAgents=%+v, want empty", after)
	}
}

// TestOrchestrator_ListAgents_NoDir returns an empty list (not an error) when
// the agents directory does not exist yet.
func TestOrchestrator_ListAgents_NoDir(t *testing.T) {
	o := &Orchestrator{config: Config{AgentsDir: filepath.Join(t.TempDir(), "does-not-exist")}}
	agents, err := o.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents on missing dir should not error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected empty agent list, got %d", len(agents))
	}
}
