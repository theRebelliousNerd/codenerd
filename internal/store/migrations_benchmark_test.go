package store

import (
	"database/sql"

	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func BenchmarkRunMigrations(b *testing.B) {
	// Create a temporary database for benchmarking
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatalf("Failed to open memory database: %v", err)
	}
	defer db.Close()

	// Create tables that match some of the pending migrations
	// so checks have something to find
	setupSQL := `
	CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT);
	CREATE TABLE prompt_atoms (id INTEGER PRIMARY KEY, prompt TEXT);
	CREATE TABLE cold_storage (id INTEGER PRIMARY KEY, data TEXT);
	`
	if _, err := db.Exec(setupSQL); err != nil {
		b.Fatalf("Failed to setup benchmark db: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// We call RunMigrations repeatedly.
		// Note: The first call might apply migrations, subsequent calls will check and find columns exist.
		// Since we want to benchmark the checking logic (the N+1 issue), this is valid.
		// However, after the first run, the columns will exist, so the "ADD COLUMN" part won't happen.
		// The checking logic happens in every run regardless.
		if err := RunMigrations(db); err != nil {
			b.Fatalf("RunMigrations failed: %v", err)
		}
	}
}
