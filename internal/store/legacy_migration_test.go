package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestLocalStore_MigratesPopulatedLegacyColdStorageAcrossReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "knowledge.db")
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE cold_storage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		predicate TEXT NOT NULL,
		args TEXT NOT NULL,
		fact_type TEXT DEFAULT 'fact',
		priority INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(predicate, args)
	)`)
	require.NoError(t, err)
	legacyArgs, err := encodeFactArgs([]any{"retained"})
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO cold_storage (predicate, args, updated_at)
		VALUES ('legacy_fact', ?, datetime('now', '-100 days'))`, legacyArgs)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	local, err := NewLocalStore(dbPath)
	require.NoError(t, err)
	var legacyUpdated, legacyAccessed sql.NullString
	require.NoError(t, local.db.QueryRow(`SELECT updated_at, last_accessed FROM cold_storage WHERE predicate='legacy_fact'`).Scan(&legacyUpdated, &legacyAccessed))
	require.True(t, legacyAccessed.Valid, "migration must backfill existing access timestamps")
	require.Equal(t, legacyUpdated.String, legacyAccessed.String, "migration must preserve the original update timestamp")
	require.NoError(t, local.StoreFact("fresh_fact", []any{"new"}, "fact", 1))
	var freshAccessed sql.NullString
	require.NoError(t, local.db.QueryRow(`SELECT last_accessed FROM cold_storage WHERE predicate='fresh_fact'`).Scan(&freshAccessed))
	require.True(t, freshAccessed.Valid, "new writes must populate the migrated nullable column")
	require.NoError(t, local.Close())

	local, err = NewLocalStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = local.Close() })
	facts, err := local.LoadFacts("legacy_fact")
	require.NoError(t, err)
	require.Len(t, facts, 1)
	require.Equal(t, "legacy_fact", facts[0].Predicate)
	require.Equal(t, []any{"retained"}, facts[0].Args)
	require.False(t, facts[0].UpdatedAt.IsZero(), "legacy fact update time must survive reopen")
}
