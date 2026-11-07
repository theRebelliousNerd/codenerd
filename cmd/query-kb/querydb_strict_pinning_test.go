package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestQueryDBPinsLimitIsEnforced(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kb.db")
	db := pinOpenAndSeed(t, dbPath)
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		t.Fatalf("create vectors: %v", err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (?, ?, ?, ?)`, i, "c", "body", 1.0); err != nil {
			t.Fatalf("insert knowledge %d: %v", i, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 2, &buf)
	out := buf.String()
	if !strings.Contains(out, "Total knowledge_atoms: 5") {
		t.Errorf("expected total 5 regardless of limit, got:\n%s", out)
	}
	// Sample data section should show exactly 2 rows when limit=2.
	// Rows are printed as "1. " and "2. " prefixes.
	if !strings.Contains(out, "1. ") || !strings.Contains(out, "2. ") {
		t.Errorf("expected 2 sample rows with limit=2, got:\n%s", out)
	}
	if strings.Contains(out, "3. ") {
		t.Errorf("limit=2 was ignored, third sample row present:\n%s", out)
	}
	if !strings.Contains(out, "─────────────────────────────────────────────────────────────") {
		t.Errorf("missing sample separator line:\n%s", out)
	}
}

func TestQueryDBPinsVectorSamplesCapped(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kb.db")
	db := pinOpenAndSeed(t, dbPath)
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1,'c','hi',1.0)`); err != nil {
		t.Fatalf("insert knowledge: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		t.Fatalf("create vectors: %v", err)
	}
	for i := 1; i <= 7; i++ {
		if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (?, ?, ?)`, i, "v", "{}"); err != nil {
			t.Fatalf("insert vector %d: %v", i, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 10, &buf)
	out := buf.String()
	if !strings.Contains(out, "Total vectors: 7") {
		t.Errorf("expected total vectors 7, got:\n%s", out)
	}
	if !strings.Contains(out, "Vector samples:") {
		t.Fatalf("missing Vector samples section:\n%s", out)
	}
	// Only first 5 vectors are sampled (hardcoded LIMIT 5).
	for _, id := range []string{"  1: ", "  2: ", "  3: ", "  4: ", "  5: "} {
		if !strings.Contains(out, id) {
			t.Errorf("expected vector sample %q, got:\n%s", id, out)
		}
	}
	if strings.Contains(out, "  6: ") || strings.Contains(out, "  7: ") {
		t.Errorf("vector samples not capped at 5:\n%s", out)
	}
}

func TestQueryDBPinsNonDatabaseFileError(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "notadb.db")
	if err := os.WriteFile(badPath, []byte("this is not a sqlite database"), 0644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	var buf bytes.Buffer
	queryDB(badPath, 10, &buf)
	out := buf.String()
	if !strings.Contains(out, "Error querying tables") {
		t.Errorf("expected Error querying tables for non-DB file, got:\n%s", out)
	}
}
