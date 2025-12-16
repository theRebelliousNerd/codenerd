package store

import (
	"codenerd/internal/logging"
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// COLD STORAGE AND ARCHIVAL (Shard D)
// =============================================================================

// StoredFact represents a persisted fact.
type StoredFact struct {
	ID           int64
	Predicate    string
	Args         []interface{}
	FactType     string
	Priority     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastAccessed time.Time
	AccessCount  int
}

// ArchivedFact represents a fact moved to archival storage.
type ArchivedFact struct {
	ID           int64
	Predicate    string
	Args         []interface{}
	FactType     string
	Priority     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastAccessed time.Time
	AccessCount  int
	ArchivedAt   time.Time
}

// MaintenanceConfig configures maintenance cleanup operations.
type MaintenanceConfig struct {
	ArchiveOlderThanDays       int  // Archive facts not accessed in N days
	MaxAccessCount             int  // Only archive if access count <= this
	PurgeArchivedOlderThanDays int  // Permanently delete archived facts older than N days
	CleanActivationLogDays     int  // Delete activation logs older than N days
	VacuumDatabase             bool // Run VACUUM to reclaim space
}

// MaintenanceStats reports results of maintenance operations.
type MaintenanceStats struct {
	FactsArchived         int
	FactsPurged           int
	ActivationLogsDeleted int
	DatabaseVacuumed      bool
}

// StoreFact persists a fact to cold storage.
func (s *LocalStore) StoreFact(predicate string, args []interface{}, factType string, priority int) error {
	timer := logging.StartTimer(logging.CategoryStore, "StoreFact")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Storing fact to cold storage: %s/%d args (type=%s, priority=%d)", predicate, len(args), factType, priority)

	argsJSON, _ := json.Marshal(args)

	_, err := s.db.Exec(
		`INSERT INTO cold_storage (predicate, args, fact_type, priority, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(predicate, args) DO UPDATE SET
		 fact_type = excluded.fact_type,
		 priority = excluded.priority,
		 updated_at = CURRENT_TIMESTAMP`,
		predicate, string(argsJSON), factType, priority,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store fact %s: %v", predicate, err)
		return err
	}

	logging.StoreDebug("Fact stored successfully in cold storage")
	return nil
}

// LoadFacts retrieves facts by predicate and updates access tracking.
func (s *LocalStore) LoadFacts(predicate string) ([]StoredFact, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadFacts")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Loading facts from cold storage: predicate=%s", predicate)

	rows, err := s.db.Query(
		"SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count FROM cold_storage WHERE predicate = ? ORDER BY priority DESC",
		predicate,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to load facts for predicate=%s: %v", predicate, err)
		return nil, err
	}
	defer rows.Close()

	var facts []StoredFact
	var factIDs []int64
	for rows.Next() {
		var fact StoredFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt, &fact.LastAccessed, &fact.AccessCount); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
		factIDs = append(factIDs, fact.ID)
	}

	// Update access tracking for retrieved facts
	for _, id := range factIDs {
		s.db.Exec(
			"UPDATE cold_storage SET last_accessed = CURRENT_TIMESTAMP, access_count = access_count + 1 WHERE id = ?",
			id,
		)
	}

	logging.StoreDebug("Loaded %d facts for predicate=%s (access tracking updated)", len(facts), predicate)
	return facts, nil
}

// LoadAllFacts retrieves all facts, optionally filtered by type.
// Does not update access tracking (use LoadFacts for that).
func (s *LocalStore) LoadAllFacts(factType string) ([]StoredFact, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadAllFacts")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Loading all facts from cold storage (type filter=%q)", factType)

	var query string
	var args []interface{}

	if factType != "" {
		query = "SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count FROM cold_storage WHERE fact_type = ? ORDER BY priority DESC"
		args = []interface{}{factType}
	} else {
		query = "SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count FROM cold_storage ORDER BY priority DESC"
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to load all facts: %v", err)
		return nil, err
	}
	defer rows.Close()

	var facts []StoredFact
	for rows.Next() {
		var fact StoredFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt, &fact.LastAccessed, &fact.AccessCount); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
	}

	logging.StoreDebug("Loaded %d total facts from cold storage", len(facts))
	return facts, nil
}

// DeleteFact removes a fact by predicate and args.
func (s *LocalStore) DeleteFact(predicate string, args []interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logging.StoreDebug("Deleting fact from cold storage: %s", predicate)

	argsJSON, _ := json.Marshal(args)
	_, err := s.db.Exec("DELETE FROM cold_storage WHERE predicate = ? AND args = ?", predicate, string(argsJSON))
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to delete fact %s: %v", predicate, err)
		return err
	}

	logging.StoreDebug("Fact deleted from cold storage: %s", predicate)
	return nil
}

