package store

import (
	"os"
	"testing"
	"time"
)

func TestArchivalTier(t *testing.T) {
	// Create temporary database
	dbPath := "test_archival.db"
	defer os.Remove(dbPath)

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Store some test facts
	facts := []struct {
		predicate string
		args      []interface{}
		factType  string
		priority  int
	}{
		{"user_preference", []interface{}{"theme", "dark"}, "preference", 10},
		{"learned_pattern", []interface{}{"bug_fix", "common_error"}, "pattern", 5},
		{"session_data", []interface{}{"session_123", "state"}, "fact", 1},
	}

	for _, f := range facts {
		err := store.StoreFact(f.predicate, f.args, f.factType, f.priority)
		if err != nil {
			t.Errorf("Failed to store fact: %v", err)
		}
	}

	// Verify facts are stored
	allFacts, err := store.LoadAllFacts("")
	if err != nil {
		t.Fatalf("Failed to load facts: %v", err)
	}
	if len(allFacts) != 3 {
		t.Errorf("Expected 3 facts, got %d", len(allFacts))
	}

	// Manually update last_accessed to simulate old facts
	store.mu.Lock()
	_, err = store.db.Exec(
		"UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 2 WHERE predicate = ?",
		"session_data",
	)
	store.mu.Unlock()
	if err != nil {
		t.Fatalf("Failed to update last_accessed: %v", err)
	}

	// Archive old facts (older than 90 days, accessed <= 5 times)
	archived, err := store.ArchiveOldFacts(90, 5)
	if err != nil {
		t.Fatalf("Failed to archive facts: %v", err)
	}
	if archived != 1 {
		t.Errorf("Expected to archive 1 fact, archived %d", archived)
	}

	// Verify fact was moved to archive
	archivedFacts, err := store.GetArchivedFacts("session_data")
	if err != nil {
		t.Fatalf("Failed to get archived facts: %v", err)
	}
	if len(archivedFacts) != 1 {
		t.Errorf("Expected 1 archived fact, got %d", len(archivedFacts))
	}

	// Verify it's no longer in cold storage
	coldFacts, err := store.LoadFacts("session_data")
	if err != nil {
		t.Fatalf("Failed to load facts: %v", err)
	}
	if len(coldFacts) != 0 {
		t.Errorf("Expected 0 facts in cold storage, got %d", len(coldFacts))
	}

	// Verify other facts are still in cold storage
	remainingFacts, err := store.LoadAllFacts("")
	if err != nil {
		t.Fatalf("Failed to load all facts: %v", err)
	}
	if len(remainingFacts) != 2 {
		t.Errorf("Expected 2 remaining facts, got %d", len(remainingFacts))
	}
}

