package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestQueryDBPinsSectionOrderAndExactTables(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kb.db")
	db := pinOpenAndSeed(t, dbPath)
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1,'c1','hi',0.5)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		t.Fatalf("create vectors: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (1,'v1','{}')`); err != nil {
		t.Fatalf("insert vector: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 10, &buf)
	out := buf.String()

	// Exact tables line pins table discovery formatting and order.
	if !strings.Contains(out, "Tables: [knowledge_atoms vectors]") {
		t.Errorf("expected exact tables line, got:\n%s", out)
	}
	// Section order pins queryDB orchestration (tables -> schema -> columns -> samples -> counts -> vectors).
	markers := []string{"Tables:", "Schema:", "Columns:", "Sample data:", "Total knowledge_atoms:", "Total vectors:", "Vector samples:"}
	last := -1
	for _, m := range markers {
		idx := strings.Index(out, m)
		if idx < 0 {
			t.Fatalf("missing section %q:\n%s", m, out)
		}
		if idx <= last {
			t.Fatalf("section %q out of order:\n%s", m, out)
		}
		last = idx
	}
	// Exact schema lines pin PRAGMA formatting through queryDB.
	for _, want := range []string{"  - id (INTEGER)", "  - concept (TEXT)", "  - content (TEXT)", "  - confidence (REAL)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing schema line %q:\n%s", want, out)
		}
	}
	// Exact vector sample formatting pins delegation to printVectorSamples.
	if !strings.Contains(out, "  1: v1\n") {
		t.Errorf("missing exact vector sample line:\n%s", out)
	}
}
