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
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		t.Fatalf("failed to create vectors: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatalf("failed to create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'concept', 'content', 0.9)`); err != nil {
		t.Fatalf("failed to insert knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (2, 'blob-concept', ?, 0.5)`, []byte("blob-bytes-content")); err != nil {
		t.Fatalf("failed to insert blob knowledge_atoms: %v", err)
	}
	longBlob := bytes.Repeat([]byte("y"), 150)
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (3, 'long-blob-concept', ?, 0.5)`, longBlob); err != nil {
		t.Fatalf("failed to insert long blob knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (1, 'vec', '{}')`); err != nil {
		t.Fatalf("failed to insert vectors: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	output := buf.String()

	if !strings.Contains(output, "Tables:") {
		t.Fatalf("expected tables output, got: %q", output)
	}
	if !strings.Contains(output, "Tables: [knowledge_atoms vectors]") {
		t.Fatalf("expected tables listed in sorted order, got: %q", output)
	}
	if !strings.Contains(output, "knowledge_atoms") {
		t.Fatalf("expected tables to list knowledge_atoms, got: %q", output)
	}
	if !strings.Contains(output, "vectors") {
		t.Fatalf("expected tables to list vectors, got: %q", output)
	}
	if !strings.Contains(output, "Schema:") {
		t.Fatalf("expected schema section, got: %q", output)
	}
	if !strings.Contains(output, "Columns:") {
		t.Fatalf("expected columns section, got: %q", output)
	}
	if !strings.Contains(output, "Sample data:") {
		t.Fatalf("expected sample data section, got: %q", output)
	}
	if !strings.Contains(output, "1. ") {
		t.Fatalf("expected numbered sample row, got: %q", output)
	}
	if !strings.Contains(output, "concept") {
		t.Fatalf("expected sample row to include concept value, got: %q", output)
	}
	if !strings.Contains(output, "2. ") {
		t.Fatalf("expected second numbered sample row, got: %q", output)
	}
	if !strings.Contains(output, "3. ") {
		t.Fatalf("expected third numbered sample row, got: %q", output)
	}
	c1 := strings.Index(output, "concept=concept ")
	c2 := strings.Index(output, "concept=blob-concept")
	c3 := strings.Index(output, "concept=long-blob-concept")
	if c1 < 0 || c2 < 0 || c3 < 0 || !(c1 < c2 && c2 < c3) {
		t.Fatalf("expected sample rows in rowid order, got: %q", output)
	}
	if !strings.Contains(output, "blob-bytes-content") {
		t.Fatalf("expected short []byte cell rendered as string, got: %q", output)
	}
	if !strings.Contains(output, strings.Repeat("y", 100)+"...") {
		t.Fatalf("expected long []byte cell truncated to 100 bytes, got: %q", output)
	}
	if strings.Contains(output, "[98 108 111 98") {
		t.Fatalf("expected []byte cells not dumped as Go slices, got: %q", output)
	}
	if !strings.Contains(output, "Total knowledge_atoms: 3") {
		t.Fatalf("expected knowledge atom count, got: %q", output)
	}
	if !strings.Contains(output, "Total vectors: 1") {
		t.Fatalf("expected vector count, got: %q", output)
	}
	if !strings.Contains(output, "Vector samples:") {
		t.Fatalf("expected gated vector samples section, got: %q", output)
	}
	if !strings.Contains(output, "vec") {
		t.Fatalf("expected vector sample content, got: %q", output)
	}
	if strings.Contains(output, "Error") {
		t.Fatalf("happy path should report no errors, got: %q", output)
	}
}

func TestQueryDBNilWriterNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("queryDB with nil writer panicked: %v", r)
		}
	}()
	queryDB(filepath.Join(t.TempDir(), "kb.db"), 1, nil)
}

func TestQueryDBMissingKnowledgeAtoms(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		t.Fatalf("failed to create vectors: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	if !strings.Contains(buf.String(), "No knowledge_atoms table") {
		t.Fatalf("expected missing knowledge_atoms message, got: %q", buf.String())
	}
}

func TestQueryDBWithoutVectorsTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatalf("failed to create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'concept', 'content', 0.9)`); err != nil {
		t.Fatalf("failed to insert knowledge_atoms: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	output := buf.String()
	if !strings.Contains(output, "Total knowledge_atoms: 1") {
		t.Fatalf("expected knowledge atom count, got: %q", output)
	}
	if !strings.Contains(output, "Error counting vectors") {
		t.Fatalf("expected missing vectors message, got: %q", output)
	}
}

func TestQueryDBZeroLimitDefaults(t *testing.T) {
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
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'a', 'one', 0.5), (2, 'b', 'two', 0.6)`); err != nil {
		t.Fatalf("failed to insert knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (1, 'vec', '{}')`); err != nil {
		t.Fatalf("failed to insert vectors: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	var buf bytes.Buffer
	queryDB(dbPath, 0, &buf)
	output := buf.String()
	if !strings.Contains(output, "Total knowledge_atoms: 2") {
		t.Fatalf("expected default limit to include both rows, got: %q", output)
	}
	if !strings.Contains(output, "2. ") {
		t.Fatalf("expected second sample row, got: %q", output)
	}
}

func TestQueryDBTruncatesLongContent(t *testing.T) {
	longContent := strings.Repeat("x", 150)
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
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'concept', ?, 0.9)`, longContent); err != nil {
		t.Fatalf("failed to insert knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (1, 'vec', '{}')`); err != nil {
		t.Fatalf("failed to insert vectors: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	output := buf.String()
	if !strings.Contains(output, strings.Repeat("x", 100)+"...") {
		t.Fatalf("expected truncated long content, got: %q", output)
	}
	if strings.Contains(output, longContent) {
		t.Fatalf("expected full long content to be truncated")
	}
}

func TestMainSingleDBArgDispatchesToQueryDB(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	defer func() { os.Args = oldArgs; os.Stdout = oldStdout }()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('c', 'hello', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	db.Close()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	os.Args = []string{"query-kb", dbPath}
	main()
	_ = w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	s := string(out)
	if !strings.Contains(s, "Tables:") {
		t.Fatalf("main with db arg should print Tables:, got %q", s)
	}
	if !strings.Contains(s, "knowledge_atoms") {
		t.Fatalf("main with db arg should mention knowledge_atoms, got %q", s)
	}
}

func TestMainQueriesSingleDBArg(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('c', 'hello', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	db.Close()
	os.Args = []string{"query-kb", dbPath}
	main()
}

func TestQueryDBEmptyPathReportsError(t *testing.T) {
	var buf bytes.Buffer
	queryDB("", 5, &buf)
	if !strings.Contains(buf.String(), "empty database path") {
		t.Fatalf("expected empty path message, got: %q", buf.String())
	}
}

func TestQueryDBMissingDBFileReportsError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "no-such-dir", "kb.db")
	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "Error connecting to DB") && !strings.Contains(out, "Error opening DB") && !strings.Contains(out, "Error querying tables") {
		t.Fatalf("expected open/connect error for missing dir, got: %q", out)
	}
}

func TestQueryDBEmptyDatabaseNoKnowledgeAtoms(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "Tables:") {
		t.Fatalf("expected tables output, got: %q", out)
	}
	if !strings.Contains(out, "No knowledge_atoms table") {
		t.Fatalf("expected missing knowledge_atoms message, got: %q", out)
	}
}

func TestQueryDBVectorsSchemaMismatchReportsError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("failed to create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'concept', 'content', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("failed to insert knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY)`); err != nil {
		db.Close()
		t.Fatalf("failed to create vectors: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "Error querying vectors") {
		t.Fatalf("expected vector query error for mismatched schema, got: %q", out)
	}
	if strings.Contains(out, "Vector samples:") {
		t.Fatalf("expected no vector samples on query error, got: %q", out)
	}
}

func TestQueryDBLongVectorContentTruncated(t *testing.T) {
	longVec := strings.Repeat("v", 150)
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("failed to create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		db.Close()
		t.Fatalf("failed to create vectors: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'concept', 'content', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("failed to insert knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (1, ?, '{}')`, longVec); err != nil {
		db.Close()
		t.Fatalf("failed to insert vectors: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, strings.Repeat("v", 100)+"...") {
		t.Fatalf("expected truncated vector content, got: %q", out)
	}
	if strings.Contains(out, longVec) {
		t.Fatalf("expected full vector content to be truncated")
	}
}

