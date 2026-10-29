package store

import (
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
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

func TestRunMigrations_WhenExistingTableCannotBeAltered_ShouldReturnError(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`CREATE TABLE knowledge_atoms (
		id INTEGER PRIMARY KEY,
		concept TEXT,
		content TEXT
	)`)
	assert.NoError(t, err)
	_, err = db.Exec("PRAGMA query_only = ON")
	assert.NoError(t, err)

	// The table exists and its legacy columns are discoverable, but query_only
	// prevents the additive source/tags migrations. Reporting success here
	// would let constructors publish a database with a stale schema.
	err = RunMigrations(db)
	assert.Error(t, err, "a failed ALTER TABLE on an existing table must fail migration")
}

func TestRunMigrations_WhenMigrationTablesAreMissing_ShouldSkipThem(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	assert.NoError(t, RunMigrations(db))
	var count int
	assert.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count))
	assert.Zero(t, count, "missing migration tables must remain skippable")
}

func TestMigrationEntryPoints_WhenSchemaVersionInspectionFails_ShouldReturnError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "malformed.db")
	db, err := sql.Open("sqlite3", dbPath)
	assert.NoError(t, err)
	_, err = db.Exec("CREATE TABLE schema_versions (id INTEGER PRIMARY KEY)")
	assert.NoError(t, err)
	assert.NoError(t, db.Close())

	needed, version, err := CheckMigrationNeeded(dbPath)
	assert.Error(t, err)
	assert.False(t, needed)
	assert.Zero(t, version)

	result, err := RunAllMigrations(dbPath, CurrentSchemaVersion)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestRunMigrations_WhenTwoOpenersRaceToAddColumns_ShouldRemainIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "knowledge.db")
	setup, err := sql.Open("sqlite3", dbPath)
	assert.NoError(t, err)
	_, err = setup.Exec("CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT)")
	assert.NoError(t, err)
	assert.NoError(t, setup.Close())

	dbs := make([]*sql.DB, 2)
	for i := range dbs {
		dbs[i], err = sql.Open("sqlite3", dbPath)
		assert.NoError(t, err)
		dbs[i].SetMaxOpenConns(1)
		dbs[i].SetMaxIdleConns(1)
		ApplyDefaultPragmas(dbs[i], ProfileHot)
		t.Cleanup(func() { _ = dbs[i].Close() })
	}

	start := make(chan struct{})
	results := make(chan error, len(dbs))
	var wg sync.WaitGroup
	for _, db := range dbs {
		wg.Add(1)
		go func(db *sql.DB) {
			defer wg.Done()
			<-start
			results <- RunMigrations(db)
		}(db)
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		assert.NoError(t, err)
	}

	for _, column := range []string{"source", "tags"} {
		assert.True(t, columnExists(dbs[0], "knowledge_atoms", column), "migration column %s missing", column)
	}
}
