// Package store provides database migrations for codeNERD knowledge bases.
// This file implements a versioned schema migration system that safely upgrades
// old databases to the new vectorstore format with sqlite-vec support.
package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"codenerd/internal/logging"
)

// Schema versions:
// v1: Basic knowledge_atoms table (concept, content, confidence, created_at)
// v2: Added embedding column for vector search
// v3: Added vec_index virtual table for sqlite-vec ANN
// v4: Added content_hash column for deduplication
const CurrentSchemaVersion = 4

// MigrationResult holds the result of a migration operation.
type MigrationResult struct {
	FromVersion    int
	ToVersion      int
	MigrationsRun  int
	BackupPath     string
	HashesComputed int
	Duration       time.Duration
	Warnings       []string
}

// Migration defines a database schema migration.
type Migration struct {
	Table  string
	Column string
	Def    string
}

// pendingMigrations lists all schema migrations to apply.
// These handle cases where tables exist but are missing newer columns.
var pendingMigrations = []Migration{
	// Cold storage access tracking columns (added for archival tier)
	{"cold_storage", "last_accessed", "DATETIME DEFAULT CURRENT_TIMESTAMP"},
	{"cold_storage", "access_count", "INTEGER DEFAULT 0"},
	// Knowledge atoms extended fields (added for shared knowledge pool)
	{"knowledge_atoms", "source", "TEXT DEFAULT ''"},
	{"knowledge_atoms", "tags", "TEXT DEFAULT '[]'"},
}

// RunMigrations applies schema migrations for existing databases.
func RunMigrations(db *sql.DB) error {
	for _, m := range pendingMigrations {
		if !columnExists(db, m.Table, m.Column) {
			query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", m.Table, m.Column, m.Def)
			if _, err := db.Exec(query); err != nil {
				logging.Get(logging.CategoryStore).Warn("Migration failed (may already exist): %s.%s: %v", m.Table, m.Column, err)
				// Don't fail on migration errors - column may already exist in a different form
			} else {
				logging.Store("Migration applied: added %s.%s", m.Table, m.Column)
			}
		}
	}
	return nil
}

// columnExists checks if a column exists in a table using PRAGMA table_info.
func columnExists(db *sql.DB, table, column string) bool {
	query := fmt.Sprintf("PRAGMA table_info(%s)", table)
	rows, err := db.Query(query)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == column {
			return true
		}
	}
	return false
}

// tableExists checks if a table exists in the database.
func tableExists(db *sql.DB, table string) bool {
	var count int
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
	if err := db.QueryRow(query, table).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// GetSchemaVersion returns the current schema version of a database.
// If no version table exists, it infers the version from table structure.
func GetSchemaVersion(db *sql.DB) int {
	// First, check if schema_versions table exists
	if tableExists(db, "schema_versions") {
		var version int
		query := "SELECT version FROM schema_versions ORDER BY applied_at DESC LIMIT 1"
		if err := db.QueryRow(query).Scan(&version); err == nil {
			return version
		}
	}

	// Infer version from table structure
	return inferSchemaVersion(db)
}

// inferSchemaVersion determines schema version by examining table structure.
func inferSchemaVersion(db *sql.DB) int {
	// Check if knowledge_atoms table exists
	if !tableExists(db, "knowledge_atoms") {
		return 0
	}

	// Check for v4: content_hash column
	if columnExists(db, "knowledge_atoms", "content_hash") {
		return 4
	}

	// Check for v3: vec_index virtual table
	if tableExists(db, "vec_index") {
		return 3
	}

	// Check for v2: embedding column
	if columnExists(db, "knowledge_atoms", "embedding") {
		return 2
	}

	// v1: Basic table
	return 1
}

// SetSchemaVersion records a new schema version in the database.
func SetSchemaVersion(db *sql.DB, version int) error {
	createTable := `
		CREATE TABLE IF NOT EXISTS schema_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version INTEGER NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			description TEXT
		)
	`
	if _, err := db.Exec(createTable); err != nil {
		return fmt.Errorf("failed to create schema_versions table: %w", err)
	}

	desc := fmt.Sprintf("Migrated to schema version %d", version)
	_, err := db.Exec(
		"INSERT INTO schema_versions (version, description) VALUES (?, ?)",
		version, desc,
	)
	if err != nil {
		return fmt.Errorf("failed to record schema version: %w", err)
	}

	logging.Store("Schema version set to %d", version)
	return nil
}

// MigrateV1ToV2 adds the embedding column for vector search.
func MigrateV1ToV2(db *sql.DB) error {
	logging.Store("Migrating v1 -> v2: Adding embedding column")

	if columnExists(db, "knowledge_atoms", "embedding") {
		logging.Store("Embedding column already exists, skipping")
		return nil
	}

	query := "ALTER TABLE knowledge_atoms ADD COLUMN embedding BLOB"
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to add embedding column: %w", err)
	}

	logging.Store("Added embedding column to knowledge_atoms")
	return nil
}

