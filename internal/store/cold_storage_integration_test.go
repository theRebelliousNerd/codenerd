//go:build integration
package store_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"codenerd/internal/store"
	"github.com/stretchr/testify/suite"
	_ "github.com/mattn/go-sqlite3"
)

type ColdStorageSuite struct {
	suite.Suite
	tmpDir string
	dbPath string
	store  *store.LocalStore
	db     *sql.DB
}

func (s *ColdStorageSuite) SetupSuite() {
	var err error
	s.tmpDir, err = os.MkdirTemp("", "cold_storage_test")
	s.Require().NoError(err)
	s.dbPath = filepath.Join(s.tmpDir, "test.db")

	s.store, err = store.NewLocalStore(s.dbPath)
	s.Require().NoError(err)
	s.db = s.store.GetDB()
}

func (s *ColdStorageSuite) TearDownSuite() {
	if s.store != nil {
		s.store.Close()
	}
	os.RemoveAll(s.tmpDir)
}

func (s *ColdStorageSuite) SetupTest() {
	// Truncate tables to ensure a clean slate before each test
	tables := []string{"cold_storage", "archived_facts", "activation_log"}
	for _, table := range tables {
		_, err := s.db.Exec(fmt.Sprintf("DELETE FROM %s", table))
		s.Require().NoError(err)
	}
}

func (s *ColdStorageSuite) TestLifecycle_HappyPath() {
	// 1. Store a fact
	predicate := "user_preference"
	args := []interface{}{"theme", "dark"}
	err := s.store.StoreFact(predicate, args, "preference", 10)
	s.Require().NoError(err)

	// 2. Load it
	facts, err := s.store.LoadFacts(predicate)
	s.Require().NoError(err)
	s.Require().Len(facts, 1)
	s.Equal("preference", facts[0].FactType)
	s.Equal(10, facts[0].Priority)

	// Manually backdate the access time to simulate passage of time,
	// rather than relying on sleep or immediate updates.
	// This ensures the next LoadFacts (which updates LastAccessed) results in a measurably different time if we cared,
	// but here we primarily care about AccessCount incrementing.
	_, err = s.db.Exec("UPDATE cold_storage SET last_accessed = datetime('now', '-1 minute') WHERE id = ?", facts[0].ID)
	s.Require().NoError(err)

	// Verify access tracking update
	factsAfter, err := s.store.LoadFacts(predicate)
	s.Require().NoError(err)
	s.Require().Greater(factsAfter[0].AccessCount, facts[0].AccessCount)

	// 3. Force update timestamps to simulate age for archival
	_, err = s.db.Exec(`
		UPDATE cold_storage
		SET last_accessed = datetime('now', '-100 days'),
		    created_at = datetime('now', '-100 days')
		WHERE id = ?`, facts[0].ID)
	s.Require().NoError(err)

	// 4. Archive it
	// Archive facts older than 90 days, accessed <= 5 times
	count, err := s.store.ArchiveOldFacts(90, 5)
	s.Require().NoError(err)
	s.Equal(1, count)

	// Verify it's gone from cold storage
	facts, err = s.store.LoadFacts(predicate)
	s.Require().NoError(err)
	s.Len(facts, 0)

	// Verify it's in archival
	archivedFacts, err := s.store.GetArchivedFacts(predicate)
	s.Require().NoError(err)
	s.Len(archivedFacts, 1)
	s.Equal("preference", archivedFacts[0].FactType)

	// 5. Restore it
	err = s.store.RestoreArchivedFact(predicate, args)
	s.Require().NoError(err)

	// Verify it's back in cold storage
	facts, err = s.store.LoadFacts(predicate)
	s.Require().NoError(err)
	s.Len(facts, 1)

	// Verify it's gone from archival
	archivedFacts, err = s.store.GetArchivedFacts(predicate)
	s.Require().NoError(err)
	s.Len(archivedFacts, 0)
}

