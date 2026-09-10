package autopoiesis

import (
	"codenerd/internal/atomicfile"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// TestAgentMemoryUpdateSurvivesAnInterruptedWrite pins why the writeFile choke
// point is atomic.
//
// UpdateAgentMemory is a read-modify-write over memory.json whose read treats a
// parse failure as a hard error -- correctly, since half a memory is not a
// memory. A truncating write interrupted partway therefore does not lose one
// learning: it permanently wedges that agent's memory, because every later
// update fails on the file it cannot parse.
func TestAgentMemoryUpdateSurvivesAnInterruptedWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	original := []byte(`{"learnings":[],"updated_at":"2026-01-01T00:00:00Z"}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// atomicfile.Open, not os.Open: this stands in for a reader holding the
	// file across the write, and on Windows a handle without FILE_SHARE_DELETE
	// blocks the replace outright. That is the contract this package exists to
	// provide, so the test states it rather than working around it.
	held, err := atomicfile.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = held.Close() }()

	// Deliberately larger, so a truncate-then-write interrupted midway could
	// not fit back what it had already destroyed.
	replacement := []byte(`{"learnings":[{"note":"` + strings.Repeat("x", 4096) + `"}]}`)
	if err := writeFile(path, replacement); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if runtime.GOOS != "windows" && os.SameFile(before, after) {
		t.Error("the write went through the existing inode; an interrupted write would leave " +
			"memory.json unparseable and every later update would fail on it")
	}

	buf := make([]byte, 8192)
	n, _ := held.Read(buf)
	if string(buf[:n]) != string(original) {
		t.Errorf("a reader holding the file saw %q, want the contents it opened", buf[:n])
	}

	// And the new contents are actually there.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(replacement) {
		t.Error("the replacement did not land in full")
	}
}
