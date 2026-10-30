package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/store"
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
	recovered_at TEXT,
	origin TEXT
)`

// Miss origins. A miss recorded from a search that succeeded and found
// nothing is missOriginLookup, a current gap. A row written before searches
// recorded their outcome (2026-10-30) has no origin: it may have been a
// row the budget left out or a failed search, so it is not a gap until
// SettleLegacyMisses decides it, and one it cannot show is still unanswered
// becomes missOriginLegacy, history.
const (
	missOriginLookup = "lookup"
	missOriginLegacy = "legacy"
)

// ensureMissesTable creates knowledge_misses, and adds the columns a table
// made before them lacks: recovered_at (never recovered) and origin (written
// before lookup outcomes; SettleLegacyMisses decides it).
func ensureMissesTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, missesSchema); err != nil {
		return fmt.Errorf("misses table: %w", err)
	}
	cols, err := missesColumns(ctx, db)
	if err != nil {
		return err
	}
	for _, col := range []string{"recovered_at", "origin"} {
		if !cols[col] {
			if _, err := db.ExecContext(ctx, `ALTER TABLE knowledge_misses ADD COLUMN `+col+` TEXT`); err != nil {
				return fmt.Errorf("misses %s: %w", col, err)
			}
		}
	}
	return nil
}

// missesColumns is the set of knowledge_misses columns.
func missesColumns(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info('knowledge_misses')`)
	if err != nil {
		return nil, fmt.Errorf("misses columns: %w", err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("misses columns: %w", err)
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// SettleLegacyMisses decides each miss row written before searches recorded
// their outcome, from what can be shown now: the question is searched in the
// KB again, and a search that returns rows (placeholders aside) shows the KB
// is not empty for it -- the row becomes history (missOriginLegacy); a search
// that succeeds and finds nothing shows it is a current gap
// (missOriginLookup). A failed search decides nothing; the row waits for the
// next run. It returns how many rows became current and how many history.
func SettleLegacyMisses(ctx context.Context, db *sql.DB) (current, history int, err error) {
	if db == nil {
		return 0, 0, nil
	}
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'knowledge_misses'`).Scan(&exists); err != nil || exists == 0 {
		return 0, 0, err
	}
	if err := ensureMissesTable(ctx, db); err != nil {
		return 0, 0, err
	}
	rows, err := db.QueryContext(ctx, `SELECT query FROM knowledge_misses WHERE origin IS NULL`)
	if err != nil {
		return 0, 0, fmt.Errorf("read legacy misses: %w", err)
	}
	var queries []string
	for rows.Next() {
		var q string
		if err := rows.Scan(&q); err != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("read legacy misses: %w", err)
		}
		queries = append(queries, q)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("read legacy misses: %w", err)
	}
	for _, q := range queries {
		atoms, searchErr := store.SearchKnowledgeAtomsLexicalDB(ctx, db, q, legacySettleRows)
		if searchErr != nil {
			continue
		}
		origin := missOriginLookup
		for _, a := range atoms {
			if !IsLegacyPlaceholder(a.Concept, a.Content) {
				origin = missOriginLegacy
				break
			}
		}
		if _, err := db.ExecContext(ctx, `UPDATE knowledge_misses SET origin = ? WHERE query = ? AND origin IS NULL`, origin, q); err != nil {
			return current, history, fmt.Errorf("settle legacy miss: %w", err)
		}
		if origin == missOriginLookup {
			current++
		} else {
			history++
		}
	}
	return current, history, nil
}

// legacySettleRows is how many rows the settling search asks for: whether
// any comes back is the whole question.
const legacySettleRows = 5

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
		`INSERT INTO knowledge_misses (query, misses, last_missed_at, origin) VALUES (?, 1, ?, ?)
		 ON CONFLICT(query) DO UPDATE SET misses = misses + 1, last_missed_at = excluded.last_missed_at, origin = excluded.origin`,
		query, now.UTC().Format(time.RFC3339), missOriginLookup); err != nil {
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
