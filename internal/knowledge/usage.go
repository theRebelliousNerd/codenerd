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
	last_missed_at TEXT NOT NULL
)`

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
	if _, err := db.ExecContext(ctx, missesSchema); err != nil {
		return fmt.Errorf("misses table: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO knowledge_misses (query, misses, last_missed_at) VALUES (?, 1, ?)
		 ON CONFLICT(query) DO UPDATE SET misses = misses + 1, last_missed_at = excluded.last_missed_at`,
		query, now.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("record miss: %w", err)
	}
	return nil
}
