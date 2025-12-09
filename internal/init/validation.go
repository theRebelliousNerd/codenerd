// Package init provides validation for codeNERD knowledge bases after initialization.
// This ensures that /init --force completed successfully and all databases are properly configured.
package init

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/store"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// ValidationResult holds the result of validating a single agent database.
type ValidationResult struct {
	AgentName       string   `json:"agent_name"`
	DBPath          string   `json:"db_path"`
	SchemaVersion   int      `json:"schema_version"`
	TotalAtoms      int      `json:"total_atoms"`
	HashesPopulated int      `json:"hashes_populated"`
	MissingHashes   int      `json:"missing_hashes"`
	TablesOK        bool     `json:"tables_ok"`
	MissingTables   []string `json:"missing_tables,omitempty"`
	Errors          []string `json:"errors,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Valid           bool     `json:"valid"`
}

// ValidationSummary holds the overall validation results.
type ValidationSummary struct {
	TotalDBs      int                          `json:"total_dbs"`
	ValidDBs      int                          `json:"valid_dbs"`
	InvalidDBs    int                          `json:"invalid_dbs"`
	Results       map[string]*ValidationResult `json:"results"`
	BackupFiles   []string                     `json:"backup_files,omitempty"`
	OverallValid  bool                         `json:"overall_valid"`
	Errors        []string                     `json:"errors,omitempty"`
}

// RequiredTables lists the tables that must exist in a valid agent KB.
var RequiredTables = []string{
	"knowledge_atoms",
	"cold_storage",
	"vectors",
	"knowledge_graph",
}

// MinAtomCount is the minimum number of atoms expected in a valid agent KB.
const MinAtomCount = 5

// CurrentSchemaVersion is the expected schema version.
const CurrentSchemaVersion = 4

// ValidateAgentDB validates a single agent knowledge database.
func ValidateAgentDB(dbPath string) (*ValidationResult, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ValidateAgentDB")
	defer timer.Stop()

	agentName := extractAgentName(dbPath)
	result := &ValidationResult{
		AgentName: agentName,
		DBPath:    dbPath,
		Errors:    make([]string, 0),
		Warnings:  make([]string, 0),
		Valid:     true,
	}

	// Check file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		result.Errors = append(result.Errors, "Database file does not exist")
		result.Valid = false
		return result, nil
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to open database: %v", err))
		result.Valid = false
		return result, nil
	}
	defer db.Close()

	// Check schema version
	result.SchemaVersion = getSchemaVersion(db)
	if result.SchemaVersion < CurrentSchemaVersion {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Schema version %d is behind current version %d", result.SchemaVersion, CurrentSchemaVersion))
	}

	// Check required tables
	result.MissingTables = make([]string, 0)
	for _, table := range RequiredTables {
		if !tableExists(db, table) {
			result.MissingTables = append(result.MissingTables, table)
		}
	}
	result.TablesOK = len(result.MissingTables) == 0
	if !result.TablesOK {
		result.Errors = append(result.Errors, fmt.Sprintf("Missing tables: %v", result.MissingTables))
		result.Valid = false
	}

	// Check atom counts and content_hash status
	if tableExists(db, "knowledge_atoms") {
		// Total atoms
		var totalAtoms int
		if err := db.QueryRow("SELECT COUNT(*) FROM knowledge_atoms").Scan(&totalAtoms); err == nil {
			result.TotalAtoms = totalAtoms
		}

		// Check if content_hash column exists
		if columnExists(db, "knowledge_atoms", "content_hash") {
			// Populated hashes
			var populated int
			if err := db.QueryRow("SELECT COUNT(*) FROM knowledge_atoms WHERE content_hash IS NOT NULL AND content_hash != ''").Scan(&populated); err == nil {
				result.HashesPopulated = populated
			}
			result.MissingHashes = result.TotalAtoms - result.HashesPopulated

			if result.MissingHashes > 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%d atoms missing content_hash", result.MissingHashes))
			}
		} else {
			result.Warnings = append(result.Warnings, "content_hash column does not exist (needs migration)")
			result.MissingHashes = result.TotalAtoms
		}

		// Check minimum atom count
		if result.TotalAtoms < MinAtomCount {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Low atom count: %d (minimum recommended: %d)", result.TotalAtoms, MinAtomCount))
		}
	}

	logging.Store("Validated %s: valid=%v atoms=%d hashes=%d/%d",
		agentName, result.Valid, result.TotalAtoms, result.HashesPopulated, result.TotalAtoms)

	return result, nil
}

// ValidateAllAgentDBs validates all agent databases in the .nerd/shards directory.
func ValidateAllAgentDBs(nerdDir string) (*ValidationSummary, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ValidateAllAgentDBs")
	defer timer.Stop()

	shardsDir := filepath.Join(nerdDir, "shards")
	summary := &ValidationSummary{
		Results:      make(map[string]*ValidationResult),
		BackupFiles:  make([]string, 0),
		Errors:       make([]string, 0),
		OverallValid: true,
	}

	// Check if shards directory exists
	if _, err := os.Stat(shardsDir); os.IsNotExist(err) {
		summary.Errors = append(summary.Errors, "Shards directory does not exist")
		summary.OverallValid = false
		return summary, nil
	}

	// Find all database files (excluding backups)
	entries, err := os.ReadDir(shardsDir)
	if err != nil {
		summary.Errors = append(summary.Errors, fmt.Sprintf("Failed to read shards directory: %v", err))
		summary.OverallValid = false
		return summary, err
	}

	for _, entry := range entries {
		name := entry.Name()

		// Track backup files
		if strings.Contains(name, ".backup_") {
			summary.BackupFiles = append(summary.BackupFiles, filepath.Join(shardsDir, name))
			continue
		}

		// Skip non-database files
		if !strings.HasSuffix(name, "_knowledge.db") {
			continue
		}

		// Skip journal files
		if strings.HasSuffix(name, "-journal") || strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") {
			continue
		}

		dbPath := filepath.Join(shardsDir, name)
		result, err := ValidateAgentDB(dbPath)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("Failed to validate %s: %v", name, err))
			continue
		}

		agentName := result.AgentName
		summary.Results[agentName] = result
		summary.TotalDBs++

		if result.Valid {
			summary.ValidDBs++
		} else {
			summary.InvalidDBs++
			summary.OverallValid = false
		}
	}

	logging.Store("Validation complete: %d/%d DBs valid, %d backups found",
		summary.ValidDBs, summary.TotalDBs, len(summary.BackupFiles))

	return summary, nil
}

// PrintValidationSummary prints a human-readable validation summary.
func PrintValidationSummary(summary *ValidationSummary) {
	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("📋 KNOWLEDGE BASE VALIDATION")
	fmt.Println(strings.Repeat("─", 60))

	if summary.TotalDBs == 0 {
		fmt.Println("No agent databases found.")
		return
	}

	// Print individual results
	for name, result := range summary.Results {
		status := "✓"
		if !result.Valid {
			status = "✗"
		}

		hashStatus := ""
		if result.TotalAtoms > 0 {
			if result.MissingHashes == 0 {
				hashStatus = fmt.Sprintf(" (hashes: %d/%d)", result.HashesPopulated, result.TotalAtoms)
			} else {
				hashStatus = fmt.Sprintf(" (hashes: %d/%d, %d missing)", result.HashesPopulated, result.TotalAtoms, result.MissingHashes)
			}
		}

		fmt.Printf("%s %s: %d atoms, schema v%d%s\n",
			status, name, result.TotalAtoms, result.SchemaVersion, hashStatus)

		// Print warnings
		for _, warn := range result.Warnings {
			fmt.Printf("  ⚠ %s\n", warn)
		}

		// Print errors
		for _, err := range result.Errors {
			fmt.Printf("  ✗ %s\n", err)
		}
	}

	// Print summary
	fmt.Println(strings.Repeat("─", 60))
	if summary.OverallValid {
		fmt.Printf("✓ All %d databases validated successfully\n", summary.TotalDBs)
	} else {
		fmt.Printf("✗ %d/%d databases have issues\n", summary.InvalidDBs, summary.TotalDBs)
	}

	// Print backup notification
	if len(summary.BackupFiles) > 0 {
		fmt.Printf("\n📦 Found %d backup files from migration\n", len(summary.BackupFiles))
		fmt.Println("   After verifying your data, you can clean them up with:")
		fmt.Println("   nerd init --cleanup-backups")
	}
}

// Helper functions

func extractAgentName(dbPath string) string {
	base := filepath.Base(dbPath)
	name := strings.TrimSuffix(base, "_knowledge.db")
	return name
}

func getSchemaVersion(db *sql.DB) int {
	// Try schema_versions table first
	if tableExists(db, "schema_versions") {
		var version int
		if err := db.QueryRow("SELECT version FROM schema_versions ORDER BY applied_at DESC LIMIT 1").Scan(&version); err == nil {
			return version
		}
	}

	// Infer from table structure
	return store.GetSchemaVersion(db)
}

func tableExists(db *sql.DB, table string) bool {
	var count int
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
	if err := db.QueryRow(query, table).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

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

// FindBackupFiles returns all backup files in the shards directory.
func FindBackupFiles(nerdDir string) []string {
	shardsDir := filepath.Join(nerdDir, "shards")
	backups := make([]string, 0)

	entries, err := os.ReadDir(shardsDir)
	if err != nil {
		return backups
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".backup_") {
			backups = append(backups, filepath.Join(shardsDir, entry.Name()))
		}
	}

	return backups
}

// CleanupBackups removes backup files from the shards directory.
// If dryRun is true, it only reports what would be deleted.
func CleanupBackups(nerdDir string, dryRun bool) (int, error) {
	backups := FindBackupFiles(nerdDir)

	if len(backups) == 0 {
		return 0, nil
	}

	if dryRun {
		fmt.Printf("Would delete %d backup files:\n", len(backups))
		for _, f := range backups {
			fmt.Printf("  - %s\n", filepath.Base(f))
		}
		return len(backups), nil
	}

	deleted := 0
	for _, f := range backups {
		if err := os.Remove(f); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to delete backup %s: %v", f, err)
			continue
		}
		deleted++
		logging.Store("Deleted backup: %s", filepath.Base(f))
	}

	fmt.Printf("Deleted %d backup files\n", deleted)
	return deleted, nil
}
