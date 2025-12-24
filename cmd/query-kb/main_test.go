package main

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestQueryDBOutput(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatalf("failed to create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		t.Fatalf("failed to create vectors: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'concept', 'content', 0.9)`); err != nil {
		t.Fatalf("failed to insert knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (1, 'vec', '{}')`); err != nil {
		t.Fatalf("failed to insert vectors: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	output := captureStdout(func() {
		queryDB(dbPath, 1)
	})

	if !strings.Contains(output, "Tables:") {
		t.Fatalf("expected tables output")
	}
	if !strings.Contains(output, "Total knowledge_atoms: 1") {
		t.Fatalf("expected knowledge atom count")
	}
	if !strings.Contains(output, "Total vectors: 1") {
		t.Fatalf("expected vector count")
	}
}

func captureStdout(fn func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}
