package system

import (
	"os"
	"path/filepath"
	"testing"
)

// --- DiscoverAgentsOnDisk ---

func TestDiscoverAgentsOnDisk_WhenEmptyWorkspace_ShouldError(t *testing.T) {
	_, err := DiscoverAgentsOnDisk("")
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestDiscoverAgentsOnDisk_WhenWhitespaceWorkspace_ShouldError(t *testing.T) {
	_, err := DiscoverAgentsOnDisk("   ")
	if err == nil {
		t.Fatal("expected error for whitespace workspace")
	}
}

func TestDiscoverAgentsOnDisk_WhenNonExistentDir_ShouldReturnNil(t *testing.T) {
	dir := t.TempDir()
	// Don't create .nerd/agents/
	agents, err := DiscoverAgentsOnDisk(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agents != nil {
		t.Errorf("expected nil agents for nonexistent dir, got %v", agents)
	}
}

func TestDiscoverAgentsOnDisk_WhenEmptyAgentsDir_ShouldReturnEmpty(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".nerd", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	agents, err := DiscoverAgentsOnDisk(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestDiscoverAgentsOnDisk_WhenAgentWithPrompts_ShouldDiscover(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, ".nerd", "agents", "test-agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	agents, err := DiscoverAgentsOnDisk(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].ID != "test-agent" {
		t.Errorf("expected agent ID 'test-agent', got %q", agents[0].ID)
	}
}

func TestDiscoverAgentsOnDisk_WhenAgentWithoutPrompts_ShouldSkip(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, ".nerd", "agents", "no-prompts")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}

	agents, err := DiscoverAgentsOnDisk(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestDiscoverAgentsOnDisk_WhenFileNotDir_ShouldSkip(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".nerd", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a file instead of a directory
	if err := os.WriteFile(filepath.Join(agentsDir, "not-a-dir"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	agents, err := DiscoverAgentsOnDisk(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestDiscoverAgentsOnDisk_WhenMultipleAgents_ShouldSortByName(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zebra", "alpha", "middle"} {
		agentDir := filepath.Join(dir, ".nerd", "agents", name)
		if err := os.MkdirAll(agentDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte("name: "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	agents, err := DiscoverAgentsOnDisk(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(agents))
	}
	if agents[0].ID != "alpha" {
		t.Errorf("expected first agent 'alpha', got %q", agents[0].ID)
	}
	if agents[1].ID != "middle" {
		t.Errorf("expected second agent 'middle', got %q", agents[1].ID)
	}
	if agents[2].ID != "zebra" {
		t.Errorf("expected third agent 'zebra', got %q", agents[2].ID)
	}
}

// --- SyncAgentRegistryFromDisk ---

func TestSyncAgentRegistryFromDisk_WhenEmptyWorkspace_ShouldError(t *testing.T) {
	err := SyncAgentRegistryFromDisk("")
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestSyncAgentRegistryFromDisk_WhenNoAgents_ShouldCreateEmptyRegistry(t *testing.T) {
	dir := t.TempDir()
	err := SyncAgentRegistryFromDisk(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	registryPath := filepath.Join(dir, ".nerd", "agents.json")
	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty registry file")
	}
}

// --- SyncAgentRegistryFromDiscovered ---

func TestSyncAgentRegistryFromDiscovered_WhenEmptyWorkspace_ShouldError(t *testing.T) {
	_, err := SyncAgentRegistryFromDiscovered("", nil)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestSyncAgentRegistryFromDiscovered_WhenNewAgents_ShouldCreate(t *testing.T) {
	dir := t.TempDir()
	agents := []AgentOnDisk{
		{ID: "agent1", DBPath: filepath.Join(dir, "agent1.db")},
	}

	changed, err := SyncAgentRegistryFromDiscovered(dir, agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed=true for new agents")
	}
}

func TestSyncAgentRegistryFromDiscovered_WhenEmptyAgents_ShouldNotChange(t *testing.T) {
	dir := t.TempDir()

	// First sync with empty
	_, err := SyncAgentRegistryFromDiscovered(dir, []AgentOnDisk{})
	if err != nil {
		t.Fatalf("unexpected error on first sync: %v", err)
	}

	// Second sync should be unchanged
	changed, err := SyncAgentRegistryFromDiscovered(dir, []AgentOnDisk{})
	if err != nil {
		t.Fatalf("unexpected error on second sync: %v", err)
	}
	if changed {
		t.Error("expected no change on second sync with same empty list")
	}
}

func TestSyncAgentRegistryFromDiscovered_WhenEmptyIDAgent_ShouldSkip(t *testing.T) {
	dir := t.TempDir()
	agents := []AgentOnDisk{
		{ID: "", DBPath: filepath.Join(dir, "bad.db")},
		{ID: "   ", DBPath: filepath.Join(dir, "bad2.db")},
	}

	_, err := SyncAgentRegistryFromDiscovered(dir, agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- HolographicCodeScope ---

func TestNewHolographicCodeScope_WhenZeroWorkers_ShouldUseDefault(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 0)
	if h == nil {
		t.Fatal("expected non-nil HolographicCodeScope")
	}
	if h.deepWorkers < 2 || h.deepWorkers > 8 {
		t.Errorf("expected deepWorkers between 2 and 8, got %d", h.deepWorkers)
	}
}

func TestNewHolographicCodeScope_WhenNegativeWorkers_ShouldUseDefault(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, -5)
	if h.deepWorkers < 2 || h.deepWorkers > 8 {
		t.Errorf("expected deepWorkers between 2 and 8, got %d", h.deepWorkers)
	}
}

func TestNewHolographicCodeScope_WhenExplicitWorkers_ShouldUse(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 4)
	if h.deepWorkers != 4 {
		t.Errorf("expected deepWorkers=4, got %d", h.deepWorkers)
	}
}

func TestHolographicCodeScope_Close_ShouldNotPanic(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 2)
	h.Close() // Should not panic
}

func TestHolographicCodeScope_GetActiveFile_WhenClosed_ShouldReturnEmpty(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 2)
	if h.GetActiveFile() != "" {
		t.Errorf("expected empty active file before opening, got %q", h.GetActiveFile())
	}
}

func TestHolographicCodeScope_GetInScopeFiles_WhenClosed_ShouldReturnEmpty(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 2)
	files := h.GetInScopeFiles()
	if len(files) != 0 {
		t.Errorf("expected 0 in-scope files before opening, got %d", len(files))
	}
}

func TestHolographicCodeScope_IsInScope_WhenClosed_ShouldReturnFalse(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 2)
	if h.IsInScope("anything.go") {
		t.Error("expected nothing to be in scope before opening")
	}
}

func TestHolographicCodeScope_ScopeFacts_WhenClosed_ShouldReturnNil(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 2)
	facts := h.ScopeFacts()
	if len(facts) != 0 {
		t.Errorf("expected 0 scope facts before opening, got %d", len(facts))
	}
}

func TestHolographicCodeScope_Open_WhenNonexistentFile_ShouldError(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 2)
	err := h.Open(filepath.Join(dir, "nonexistent.go"))
	if err == nil {
		t.Error("expected error opening nonexistent file")
	}
}

func TestHolographicCodeScope_RefreshWithRetry_WhenSucceeds_ShouldReturnNil(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 2)
	// Refresh with no open files should succeed
	err := h.RefreshWithRetry(3)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestHolographicCodeScope_EnsureDeepFacts_WhenNilKernel_ShouldNoOp(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 2)
	// Should not panic with nil kernel
	h.ensureDeepFacts(nil, []string{"some/file.go"})
}

func TestHolographicCodeScope_EnsureDeepFacts_WhenEmptyPaths_ShouldNoOp(t *testing.T) {
	dir := t.TempDir()
	h := NewHolographicCodeScope(dir, nil, nil, 2)
	// Should not panic with empty paths
	h.ensureDeepFacts(nil, nil)
	h.ensureDeepFacts(nil, []string{})
}

// --- Cortex.Close ---

func TestCortexClose_WhenNil_ShouldReturnNil(t *testing.T) {
	var c *Cortex
	err := c.Close()
	if err != nil {
		t.Errorf("expected nil error for nil Cortex, got %v", err)
	}
}
