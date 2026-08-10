package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestVecExtensionAvailableAgreesWithDetection pins the read-only probe used by
// search-time fallback logging to the same answer the init-time probe gives.
// If these two ever disagree the fallback message blames the wrong layer, which
// is exactly the defect this probe was added to fix: a cold learning index was
// reported as "sqlite-vec not available" on a build where sqlite-vec worked.
func TestVecExtensionAvailableAgreesWithDetection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probe.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	s := &LocalStore{db: db}
	s.detectVecExtension()

	if got := vecExtensionAvailable(db); got != s.vectorExt {
		t.Fatalf("vecExtensionAvailable()=%v but detectVecExtension set vectorExt=%v; "+
			"the search-time fallback would report the wrong cause", got, s.vectorExt)
	}
}

// TestVecExtensionAvailableIsReadOnly guards the reason this helper exists
// separately from detectVecExtension: it runs on a search path, so it must not
// create or drop tables the way the init-time probe does.
func TestVecExtensionAvailableIsReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probe_ro.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	before := vecProbeTableNames(t, db)
	_ = vecExtensionAvailable(db)
	after := vecProbeTableNames(t, db)

	if len(before) != len(after) {
		t.Fatalf("probe mutated schema: before=%v after=%v", before, after)
	}
}

func vecProbeTableNames(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM sqlite_master ORDER BY name")
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, n)
	}
	return names
}
