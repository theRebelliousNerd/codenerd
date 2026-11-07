package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// deterministicFakeIter is a deterministic queryRowIter fake: no corruption,
// no I/O. total controls how many times Next returns true. scanErrAt marks
// 1-based Next calls whose Scan fails. errResult is returned from Err.
type deterministicFakeIter struct {
	total     int
	calls     int
	scanErrAt map[int]bool
	errResult error
}

func (f *deterministicFakeIter) Next() bool {
	if f.calls < f.total {
		f.calls++
		return true
	}
	return false
}

func (f *deterministicFakeIter) Scan(dest ...any) error {
	if f.scanErrAt != nil && f.scanErrAt[f.calls] {
		return fmt.Errorf("scan boom %d", f.calls)
	}
	return nil
}

func (f *deterministicFakeIter) Err() error { return f.errResult }

func TestDeterministicPrintVectorSamplesSuccess(t *testing.T) {
	var buf bytes.Buffer
	printVectorSamples(&deterministicFakeIter{total: 2}, &buf)
	if buf.Len() == 0 {
		t.Fatalf("expected output for 2 vector rows, got empty")
	}
}

func TestDeterministicPrintVectorSamplesScanErr(t *testing.T) {
	var buf bytes.Buffer
	printVectorSamples(&deterministicFakeIter{total: 3, scanErrAt: map[int]bool{2: true}}, &buf)
	out := buf.String()
	if !strings.Contains(out, "scan boom 2") {
		t.Fatalf("expected scan error text in output, got %q", out)
	}
}

func TestDeterministicPrintVectorSamplesIterErr(t *testing.T) {
	var buf bytes.Buffer
	printVectorSamples(&deterministicFakeIter{total: 1, errResult: fmt.Errorf("iter boom vectors")}, &buf)
	out := buf.String()
	if !strings.Contains(out, "iter boom vectors") {
		t.Fatalf("expected iter error text in output, got %q", out)
	}
}

func TestDeterministicPrintVectorSamplesEmpty(t *testing.T) {
	var buf bytes.Buffer
	printVectorSamples(&deterministicFakeIter{total: 0}, &buf)
	if buf.Len() != 0 {
		t.Fatalf("expected empty output for no rows, got %q", buf.String())
	}
}

func TestDeterministicPrintDeepVectorsSuccess(t *testing.T) {
	var buf bytes.Buffer
	n := printDeepVectors(&deterministicFakeIter{total: 3}, &buf)
	if n != 3 {
		t.Fatalf("expected count 3, got %d", n)
	}
	if buf.Len() == 0 {
		t.Fatalf("expected output for 3 vector rows")
	}
}

func TestDeterministicPrintDeepVectorsScanErrSkips(t *testing.T) {
	var buf bytes.Buffer
	n := printDeepVectors(&deterministicFakeIter{total: 3, scanErrAt: map[int]bool{2: true}}, &buf)
	if n != 2 {
		t.Fatalf("expected count 2 with one scan failure, got %d", n)
	}
	if !strings.Contains(buf.String(), "scan boom 2") {
		t.Fatalf("expected scan error text, got %q", buf.String())
	}
}

func TestDeterministicPrintDeepVectorsIterErr(t *testing.T) {
	var buf bytes.Buffer
	n := printDeepVectors(&deterministicFakeIter{total: 2, errResult: fmt.Errorf("iter boom deep")}, &buf)
	if n != 2 {
		t.Fatalf("expected count 2 despite iter err, got %d", n)
	}
	if !strings.Contains(buf.String(), "iter boom deep") {
		t.Fatalf("expected iter error text, got %q", buf.String())
	}
}

func TestDeterministicPrintDeepVectorsEmpty(t *testing.T) {
	var buf bytes.Buffer
	n := printDeepVectors(&deterministicFakeIter{total: 0}, &buf)
	if n != 0 {
		t.Fatalf("expected count 0, got %d", n)
	}
}

func TestDeterministicPrintDeepAtomsBranches(t *testing.T) {
	var ok bytes.Buffer
	printDeepAtoms(&deterministicFakeIter{total: 1}, &ok)
	if ok.Len() == 0 {
		t.Fatalf("expected output for 1 atom row")
	}
	var scanBuf bytes.Buffer
	printDeepAtoms(&deterministicFakeIter{total: 2, scanErrAt: map[int]bool{1: true}}, &scanBuf)
	if !strings.Contains(scanBuf.String(), "scan boom 1") {
		t.Fatalf("expected scan error text for atoms, got %q", scanBuf.String())
	}
	var errBuf bytes.Buffer
	printDeepAtoms(&deterministicFakeIter{total: 1, errResult: fmt.Errorf("iter boom atoms")}, &errBuf)
	if !strings.Contains(errBuf.String(), "iter boom atoms") {
		t.Fatalf("expected iter error text for atoms, got %q", errBuf.String())
	}
}

func deterministicOpenSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite %s: %v", path, err)
	}
	return db
}