// ArchiveOldFacts moves old, rarely-accessed facts to archival storage.
// Facts are archived if they meet ALL criteria:
// - Older than olderThanDays
// - Access count below maxAccessCount
// Returns the number of facts archived.
func (s *LocalStore) ArchiveOldFacts(olderThanDays int, maxAccessCount int) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ArchiveOldFacts")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	if olderThanDays <= 0 {
		olderThanDays = 90 // Default to 90 days
	}
	if maxAccessCount < 0 {
		maxAccessCount = 5 // Default: archive facts accessed 5 times or less
	}

	logging.Store("Archiving facts older than %d days with access count <= %d", olderThanDays, maxAccessCount)

	// Start transaction for atomic move
	tx, err := s.db.Begin()
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to start archive transaction: %v", err)
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Find facts to archive
	rows, err := tx.Query(
		`SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count
		 FROM cold_storage
		 WHERE datetime(last_accessed) < datetime('now', '-' || ? || ' days')
		 AND access_count <= ?`,
		olderThanDays, maxAccessCount,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to query old facts for archival: %v", err)
		return 0, fmt.Errorf("failed to query old facts: %w", err)
	}
	defer rows.Close()

	var archivedCount int
	var idsToDelete []int64

	// Insert into archived_facts
	for rows.Next() {
		var id int64
		var predicate, argsJSON, factType string
		var priority, accessCount int
		var createdAt, updatedAt, lastAccessed time.Time

		if err := rows.Scan(&id, &predicate, &argsJSON, &factType, &priority, &createdAt, &updatedAt, &lastAccessed, &accessCount); err != nil {
			continue
		}

		// Insert into archive
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO archived_facts (predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			predicate, argsJSON, factType, priority, createdAt, updatedAt, lastAccessed, accessCount,
		)
		if err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to archive fact %s: %v", predicate, err)
			continue
		}

		idsToDelete = append(idsToDelete, id)
		archivedCount++
	}

	// Delete from cold_storage
	for _, id := range idsToDelete {
		_, err := tx.Exec("DELETE FROM cold_storage WHERE id = ?", id)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Failed to delete archived fact id=%d: %v", id, err)
			return 0, fmt.Errorf("failed to delete archived fact: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to commit archive transaction: %v", err)
		return 0, fmt.Errorf("failed to commit archive transaction: %w", err)
	}

	logging.Store("Archived %d facts from cold storage to archival tier", archivedCount)
	return archivedCount, nil
}

// GetArchivedFacts retrieves archived facts by predicate.
func (s *LocalStore) GetArchivedFacts(predicate string) ([]ArchivedFact, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetArchivedFacts")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Retrieving archived facts: predicate=%s", predicate)

	rows, err := s.db.Query(
		`SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count, archived_at
		 FROM archived_facts
		 WHERE predicate = ?
		 ORDER BY archived_at DESC`,
		predicate,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to retrieve archived facts for %s: %v", predicate, err)
		return nil, err
	}
	defer rows.Close()

	var facts []ArchivedFact
	for rows.Next() {
		var fact ArchivedFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt, &fact.LastAccessed, &fact.AccessCount, &fact.ArchivedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
	}

	logging.StoreDebug("Retrieved %d archived facts for predicate=%s", len(facts), predicate)
	return facts, nil
}

// GetAllArchivedFacts retrieves all archived facts, optionally filtered by type.
func (s *LocalStore) GetAllArchivedFacts(factType string) ([]ArchivedFact, error) {
	timer := logging.StartTimer(logging.CategoryStore, "GetAllArchivedFacts")
	defer timer.Stop()

	s.mu.RLock()
	defer s.mu.RUnlock()

	logging.StoreDebug("Retrieving all archived facts (type filter=%q)", factType)

	var query string
	var args []interface{}

	if factType != "" {
		query = `SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count, archived_at
				 FROM archived_facts WHERE fact_type = ? ORDER BY archived_at DESC`
		args = []interface{}{factType}
	} else {
		query = `SELECT id, predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count, archived_at
				 FROM archived_facts ORDER BY archived_at DESC`
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to retrieve all archived facts: %v", err)
		return nil, err
	}
	defer rows.Close()

	var facts []ArchivedFact
	for rows.Next() {
		var fact ArchivedFact
		var argsJSON string
		if err := rows.Scan(&fact.ID, &fact.Predicate, &argsJSON, &fact.FactType, &fact.Priority, &fact.CreatedAt, &fact.UpdatedAt, &fact.LastAccessed, &fact.AccessCount, &fact.ArchivedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(argsJSON), &fact.Args)
		facts = append(facts, fact)
	}

	logging.StoreDebug("Retrieved %d archived facts", len(facts))
	return facts, nil
}

