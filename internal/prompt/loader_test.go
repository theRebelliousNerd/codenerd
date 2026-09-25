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

	t.Logf("Prompts successfully loaded into unified knowledge DB at %s", kbPath)
}

func TestEnsureSchema_FreshDatabaseHasEveryColumn(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "schema_columns_test.db")
	ctx := context.Background()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	loader := NewAtomLoader(nil)
	if err := loader.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(prompt_atoms)`)
	if err != nil {
		t.Fatalf("Failed to read table info: %v", err)
	}
	defer rows.Close()

	colTypes := map[string]string{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("Failed to scan table info: %v", err)
		}
		colTypes[name] = ctype
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("Failed to iterate table info: %v", err)
	}

	wantColumns := []string{
		"atom_id", "version", "content", "token_count", "content_hash",
		"description", "content_concise", "content_min",
		"category", "subcategory", "priority",
		"is_mandatory", "is_exclusive",
		"depends_on", "conflicts_with",
		"embedding", "embedding_task", "embedding_model",
		"source_file", "created_at",
	}
	for _, col := range wantColumns {
		if _, ok := colTypes[col]; !ok {
			t.Errorf("Missing column %q in fresh prompt_atoms schema (have %v)", col, colTypes)
		}
	}

	if got, ok := colTypes["embedding_model"]; ok && got != "TEXT" {
		t.Errorf("embedding_model declared type = %q, want %q", got, "TEXT")
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "clean alphanumeric",
			input:    "description",
			expected: true,
		},
		{
			name:     "with underscores",
			input:    "content_concise",
			expected: true,
		},
		{
			name:     "with SQL injection attempt",
			input:    "content_min\"; DROP TABLE prompt_atoms; --",
			expected: false,
		},
		{
			name:     "with spaces",
			input:    "col name",
			expected: false,
		},
		{
			name:     "with special chars",
			input:    "col@name#123",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
