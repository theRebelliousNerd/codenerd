package sqlpragmas

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

// TestApplyDefaultPragmas_AllProfilesIntegration is the cross-profile
// integration test: it walks every PragmaProfile, opens a fresh tempfile
// SQLite DB per profile, applies the preset, and reads each documented
// pragma back. This catches regressions where adding a new profile
// accidentally breaks another (e.g. a copy-pasted journal_mode line
// landing in ProfileReadOnly).
//
// Per-profile tests already exist in pragmas_test.go; this test focuses
// on the integration across profiles AND validates the table of
// documented expected values in one place.
func TestApplyDefaultPragmas_AllProfilesIntegration(t *testing.T) {
	tests := []struct {
		name    string
		profile PragmaProfile
		// wantJournalIsWAL: true means we expect "wal"; false means we expect
		// NOT "wal" (ReadOnly leaves the default in place).
		wantJournalIsWAL bool
		wantCacheSize    int64
		wantSynchronous  int64 // -1 means "don't assert" (read-only profile)
		wantBusyTimeout  int64
		wantTempStore    int64
		// wantWALCheckpoint: 0 means "no assertion" (Query+ReadOnly omit it).
		wantWALCheckpoint int64
	}{
		{
			name:              "ProfileHot",
			profile:           ProfileHot,
			wantJournalIsWAL:  true,
			wantCacheSize:     -2097152,
			wantSynchronous:   1,
			wantBusyTimeout:   10000,
			wantTempStore:     2,
			wantWALCheckpoint: 10000,
		},
		{
			name:              "ProfileBulkBuild",
			profile:           ProfileBulkBuild,
			wantJournalIsWAL:  true,
			wantCacheSize:     -4194304,
			wantSynchronous:   1,
			wantBusyTimeout:   10000,
			wantTempStore:     2,
			wantWALCheckpoint: 20000,
		},
		{
			name:             "ProfileQuery",
			profile:          ProfileQuery,
			wantJournalIsWAL: true,
			wantCacheSize:    -524288,
			wantSynchronous:  1,
			wantBusyTimeout:  10000,
			wantTempStore:    2,
		},
		{
			name:             "ProfileReadOnly",
			profile:          ProfileReadOnly,
			wantJournalIsWAL: false,
			wantCacheSize:    -524288,
			wantSynchronous:  -1, // not asserted; read-only skips this PRAGMA
			wantBusyTimeout:  10000,
			wantTempStore:    2,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			db := openFreshTempDB(t)

			ApplyDefaultPragmas(db, tc.profile)

			// mmap_size must be positive across every profile; we don't pin
			// an exact value because SQLite caps mmap on Windows below the
			// requested 8/16 GiB (matches existing test rationale).
			require.Positive(t, readPragmaInt(t, db, "mmap_size"),
				"%s: mmap should be enabled (positive size)", tc.name)

			require.Equal(t, tc.wantCacheSize, readPragmaInt(t, db, "cache_size"),
				"%s: cache_size mismatch", tc.name)
			require.Equal(t, tc.wantBusyTimeout, readPragmaInt(t, db, "busy_timeout"),
				"%s: busy_timeout mismatch", tc.name)
			require.Equal(t, tc.wantTempStore, readPragmaInt(t, db, "temp_store"),
				"%s: temp_store mismatch", tc.name)

			journal := readPragmaString(t, db, "journal_mode")
			if tc.wantJournalIsWAL {
				require.Equal(t, "wal", journal,
					"%s: should set journal_mode=wal", tc.name)
			} else {
				require.NotEqual(t, "wal", journal,
					"%s: must NOT enable WAL (read-only profile)", tc.name)
			}

			if tc.wantSynchronous >= 0 {
				require.Equal(t, tc.wantSynchronous,
					readPragmaInt(t, db, "synchronous"),
					"%s: synchronous mismatch", tc.name)
			}

			if tc.wantWALCheckpoint > 0 {
				require.Equal(t, tc.wantWALCheckpoint,
					readPragmaInt(t, db, "wal_autocheckpoint"),
					"%s: wal_autocheckpoint mismatch", tc.name)
			}
		})
	}
}

// TestApplyDefaultPragmas_Idempotent verifies repeated application of
// the same profile is a no-op for observable pragma state. This matters
// because some code paths (autopoiesis startup, prompt cache rebuild)
// re-apply pragmas on every reopen of a long-lived DB.
func TestApplyDefaultPragmas_Idempotent(t *testing.T) {
	db := openFreshTempDB(t)
	ApplyDefaultPragmas(db, ProfileHot)

	cache1 := readPragmaInt(t, db, "cache_size")
	journal1 := readPragmaString(t, db, "journal_mode")

	ApplyDefaultPragmas(db, ProfileHot)
	ApplyDefaultPragmas(db, ProfileHot)

	require.Equal(t, cache1, readPragmaInt(t, db, "cache_size"),
		"cache_size must not drift across re-applications")
	require.Equal(t, journal1, readPragmaString(t, db, "journal_mode"),
		"journal_mode must not drift across re-applications")
}

// openFreshTempDB opens a one-connection tempfile sqlite DB. We pin to
// MaxOpenConns=1 so subsequent PRAGMA reads land on the same connection
// the writes targeted — pragmas are per-connection in SQLite, so a
// connection pool would scramble the assertions.
func openFreshTempDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "integration.db")
	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err, "sql.Open")
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db
}

func readPragmaInt(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	var v int64
	require.NoError(t, db.QueryRow("PRAGMA "+name).Scan(&v),
		"PRAGMA %s read", name)
	return v
}

func readPragmaString(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var v string
	require.NoError(t, db.QueryRow("PRAGMA "+name).Scan(&v),
		"PRAGMA %s read", name)
	return v
}