// RestoreArchivedFact moves a fact from archive back to cold storage.
func (s *LocalStore) RestoreArchivedFact(predicate string, args []interface{}) error {
	timer := logging.StartTimer(logging.CategoryStore, "RestoreArchivedFact")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	logging.Store("Restoring archived fact: %s (promoting to cold storage)", predicate)

	argsJSON, _ := json.Marshal(args)

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to start restore transaction: %v", err)
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get archived fact
	var id int64
	var factType string
	var priority, accessCount int
	var createdAt, updatedAt, lastAccessed time.Time

	err = tx.QueryRow(
		"SELECT id, fact_type, priority, created_at, updated_at, last_accessed, access_count FROM archived_facts WHERE predicate = ? AND args = ?",
		predicate, string(argsJSON),
	).Scan(&id, &factType, &priority, &createdAt, &updatedAt, &lastAccessed, &accessCount)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Fact not found in archive: %s: %v", predicate, err)
		return fmt.Errorf("fact not found in archive: %w", err)
	}

	// Insert back into cold_storage
	_, err = tx.Exec(
		`INSERT OR REPLACE INTO cold_storage (predicate, args, fact_type, priority, created_at, updated_at, last_accessed, access_count)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)`,
		predicate, string(argsJSON), factType, priority, createdAt, updatedAt, accessCount,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to restore fact to cold storage: %v", err)
		return fmt.Errorf("failed to restore fact: %w", err)
	}

	// Delete from archive
	_, err = tx.Exec("DELETE FROM archived_facts WHERE id = ?", id)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to delete from archive after restore: %v", err)
		return fmt.Errorf("failed to delete from archive: %w", err)
	}

	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to commit restore transaction: %v", err)
		return fmt.Errorf("failed to commit restore transaction: %w", err)
	}

	logging.Store("Restored fact %s from archival tier to cold storage", predicate)
	return nil
}

// PurgeOldArchivedFacts permanently deletes archived facts older than specified days.
// Use with caution - this is irreversible.
func (s *LocalStore) PurgeOldArchivedFacts(olderThanDays int) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "PurgeOldArchivedFacts")
	defer timer.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	if olderThanDays <= 0 {
		return 0, fmt.Errorf("olderThanDays must be positive")
	}

	logging.Get(logging.CategoryStore).Warn("Purging archived facts older than %d days (IRREVERSIBLE)", olderThanDays)

	result, err := s.db.Exec(
		`DELETE FROM archived_facts
		 WHERE datetime(archived_at) < datetime('now', '-' || ? || ' days')`,
		olderThanDays,
	)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to purge old archived facts: %v", err)
		return 0, fmt.Errorf("failed to purge old archived facts: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	logging.Store("Purged %d archived facts older than %d days", rowsAffected, olderThanDays)
	return int(rowsAffected), nil
}

// MaintenanceCleanup performs periodic maintenance on the storage tiers.
// Returns statistics about cleanup operations.
func (s *LocalStore) MaintenanceCleanup(config MaintenanceConfig) (MaintenanceStats, error) {
	timer := logging.StartTimer(logging.CategoryStore, "MaintenanceCleanup")
	defer timer.Stop()

	logging.Store("Starting maintenance cleanup cycle")
	stats := MaintenanceStats{}

	// Archive old facts
	if config.ArchiveOlderThanDays > 0 {
		logging.StoreDebug("Archiving facts older than %d days", config.ArchiveOlderThanDays)
		archived, err := s.ArchiveOldFacts(config.ArchiveOlderThanDays, config.MaxAccessCount)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Archival failed during maintenance: %v", err)
			return stats, fmt.Errorf("archival failed: %w", err)
		}
		stats.FactsArchived = archived
	}

	// Purge very old archived facts
	if config.PurgeArchivedOlderThanDays > 0 {
		logging.StoreDebug("Purging archived facts older than %d days", config.PurgeArchivedOlderThanDays)
		purged, err := s.PurgeOldArchivedFacts(config.PurgeArchivedOlderThanDays)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("Purge failed during maintenance: %v", err)
			return stats, fmt.Errorf("purge failed: %w", err)
		}
		stats.FactsPurged = purged
	}

	// Clean old activation logs
	if config.CleanActivationLogDays > 0 {
		logging.StoreDebug("Cleaning activation logs older than %d days", config.CleanActivationLogDays)
		s.mu.Lock()
		result, err := s.db.Exec(
			`DELETE FROM activation_log
			 WHERE datetime(timestamp) < datetime('now', '-' || ? || ' days')`,
			config.CleanActivationLogDays,
		)
		s.mu.Unlock()
		if err == nil {
			rows, _ := result.RowsAffected()
			stats.ActivationLogsDeleted = int(rows)
		} else {
			logging.Get(logging.CategoryStore).Warn("Failed to clean activation logs: %v", err)
		}
	}

	// Vacuum database to reclaim space
	if config.VacuumDatabase {
		logging.StoreDebug("Running VACUUM to reclaim disk space")
		s.mu.Lock()
		_, err := s.db.Exec("VACUUM")
		s.mu.Unlock()
		if err != nil {
			logging.Get(logging.CategoryStore).Error("VACUUM failed: %v", err)
			return stats, fmt.Errorf("vacuum failed: %w", err)
		}
		stats.DatabaseVacuumed = true
	}

	logging.Store("Maintenance complete: archived=%d, purged=%d, activation_logs_deleted=%d, vacuumed=%v",
		stats.FactsArchived, stats.FactsPurged, stats.ActivationLogsDeleted, stats.DatabaseVacuumed)
	return stats, nil
}
