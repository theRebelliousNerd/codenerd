package sync

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/prompt"
)

func TestAgentSynchronizerSyncAllNoAgents(t *testing.T) {
	root := t.TempDir()
	loader := prompt.NewAtomLoader(nil)
	syncer := NewAgentSynchronizer(root, loader)

	if err := syncer.SyncAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syncer.GetDiscoveredAgents()) != 0 {
		t.Fatalf("expected no discovered agents")
	}
}

func TestAgentSynchronizerSyncAll(t *testing.T) {
	root := t.TempDir()
	agentDir := filepath.Join(root, ".nerd", "agents", "AgentOne")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create agent dir: %v", err)
	}

	yaml := `- id: agent-one-atom
  category: identity
  content: "You are agent one."
`
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write prompts.yaml: %v", err)
	}

	loader := prompt.NewAtomLoader(nil)
	syncer := NewAgentSynchronizer(root, loader)

	if err := syncer.SyncAll(context.Background()); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	agents := syncer.GetDiscoveredAgents()
	if len(agents) != 1 {
		t.Fatalf("expected 1 discovered agent, got %d", len(agents))
	}
	if agents[0].ID != "AgentOne" {
		t.Fatalf("unexpected agent ID: %s", agents[0].ID)
	}

	expectedDB := filepath.Join(root, ".nerd", "shards", "agentone_knowledge.db")
	if agents[0].DBPath != expectedDB {
		t.Fatalf("unexpected db path: %s", agents[0].DBPath)
	}
	if _, err := os.Stat(expectedDB); err != nil {
		t.Fatalf("expected db to exist: %v", err)
	}

	db, err := sql.Open("sqlite3", expectedDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&count); err != nil {
		t.Fatalf("failed to query atom count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 atom in db, got %d", count)
	}
}

func TestAgentSynchronizerSyncAll_PrunesRemovedAtoms(t *testing.T) {
	root := t.TempDir()
	agentDir := filepath.Join(root, ".nerd", "agents", "AgentOne")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create agent dir: %v", err)
	}

	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	initialYAML := `- id: agent-one-atom
  category: identity
  content: "You are agent one."
- id: stale-atom
  category: methodology
  content: "This atom should be pruned."
`
	if err := os.WriteFile(promptsPath, []byte(initialYAML), 0644); err != nil {
		t.Fatalf("failed to write initial prompts.yaml: %v", err)
	}

	loader := prompt.NewAtomLoader(nil)
	syncer := NewAgentSynchronizer(root, loader)
	if err := syncer.SyncAll(context.Background()); err != nil {
		t.Fatalf("initial sync failed: %v", err)
	}

	updatedYAML := `- id: agent-one-atom
  category: identity
  content: "You are agent one, updated."
`
	if err := os.WriteFile(promptsPath, []byte(updatedYAML), 0644); err != nil {
		t.Fatalf("failed to write updated prompts.yaml: %v", err)
	}

	if err := syncer.SyncAll(context.Background()); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	expectedDB := filepath.Join(root, ".nerd", "shards", "agentone_knowledge.db")
	db, err := sql.Open("sqlite3", expectedDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&count); err != nil {
		t.Fatalf("failed to query atom count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 atom after prune, got %d", count)
	}

	var content string
	if err := db.QueryRow("SELECT content FROM prompt_atoms WHERE atom_id = 'agent-one-atom'").Scan(&content); err != nil {
		t.Fatalf("failed to query updated atom: %v", err)
	}
	if content != "You are agent one, updated." {
		t.Fatalf("unexpected updated content: %q", content)
	}

	var staleCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms WHERE atom_id = 'stale-atom'").Scan(&staleCount); err != nil {
		t.Fatalf("failed to query stale atom: %v", err)
	}
	if staleCount != 0 {
		t.Fatalf("expected stale atom to be pruned, got %d rows", staleCount)
	}
}
