package main

import (
	"bytes"
	"database/sql"
	"errors"
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

	var buf bytes.Buffer
	queryDB(dbPath, 1, &buf)
	output := buf.String()

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

type fakeRowIter struct {
	remaining int
	calls     int
	failAt    map[int]bool
	iterErr   error
	onScan    func(call int, dest ...any)
}

func (f *fakeRowIter) Next() bool {
	if f.remaining <= 0 {
		return false
	}
	f.remaining--
	return true
}

func (f *fakeRowIter) Scan(dest ...any) error {
	call := f.calls
	f.calls++
	if f.failAt[call] {
		return errors.New("fake scan boom")
	}
	if f.onScan != nil {
		f.onScan(call, dest...)
	}
	return nil
}

func (f *fakeRowIter) Err() error {
	return f.iterErr
}

func TestCollectTableNamesScanAndIterErrors(t *testing.T) {
	var buf bytes.Buffer
	rows := &fakeRowIter{
		remaining: 3,
		failAt:    map[int]bool{0: true},
		iterErr:   errors.New("fake iter boom"),
		onScan: func(_ int, dest ...any) {
			if s, ok := dest[0].(*string); ok {
				*s = "tbl"
			}
		},
	}
	tables := collectTableNames(rows, &buf)
	out := buf.String()
	if len(tables) != 2 {
		t.Fatalf("expected 2 tables after 1 scan failure, got %d (%v)", len(tables), tables)
	}
	if !strings.Contains(out, "Error scanning table row:") {
		t.Fatalf("expected scan error, got %q", out)
	}
	if !strings.Contains(out, "Error iterating tables:") {
		t.Fatalf("expected iteration error, got %q", out)
	}
	if !strings.Contains(out, "fake scan boom") || !strings.Contains(out, "fake iter boom") {
		t.Fatalf("expected fake error text, got %q", out)
	}

	var clean bytes.Buffer
	cleanRows := &fakeRowIter{remaining: 1}
	if got := collectTableNames(cleanRows, &clean); len(got) != 1 {
		t.Fatalf("expected 1 table on clean path, got %v", got)
	}
	if strings.Contains(clean.String(), "Error scanning table row:") || strings.Contains(clean.String(), "Error iterating tables:") {
		t.Fatalf("unexpected error on clean path: %q", clean.String())
	}
}

func TestPrintSchemaRowsScanAndIterErrors(t *testing.T) {
	var buf bytes.Buffer
	rows := &fakeRowIter{
		remaining: 2,
		failAt:    map[int]bool{0: true},
		iterErr:   errors.New("fake iter boom"),
		onScan: func(_ int, dest ...any) {
			if s, ok := dest[1].(*string); ok {
				*s = "col"
			}
			if s, ok := dest[2].(*string); ok {
				*s = "TEXT"
			}
		},
	}
	printSchemaRows(rows, &buf)
	out := buf.String()
	if !strings.Contains(out, "<scan error:") {
		t.Fatalf("expected scan error, got %q", out)
	}
	if !strings.Contains(out, "<iteration error:") {
		t.Fatalf("expected iteration error, got %q", out)
	}
	if !strings.Contains(out, "col (TEXT)") {
		t.Fatalf("expected successful row to print, got %q", out)
	}

	var clean bytes.Buffer
	printSchemaRows(&fakeRowIter{}, &clean)
	if strings.Contains(clean.String(), "<scan error:") || strings.Contains(clean.String(), "<iteration error:") {
		t.Fatalf("unexpected error on clean path: %q", clean.String())
	}
}

func TestPrintKnowledgeSamplesScanAndIterErrors(t *testing.T) {
	cols := []string{"id", "concept"}
	var buf bytes.Buffer
	rows := &fakeRowIter{
		remaining: 2,
		failAt:    map[int]bool{0: true},
		iterErr:   errors.New("fake iter boom"),
		onScan: func(_ int, dest ...any) {
			for i := range dest {
				if p, ok := dest[i].(*any); ok {
					*p = "v"
				}
			}
		},
	}
	printKnowledgeSamples(rows, cols, &buf)
	out := buf.String()
	if !strings.Contains(out, "Scan error:") {
		t.Fatalf("expected scan error, got %q", out)
	}
	if !strings.Contains(out, "Error iterating knowledge:") {
		t.Fatalf("expected iteration error, got %q", out)
	}
	if !strings.Contains(out, "id=v") {
		t.Fatalf("expected successful row to print, got %q", out)
	}

	var clean bytes.Buffer
	printKnowledgeSamples(&fakeRowIter{}, cols, &clean)
	if strings.Contains(clean.String(), "Scan error:") || strings.Contains(clean.String(), "Error iterating knowledge:") {
		t.Fatalf("unexpected error on clean path: %q", clean.String())
	}
}

func TestPrintVectorSamplesScanAndIterErrors(t *testing.T) {
	var buf bytes.Buffer
	rows := &fakeRowIter{
		remaining: 2,
		failAt:    map[int]bool{0: true},
		iterErr:   errors.New("fake iter boom"),
		onScan: func(_ int, dest ...any) {
			if p, ok := dest[0].(*int); ok {
				*p = 7
			}
			if p, ok := dest[1].(*string); ok {
				*p = "hello"
			}
			if p, ok := dest[2].(*string); ok {
				*p = "{}"
			}
		},
	}
	printVectorSamples(rows, &buf)
	out := buf.String()
	if !strings.Contains(out, "<scan error:") {
		t.Fatalf("expected scan error, got %q", out)
	}
	if !strings.Contains(out, "<iteration error:") {
		t.Fatalf("expected iteration error, got %q", out)
	}
	if !strings.Contains(out, "7: hello") {
		t.Fatalf("expected successful row to print, got %q", out)
	}

	var clean bytes.Buffer
	printVectorSamples(&fakeRowIter{}, &clean)
	if strings.Contains(clean.String(), "<scan error:") || strings.Contains(clean.String(), "<iteration error:") {
		t.Fatalf("unexpected error on clean path: %q", clean.String())
	}
}

func TestPrintDeepAtomsScanAndIterErrors(t *testing.T) {
	var buf bytes.Buffer
	rows := &fakeRowIter{
		remaining: 2,
		failAt:    map[int]bool{0: true},
		iterErr:   errors.New("fake iter boom"),
		onScan: func(_ int, dest ...any) {
			if p, ok := dest[0].(*int); ok {
				*p = 1
			}
			if p, ok := dest[1].(*string); ok {
				*p = "c"
			}
			if p, ok := dest[2].(*string); ok {
				*p = "body"
			}
			if p, ok := dest[3].(*float64); ok {
				*p = 0.9
			}
		},
	}
	printDeepAtoms(rows, &buf)
	out := buf.String()
	if !strings.Contains(out, "Error scanning atom row:") {
		t.Fatalf("expected scan error, got %q", out)
	}
	if !strings.Contains(out, "Error iterating atoms:") {
		t.Fatalf("expected iteration error, got %q", out)
	}
	if !strings.Contains(out, "[1] c") {
		t.Fatalf("expected successful row to print, got %q", out)
	}

	var clean bytes.Buffer
	printDeepAtoms(&fakeRowIter{}, &clean)
	if strings.Contains(clean.String(), "Error scanning atom row:") || strings.Contains(clean.String(), "Error iterating atoms:") {
		t.Fatalf("unexpected error on clean path: %q", clean.String())
	}
}

func TestPrintDeepVectorsScanAndIterErrors(t *testing.T) {
	var buf bytes.Buffer
	rows := &fakeRowIter{
		remaining: 2,
		failAt:    map[int]bool{0: true},
		iterErr:   errors.New("fake iter boom"),
		onScan: func(_ int, dest ...any) {
			if p, ok := dest[0].(*int); ok {
				*p = 2
			}
			if p, ok := dest[1].(*string); ok {
				*p = "vec-body"
			}
			if p, ok := dest[2].(*string); ok {
				*p = "{}"
			}
		},
	}
	count := printDeepVectors(rows, &buf)
	out := buf.String()
	if count != 1 {
		t.Fatalf("expected 1 successful vector after 1 scan failure, got %d", count)
	}
	if !strings.Contains(out, "Error scanning vector row:") {
		t.Fatalf("expected scan error, got %q", out)
	}
	if !strings.Contains(out, "Error iterating vectors:") {
		t.Fatalf("expected iteration error, got %q", out)
	}
	if !strings.Contains(out, "vec-body") {
		t.Fatalf("expected successful row to print, got %q", out)
	}

	var clean bytes.Buffer
	if got := printDeepVectors(&fakeRowIter{}, &clean); got != 0 {
		t.Fatalf("expected 0 on clean empty path, got %d", got)
	}
	if strings.Contains(clean.String(), "Error scanning vector row:") || strings.Contains(clean.String(), "Error iterating vectors:") {
		t.Fatalf("unexpected error on clean path: %q", clean.String())
	}
}
