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
	// Archived facts access tracking columns (mirrored from cold_storage)
	{"archived_facts", "last_accessed", "DATETIME"},
	{"archived_facts", "access_count", "INTEGER DEFAULT 0"},
	// Knowledge atoms extended fields (added for shared knowledge pool)
	{"knowledge_atoms", "source", "TEXT DEFAULT ''"},
	{"knowledge_atoms", "tags", "TEXT DEFAULT '[]'"},
	// Prompt atoms polymorphism columns (for different verbosity levels)
	{"prompt_atoms", "description", "TEXT"},
	{"prompt_atoms", "content_concise", "TEXT"},
	{"prompt_atoms", "content_min", "TEXT"},
	// Prompt atoms metadata column
	{"prompt_atoms", "source_file", "TEXT"},
}

// RunMigrations applies schema migrations for existing databases.
func RunMigrations(db *sql.DB) error {
	timer := logging.StartTimer(logging.CategoryStore, "RunMigrations")
	defer timer.Stop()

	logging.Store("Running schema migrations (%d pending)", len(pendingMigrations))

	appliedCount := 0
	skippedCount := 0

	for _, m := range pendingMigrations {
		logging.StoreDebug("Checking migration: %s.%s", m.Table, m.Column)

		// If the table doesn't exist in this DB, skip quietly.
		if !tableExists(db, m.Table) {
			logging.StoreDebug("Table missing, skipping migration: %s.%s", m.Table, m.Column)
			skippedCount++
			continue
		}

		if !columnExists(db, m.Table, m.Column) {
			query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", m.Table, m.Column, m.Def)
			logging.StoreDebug("Executing migration: %s", query)

			if _, err := db.Exec(query); err != nil {
				logging.Get(logging.CategoryStore).Warn("Migration failed (may already exist): %s.%s: %v", m.Table, m.Column, err)
				// Don't fail on migration errors - column may already exist in a different form
				skippedCount++
			} else {
				logging.Store("Migration applied: added %s.%s", m.Table, m.Column)
				appliedCount++
			}
		} else {
			logging.StoreDebug("Column already exists, skipping: %s.%s", m.Table, m.Column)
			skippedCount++
		}
	}

	logging.Store("Schema migrations complete: applied=%d, skipped=%d", appliedCount, skippedCount)
	return nil
}

// columnExists checks if a column exists in a table using PRAGMA table_info.
func columnExists(db *sql.DB, table, column string) bool {
	query := fmt.Sprintf("PRAGMA table_info(%s)", table)
	rows, err := db.Query(query)
	if err != nil {
		logging.StoreDebug("PRAGMA table_info(%s) failed: %v", table, err)
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
			logging.StoreDebug("Column exists: %s.%s (type=%s)", table, column, ctype)
			return true
		}
	}
	logging.StoreDebug("Column does not exist: %s.%s", table, column)
	return false
}

// tableExists checks if a table exists in the database.
func tableExists(db *sql.DB, table string) bool {
	var count int
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
	if err := db.QueryRow(query, table).Scan(&count); err != nil {
		logging.StoreDebug("Table existence check failed for %s: %v", table, err)
		return false
	}
	exists := count > 0
	logging.StoreDebug("Table %s exists: %v", table, exists)
	return exists
}

// GetSchemaVersion returns the current schema version of a database.
// If no version table exists, it infers the version from table structure.
func GetSchemaVersion(db *sql.DB) int {
	logging.StoreDebug("Detecting schema version")

	// First, check if schema_versions table exists
	if tableExists(db, "schema_versions") {
		var version int
		query := "SELECT version FROM schema_versions ORDER BY applied_at DESC LIMIT 1"
		if err := db.QueryRow(query).Scan(&version); err == nil {
			logging.StoreDebug("Schema version from schema_versions table: %d", version)
			return version
		}
		logging.StoreDebug("schema_versions table exists but no version record found")
	}

	// Infer version from table structure
	version := inferSchemaVersion(db)
	logging.StoreDebug("Inferred schema version: %d", version)
	return version
}