func TestRestoreArchivedFact(t *testing.T) {
	dbPath := "test_restore.db"
	defer os.Remove(dbPath)

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Store and archive a fact
	args := []interface{}{"test_key", "test_value"}
	err = store.StoreFact("test_predicate", args, "fact", 5)
	if err != nil {
		t.Fatalf("Failed to store fact: %v", err)
	}

	// Manually archive it
	store.mu.Lock()
	_, err = store.db.Exec(
		"UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 1",
	)
	store.mu.Unlock()
	if err != nil {
		t.Fatalf("Failed to update fact: %v", err)
	}

	archived, err := store.ArchiveOldFacts(90, 5)
	if err != nil || archived != 1 {
		t.Fatalf("Failed to archive fact: %v", err)
	}

	// Restore the fact
	err = store.RestoreArchivedFact("test_predicate", args)
	if err != nil {
		t.Fatalf("Failed to restore fact: %v", err)
	}

	// Verify it's back in cold storage
	facts, err := store.LoadFacts("test_predicate")
	if err != nil {
		t.Fatalf("Failed to load facts: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("Expected 1 restored fact, got %d", len(facts))
	}

	// Verify it's no longer in archive
	archivedFacts, err := store.GetArchivedFacts("test_predicate")
	if err != nil {
		t.Fatalf("Failed to get archived facts: %v", err)
	}
	if len(archivedFacts) != 0 {
		t.Errorf("Expected 0 archived facts, got %d", len(archivedFacts))
	}
}

func TestAccessTracking(t *testing.T) {
	dbPath := "test_access.db"
	defer os.Remove(dbPath)

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Store a fact
	args := []interface{}{"key", "value"}
	err = store.StoreFact("test_access", args, "fact", 5)
	if err != nil {
		t.Fatalf("Failed to store fact: %v", err)
	}

	// Load it multiple times to track access
	for i := 0; i < 3; i++ {
		_, err = store.LoadFacts("test_access")
		if err != nil {
			t.Fatalf("Failed to load facts: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	}

	// Check access count increased
	facts, err := store.LoadAllFacts("") // Use LoadAllFacts to avoid incrementing count
	if err != nil {
		t.Fatalf("Failed to load all facts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("Expected 1 fact, got %d", len(facts))
	}

	// Access count should be 3 (from LoadFacts calls)
	if facts[0].AccessCount != 3 {
		t.Errorf("Expected access count 3, got %d", facts[0].AccessCount)
	}
}

func TestMaintenanceCleanup(t *testing.T) {
	dbPath := "test_maintenance.db"
	defer os.Remove(dbPath)

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test data
	for i := 0; i < 5; i++ {
		args := []interface{}{"key", i}
		err := store.StoreFact("test_fact", args, "fact", i)
		if err != nil {
			t.Fatalf("Failed to store fact: %v", err)
		}
	}

	// Simulate old facts
	store.mu.Lock()
	_, err = store.db.Exec(
		"UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 1",
	)
	store.mu.Unlock()
	if err != nil {
		t.Fatalf("Failed to update facts: %v", err)
	}

	// Add some activation logs
	for i := 0; i < 10; i++ {
		err := store.LogActivation("fact_1", 0.5)
		if err != nil {
			t.Fatalf("Failed to log activation: %v", err)
		}
	}

	// Simulate old activation logs
	store.mu.Lock()
	_, err = store.db.Exec(
		"UPDATE activation_log SET timestamp = datetime('now', '-40 days')",
	)
	store.mu.Unlock()
	if err != nil {
		t.Fatalf("Failed to update activation logs: %v", err)
	}

	// Run maintenance
	config := MaintenanceConfig{
		ArchiveOlderThanDays:       90,
		MaxAccessCount:             5,
		PurgeArchivedOlderThanDays: 0, // Don't purge in this test
		CleanActivationLogDays:     30,
		VacuumDatabase:             true,
	}

	stats, err := store.MaintenanceCleanup(config)
	if err != nil {
		t.Fatalf("Maintenance cleanup failed: %v", err)
	}

	// Verify statistics
	if stats.FactsArchived != 5 {
		t.Errorf("Expected 5 facts archived, got %d", stats.FactsArchived)
	}

	if stats.ActivationLogsDeleted != 10 {
		t.Errorf("Expected 10 activation logs deleted, got %d", stats.ActivationLogsDeleted)
	}

	if !stats.DatabaseVacuumed {
		t.Error("Expected database to be vacuumed")
	}
}

func TestPurgeOldArchivedFacts(t *testing.T) {
	dbPath := "test_purge.db"
	defer os.Remove(dbPath)

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Store and archive facts
	for i := 0; i < 3; i++ {
		args := []interface{}{"key", i}
		err := store.StoreFact("test_purge", args, "fact", i)
		if err != nil {
			t.Fatalf("Failed to store fact: %v", err)
		}
	}

	// Archive all facts
	store.mu.Lock()
	_, err = store.db.Exec(
		"UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 1",
	)
	store.mu.Unlock()

	archived, err := store.ArchiveOldFacts(90, 5)
	if err != nil || archived != 3 {
		t.Fatalf("Failed to archive facts: %v, archived: %d", err, archived)
	}

	// Simulate very old archived facts
	store.mu.Lock()
	_, err = store.db.Exec(
		"UPDATE archived_facts SET archived_at = datetime('now', '-200 days')",
	)
	store.mu.Unlock()
	if err != nil {
		t.Fatalf("Failed to update archived_at: %v", err)
	}

	// Purge old archived facts (older than 180 days)
	purged, err := store.PurgeOldArchivedFacts(180)
	if err != nil {
		t.Fatalf("Failed to purge: %v", err)
	}

	if purged != 3 {
		t.Errorf("Expected 3 facts purged, got %d", purged)
	}

	// Verify archive is empty
	allArchived, err := store.GetAllArchivedFacts("")
	if err != nil {
		t.Fatalf("Failed to get archived facts: %v", err)
	}
	if len(allArchived) != 0 {
		t.Errorf("Expected 0 archived facts after purge, got %d", len(allArchived))
	}
}
