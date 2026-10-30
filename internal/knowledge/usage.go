package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Consumption. A shard agent's knowledge is consumed when a compile pushes
// it into a prompt: each consultation records which atoms it retrieved, and
// a consultation that found nothing records the question it could not
// answer. The refresh policy (policy/knowledge_refresh.mg) reads both:
// heavily used agents are refreshed first, never-retrieved acquired atoms are
// pruned, and a question no atom answered is a gap to research.

const usageSchema = `CREATE TABLE IF NOT EXISTS knowledge_usage (
	concept TEXT PRIMARY KEY,
	retrieved INTEGER NOT NULL,
	last_retrieved_at TEXT NOT NULL
)`

const missesSchema = `CREATE TABLE IF NOT EXISTS knowledge_misses (
	query TEXT PRIMARY KEY,
	misses INTEGER NOT NULL,
	last_missed_at TEXT NOT NULL,
	recovered_at TEXT
)`

// ensureMissesTable creates knowledge_misses, and adds recovered_at to a
// table made before it existed. A row without it has never recovered.
func ensureMissesTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, missesSchema); err != nil {
		return fmt.Errorf("misses table: %w", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info('knowledge_misses')`)
	if err != nil {
		return fmt.Errorf("misses columns: %w", err)
	}
	has := false
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("misses columns: %w", err)
		}
		has = has || name == "recovered_at"
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("misses columns: %w", err)
	}
	if !has {
		if _, err := db.ExecContext(ctx, `ALTER TABLE knowledge_misses ADD COLUMN recovered_at TEXT`); err != nil {
			return fmt.Errorf("misses recovered_at: %w", err)
		}
	}
	return nil
}

// RecordRetrieval counts one consultation's retrieval of each concept.
func RecordRetrieval(ctx context.Context, db *sql.DB, concepts []string, now time.Time) error {
	if db == nil || len(concepts) == 0 {
		return nil
	}
	if _, err := db.ExecContext(ctx, usageSchema); err != nil {
		return fmt.Errorf("usage table: %w", err)
	}
	at := now.UTC().Format(time.RFC3339)
	seen := make(map[string]bool, len(concepts))
	for _, concept := range concepts {
		if concept == "" || seen[concept] {
			continue
		}
		seen[concept] = true
		if _, err := db.ExecContext(ctx,
			`INSERT INTO knowledge_usage (concept, retrieved, last_retrieved_at) VALUES (?, 1, ?)
			 ON CONFLICT(concept) DO UPDATE SET retrieved = retrieved + 1, last_retrieved_at = excluded.last_retrieved_at`,
			concept, at); err != nil {
			return fmt.Errorf("record retrieval of %s: %w", concept, err)
		}
	}
	return nil
}

// RecordMiss counts a consultation that found nothing for query (the task's
// own words, kept whole: it is what a gap is researched as).
func RecordMiss(ctx context.Context, db *sql.DB, query string, now time.Time) error {
	query = strings.TrimSpace(query)
	if db == nil || query == "" {
		return nil
	}
	if err := ensureMissesTable(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO knowledge_misses (query, misses, last_missed_at) VALUES (?, 1, ?)
		 ON CONFLICT(query) DO UPDATE SET misses = misses + 1, last_missed_at = excluded.last_missed_at`,
		query, now.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("record miss: %w", err)
	}
	return nil
}

// RecordRecovery marks query recovered: a compile delivered this KB's rows
// for it. The miss row stays as history; the measurement reports it
// recovered while no miss is newer (knowledge_miss_recovered), and a later
// genuine miss reopens it. A query with no miss row records nothing.
func RecordRecovery(ctx context.Context, db *sql.DB, query string, now time.Time) error {
	query = strings.TrimSpace(query)
	if db == nil || query == "" {
		return nil
	}
	if err := ensureMissesTable(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE knowledge_misses SET recovered_at = ? WHERE query = ?`,
		now.UTC().Format(time.RFC3339), query); err != nil {
		return fmt.Errorf("record recovery: %w", err)
	}
	return nil
}