// inferSchemaVersion determines schema version by examining table structure.
func inferSchemaVersion(db *sql.DB) int {
	logging.StoreDebug("Inferring schema version from table structure")

	// Check if knowledge_atoms table exists
	if !tableExists(db, "knowledge_atoms") {
		logging.StoreDebug("No knowledge_atoms table - version 0")
		return 0
	}

	// Check for v4: content_hash column
	if columnExists(db, "knowledge_atoms", "content_hash") {
		logging.StoreDebug("Found content_hash column - version 4")
		return 4
	}

	// Check for v3: vec_index virtual table
	if tableExists(db, "vec_index") {
		logging.StoreDebug("Found vec_index table - version 3")
		return 3
	}

	// Check for v2: embedding column
	if columnExists(db, "knowledge_atoms", "embedding") {
		logging.StoreDebug("Found embedding column - version 2")
		return 2
	}

	// v1: Basic table
	logging.StoreDebug("Basic table structure - version 1")
	return 1
}

// SetSchemaVersion records a new schema version in the database.
func SetSchemaVersion(db *sql.DB, version int) error {
	logging.StoreDebug("Setting schema version to %d", version)

	createTable := `
		CREATE TABLE IF NOT EXISTS schema_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version INTEGER NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			description TEXT
		)
	`
	if _, err := db.Exec(createTable); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create schema_versions table: %v", err)
		return fmt.Errorf("failed to create schema_versions table: %w", err)
	}
	logging.StoreDebug("schema_versions table ensured")

	desc := fmt.Sprintf("Migrated to schema version %d", version)
	_, err := db.Exec(
		"INSERT INTO schema_versions (version, description) VALUES (?, ?)",
		version, desc,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to record schema version %d: %v", version, err)
		return fmt.Errorf("failed to record schema version: %w", err)
	}

	logging.Store("Schema version set to %d", version)
	return nil
}

// MigrateV1ToV2 adds the embedding column for vector search.
func MigrateV1ToV2(db *sql.DB) error {
	timer := logging.StartTimer(logging.CategoryStore, "MigrateV1ToV2")
	defer timer.Stop()

	logging.Store("Migrating v1 -> v2: Adding embedding column")

	if columnExists(db, "knowledge_atoms", "embedding") {
		logging.Store("Embedding column already exists, skipping")
		return nil
	}

	query := "ALTER TABLE knowledge_atoms ADD COLUMN embedding BLOB"
	logging.StoreDebug("Executing: %s", query)
	if _, err := db.Exec(query); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to add embedding column: %v", err)
		return fmt.Errorf("failed to add embedding column: %w", err)
	}

	logging.Store("Added embedding column to knowledge_atoms")
	return nil
}

// MigrateV2ToV3 creates the vec_index virtual table for sqlite-vec ANN search.
func MigrateV2ToV3(db *sql.DB) error {
	timer := logging.StartTimer(logging.CategoryStore, "MigrateV2ToV3")
	defer timer.Stop()

	logging.Store("Migrating v2 -> v3: Creating vec_index table")

	if tableExists(db, "vec_index") {
		logging.Store("vec_index table already exists, skipping")
		return nil
	}

	// Try to create vec_index - may fail if sqlite-vec is not available
	query := `CREATE VIRTUAL TABLE IF NOT EXISTS vec_index USING vec0(
		embedding float[3072],
		content TEXT,
		metadata TEXT
	)`
	logging.StoreDebug("Executing vec_index creation")
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
	timer := logging.StartTimer(logging.CategoryStore, "MigrateV3ToV4")
	defer timer.Stop()

	logging.Store("Migrating v3 -> v4: Adding content_hash column")

	if columnExists(db, "knowledge_atoms", "content_hash") {
		logging.Store("content_hash column already exists, skipping")
		return nil
	}

	query := "ALTER TABLE knowledge_atoms ADD COLUMN content_hash TEXT"
	logging.StoreDebug("Executing: %s", query)
	if _, err := db.Exec(query); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to add content_hash column: %v", err)
		return fmt.Errorf("failed to add content_hash column: %w", err)
	}

	indexQuery := "CREATE INDEX IF NOT EXISTS idx_atoms_content_hash ON knowledge_atoms(content_hash)"
	logging.StoreDebug("Creating index: %s", indexQuery)
	if _, err := db.Exec(indexQuery); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to create content_hash index: %v", err)
	}

	logging.Store("Added content_hash column to knowledge_atoms")
	return nil
}

