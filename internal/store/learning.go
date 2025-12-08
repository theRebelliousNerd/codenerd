// Package store implements persistence for the Cortex framework.
// This file implements the LearningStore for Autopoiesis (§8.3).
// Each shard can learn from patterns and persist them for future sessions.
package store

import (
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
		basePath = ".nerd/shards"
	}

	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create learnings directory: %w", err)
	}

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
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open learnings db: %w", err)
	}

	// Initialize schema
	if err := ls.initializeSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	ls.dbs[shardType] = db
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
	db, err := ls.getDB(shardType)
	if err != nil {
		return err
	}

	argsJSON, err := json.Marshal(factArgs)
	if err != nil {
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

	return err
}

// Load retrieves all learnings for a shard type.
func (ls *LearningStore) Load(shardType string) ([]Learning, error) {
	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT id, shard_type, fact_predicate, fact_args, learned_at, source_campaign, confidence
		FROM learnings
		WHERE confidence > 0.3
		ORDER BY confidence DESC, learned_at DESC
	`)
	if err != nil {
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

	return learnings, nil
}

// LoadByPredicate retrieves learnings filtered by predicate.
func (ls *LearningStore) LoadByPredicate(shardType, predicate string) ([]Learning, error) {
	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT id, shard_type, fact_predicate, fact_args, learned_at, source_campaign, confidence
		FROM learnings
		WHERE fact_predicate = ? AND confidence > 0.3
		ORDER BY confidence DESC
	`, predicate)
	if err != nil {
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

	return learnings, nil
}

// DecayConfidence reduces confidence of old learnings over time.
// This implements "forgetting" - learnings not reinforced will fade.
func (ls *LearningStore) DecayConfidence(shardType string, decayFactor float64) error {
	db, err := ls.getDB(shardType)
	if err != nil {
		return err
	}

	// Decay learnings older than 7 days that haven't been reinforced
	_, err = db.Exec(`
		UPDATE learnings
		SET confidence = confidence * ?
		WHERE learned_at < datetime('now', '-7 days')
	`, decayFactor)

	// Clean up very low confidence learnings
	_, err = db.Exec(`DELETE FROM learnings WHERE confidence < 0.1`)

	return err
}

// Delete removes a specific learning.
func (ls *LearningStore) Delete(shardType string, factPredicate string, factArgs []any) error {
	db, err := ls.getDB(shardType)
	if err != nil {
		return err
	}

	argsJSON, _ := json.Marshal(factArgs)
	_, err = db.Exec(`DELETE FROM learnings WHERE fact_predicate = ? AND fact_args = ?`,
		factPredicate, string(argsJSON))
	return err
}

// GetStats returns statistics about stored learnings.
func (ls *LearningStore) GetStats(shardType string) (map[string]interface{}, error) {
	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}

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

	return stats, nil
}

// Close closes all database connections.
func (ls *LearningStore) Close() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	for _, db := range ls.dbs {
		db.Close()
	}
	ls.dbs = make(map[string]*sql.DB)
	return nil
}
