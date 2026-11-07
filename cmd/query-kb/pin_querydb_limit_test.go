package main

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestPinQueryDBLimitEnforcedOnAtoms(t *testing.T) {
	dbPath := t.TempDir() + "/kb.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create vectors: %v", err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (?, ?, ?, 0.5)`, i, strings.Repeat("c", i)+"concept", "content"); err != nil {
			db.Close()
			t.Fatalf("insert atoms: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 2, &buf)
	out := buf.String()
	if !strings.Contains(out, "1. ") || !strings.Contains(out, "2. ") {
		t.Fatalf("expected first two sample rows, got: %q", out)
	}
	if strings.Contains(out, "3. ") {
		t.Fatalf("expected LIMIT 2 to suppress third sample row, got: %q", out)
	}
	if !strings.Contains(out, "Total knowledge_atoms: 5") {
		t.Fatalf("expected total count 5 despite limit, got: %q", out)
	}
}

func TestPinQueryDBLimitEnforcedOnVectors(t *testing.T) {
	dbPath := t.TempDir() + "/kb.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'c', 'x', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create vectors: %v", err)
	}
	contents := []string{"alpha-one", "alpha-two", "alpha-three", "alpha-four", "alpha-five"}
	for i, c := range contents {
		if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (?, ?, '{}')`, i+1, c); err != nil {
			db.Close()
			t.Fatalf("insert vectors: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 2, &buf)
	out := buf.String()
	if !strings.Contains(out, "alpha-one") || !strings.Contains(out, "alpha-two") {
		t.Fatalf("expected first two vector samples, got: %q", out)
	}
	if strings.Contains(out, "alpha-three") {
		t.Fatalf("expected LIMIT 2 to suppress third vector sample, got: %q", out)
	}
	if !strings.Contains(out, "Total vectors: 5") {
		t.Fatalf("expected total vectors 5 despite limit, got: %q", out)
	}
}

func TestPinQueryDBMissingVectorsExact(t *testing.T) {
	dbPath := t.TempDir() + "/kb.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'c', 'x', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "Error counting vectors (table may not exist)") {
		t.Fatalf("expected missing-vectors count error, got: %q", out)
	}
	if strings.Contains(out, "Vector samples:") {
		t.Fatalf("expected no vector samples when count fails, got: %q", out)
	}
	if strings.Contains(out, "Total vectors:") {
		t.Fatalf("expected no total vectors when table missing, got: %q", out)
	}
}
