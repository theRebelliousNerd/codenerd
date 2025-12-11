// Package store implements persistence for the Cortex framework.
// This file implements the LearningStore for Autopoiesis (§8.3).
// Each shard can learn from patterns and persist them for future sessions.
package store

import (
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Learning represents a persisted learning from Autopoiesis.
type Learning struct {
	ID             int64     `json:"id"`
	ShardType      string    `json:"shard_type"`     // "coder", "tester", "reviewer"
	FactPredicate  string    `json:"fact_predicate"` // e.g., "style_preference", "avoid_pattern"
	FactArgs       []any     `json:"fact_args"`      // Arguments to the predicate
	LearnedAt      time.Time `json:"learned_at"`
	SourceCampaign string    `json:"source_campaign"` // Campaign that taught this
	Confidence     float64   `json:"confidence"`      // Can decay over time (0.0-1.0)
}

// LearningStore manages shard learnings persistence per Cortex §8.3 Autopoiesis.
// Learnings are stored in SQLite files per shard type under .nerd/shards/.
type LearningStore struct {
	mu       sync.RWMutex
	basePath string
	dbs      map[string]*sql.DB // One DB per shard type
}

// NewLearningStore creates a new learning store at the specified base path.
// Default path is ".nerd/shards" in the working directory.
func NewLearningStore(basePath string) (*LearningStore, error) {
	if basePath == "" {
		workspace, err := config.FindWorkspaceRoot()
		if err != nil {
			workspace = "."
		}
		basePath = filepath.Join(workspace, ".nerd", "shards")
	}

	logging.Store("Initializing LearningStore at path: %s", basePath)

	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create learnings directory %s: %v", basePath, err)
		return nil, fmt.Errorf("failed to create learnings directory: %w", err)
	}

	logging.Store("LearningStore initialized for Autopoiesis persistence")
	return &LearningStore{
		basePath: basePath,
		dbs:      make(map[string]*sql.DB),
	}, nil
}

// getDB returns the database connection for a shard type, creating it if needed.
func (ls *LearningStore) getDB(shardType string) (*sql.DB, error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if db, ok := ls.dbs[shardType]; ok {
		return db, nil
	}

	// Create new DB for this shard type
	dbPath := filepath.Join(ls.basePath, fmt.Sprintf("%s_learnings.db", shardType))
	logging.StoreDebug("Opening learning database for shard=%s at %s", shardType, dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to open learnings db for %s: %v", shardType, err)
		return nil, fmt.Errorf("failed to open learnings db: %w", err)
	}

	// Initialize schema
	if err := ls.initializeSchema(db); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to initialize learning schema for %s: %v", shardType, err)
		db.Close()
		return nil, err
	}

	ls.dbs[shardType] = db
	logging.StoreDebug("Learning database ready for shard=%s", shardType)
	return db, nil
}

