package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Source generations give a SQLite table an owned, transactional version row
// so a consumer that caches something derived from the table (the JIT prompt
// cache, today) can tell "same data" from "changed data" with one cheap read.
//
// The row lives in sourceGenerationTable and is advanced by AFTER triggers on
// the tracked table, so it commits in the same transaction as the write that
// caused it, from any connection and any process. That is the property
// PRAGMA data_version lacks: data_version is comparable only on one
// connection and does not move for that connection's own commits, so it
// cannot certify pooled reads.
//
// Every token also carries the schema version. Dropping and recreating a
// tracked table drops its triggers with it; the schema change alters the
// token, and the next read re-installs the triggers before trusting the row.

const sourceGenerationTable = "codenerd_source_generation"

var sourceGenerationTableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

func sourceGenerationTriggers(table string) [3]string {
	return [3]string{
		"codenerd_srcgen_" + table + "_ins",
		"codenerd_srcgen_" + table + "_upd",
		"codenerd_srcgen_" + table + "_del",
	}
}

// EnsureSourceGenerationTracking installs the generation row and triggers for
// every listed table that exists. Absent tables are skipped: their absence is
// itself part of the token ReadSourceGeneration returns. Idempotent.
func EnsureSourceGenerationTracking(ctx context.Context, db *sql.DB, tables ...string) error {
	if db == nil {
		return fmt.Errorf("source generation: nil database")
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+sourceGenerationTable+` (
		source TEXT PRIMARY KEY,
		generation INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		return fmt.Errorf("source generation: create table: %w", err)
	}
	for _, table := range tables {
		if !sourceGenerationTableName.MatchString(table) {
			return fmt.Errorf("source generation: invalid table name %q", table)
		}
		var exists int
		if err := db.QueryRowContext(ctx,
			`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&exists); err != nil {
			return fmt.Errorf("source generation: probe %s: %w", table, err)
		}
		if exists == 0 {
			continue
		}
		if _, err := db.ExecContext(ctx,
			`INSERT OR IGNORE INTO `+sourceGenerationTable+` (source, generation) VALUES (?, 0)`, table,
		); err != nil {
			return fmt.Errorf("source generation: seed %s: %w", table, err)
		}
		bump := `BEGIN UPDATE ` + sourceGenerationTable +
			` SET generation = generation + 1 WHERE source = '` + table + `'; END`
		names := sourceGenerationTriggers(table)
		events := [3]string{"INSERT", "UPDATE", "DELETE"}
		for i, name := range names {
			stmt := `CREATE TRIGGER IF NOT EXISTS ` + name + ` AFTER ` + events[i] + ` ON ` + table + ` ` + bump
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("source generation: trigger %s: %w", name, err)
			}
		}
	}
	return nil
}

// ReadSourceGeneration returns a token that changes whenever any listed table
// is written, created, dropped, or altered. Two equal tokens from the same
// database mean the tables held the same rows. A table without its triggers
// is repaired before the token is trusted; if repair fails (a read-only
// database, say) the error is returned and the caller must treat the source
// as untracked rather than unchanged.
func ReadSourceGeneration(ctx context.Context, db *sql.DB, tables ...string) (string, error) {
	token, complete, err := readSourceGeneration(ctx, db, tables)
	if err == nil && complete {
		return token, nil
	}
	if ensureErr := EnsureSourceGenerationTracking(ctx, db, tables...); ensureErr != nil {
		if err != nil {
			return "", fmt.Errorf("%w (repair failed: %v)", err, ensureErr)
		}
		return "", ensureErr
	}
	token, complete, err = readSourceGeneration(ctx, db, tables)
	if err != nil {
		return "", err
	}
	if !complete {
		return "", fmt.Errorf("source generation: tracking incomplete after repair")
	}
	return token, nil
}

func readSourceGeneration(ctx context.Context, db *sql.DB, tables []string) (string, bool, error) {
	if db == nil {
		return "", false, fmt.Errorf("source generation: nil database")
	}
	// One read transaction, so the schema version, trigger presence and
	// generation rows all describe the same committed state.
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", false, fmt.Errorf("source generation: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var schemaVersion int64
	if err := tx.QueryRowContext(ctx, `PRAGMA schema_version`).Scan(&schemaVersion); err != nil {
		return "", false, fmt.Errorf("source generation: schema version: %w", err)
	}
	var b strings.Builder
	b.WriteString("schema=")
	b.WriteString(strconv.FormatInt(schemaVersion, 10))
	complete := true
	for _, table := range tables {
		if !sourceGenerationTableName.MatchString(table) {
			return "", false, fmt.Errorf("source generation: invalid table name %q", table)
		}
		names := sourceGenerationTriggers(table)
		var tableExists, triggers int
		if err := tx.QueryRowContext(ctx, `SELECT
			(SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?),
			(SELECT count(*) FROM sqlite_master WHERE type = 'trigger' AND tbl_name = ? AND name IN (?, ?, ?))`,
			table, table, names[0], names[1], names[2],
		).Scan(&tableExists, &triggers); err != nil {
			return "", false, fmt.Errorf("source generation: inspect %s: %w", table, err)
		}
		b.WriteByte(';')
		b.WriteString(table)
		b.WriteByte('=')
		if tableExists == 0 {
			b.WriteString("absent")
			continue
		}
		if triggers != len(names) {
			complete = false
			continue
		}
		var generation int64
		err := tx.QueryRowContext(ctx,
			`SELECT generation FROM `+sourceGenerationTable+` WHERE source = ?`, table,
		).Scan(&generation)
		if err != nil {
			// A missing row or a missing generation table both mean the
			// tracking is incomplete, not that the data is unchanged.
			complete = false
			continue
		}
		b.WriteString(strconv.FormatInt(generation, 10))
	}
	return b.String(), complete, nil
}

// knowledgeSourceTables are what JIT knowledge retrieval reads from the
// project LocalStore: the atoms themselves and the vectors semantic recall
// ranks them by (vec_index is a virtual table kept in step with vectors and
// cannot carry triggers).
var knowledgeSourceTables = []string{"knowledge_atoms", "vectors"}

// KnowledgeSourceGeneration identifies the knowledge rows JIT retrieval can
// read from this store. See ReadSourceGeneration.
func (s *LocalStore) KnowledgeSourceGeneration(ctx context.Context) (string, error) {
	if s == nil {
		return "", fmt.Errorf("source generation: nil local store")
	}
	return ReadSourceGeneration(ctx, s.GetDB(), knowledgeSourceTables...)
}

// SourceGeneration identifies every learnings row recall can read: the same
// shard databases, enumerated the same way, that RecallLearnings* consult.
func (ls *LearningStore) SourceGeneration(ctx context.Context) (string, error) {
	if ls == nil {
		return "", fmt.Errorf("source generation: nil learning store")
	}
	var b strings.Builder
	for _, shardType := range ls.listShardTypes() {
		db, err := ls.getDB(shardType)
		if err != nil {
			return "", fmt.Errorf("source generation: learnings %s: %w", shardType, err)
		}
		token, err := ReadSourceGeneration(ctx, db, "learnings")
		if err != nil {
			return "", fmt.Errorf("source generation: learnings %s: %w", shardType, err)
		}
		b.WriteString(shardType)
		b.WriteByte('{')
		b.WriteString(token)
		b.WriteByte('}')
	}
	return b.String(), nil
}
