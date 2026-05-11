package store

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	_ "github.com/mattn/go-sqlite3"
)

func TestBackfillContentHashes(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err)
	defer db.Close()

	setupSQL := `
	CREATE TABLE knowledge_atoms (
		id INTEGER PRIMARY KEY,
		concept TEXT,
		content TEXT,
		content_hash TEXT
	);
	`
	_, err = db.Exec(setupSQL)
	assert.NoError(t, err)

	_, err = db.Exec("INSERT INTO knowledge_atoms (concept, content) VALUES ('a', 'b'), ('c', 'd')")
	assert.NoError(t, err)

	updated, err := BackfillContentHashes(db)
	assert.NoError(t, err)
	assert.Equal(t, 2, updated)

	rows, err := db.Query("SELECT content_hash FROM knowledge_atoms WHERE concept = 'a'")
	assert.NoError(t, err)
	defer rows.Close()

	assert.True(t, rows.Next())
	var hash string
	assert.NoError(t, rows.Scan(&hash))
	assert.NotEmpty(t, hash)

	expectedHash := ComputeContentHash("a", "b")
	assert.Equal(t, expectedHash, hash)
}