// MigrateV2ToV3 creates the vec_index virtual table for sqlite-vec ANN search.
func MigrateV2ToV3(db *sql.DB) error {
	logging.Store("Migrating v2 -> v3: Creating vec_index table")

	if tableExists(db, "vec_index") {
		logging.Store("vec_index table already exists, skipping")
		return nil
	}

	// Try to create vec_index - may fail if sqlite-vec is not available
	query := `CREATE VIRTUAL TABLE IF NOT EXISTS vec_index USING vec0(
		embedding float[768],
		content TEXT,
		metadata TEXT
	)`
	if _, err := db.Exec(query); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to create vec_index (sqlite-vec may not be available): %v", err)
		// Don't fail - sqlite-vec is optional
		return nil
	}

	logging.Store("Created vec_index virtual table for ANN search")
	return nil
}

// MigrateV3ToV4 adds the content_hash column for deduplication.
func MigrateV3ToV4(db *sql.DB) error {
	logging.Store("Migrating v3 -> v4: Adding content_hash column")

	if columnExists(db, "knowledge_atoms", "content_hash") {
		logging.Store("content_hash column already exists, skipping")
		return nil
	}

	query := "ALTER TABLE knowledge_atoms ADD COLUMN content_hash TEXT"
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to add content_hash column: %w", err)
	}

	indexQuery := "CREATE INDEX IF NOT EXISTS idx_atoms_content_hash ON knowledge_atoms(content_hash)"
	if _, err := db.Exec(indexQuery); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to create content_hash index: %v", err)
	}

	logging.Store("Added content_hash column to knowledge_atoms")
	return nil
}

// BackfillContentHashes computes SHA256 hashes for all atoms missing content_hash.
func BackfillContentHashes(db *sql.DB) (int, error) {
	logging.Store("Backfilling content hashes for existing atoms")

	query := "SELECT id, concept, content FROM knowledge_atoms WHERE content_hash IS NULL OR content_hash = ''"
	rows, err := db.Query(query)
	if err != nil {
		return 0, fmt.Errorf("failed to query atoms for hash backfill: %w", err)
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var id int64
		var concept, content string
		if err := rows.Scan(&id, &concept, &content); err != nil {
			continue
		}

		hash := ComputeContentHash(concept, content)

		updateQuery := "UPDATE knowledge_atoms SET content_hash = ? WHERE id = ?"
		if _, err := db.Exec(updateQuery, hash, id); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to update hash for atom %d: %v", id, err)
			continue
		}
		updated++
	}

	if err := rows.Err(); err != nil {
		return updated, fmt.Errorf("error iterating atoms: %w", err)
	}

	logging.Store("Backfilled content hashes for %d atoms", updated)
	return updated, nil
}

// ComputeContentHash generates a SHA256 hash for a knowledge atom.
func ComputeContentHash(concept, content string) string {
	combined := concept + "::" + content
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// CreateBackup creates a backup copy of the database file.
func CreateBackup(dbPath string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	backupPath := dbPath + fmt.Sprintf(".backup_%s", timestamp)

	logging.Store("Creating database backup: %s", backupPath)

	src, err := os.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source database: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy database to backup: %w", err)
	}

	if err := dst.Sync(); err != nil {
		return "", fmt.Errorf("failed to sync backup to disk: %w", err)
	}

	logging.Store("Database backup created: %s", backupPath)
	return backupPath, nil
}

// RestoreBackup restores a database from a backup file.
func RestoreBackup(dbPath, backupPath string) error {
	logging.Store("Restoring database from backup: %s", backupPath)

	src, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("failed to sync restored database: %w", err)
	}

	logging.Store("Database restored from backup")
	return nil
}

