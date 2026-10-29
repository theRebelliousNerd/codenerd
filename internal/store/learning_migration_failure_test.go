package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestLearningStore_WhenMigrationFails_ShouldNotPublishShardDB(t *testing.T) {
	basePath := t.TempDir()
	dbPath := filepath.Join(basePath, "expert_learnings.db")
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	maxColumns := sqliteMaxColumnLimit(t, db)
	require.Greater(t, maxColumns, 7)

	columns := []string{
		"id INTEGER PRIMARY KEY AUTOINCREMENT",
		"shard_type TEXT NOT NULL",
		"fact_predicate TEXT NOT NULL",
		"fact_args TEXT NOT NULL",
		"learned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP",
		"source_campaign TEXT DEFAULT ''",
		"confidence REAL DEFAULT 1.0",
	}
	for i := len(columns); i < maxColumns; i++ {
		columns = append(columns, fmt.Sprintf("legacy_%d TEXT", i))
	}
	_, err = db.Exec("CREATE TABLE learnings (" + strings.Join(columns, ",") + ")")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	ls, err := NewLearningStore(basePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ls.Close() })

	_, err = ls.getDB("expert")
	require.Error(t, err, "the max-column legacy schema cannot accept the required additive fields")
	ls.mu.RLock()
	_, published := ls.dbs["expert"]
	ls.mu.RUnlock()
	require.False(t, published, "a failed migration must not publish the shard database")
}

func TestLearningStore_LegacyMigrationSurvivesCloseAndReopen(t *testing.T) {
	basePath := t.TempDir()
	dbPath := filepath.Join(basePath, "expert_learnings.db")
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE learnings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		shard_type TEXT NOT NULL,
		fact_predicate TEXT NOT NULL,
		fact_args TEXT NOT NULL,
		learned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		source_campaign TEXT DEFAULT '',
		confidence REAL DEFAULT 1.0,
		UNIQUE(fact_predicate, fact_args)
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO learnings (shard_type, fact_predicate, fact_args)
		VALUES ('expert', 'legacy_fact', '["retained"]')`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	for attempt := 0; attempt < 2; attempt++ {
		ls, err := NewLearningStore(basePath)
		require.NoError(t, err)
		candidates, err := ls.ListLearningEmbeddingCandidates("expert", 10, "", 0, "")
		require.NoError(t, err)
		require.Len(t, candidates, 1)
		require.Equal(t, "legacy_fact", candidates[0].FactPredicate)
		require.Equal(t, []any{"retained"}, candidates[0].FactArgs)
		require.NoError(t, ls.Close())
	}
}

func sqliteMaxColumnLimit(t *testing.T, db *sql.DB) int {
	t.Helper()
	rows, err := db.Query("PRAGMA compile_options")
	require.NoError(t, err)
	defer rows.Close()

	limit := 2000 // SQLite's documented default SQLITE_MAX_COLUMN.
	for rows.Next() {
		var option string
		require.NoError(t, rows.Scan(&option))
		if strings.HasPrefix(option, "MAX_COLUMN=") {
			limit, err = strconv.Atoi(strings.TrimPrefix(option, "MAX_COLUMN="))
			require.NoError(t, err)
			break
		}
	}
	require.NoError(t, rows.Err())
	return limit
}