// BackfillContentHashes computes SHA256 hashes for all atoms missing content_hash.
func BackfillContentHashes(db *sql.DB) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "BackfillContentHashes")
	defer timer.Stop()

	logging.Store("Backfilling content hashes for existing atoms")

	query := "SELECT id, concept, content FROM knowledge_atoms WHERE content_hash IS NULL OR content_hash = ''"
	logging.StoreDebug("Querying atoms without content_hash")

	rows, err := db.Query(query)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query atoms for hash backfill: %v", err)
		return 0, fmt.Errorf("failed to query atoms for hash backfill: %w", err)
	}
	defer rows.Close()

	updated := 0
	skipped := 0
	for rows.Next() {
		var id int64
		var concept, content string
		if err := rows.Scan(&id, &concept, &content); err != nil {
			skipped++
			continue
		}

		hash := ComputeContentHash(concept, content)

		updateQuery := "UPDATE knowledge_atoms SET content_hash = ? WHERE id = ?"
		if _, err := db.Exec(updateQuery, hash, id); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to update hash for atom %d: %v", id, err)
			skipped++
			continue
		}
		updated++
	}

	if err := rows.Err(); err != nil {
		logging.Get(logging.CategoryStore).Error("Error iterating atoms during backfill: %v", err)
		return updated, fmt.Errorf("error iterating atoms: %w", err)
	}

	logging.Store("Backfilled content hashes: updated=%d, skipped=%d", updated, skipped)
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
	timer := logging.StartTimer(logging.CategoryStore, "CreateBackup")
	defer timer.Stop()

	timestamp := time.Now().Format("20060102_150405")
	backupPath := dbPath + fmt.Sprintf(".backup_%s", timestamp)

	logging.Store("Creating database backup: %s -> %s", dbPath, backupPath)

	src, err := os.Open(dbPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to open source database for backup: %v", err)
		return "", fmt.Errorf("failed to open source database: %w", err)
	}
	defer src.Close()

	// Get source file size for logging
	srcInfo, _ := src.Stat()
	if srcInfo != nil {
		logging.StoreDebug("Source database size: %d bytes", srcInfo.Size())
	}

	dst, err := os.Create(backupPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create backup file: %v", err)
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	bytesCopied, err := io.Copy(dst, src)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to copy database to backup: %v", err)
		return "", fmt.Errorf("failed to copy database to backup: %w", err)
	}

	if err := dst.Sync(); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to sync backup to disk: %v", err)
		return "", fmt.Errorf("failed to sync backup to disk: %w", err)
	}

	logging.Store("Database backup created: %s (%d bytes)", backupPath, bytesCopied)
	return backupPath, nil
}

// RestoreBackup restores a database from a backup file.
func RestoreBackup(dbPath, backupPath string) error {
	timer := logging.StartTimer(logging.CategoryStore, "RestoreBackup")
	defer timer.Stop()

	logging.Store("Restoring database from backup: %s -> %s", backupPath, dbPath)

	src, err := os.Open(backupPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to open backup file for restore: %v", err)
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dbPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create database file during restore: %v", err)
		return fmt.Errorf("failed to create database file: %w", err)
	}
	defer dst.Close()

	bytesCopied, err := io.Copy(dst, src)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to restore from backup: %v", err)
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	if err := dst.Sync(); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to sync restored database: %v", err)
		return fmt.Errorf("failed to sync restored database: %w", err)
	}

	logging.Store("Database restored from backup (%d bytes)", bytesCopied)
	return nil
}

