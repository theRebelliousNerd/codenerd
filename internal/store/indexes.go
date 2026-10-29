package store

import (
	"database/sql"
	"fmt"

	"codenerd/internal/logging"
)

func ensureIndexIfColumn(db *sql.DB, table, column, indexName string) {
	if db == nil {
		return
	}
	if !tableExists(db, table) || !columnExists(db, table, column) {
		return
	}
	query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s(%s);", indexName, table, column)
	if _, err := db.Exec(query); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to create index %s on %s(%s): %v", indexName, table, column, err)
	}
}

func ensureReasoningTraceIndexes(db *sql.DB) {
	ensureIndexIfColumn(db, "reasoning_traces", "shard_type", "idx_traces_shard_type")
	ensureIndexIfColumn(db, "reasoning_traces", "session_id", "idx_traces_session")
	ensureIndexIfColumn(db, "reasoning_traces", "shard_id", "idx_traces_shard_id")
	ensureIndexIfColumn(db, "reasoning_traces", "success", "idx_traces_success")
	ensureIndexIfColumn(db, "reasoning_traces", "created_at", "idx_traces_created")
	ensureIndexIfColumn(db, "reasoning_traces", "shard_category", "idx_traces_category")
	ensureIndexIfColumn(db, "reasoning_traces", "descriptor_hash", "idx_traces_descriptor_hash")
}