// initializeSchema creates the learnings table.
func (ls *LearningStore) initializeSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS learnings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		shard_type TEXT NOT NULL,
		fact_predicate TEXT NOT NULL,
		fact_args TEXT NOT NULL,
		learned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		source_campaign TEXT DEFAULT '',
		confidence REAL DEFAULT 1.0,
		UNIQUE(fact_predicate, fact_args)
	);
	CREATE INDEX IF NOT EXISTS idx_learnings_predicate ON learnings(fact_predicate);
	CREATE INDEX IF NOT EXISTS idx_learnings_confidence ON learnings(confidence);
	`
	_, err := db.Exec(schema)
	return err
}

// Save persists a learning to the store.
func (ls *LearningStore) Save(shardType string, factPredicate string, factArgs []any, sourceCampaign string) error {
	timer := logging.StartTimer(logging.CategoryStore, "LearningStore.Save")
	defer timer.Stop()

	db, err := ls.getDB(shardType)
	if err != nil {
		return err
	}

	logging.StoreDebug("Saving learning: shard=%s predicate=%s campaign=%s", shardType, factPredicate, sourceCampaign)

	argsJSON, err := json.Marshal(factArgs)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to marshal fact args: %v", err)
		return fmt.Errorf("failed to marshal fact args: %w", err)
	}

	// Upsert - if exists, just update confidence (reinforce learning)
	_, err = db.Exec(`
		INSERT INTO learnings (shard_type, fact_predicate, fact_args, source_campaign, confidence)
		VALUES (?, ?, ?, ?, 1.0)
		ON CONFLICT(fact_predicate, fact_args) DO UPDATE SET
			confidence = MIN(1.0, confidence + 0.1),
			learned_at = CURRENT_TIMESTAMP,
			source_campaign = excluded.source_campaign
	`, shardType, factPredicate, string(argsJSON), sourceCampaign)

	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to save learning %s: %v", factPredicate, err)
		return err
	}

	logging.StoreDebug("Learning saved/reinforced: %s for shard=%s", factPredicate, shardType)
	return nil
}

// Load retrieves all learnings for a shard type.
func (ls *LearningStore) Load(shardType string) ([]Learning, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LearningStore.Load")
	defer timer.Stop()

	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}

	logging.StoreDebug("Loading learnings for shard=%s (confidence > 0.3)", shardType)

	rows, err := db.Query(`
		SELECT id, shard_type, fact_predicate, fact_args, learned_at, source_campaign, confidence
		FROM learnings
		WHERE confidence > 0.3
		ORDER BY confidence DESC, learned_at DESC
	`)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to load learnings for %s: %v", shardType, err)
		return nil, err
	}
	defer rows.Close()

	var learnings []Learning
	for rows.Next() {
		var l Learning
		var argsJSON string
		if err := rows.Scan(&l.ID, &l.ShardType, &l.FactPredicate, &argsJSON, &l.LearnedAt, &l.SourceCampaign, &l.Confidence); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(argsJSON), &l.FactArgs); err != nil {
			continue
		}
		learnings = append(learnings, l)
	}

	logging.StoreDebug("Loaded %d learnings for shard=%s", len(learnings), shardType)
	return learnings, nil
}

// LoadByPredicate retrieves learnings filtered by predicate.
func (ls *LearningStore) LoadByPredicate(shardType, predicate string) ([]Learning, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LearningStore.LoadByPredicate")
	defer timer.Stop()

	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}

	logging.StoreDebug("Loading learnings by predicate: shard=%s predicate=%s", shardType, predicate)

	rows, err := db.Query(`
		SELECT id, shard_type, fact_predicate, fact_args, learned_at, source_campaign, confidence
		FROM learnings
		WHERE fact_predicate = ? AND confidence > 0.3
		ORDER BY confidence DESC
	`, predicate)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to load learnings by predicate: %v", err)
		return nil, err
	}
	defer rows.Close()

	var learnings []Learning
	for rows.Next() {
		var l Learning
		var argsJSON string
		if err := rows.Scan(&l.ID, &l.ShardType, &l.FactPredicate, &argsJSON, &l.LearnedAt, &l.SourceCampaign, &l.Confidence); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(argsJSON), &l.FactArgs); err != nil {
			continue
		}
		learnings = append(learnings, l)
	}

	logging.StoreDebug("Loaded %d learnings for predicate=%s", len(learnings), predicate)
	return learnings, nil
}

// DecayConfidence reduces confidence of old learnings over time.
// This implements "forgetting" - learnings not reinforced will fade.
func (ls *LearningStore) DecayConfidence(shardType string, decayFactor float64) error {
	timer := logging.StartTimer(logging.CategoryStore, "LearningStore.DecayConfidence")
	defer timer.Stop()

	db, err := ls.getDB(shardType)
	if err != nil {
		return err
	}

	logging.Store("Decaying confidence for shard=%s (factor=%.2f)", shardType, decayFactor)

	// Decay learnings older than 7 days that haven't been reinforced
	result, err := db.Exec(`
		UPDATE learnings
		SET confidence = confidence * ?
		WHERE learned_at < datetime('now', '-7 days')
	`, decayFactor)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to decay confidence: %v", err)
		return err
	}

	decayedRows, _ := result.RowsAffected()
	logging.StoreDebug("Decayed confidence on %d learnings", decayedRows)

	// Clean up very low confidence learnings
	result, err = db.Exec(`DELETE FROM learnings WHERE confidence < 0.1`)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to cleanup low-confidence learnings: %v", err)
		return err
	}

	deletedRows, _ := result.RowsAffected()
	if deletedRows > 0 {
		logging.Store("Pruned %d forgotten learnings (confidence < 0.1) for shard=%s", deletedRows, shardType)
	}

	return nil
}

// Delete removes a specific learning.
func (ls *LearningStore) Delete(shardType string, factPredicate string, factArgs []any) error {
	db, err := ls.getDB(shardType)
	if err != nil {
		return err
	}

	logging.StoreDebug("Deleting learning: shard=%s predicate=%s", shardType, factPredicate)

	argsJSON, _ := json.Marshal(factArgs)
	_, err = db.Exec(`DELETE FROM learnings WHERE fact_predicate = ? AND fact_args = ?`,
		factPredicate, string(argsJSON))
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to delete learning %s: %v", factPredicate, err)
		return err
	}

	logging.StoreDebug("Learning deleted: %s for shard=%s", factPredicate, shardType)
	return nil
}

// GetStats returns statistics about stored learnings.
func (ls *LearningStore) GetStats(shardType string) (map[string]interface{}, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LearningStore.GetStats")
	defer timer.Stop()

	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}

	logging.StoreDebug("Computing learning statistics for shard=%s", shardType)

	stats := make(map[string]interface{})

	var total int64
	db.QueryRow(`SELECT COUNT(*) FROM learnings`).Scan(&total)
	stats["total_learnings"] = total

	var avgConfidence float64
	db.QueryRow(`SELECT AVG(confidence) FROM learnings`).Scan(&avgConfidence)
	stats["avg_confidence"] = avgConfidence

	// Count by predicate
	predicateCounts := make(map[string]int64)
	rows, _ := db.Query(`SELECT fact_predicate, COUNT(*) FROM learnings GROUP BY fact_predicate`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var pred string
			var count int64
			rows.Scan(&pred, &count)
			predicateCounts[pred] = count
		}
	}
	stats["by_predicate"] = predicateCounts

	logging.StoreDebug("Learning stats for shard=%s: total=%d, avg_confidence=%.2f", shardType, total, avgConfidence)
	return stats, nil
}

// Close closes all database connections.
func (ls *LearningStore) Close() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	logging.Store("Closing LearningStore (closing %d database connections)", len(ls.dbs))

	for shardType, db := range ls.dbs {
		logging.StoreDebug("Closing learning database for shard=%s", shardType)
		db.Close()
	}
	ls.dbs = make(map[string]*sql.DB)
	return nil
}