func deterministicMakeKB(t *testing.T, path string, withAtoms, withVectors bool) {
	t.Helper()
	db := deterministicOpenSQLite(t, path)
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil && withAtoms {
		t.Fatalf("create knowledge_atoms: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS vectors (id INTEGER PRIMARY KEY, content TEXT, metadata TEXT)`); err != nil && withVectors {
		t.Fatalf("create vectors: %v", err)
	}
	if withAtoms {
		if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('concept-alpha', 'alpha', 0.9), ('concept-beta', 'beta', 0.8)`); err != nil {
			t.Fatalf("insert atoms: %v", err)
		}
	}
	if withVectors {
		if _, err := db.Exec(`INSERT INTO vectors (content, metadata) VALUES ('alpha', 'meta-alpha'), ('beta', 'meta-beta')`); err != nil {
			t.Fatalf("insert vectors: %v", err)
		}
	}
}

func TestDeterministicQueryDBMissingFile(t *testing.T) {
	var buf bytes.Buffer
	missing := filepath.Join(t.TempDir(), "missing.db")
	queryDB(missing, 5, &buf)
	out := buf.String()
	if strings.Contains(out, "alpha") {
		t.Fatalf("missing db should not list atom content, got %q", out)
	}
}

func TestDeterministicQueryDBEmptyDirDB(t *testing.T) {
	var buf bytes.Buffer
	p := filepath.Join(t.TempDir(), "empty.db")
	db := deterministicOpenSQLite(t, p)
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	queryDB(p, 5, &buf)
	// Must complete without panic; empty DB has no user content.
	if strings.Contains(buf.String(), "alpha") {
		t.Fatalf("empty db should not contain atom content, got %q", buf.String())
	}
}

func TestDeterministicQueryDBWithContent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "kb.db")
	deterministicMakeKB(t, p, true, true)
	var buf bytes.Buffer
	queryDB(p, 5, &buf)
	out := buf.String()
	if !strings.Contains(out, "knowledge_atoms") && !strings.Contains(out, "atoms") {
		t.Fatalf("expected table listing in output, got %q", out)
	}
	if !strings.Contains(out, "alpha") {
		t.Fatalf("expected atom sample in output, got %q", out)
	}
}

func TestDeterministicQueryDBLimits(t *testing.T) {
	p := filepath.Join(t.TempDir(), "kb.db")
	deterministicMakeKB(t, p, true, true)
	var one bytes.Buffer
	queryDB(p, 1, &one)
	var many bytes.Buffer
	queryDB(p, 100, &many)
	if many.Len() < one.Len() {
		t.Fatalf("larger limit should not produce less output: %d vs %d", many.Len(), one.Len())
	}
}

func TestDeterministicRunDeepQueryMissingFile(t *testing.T) {
	for _, cfg := range []struct {
		name        string
		showVectors bool
		showAtoms   bool
	}{
		{"both", true, true},
		{"vectorsOnly", true, false},
		{"atomsOnly", false, true},
		{"neither", false, false},
	} {
		t.Run(cfg.name, func(t *testing.T) {
			var buf bytes.Buffer
			missing := filepath.Join(t.TempDir(), "missing.db")
			runDeepQuery(missing, cfg.showVectors, cfg.showAtoms, &buf)
			if strings.Contains(buf.String(), "alpha") {
				t.Fatalf("missing db should not contain atom content, got %q", buf.String())
			}
		})
	}
}

func TestDeterministicRunDeepQueryNoTables(t *testing.T) {
	p := filepath.Join(t.TempDir(), "empty.db")
	db := deterministicOpenSQLite(t, p)
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var buf bytes.Buffer
	runDeepQuery(p, true, true, &buf)
	if strings.Contains(buf.String(), "alpha") {
		t.Fatalf("empty db should not contain atom content, got %q", buf.String())
	}
}

func TestDeterministicRunDeepQueryFlags(t *testing.T) {
	p := filepath.Join(t.TempDir(), "kb.db")
	deterministicMakeKB(t, p, true, true)
	var both bytes.Buffer
	runDeepQuery(p, true, true, &both)
	var atomsOnly bytes.Buffer
	runDeepQuery(p, false, true, &atomsOnly)
	var vectorsOnly bytes.Buffer
	runDeepQuery(p, true, false, &vectorsOnly)
	var neither bytes.Buffer
	runDeepQuery(p, false, false, &neither)
	if !strings.Contains(both.String(), "alpha") {
		t.Fatalf("expected atoms in both output, got %q", both.String())
	}
	// Flag-gated output must differ: disabling a section cannot add output.
	if len(neither.String()) > len(both.String()) {
		t.Fatalf("neither should not exceed both: %d vs %d", len(neither.String()), len(both.String()))
	}
	if len(atomsOnly.String()) > len(both.String()) {
		t.Fatalf("atomsOnly should not exceed both: %d vs %d", len(atomsOnly.String()), len(both.String()))
	}
	if len(vectorsOnly.String()) > len(both.String()) {
		t.Fatalf("vectorsOnly should not exceed both: %d vs %d", len(vectorsOnly.String()), len(both.String()))
	}
}

func TestDeterministicRunDeepQueryAtomsOnlyDB(t *testing.T) {
	p := filepath.Join(t.TempDir(), "atoms.db")
	db := deterministicOpenSQLite(t, p)
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('concept-solo', 'solo', 0.7)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Exercise vectors-missing error branch deterministically (no corruption).
	var buf bytes.Buffer
	runDeepQuery(p, true, true, &buf)
	if !strings.Contains(buf.String(), "solo") {
		t.Fatalf("expected solo atom, got %q", buf.String())
	}
}