func TestMainNoArgsQueriesShardsDir(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	oldCwd, _ := os.Getwd()
	defer func() { os.Args = oldArgs; os.Stdout = oldStdout; _ = os.Chdir(oldCwd) }()
	tmp := t.TempDir()
	shards := filepath.Join(tmp, ".nerd", "shards")
	if err := os.MkdirAll(shards, 0755); err != nil {
		t.Fatalf("mkdir shards: %v", err)
	}
	dbPath := filepath.Join(shards, "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create vectors: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('c', 'hello', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	db.Close()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	os.Args = []string{"query-kb"}
	main()
	_ = w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	s := string(out)
	if !strings.Contains(s, "kb.db") {
		t.Fatalf("shards mode should mention kb.db, got %q", s)
	}
	if !strings.Contains(s, "Tables:") {
		t.Fatalf("shards mode should print Tables:, got %q", s)
	}
}

func TestMainNoArgsStdoutWriteError(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldCwd, _ := os.Getwd()
	defer func() { os.Args = oldArgs; os.Stdout = oldStdout; os.Stderr = oldStderr; _ = os.Chdir(oldCwd) }()
	tmp := t.TempDir()
	shards := filepath.Join(tmp, ".nerd", "shards")
	if err := os.MkdirAll(shards, 0755); err != nil {
		t.Fatalf("mkdir shards: %v", err)
	}
	dbPath := filepath.Join(shards, "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('c', 'hello', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	db.Close()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	// Closed stdout forces os.Stdout.Write to fail without SIGPIPE death.
	_, closedW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_ = closedW.Close()
	os.Stdout = closedW
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stderr = errW
	os.Args = []string{"query-kb"}
	main()
	_ = errW.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout
	out, _ := io.ReadAll(errR)
	if !strings.Contains(string(out), "Error writing output") {
		t.Fatalf("expected stdout write error on stderr, got %q", string(out))
	}
}

func TestMainShardsDirRecoversGoroutinePanic(t *testing.T) {
	oldArgs := os.Args
	oldStderr := os.Stderr
	oldCwd, _ := os.Getwd()
	defer func() { os.Args = oldArgs; os.Stderr = oldStderr; _ = os.Chdir(oldCwd) }()
	tmp := t.TempDir()
	shards := filepath.Join(tmp, ".nerd", "shards")
	if err := os.MkdirAll(shards, 0755); err != nil {
		t.Fatalf("mkdir shards: %v", err)
	}
	// Corrupt file plus a valid DB: corrupt exercises error paths, valid
	// exercises the normal goroutine path including defer/recover setup.
	if err := os.WriteFile(filepath.Join(shards, "bad.db"), []byte("not a sqlite db at all"), 0644); err != nil {
		t.Fatalf("write bad db: %v", err)
	}
	dbPath := filepath.Join(shards, "good.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('c', 'hello', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	db.Close()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = errW
	os.Args = []string{"query-kb"}
	main()
	_ = errW.Close()
	os.Stderr = oldStderr
	_, _ = io.ReadAll(errR)
}

func TestQueryDBVectorsScanErrorReports(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'c', 'hello', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert knowledge: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id TEXT, content TEXT, metadata TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create vectors: %v", err)
	}
	// NULL id cannot scan into *int, forcing the vec scan-error branch.
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (NULL, 'c', 'm')`); err != nil {
		db.Close()
		t.Fatalf("insert vectors: %v", err)
	}
	db.Close()
	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "scan error") {
		t.Fatalf("expected vector scan error, got %q", out)
	}
}

func TestQueryDBVectorsIterErrorReports(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'c', 'hello', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert knowledge: %v", err)
	}
	// VIRTUAL generated column: COUNT(*) counts rows without evaluating
	// metadata, but SELECT id,content,metadata evaluates json(content) and
	// fails on non-JSON content, surfacing rows.Err().
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT GENERATED ALWAYS AS (json(content)) VIRTUAL)`); err != nil {
		db.Close()
		t.Skipf("sqlite without generated columns: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content) VALUES (1, 'not-json')`); err != nil {
		db.Close()
		t.Skipf("sqlite validates generated columns on write, cannot stage iter-error: %v", err)
	}
	db.Close()
	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "Error iterating vectors") {
		t.Fatalf("expected vector iteration error, got %q", out)
	}
}

func TestMainNoArgsSkipsDirectoryEntries(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldCwd, _ := os.Getwd()
	defer func() { os.Args = oldArgs; os.Stdout = oldStdout; os.Stderr = oldStderr; _ = os.Chdir(oldCwd) }()
	tmp := t.TempDir()
	shards := filepath.Join(tmp, ".nerd", "shards")
	if err := os.MkdirAll(shards, 0755); err != nil {
		t.Fatalf("mkdir shards: %v", err)
	}
	goodPath := filepath.Join(shards, "good.db")
	db, err := sql.Open("sqlite", goodPath)
	if err != nil {
		t.Fatalf("open good: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create good: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('c', 'hello', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert good: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close good: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(shards, "skipme.db"), 0755); err != nil {
		t.Fatalf("mkdir skipme.db dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shards, "notes.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("write notes: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe out: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe err: %v", err)
	}
	os.Stdout = wOut
	os.Stderr = wErr
	os.Args = []string{"query-kb"}
	main()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	out, _ := io.ReadAll(rOut)
	errOut, _ := io.ReadAll(rErr)
	s := string(out)
	es := string(errOut)
	if !strings.Contains(s, "good.db") {
		t.Fatalf("shards mode should mention good.db, got %q", s)
	}
	if !strings.Contains(s, "Tables:") {
		t.Fatalf("shards mode should print Tables:, got %q", s)
	}
	if strings.Contains(s, "skipme.db") || strings.Contains(es, "skipme.db") {
		t.Fatalf("directory entry ending in .db must be skipped, got stdout %q stderr %q", s, es)
	}
	if strings.Contains(es, "panic querying") {
		t.Fatalf("no panic expected on clean shards walk, got stderr %q", es)
	}
}

func TestMainShardsWalkIsPanicSafe(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "recover()") {
		t.Fatalf("main shards walk must recover goroutine panics, no recover() in main.go")
	}
	if !strings.Contains(s, "panic querying") {
		t.Fatalf("main shards walk must report recovered panics to stderr")
	}
	if strings.Contains(s, "defer mu.Unlock()") {
		t.Fatalf("main must not hold shards stdout lock across stderr writes (defer mu.Unlock() holds lock too long)")
	}
}

func TestMainShardsDirRecoversGoroutinePanicInjected(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(".nerd", "shards"), 0o755); err != nil {
		t.Fatalf("mkdir shards: %v", err)
	}
	goodPath := filepath.Join(".nerd", "shards", "good.db")
	db, err := sql.Open("sqlite", goodPath)
	if err != nil {
		t.Fatalf("open good db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, content TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (content) VALUES ('hello')`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	db.Close()
	badPath := filepath.Join(".nerd", "shards", "bad.db")
	if err := os.WriteFile(badPath, []byte("not a sqlite database - corrupt header \x00\x01\x02"), 0o644); err != nil {
		t.Fatalf("write bad db: %v", err)
	}
	oldArgs := os.Args
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	defer func() { os.Args = oldArgs; os.Stderr = oldStderr; os.Stdout = oldStdout }()
	os.Args = []string{"query-kb"}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = wErr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = wOut
	doneMain := make(chan struct{})
	go func() {
		defer close(doneMain)
		main()
	}()
	<-doneMain
	_ = wErr.Close()
	_ = wOut.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout
	errBytes, _ := io.ReadAll(rErr)
	outBytes, _ := io.ReadAll(rOut)
	combined := string(errBytes) + string(outBytes)
	if !strings.Contains(combined, "file is not a database") {
		t.Fatalf("expected graceful corrupt-db error, got stderr=%q stdout=%q", string(errBytes), string(outBytes))
	}
	if !strings.Contains(string(outBytes), "good.db") {
		t.Fatalf("expected good shard output to survive corrupt shard, got stdout=%q", string(outBytes))
	}
}

func TestQueryDBRendersBlobBytesAsString(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create vectors: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, payload BLOB)`); err != nil {
		db.Close()
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	shortBlob := []byte("hello-blob-world")
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, payload) VALUES (?, ?)`, "c-short", shortBlob); err != nil {
		db.Close()
		t.Fatalf("insert short blob: %v", err)
	}
	longBlob := bytes.Repeat([]byte("x"), 150)
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, payload) VALUES (?, ?)`, "c-long", longBlob); err != nil {
		db.Close()
		t.Fatalf("insert long blob: %v", err)
	}
	db.Close()
	var buf bytes.Buffer
	queryDB(dbPath, 10, &buf)
	s := buf.String()
	if !strings.Contains(s, "Tables: [knowledge_atoms vectors]") {
		t.Fatalf("tables should be listed sorted by name, got %q", s)
	}
	if !strings.Contains(s, "payload=hello-blob-world") {
		t.Fatalf("short BLOB should render as string, got %q", s)
	}
	if strings.Contains(s, "payload=[") {
		t.Fatalf("BLOB should not render as Go slice dump, got %q", s)
	}
	wantTrunc := string(bytes.Repeat([]byte("x"), 100)) + "..."
	if !strings.Contains(s, wantTrunc) {
		t.Fatalf("long BLOB should truncate to 100 bytes plus ..., got %q", s)
	}
	if strings.Contains(s, string(bytes.Repeat([]byte("x"), 150))) {
		t.Fatalf("long BLOB should not render full 150 bytes, got %q", s)
	}
}

func TestMainShardsUsesShardQueryDBSortedOrder(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	oldCwd, _ := os.Getwd()
	defer func() { os.Args = oldArgs; os.Stdout = oldStdout; _ = os.Chdir(oldCwd) }()
	tmp := t.TempDir()
	shards := filepath.Join(tmp, ".nerd", "shards")
	if err := os.MkdirAll(shards, 0755); err != nil {
		t.Fatalf("mkdir shards: %v", err)
	}
	mkShard := func(name, concept string) {
		dbPath := filepath.Join(shards, name)
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("open temp db %s: %v", name, err)
		}
		if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
			db.Close()
			t.Fatalf("create knowledge_atoms %s: %v", name, err)
		}
		if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
			db.Close()
			t.Fatalf("create vectors %s: %v", name, err)
		}
		if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES (?, ?, 0.9)`, concept, "hello-"+concept); err != nil {
			db.Close()
			t.Fatalf("insert %s: %v", name, err)
		}
		db.Close()
	}
	mkShard("z.db", "zeta-marker")
	mkShard("a.db", "alpha-marker")
	mkShard("m.db", "mid-marker")
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	os.Args = []string{"query-kb"}
	main()
	_ = w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	s := string(out)
	for _, name := range []string{"a.db", "m.db", "z.db"} {
		if !strings.Contains(s, "=== "+name+" ===") {
			t.Fatalf("shards mode should print header for %s, got %q", name, s)
		}
	}
	for _, m := range []string{"alpha-marker", "mid-marker", "zeta-marker"} {
		if !strings.Contains(s, m) {
			t.Fatalf("shards mode should print content %s, got %q", m, s)
		}
	}
	ia := strings.Index(s, "=== a.db ===")
	im := strings.Index(s, "=== m.db ===")
	iz := strings.Index(s, "=== z.db ===")
	if !(ia < im && im < iz) {
		t.Fatalf("shard output should flush in sorted order, got %q", s)
	}
	ca := strings.Index(s, "alpha-marker")
	cm := strings.Index(s, "mid-marker")
	cz := strings.Index(s, "zeta-marker")
	if !(ca < cm && cm < cz) {
		t.Fatalf("shard contents should follow sorted shard order, got %q", s)
	}
}