// RunAllMigrations runs all necessary migrations to bring the database to the target version.
func RunAllMigrations(dbPath string, targetVersion int) (*MigrationResult, error) {
	startTime := time.Now()
	result := &MigrationResult{
		Warnings: make([]string, 0),
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	currentVersion := GetSchemaVersion(db)
	result.FromVersion = currentVersion
	result.ToVersion = targetVersion

	logging.Store("Database at version %d, target version %d", currentVersion, targetVersion)

	if currentVersion >= targetVersion {
		logging.Store("Database already at version %d, no migration needed", currentVersion)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Create backup before migration
	backupPath, err := CreateBackup(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}
	result.BackupPath = backupPath

	migrationSuccess := false
	defer func() {
		if !migrationSuccess {
			if restoreErr := RestoreBackup(dbPath, backupPath); restoreErr != nil {
				logging.Get(logging.CategoryStore).Error("Failed to restore backup after migration failure: %v", restoreErr)
			}
		}
	}()

	// Run migrations sequentially
	for v := currentVersion; v < targetVersion; v++ {
		nextVersion := v + 1
		logging.Store("Running migration v%d -> v%d", v, nextVersion)

		var migrationErr error
		switch nextVersion {
		case 2:
			migrationErr = MigrateV1ToV2(db)
		case 3:
			migrationErr = MigrateV2ToV3(db)
		case 4:
			migrationErr = MigrateV3ToV4(db)
		default:
			migrationErr = fmt.Errorf("unknown migration: v%d -> v%d", v, nextVersion)
		}

		if migrationErr != nil {
			return nil, fmt.Errorf("migration v%d -> v%d failed: %w", v, nextVersion, migrationErr)
		}

		result.MigrationsRun++
	}

	migrationSuccess = true

	// Record schema version
	if err := SetSchemaVersion(db, targetVersion); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to record schema version: %v", err))
	}

	// Backfill content hashes if we migrated to v4
	if targetVersion >= 4 && currentVersion < 4 {
		hashCount, err := BackfillContentHashes(db)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Hash backfill had issues: %v", err))
		}
		result.HashesComputed = hashCount
	}

	result.Duration = time.Since(startTime)
	logging.Store("Migration complete: v%d -> v%d in %v", currentVersion, targetVersion, result.Duration)

	return result, nil
}

// MigrateKnowledgeDB is the main entry point for migrating a knowledge database.
func MigrateKnowledgeDB(dbPath string) (*MigrationResult, error) {
	return RunAllMigrations(dbPath, CurrentSchemaVersion)
}

// CheckMigrationNeeded returns true if the database needs migration.
func CheckMigrationNeeded(dbPath string) (bool, int, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return false, 0, nil
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return false, 0, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	currentVersion := GetSchemaVersion(db)
	return currentVersion < CurrentSchemaVersion, currentVersion, nil
}

// MigrateAllAgentDBs migrates all agent knowledge databases in the shards directory.
func MigrateAllAgentDBs(nerdDir string) (map[string]*MigrationResult, error) {
	shardsDir := filepath.Join(nerdDir, "shards")
	results := make(map[string]*MigrationResult)

	if _, err := os.Stat(shardsDir); os.IsNotExist(err) {
		return results, nil
	}

	entries, err := os.ReadDir(shardsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read shards directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".db" {
			continue
		}

		dbPath := filepath.Join(shardsDir, name)
		agentName := name[:len(name)-3]

		needed, currentVersion, err := CheckMigrationNeeded(dbPath)
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to check migration for %s: %v", name, err)
			continue
		}

		if !needed {
			logging.Store("Agent DB %s already at v%d, no migration needed", agentName, currentVersion)
			continue
		}

		logging.Store("Migrating agent DB %s from v%d to v%d", agentName, currentVersion, CurrentSchemaVersion)

		result, err := MigrateKnowledgeDB(dbPath)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Migration failed for %s: %v", agentName, err)
			results[agentName] = &MigrationResult{
				FromVersion: currentVersion,
				ToVersion:   CurrentSchemaVersion,
				Warnings:    []string{fmt.Sprintf("Migration failed: %v", err)},
			}
			continue
		}

		results[agentName] = result
	}

	return results, nil
}
