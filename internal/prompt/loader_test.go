package prompt

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestEnsurePromptAtomsTable(t *testing.T) {
	// Create temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_knowledge.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// Test EnsureSchema
	loader := NewAtomLoader(nil)
	if err := loader.EnsureSchema(context.Background(), db); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify table was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='prompt_atoms'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tables: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 prompt_atoms table, got %d", count)
	}
}

func TestLoadAgentPromptsUnifiedStorage(t *testing.T) {
	// Create temp workspace
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	shardsDir := filepath.Join(nerdDir, "shards")
	agentsDir := filepath.Join(nerdDir, "agents", "testAgent")

	// Create directories
	if err := os.MkdirAll(shardsDir, 0755); err != nil {
		t.Fatalf("Failed to create shards dir: %v", err)
	}
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}

	// Create a knowledge DB (must execute something to create the file)
	kbPath := filepath.Join(shardsDir, "testagent_knowledge.db")
	db, err := sql.Open("sqlite3", kbPath)
	if err != nil {
		t.Fatalf("Failed to create knowledge DB: %v", err)
	}
	// Execute something to actually create the file
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS knowledge_atoms (id INTEGER PRIMARY KEY)")
	if err != nil {
		db.Close()
		t.Fatalf("Failed to initialize knowledge DB: %v", err)
	}
	db.Close()

	// Create a test prompts.yaml
	promptsYAML := `- id: test/atom
  category: identity
  priority: 100
  is_mandatory: true
  content: "This is a test atom"
`
	promptsPath := filepath.Join(agentsDir, "prompts.yaml")
	if err := os.WriteFile(promptsPath, []byte(promptsYAML), 0644); err != nil {
		t.Fatalf("Failed to write prompts.yaml: %v", err)
	}

	// Test LoadAgentPrompts
	ctx := context.Background()
	count, err := LoadAgentPrompts(ctx, "testAgent", nerdDir, nil)
	if err != nil {
		t.Fatalf("LoadAgentPrompts failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 atom loaded, got %d", count)
	}

	// Verify atom was stored in knowledge DB
	db, err = sql.Open("sqlite3", kbPath)
	if err != nil {
		t.Fatalf("Failed to open knowledge DB: %v", err)
	}
	defer db.Close()

	var atomCount int
	err = db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&atomCount)
	if err != nil {
		t.Fatalf("Failed to query prompt_atoms: %v", err)
	}

	if atomCount != 1 {
		t.Errorf("Expected 1 atom in DB, got %d", atomCount)
	}

	// Verify atom content
	var atomID, content string
	err = db.QueryRow("SELECT atom_id, content FROM prompt_atoms WHERE atom_id = ?", "test/atom").Scan(&atomID, &content)
	if err != nil {
		t.Fatalf("Failed to query atom: %v", err)
	}

	if atomID != "test/atom" {
		t.Errorf("Expected atom_id 'test/atom', got '%s'", atomID)
	}

	if content != "This is a test atom" {
		t.Errorf("Expected content 'This is a test atom', got '%s'", content)
	}

	t.Logf("✓ Prompts successfully loaded into unified knowledge DB at %s", kbPath)
}
