package store

import (
	"testing"
)

func TestLocalStore_ColdStorage_Extra(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// 1. Store Fact
	err = s.StoreFact("test_pred", []interface{}{"arg1"}, "test_type", 10)
	if err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	// 2. LoadAllFacts
	allFacts, err := s.LoadAllFacts("test_type")
	if err != nil {
		t.Errorf("LoadAllFacts failed: %v", err)
	}
	if len(allFacts) != 1 {
		t.Errorf("Expected 1 fact, got %d", len(allFacts))
	}
	
	allFactsEmpty, _ := s.LoadAllFacts("")
	if len(allFactsEmpty) != 1 {
		t.Errorf("Expected 1 fact, got %d", len(allFactsEmpty))
	}

	// 3. LoadFacts
	facts, err := s.LoadFacts("test_pred")
	if err != nil {
		t.Errorf("LoadFacts failed: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("Expected 1 fact, got %d", len(facts))
	}

	// 4. ArchiveOldFacts (simulate archiving by bypassing timestamps just for coverage, or modifying DB)
	_, err = s.db.Exec("UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 1")
	if err != nil {
		t.Fatalf("Failed to update db: %v", err)
	}

	archivedCount, err := s.ArchiveOldFacts(90, 5)
	if err != nil {
		t.Errorf("ArchiveOldFacts failed: %v", err)
	}
	if archivedCount != 1 {
		t.Errorf("Expected 1 fact archived, got %d", archivedCount)
	}

	// 5. GetArchivedFacts / GetAllArchivedFacts
	archivedFacts, err := s.GetArchivedFacts("test_pred")
	if err != nil {
		t.Errorf("GetArchivedFacts failed: %v", err)
	}
	if len(archivedFacts) != 1 {
		t.Errorf("Expected 1 archived fact, got %d", len(archivedFacts))
	}

	allArch, err := s.GetAllArchivedFacts("test_type")
	if err != nil || len(allArch) != 1 {
		t.Errorf("GetAllArchivedFacts failed")
	}

	allArchEmpty, err := s.GetAllArchivedFacts("")
	if err != nil || len(allArchEmpty) != 1 {
		t.Errorf("GetAllArchivedFacts empty type failed")
	}

	// 6. RestoreArchivedFact
	err = s.RestoreArchivedFact("test_pred", []interface{}{"arg1"})
	if err != nil {
		t.Errorf("RestoreArchivedFact failed: %v", err)
	}

	// 7. DeleteFact
	err = s.DeleteFact("test_pred", []interface{}{"arg1"})
	if err != nil {
		t.Errorf("DeleteFact failed: %v", err)
	}
	
	// 8. MaintenanceCleanup
	// Insert test data for maintenance
	s.db.Exec("INSERT INTO archived_facts (predicate, args, archived_at) VALUES ('to_purge', '[]', datetime('now', '-400 days'))")
	s.db.Exec("INSERT INTO activation_log (fact_id, activation_score, timestamp) VALUES ('fact1', 1.0, datetime('now', '-60 days'))")
	s.StoreFact("to_archive", []interface{}{}, "fact", 0)
	s.db.Exec("UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 1")

	config := MaintenanceConfig{
		ArchiveOlderThanDays:       90,
		MaxAccessCount:             5,
		PurgeArchivedOlderThanDays: 365,
		CleanActivationLogDays:     30,
		VacuumDatabase:             true,
	}
	stats, err := s.MaintenanceCleanup(config)
	if err != nil {
		t.Errorf("MaintenanceCleanup failed: %v", err)
	}
	if stats.FactsArchived != 1 || stats.FactsPurged != 1 || stats.ActivationLogsDeleted != 1 || !stats.DatabaseVacuumed {
		t.Errorf("Maintenance stats unexpected: %+v", stats)
	}
}
