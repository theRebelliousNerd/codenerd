package context

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/sqlpragmas"

	"github.com/stretchr/testify/require"
)

// An archive opened before handles existed has the table and not the column.
// Opening it fills a short ordinal per scope, in step order, and a second
// open does not renumber the rows the model may already have been shown.
func TestWorkingStore_LegacyArchiveGainsShortHandles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sqlpragmas.OpenWithPragmas("sqlite3", filepath.ToSlash(path)+"?_busy_timeout=5000&_journal_mode=WAL", sqlpragmas.ProfileHot)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE working_records (
		id TEXT PRIMARY KEY, scope TEXT NOT NULL, entity TEXT NOT NULL,
		revision TEXT NOT NULL, kind TEXT NOT NULL, step INTEGER NOT NULL,
		body TEXT NOT NULL, failed INTEGER NOT NULL, digest TEXT NOT NULL DEFAULT '',
		span_start INTEGER NOT NULL DEFAULT 0, span_end INTEGER NOT NULL DEFAULT 0)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO working_records (id, scope, entity, revision, kind, step, body, failed) VALUES
		('aaa', 'scope-a', 'a.go', 'rev', 'read', 1, 'body-a', 0),
		('bbb', 'scope-a', 'a.go', 'rev', 'read', 2, 'body-b', 0),
		('ccc', 'scope-b', 'b.go', 'rev', 'read', 5, 'body-c', 0)`)
	require.NoError(t, err)

	require.NoError(t, ensureWorkingHandles(db))
	require.Equal(t, map[string]string{"aaa": "1", "bbb": "2", "ccc": "1"}, workingHandles(t, db))

	require.NoError(t, ensureWorkingHandles(db))
	require.Equal(t, map[string]string{"aaa": "1", "bbb": "2", "ccc": "1"}, workingHandles(t, db), "a second open must not renumber")
}

func workingHandles(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT id, handle FROM working_records ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var id, handle string
		require.NoError(t, rows.Scan(&id, &handle))
		got[id] = handle
	}
	require.NoError(t, rows.Err())
	return got
}

// The ordinal is per stored observation. Saving the same id again keeps the
// handle the model was shown, and the storage id is not a handle.
func TestWorkingStore_ASecondSaveKeepsTheOrdinal(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0o600))
	w, err := NewWorkingSet(root, "handles", config.DefaultWorkingConfig())
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	first := WorkingRecord{ID: "stor-aaa", Entity: "a.go", Revision: w.Revision("a.go"), Kind: "read", Step: 1, Body: "payload-one"}
	require.NoError(t, w.Save(t.Context(), first))
	require.NoError(t, w.Save(t.Context(), first))
	require.NoError(t, w.Save(t.Context(), WorkingRecord{ID: "stor-bbb", Entity: "a.go", Revision: w.Revision("a.go"), Kind: "read", Step: 2, Body: "payload-two"}))

	handle, err := w.Handle(t.Context(), "stor-aaa")
	require.NoError(t, err)
	require.Equal(t, "1", handle)
	handle, err = w.Handle(t.Context(), "stor-bbb")
	require.NoError(t, err)
	require.Equal(t, "2", handle, "the second save of stor-aaa must not consume an ordinal")

	_, err = w.Recall(t.Context(), "stor-aaa", 0, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), `no archived observation has id "stor-aaa"`)
	require.NotContains(t, err.Error(), "sql: no rows")

	page, err := w.Recall(t.Context(), "1", 0, 0)
	require.NoError(t, err)
	require.Contains(t, page, "payload-one")
	require.Contains(t, page, `"id":"1"`)
	require.NotContains(t, page, "stor-aaa")

	out, err := w.Search(t.Context(), "payload-one", 0, 10)
	require.NoError(t, err)
	require.Contains(t, out, `"id":"1"`)
	require.NotContains(t, out, "stor-aaa")
}
