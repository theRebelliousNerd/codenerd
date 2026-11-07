package main

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func pinOpenAndSeed(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	return db
}

func TestQueryDBPinsCurrentBehaviorViaIntegration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kb.db")
	db := pinOpenAndSeed(t, dbPath)
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		t.Fatalf("create vectors: %v", err)
	}
	longKnowledge := strings.Repeat("x", 150)
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'c1', 'short', 0.9), (2, 'c2', ?, 0.8)`, longKnowledge); err != nil {
		t.Fatalf("insert knowledge_atoms: %v", err)
	}
	longVector := strings.Repeat("y", 150)
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (1, 'hello', '{}')`); err != nil {
		t.Fatalf("insert vector 1: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (2, NULL, NULL)`); err != nil {
		t.Fatalf("insert vector null: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (3, ?, '{}')`, longVector); err != nil {
		t.Fatalf("insert vector long: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	var buf bytes.Buffer
	queryDB(dbPath, 10, &buf)
	out := buf.String()

	for _, want := range []string{
		"Tables:",
		"knowledge_atoms",
		"vectors",
		"Schema:",
		"Columns:",
		"Sample data:",
		"Total knowledge_atoms: 2",
		"Total vectors: 3",
		"Vector samples:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("queryDB output missing %q\nfull output:\n%s", want, out)
		}
	}
	// Truncation applied through queryDB path for knowledge_atoms long content.
	if strings.Contains(out, longKnowledge) {
		t.Errorf("queryDB did not truncate long knowledge content; full 150-char string present")
	}
	if !strings.Contains(out, strings.Repeat("x", 100)+"...") {
		t.Errorf("queryDB output missing truncated knowledge content marker")
	}
	// Truncation applied through queryDB path for vector long content.
	if strings.Contains(out, longVector) {
		t.Errorf("queryDB did not truncate long vector content; full 150-char string present")
	}
	if !strings.Contains(out, strings.Repeat("y", 100)+"...") {
		t.Errorf("queryDB output missing truncated vector content marker")
	}
	// Scan-error reporting reached through queryDB (NULL vector row).
	if !strings.Contains(out, "scan error") {
		t.Errorf("queryDB output missing scan error from NULL vector row\nfull output:\n%s", out)
	}
}

func TestQueryDBPinsMissingVectorsBehavior(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kb.db")
	db := pinOpenAndSeed(t, dbPath)
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'c1', 'hi', 1.0)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 10, &buf)
	out := buf.String()
	if !strings.Contains(out, "Total knowledge_atoms: 1") {
		t.Errorf("missing knowledge count\n%s", out)
	}
	if !strings.Contains(out, "Error counting vectors") {
		t.Errorf("missing vectors-count error branch\n%s", out)
	}
	if strings.Contains(out, "Vector samples:") {
		t.Errorf("unexpected vector samples when vectors table missing\n%s", out)
	}
}

func TestQueryDBPinsNoKnowledgeAtomsTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "empty.db")
	db := pinOpenAndSeed(t, dbPath)
	if _, err := db.Exec(`CREATE TABLE other (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create other: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 10, &buf)
	out := buf.String()
	if !strings.Contains(out, "Tables:") {
		t.Errorf("missing Tables line\n%s", out)
	}
	if !strings.Contains(out, "Error querying knowledge") {
		t.Errorf("missing knowledge query error when knowledge_atoms absent\n%s", out)
	}
}