func (s *ColdStorageSuite) TestArchivalLogic() {
	// Create facts with different ages and access counts

	// Case 1: Old enough, low access -> Should Archive
	s.store.StoreFact("fact_archive", []interface{}{1}, "fact", 0)
	s.db.Exec("UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 1 WHERE predicate = 'fact_archive'")

	// Case 2: Old enough, high access -> Should Keep
	s.store.StoreFact("fact_keep_popular", []interface{}{2}, "fact", 0)
	s.db.Exec("UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 100 WHERE predicate = 'fact_keep_popular'")

	// Case 3: New, low access -> Should Keep
	s.store.StoreFact("fact_keep_new", []interface{}{3}, "fact", 0)
	// Default timestamp is now, access count 0

	count, err := s.store.ArchiveOldFacts(90, 5)
	s.Require().NoError(err)
	s.Equal(1, count)

	// Verify results
	archived, _ := s.store.GetArchivedFacts("fact_archive")
	s.Len(archived, 1)

	keptPopular, _ := s.store.LoadFacts("fact_keep_popular")
	s.Len(keptPopular, 1)

	keptNew, _ := s.store.LoadFacts("fact_keep_new")
	s.Len(keptNew, 1)
}

func (s *ColdStorageSuite) TestPurgeLogic() {
	// Insert archived facts manually
	// 1. Very old archived fact -> Should Purge
	s.db.Exec(`INSERT INTO archived_facts (predicate, args, archived_at) VALUES ('old_archived', '[]', datetime('now', '-400 days'))`)

	// 2. Recently archived fact -> Should Keep
	s.db.Exec(`INSERT INTO archived_facts (predicate, args, archived_at) VALUES ('new_archived', '[]', datetime('now', '-10 days'))`)

	// Purge older than 365 days
	count, err := s.store.PurgeOldArchivedFacts(365)
	s.Require().NoError(err)
	s.Equal(1, count)

	// Verify
	var exists int
	s.db.QueryRow("SELECT COUNT(*) FROM archived_facts WHERE predicate = 'old_archived'").Scan(&exists)
	s.Equal(0, exists)

	s.db.QueryRow("SELECT COUNT(*) FROM archived_facts WHERE predicate = 'new_archived'").Scan(&exists)
	s.Equal(1, exists)
}

func (s *ColdStorageSuite) TestMaintenanceCleanup() {
	// Setup:
	// 1. Fact to archive
	s.store.StoreFact("to_archive", []interface{}{}, "fact", 0)
	s.db.Exec("UPDATE cold_storage SET last_accessed = datetime('now', '-100 days'), access_count = 1 WHERE predicate = 'to_archive'")

	// 2. Fact to purge (already in archive)
	s.db.Exec(`INSERT INTO archived_facts (predicate, args, archived_at) VALUES ('to_purge', '[]', datetime('now', '-400 days'))`)

	// 3. Activation log to clean
	s.db.Exec(`INSERT INTO activation_log (fact_id, activation_score, timestamp) VALUES ('fact1', 1.0, datetime('now', '-60 days'))`)

	config := store.MaintenanceConfig{
		ArchiveOlderThanDays:       90,
		MaxAccessCount:             5,
		PurgeArchivedOlderThanDays: 365,
		CleanActivationLogDays:     30,
		VacuumDatabase:             true,
	}

	stats, err := s.store.MaintenanceCleanup(config)
	s.Require().NoError(err)

	s.Equal(1, stats.FactsArchived)
	s.Equal(1, stats.FactsPurged)
	s.Equal(1, stats.ActivationLogsDeleted)
	s.True(stats.DatabaseVacuumed)
}

func (s *ColdStorageSuite) TestConcurrency_SafeAccess() {
	var wg sync.WaitGroup
	workers := 10
	iterations := 50
	predicate := "concurrent_fact"

	// Channel to collect errors from goroutines
	errChan := make(chan error, workers*iterations)

	// Start concurrent readers and writers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Alternating read/write
				if j%2 == 0 {
					args := []interface{}{id, j}
					if err := s.store.StoreFact(predicate, args, "concurrent", 1); err != nil {
						errChan <- fmt.Errorf("StoreFact failed: %w", err)
					}
				} else {
					if _, err := s.store.LoadFacts(predicate); err != nil {
						errChan <- fmt.Errorf("LoadFacts failed: %w", err)
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check if any errors occurred
	for err := range errChan {
		s.Fail("Concurrent operation error", err.Error())
	}

	// Verify final state is consistent (no crashes, data exists)
	facts, err := s.store.LoadFacts(predicate)
	s.Require().NoError(err)
	s.Require().NotEmpty(facts)
}

func TestColdStorageSuite(t *testing.T) {
	suite.Run(t, new(ColdStorageSuite))
}
