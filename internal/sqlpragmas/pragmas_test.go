package sqlpragmas

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Open a tempfile-backed sqlite (NOT :memory:) — PRAGMA mmap_size reports 0
// on in-memory databases regardless of what we set.
func openTempDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "pragmas_test.db")
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	// Pin to a single connection so all PRAGMAs (which are per-connection)
	// land on the same handle we later interrogate.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db
}

func pragmaInt(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	var v int64
	if err := db.QueryRow("PRAGMA " + name).Scan(&v); err != nil {
		t.Fatalf("read PRAGMA %s: %v", name, err)
	}
	return v
}

func pragmaString(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var v string
	if err := db.QueryRow("PRAGMA " + name).Scan(&v); err != nil {
		t.Fatalf("read PRAGMA %s: %v", name, err)
	}
	return v
}

// assertMmapEnabled checks that mmap_size was set to a positive value. SQLite
// (especially the bundled CGO build on Windows) silently caps mmap_size to
// somewhere below 2 GiB even when we request 8/16 GiB, so we cannot assert the
// exact requested value cross-platform. A positive value confirms the PRAGMA
// landed and the OS accepted some memory-mapped window.
func assertMmapEnabled(t *testing.T, db *sql.DB) {
	t.Helper()
	if got := pragmaInt(t, db, "mmap_size"); got <= 0 {
		t.Errorf("mmap_size = %d, want > 0 (mmap should be enabled)", got)
	}
}

func TestApplyDefaultPragmas_ProfileHot(t *testing.T) {
	db := openTempDB(t)
	ApplyDefaultPragmas(db, ProfileHot)

	assertMmapEnabled(t, db)
	if got, want := pragmaInt(t, db, "cache_size"), int64(-2097152); got != want {
		t.Errorf("cache_size = %d, want %d", got, want)
	}
	if got, want := pragmaInt(t, db, "temp_store"), int64(2); got != want {
		t.Errorf("temp_store = %d, want 2 (MEMORY)", got)
	}
	if got, want := pragmaString(t, db, "journal_mode"), "wal"; got != want {
		t.Errorf("journal_mode = %q, want %q", got, want)
	}
	if got, want := pragmaInt(t, db, "synchronous"), int64(1); got != want {
		t.Errorf("synchronous = %d, want 1 (NORMAL)", got)
	}
	if got, want := pragmaInt(t, db, "busy_timeout"), int64(10000); got != want {
		t.Errorf("busy_timeout = %d, want 10000", got)
	}
}

func TestApplyDefaultPragmas_ProfileBulkBuild(t *testing.T) {
	db := openTempDB(t)
	ApplyDefaultPragmas(db, ProfileBulkBuild)

	assertMmapEnabled(t, db)
	if got, want := pragmaInt(t, db, "cache_size"), int64(-4194304); got != want {
		t.Errorf("cache_size = %d, want %d", got, want)
	}
	if got, want := pragmaInt(t, db, "wal_autocheckpoint"), int64(20000); got != want {
		t.Errorf("wal_autocheckpoint = %d, want 20000", got)
	}
}

func TestApplyDefaultPragmas_ProfileQuery(t *testing.T) {
	db := openTempDB(t)
	ApplyDefaultPragmas(db, ProfileQuery)

	assertMmapEnabled(t, db)
	if got, want := pragmaInt(t, db, "cache_size"), int64(-524288); got != want {
		t.Errorf("cache_size = %d, want %d", got, want)
	}
}

func TestApplyDefaultPragmas_ProfileReadOnly_NoJournalChange(t *testing.T) {
	db := openTempDB(t)
	ApplyDefaultPragmas(db, ProfileReadOnly)

	if got := pragmaString(t, db, "journal_mode"); got == "wal" {
		t.Errorf("ProfileReadOnly should not enable WAL, got journal_mode=%q", got)
	}
	assertMmapEnabled(t, db)
}

func TestApplyDefaultPragmas_NilDB_NoPanic(t *testing.T) {
	ApplyDefaultPragmas(nil, ProfileHot)
}