// RunAllMigrations runs all necessary migrations to bring the database to the target version.
func RunAllMigrations(dbPath string, targetVersion int) (*MigrationResult, error) {
	timer := logging.StartTimer(logging.CategoryStore, "RunAllMigrations")
	defer timer.Stop()

	startTime := time.Now()
	result := &MigrationResult{
		Warnings: make([]string, 0),
	}

	logging.Store("Starting migration process for database: %s", dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to open database for migration: %v", err)
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
	logging.StoreDebug("Creating pre-migration backup")
	backupPath, err := CreateBackup(dbPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create backup before migration: %v", err)
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}
	result.BackupPath = backupPath

	migrationSuccess := false
	defer func() {
		if !migrationSuccess {
			logging.Get(logging.CategoryStore).Warn("Migration failed, restoring from backup")
			if restoreErr := RestoreBackup(dbPath, backupPath); restoreErr != nil {
				logging.Get(logging.CategoryStore).Error("Failed to restore backup after migration failure: %v", restoreErr)
			} else {
				logging.Store("Database restored from backup after migration failure")
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
			logging.Get(logging.CategoryStore).Error("Migration v%d -> v%d failed: %v", v, nextVersion, migrationErr)
			return nil, fmt.Errorf("migration v%d -> v%d failed: %w", v, nextVersion, migrationErr)
		}

		logging.StoreDebug("Migration v%d -> v%d completed successfully", v, nextVersion)
		result.MigrationsRun++
	}

	migrationSuccess = true

	// Record schema version
	if err := SetSchemaVersion(db, targetVersion); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to record schema version: %v", err)
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to record schema version: %v", err))
	}

	// Backfill content hashes if we migrated to v4
	if targetVersion >= 4 && currentVersion < 4 {
		logging.StoreDebug("Backfilling content hashes for v4 migration")
		hashCount, err := BackfillContentHashes(db)
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Hash backfill had issues: %v", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Hash backfill had issues: %v", err))
		}
		result.HashesComputed = hashCount
	}

	result.Duration = time.Since(startTime)
	logging.Store("Migration complete: v%d -> v%d in %v (migrations=%d, hashes=%d)",
		currentVersion, targetVersion, result.Duration, result.MigrationsRun, result.HashesComputed)

	return result, nil
}

// MigrateKnowledgeDB is the main entry point for migrating a knowledge database.
func MigrateKnowledgeDB(dbPath string) (*MigrationResult, error) {
	logging.Store("MigrateKnowledgeDB called for: %s (target=v%d)", dbPath, CurrentSchemaVersion)
	return RunAllMigrations(dbPath, CurrentSchemaVersion)
}

// CheckMigrationNeeded returns true if the database needs migration.
func CheckMigrationNeeded(dbPath string) (bool, int, error) {
	logging.StoreDebug("Checking if migration needed for: %s", dbPath)

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		logging.StoreDebug("Database does not exist: %s", dbPath)
		return false, 0, nil
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to open database for migration check: %v", err)
		return false, 0, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	currentVersion := GetSchemaVersion(db)
	needed := currentVersion < CurrentSchemaVersion
	logging.StoreDebug("Migration check: current=v%d, target=v%d, needed=%v", currentVersion, CurrentSchemaVersion, needed)
	return needed, currentVersion, nil
}

// MigrateAllAgentDBs migrates all agent knowledge databases in the shards directory.
func MigrateAllAgentDBs(nerdDir string) (map[string]*MigrationResult, error) {
	timer := logging.StartTimer(logging.CategoryStore, "MigrateAllAgentDBs")
	defer timer.Stop()

	shardsDir := filepath.Join(nerdDir, "shards")
	results := make(map[string]*MigrationResult)

	logging.Store("Scanning for agent databases in: %s", shardsDir)

	if _, err := os.Stat(shardsDir); os.IsNotExist(err) {
		logging.StoreDebug("Shards directory does not exist: %s", shardsDir)
		return results, nil
	}

	entries, err := os.ReadDir(shardsDir)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to read shards directory: %v", err)
		return nil, fmt.Errorf("failed to read shards directory: %w", err)
	}

	logging.StoreDebug("Found %d entries in shards directory", len(entries))

	migratedCount := 0
	skippedCount := 0
	failedCount := 0

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

		logging.StoreDebug("Checking agent database: %s", agentName)

		needed, currentVersion, err := CheckMigrationNeeded(dbPath)
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to check migration for %s: %v", name, err)
			failedCount++
			continue
		}

		if !needed {
			logging.Store("Agent DB %s already at v%d, no migration needed", agentName, currentVersion)
			skippedCount++
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
			failedCount++
			continue
		}

		results[agentName] = result
		migratedCount++
	}

	logging.Store("Agent DB migration scan complete: migrated=%d, skipped=%d, failed=%d",
		migratedCount, skippedCount, failedCount)

	return results, nil
}
