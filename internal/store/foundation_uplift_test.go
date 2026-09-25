package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// openWALTestDB creates a scratch WAL-mode database with one table. WAL is
// the production journal mode (ProfileHot), and it is exactly the mode where
// a naive file copy misses uncheckpointed commits.
func openWALTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE t (v TEXT)"); err != nil {
		t.Fatal(err)
	}
	return db
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A backup taken while the writer is open must still contain every committed
// row: without the pre-backup checkpoint, commits sitting in the -wal file
// never reach the copy, and the backup opens fine but is silently stale.
func TestBackup_CapturesUncheckpointedWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.db")
	w := openWALTestDB(t, path)
	if _, err := w.Exec("INSERT INTO t (v) VALUES ('a'), ('b'), ('c')"); err != nil {
		t.Fatal(err)
	}
	// Writer stays open with frames uncheckpointed — the production shape at
	// migration time, where RunAllMigrations holds its handle throughout.

	backupPath, err := CreateBackup(path)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	ro, err := sql.Open("sqlite3", backupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if got := countRows(t, ro, "t"); got != 3 {
		t.Errorf("backup holds %d rows, want 3 committed rows", got)
	}
}

// Restore rewinds to the backup point and leaves no stale WAL sidecars
// behind: replaying pre-restore frames over restored bytes would corrupt the
// restore it just reported as successful.
func TestRestore_RewindsAndDropsSidecars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.db")
	w := openWALTestDB(t, path)
	if _, err := w.Exec("INSERT INTO t (v) VALUES ('before')"); err != nil {
		t.Fatal(err)
	}
	backupPath, err := CreateBackup(path)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if _, err := w.Exec("INSERT INTO t (v) VALUES ('after')"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	if err := RestoreBackup(path, backupPath); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	for _, sidecar := range []string{path + "-wal", path + "-shm"} {
		if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
			t.Errorf("stale sidecar %s survived the restore", sidecar)
		}
	}
	ro, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	var v string
	if err := ro.QueryRow("SELECT v FROM t").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != "before" || countRows(t, ro, "t") != 1 {
		t.Errorf("restored content = %q with %d rows, want exactly [before]", v, countRows(t, ro, "t"))
	}
}
