package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestPinQueryDBTablesSortedOrder(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	// Create in reverse alphabetical order: without ORDER BY name the
	// listing would come back as [vectors knowledge_atoms].
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create vectors: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'c1', 'one', 0.5), (2, 'c2', 'two', 0.6)`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id, content, metadata) VALUES (1, 'vec', '{}')`); err != nil {
		db.Close()
		t.Fatalf("insert vectors: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "Tables: [knowledge_atoms vectors]") {
		t.Fatalf("expected tables in sorted order, got: %q", out)
	}
	c1 := strings.Index(out, "concept=c1")
	c2 := strings.Index(out, "concept=c2")
	if c1 < 0 || c2 < 0 || !(c1 < c2) {
		t.Fatalf("expected sample rows in rowid order, got: %q", out)
	}
}

func TestPinQueryDBNilWriterDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("queryDB(nil) panicked: %v", r)
		}
	}()
	queryDB(filepath.Join(t.TempDir(), "kb.db"), 5, nil)
}

func TestPinQueryDBEmptyPathReportsError(t *testing.T) {
	var buf bytes.Buffer
	queryDB("", 5, &buf)
	if !strings.Contains(buf.String(), "empty database path") {
		t.Fatalf("expected empty path message, got: %q", buf.String())
	}
}

func TestPinQueryDBZeroLimitDefaultsToTen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create vectors: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'a', 'one', 0.5), (2, 'b', 'two', 0.6)`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	// LIMIT 0 in SQLite returns zero rows; the default (10) must return both.
	queryDB(dbPath, 0, &buf)
	out := buf.String()
	if !strings.Contains(out, "2. ") {
		t.Fatalf("expected default limit to return second row, got: %q", out)
	}
	if !strings.Contains(out, "Total knowledge_atoms: 2") {
		t.Fatalf("expected count 2, got: %q", out)
	}
}

func TestPinQueryDBVecGatingNoSamplesOnError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'c', 'x', 0.9)`); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	// vectors exists but lacks content/metadata columns: countable but not samplable.
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY)`); err != nil {
		db.Close()
		t.Fatalf("create vectors: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO vectors (id) VALUES (1)`); err != nil {
		db.Close()
		t.Fatalf("insert vectors: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "Error querying vectors") {
		t.Fatalf("expected vector query error, got: %q", out)
	}
	if strings.Contains(out, "Vector samples:") {
		t.Fatalf("expected no vector samples on query error, got: %q", out)
	}
}

func TestPinQueryDBBlobBytesRenderedAsString(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		db.Close()
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create vectors: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (1, 'blob-concept', ?, 0.5)`, []byte("blob-bytes-content")); err != nil {
		db.Close()
		t.Fatalf("insert blob: %v", err)
	}
	longBlob := bytes.Repeat([]byte("y"), 150)
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (id, concept, content, confidence) VALUES (2, 'long-blob', ?, 0.5)`, longBlob); err != nil {
		db.Close()
		t.Fatalf("insert long blob: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	queryDB(dbPath, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "blob-bytes-content") {
		t.Fatalf("expected []byte rendered as string, got: %q", out)
	}
	if strings.Contains(out, "[98 108 111 98") {
		t.Fatalf("expected []byte not dumped as Go slice, got: %q", out)
	}
	if !strings.Contains(out, strings.Repeat("y", 100)+"...") {
		t.Fatalf("expected long []byte truncated, got: %q", out)
	}
}

func TestPinMainSingleArgDispatches(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	defer func() { os.Args = oldArgs; os.Stdout = oldStdout }()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kb.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
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
		t.Fatalf("main single-arg should print Tables:, got %q", s)
	}
	if !strings.Contains(s, "knowledge_atoms") {
		t.Fatalf("main single-arg should mention knowledge_atoms, got %q", s)
	}
}

func TestPinMainShardsSortedDeterministic(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { os.Args = oldArgs; os.Stdout = oldStdout; _ = os.Chdir(cwd) }()
	dir := t.TempDir()
	shards := filepath.Join(dir, ".nerd", "shards")
	if err := os.MkdirAll(shards, 0o755); err != nil {
		t.Fatalf("mkdir shards: %v", err)
	}
	mkShard := func(name, concept string) {
		p := filepath.Join(shards, name)
		db, err := sql.Open("sqlite", p)
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
			db.Close()
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES (?, 'x', 0.9)`, concept); err != nil {
			db.Close()
			t.Fatalf("insert %s: %v", name, err)
		}
		if _, err := db.Exec(`CREATE TABLE vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil {
			db.Close()
			t.Fatalf("create vectors %s: %v", name, err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("close %s: %v", name, err)
		}
	}
	// Create in reverse alphabetical order to catch missing sort.
	mkShard("b.db", "concept-b")
	mkShard("a.db", "concept-a")
	if err := os.Chdir(dir); err != nil {
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
	ia := strings.Index(s, "=== a.db ===")
	ib := strings.Index(s, "=== b.db ===")
	if ia < 0 || ib < 0 {
		t.Fatalf("expected both shard headers, got %q", s)
	}
	if !(ia < ib) {
		t.Fatalf("expected shards flushed in sorted order (a before b), got %q", s)
	}
}

func TestPinMainPanicRecoveryKeepsOtherShards(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	oldQuery := shardQueryDB
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
		shardQueryDB = oldQuery
		_ = os.Chdir(cwd)
	}()
	dir := t.TempDir()
	shards := filepath.Join(dir, ".nerd", "shards")
	if err := os.MkdirAll(shards, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, n := range []string{"a.db", "b.db"} {
		f, err := os.Create(filepath.Join(shards, n))
		if err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
		_ = f.Close()
	}
	shardQueryDB = func(dbPath string, limit int, w io.Writer) {
		if strings.HasSuffix(dbPath, "b.db") {
			panic("boom-b")
		}
		fmt.Fprint(w, "marker-a")
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	os.Args = []string{"query-kb"}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("main propagated goroutine panic: %v", r)
			}
		}()
		main()
	}()
	_ = w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "marker-a") {
		t.Fatalf("expected surviving shard output despite sibling panic, got %q", string(out))
	}
}
