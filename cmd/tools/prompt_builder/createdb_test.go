package main

import (
	"path/filepath"
	"testing"
)

func TestCreateDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "atoms.db")
	db, err := createDatabase(dbPath)
	if err != nil {
		t.Fatalf("createDatabase: %v", err)
	}
	defer db.Close()

	// The prompt_atoms table must exist with the expected key columns.
	var name string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='prompt_atoms'`).Scan(&name)
	if err != nil {
		t.Fatalf("prompt_atoms table not created: %v", err)
	}

	// A row insert/select round trip confirms the schema is usable.
	if _, err := db.Exec(
		`INSERT INTO prompt_atoms (atom_id, content, token_count, content_hash, category) VALUES (?,?,?,?,?)`,
		"a1", "hello", 3, "deadbeef", "identity",
	); err != nil {
		t.Fatalf("insert into prompt_atoms: %v", err)
	}
	var content string
	if err := db.QueryRow(`SELECT content FROM prompt_atoms WHERE atom_id='a1'`).Scan(&content); err != nil {
		t.Fatalf("select back: %v", err)
	}
	if content != "hello" {
		t.Errorf("round-tripped content=%q, want hello", content)
	}

	// The atom_context_tags table should also exist.
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='atom_context_tags'`).Scan(&name); err != nil {
		t.Errorf("atom_context_tags table not created: %v", err)
	}
}

func TestCreateDatabase_OverwritesExisting(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "atoms.db")
	// First creation seeds a row.
	db1, err := createDatabase(dbPath)
	if err != nil {
		t.Fatalf("first createDatabase: %v", err)
	}
	_, _ = db1.Exec(`INSERT INTO prompt_atoms (atom_id, content, token_count, content_hash, category) VALUES ('x','c',1,'h','identity')`)
	db1.Close()

	// Re-creating the database removes the existing file, so the row is gone.
	db2, err := createDatabase(dbPath)
	if err != nil {
		t.Fatalf("second createDatabase: %v", err)
	}
	defer db2.Close()
	var count int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM prompt_atoms`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected a fresh empty table after re-create, got %d rows", count)
	}
}

func TestGetAPIKey_FromEnv(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "pb-key-456")
	if got := getAPIKey(); got != "pb-key-456" {
		t.Errorf("getAPIKey()=%q, want pb-key-456", got)
	}
}
