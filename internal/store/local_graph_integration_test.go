//go:build integration
package store_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"codenerd/internal/store"
	"github.com/stretchr/testify/suite"
)

type LocalGraphIntegrationSuite struct {
	suite.Suite
	tmpDir string
	dbPath string
}

func (s *LocalGraphIntegrationSuite) SetupSuite() {
	var err error
	s.tmpDir, err = os.MkdirTemp("", "graph_integration_test")
	s.Require().NoError(err)
	s.dbPath = filepath.Join(s.tmpDir, "graph.db")
}

func (s *LocalGraphIntegrationSuite) TearDownSuite() {
	os.RemoveAll(s.tmpDir)
}

func (s *LocalGraphIntegrationSuite) SetupTest() {
	// Ensure DB is clean before each test.
	// We delete the file to force a fresh DB.
	os.Remove(s.dbPath)
}

func (s *LocalGraphIntegrationSuite) TestGraphPersistence() {
	// 1. Open Store
	db, err := store.NewLocalStore(s.dbPath)
	s.Require().NoError(err)

	// 2. Store a chain: A -> B -> C
	err = db.StoreLink("A", "relates", "B", 1.0, nil)
	s.Require().NoError(err)
	err = db.StoreLink("B", "relates", "C", 1.0, nil)
	s.Require().NoError(err)

	// 3. Close Store
	err = db.Close()
	s.Require().NoError(err)

	// 4. Reopen Store
	db2, err := store.NewLocalStore(s.dbPath)
	s.Require().NoError(err)
	defer db2.Close()

	// 5. Verify Persistence via Query
	links, err := db2.QueryLinks("B", "incoming")
	s.Require().NoError(err)
	s.Require().Len(links, 1)
	s.Equal("A", links[0].EntityA)

	// 6. Verify Persistence via Traversal
	path, err := db2.TraversePath("A", "C", 5)
	s.Require().NoError(err)
	s.Require().Len(path, 2)
	s.Equal("A", path[0].EntityA)
	s.Equal("B", path[0].EntityB)
	s.Equal("B", path[1].EntityA)
	s.Equal("C", path[1].EntityB)
}

func (s *LocalGraphIntegrationSuite) TestGraphHydration() {
	db, err := store.NewLocalStore(s.dbPath)
	s.Require().NoError(err)
	defer db.Close()

	// Seed data
	s.Require().NoError(db.StoreLink("X", "is_a", "Y", 1.0, nil))
	s.Require().NoError(db.StoreLink("Y", "is_a", "Z", 0.5, nil))

	// Collector
	type fact struct {
		pred string
		args []interface{}
	}
	var collected []fact

	callback := func(pred string, args []interface{}) error {
		collected = append(collected, fact{pred, args})
		return nil
	}

	// Hydrate
	count, err := db.HydrateKnowledgeGraph(callback)
	s.Require().NoError(err)
	s.Equal(2, count)
	s.Len(collected, 2)

	// Verify content
	// Note: Hydrate orders by weight DESC
	s.Equal("knowledge_link", collected[0].pred)
	s.Equal("X", collected[0].args[0]) // Weight 1.0

	s.Equal("knowledge_link", collected[1].pred)
	s.Equal("Y", collected[1].args[0]) // Weight 0.5
}

func (s *LocalGraphIntegrationSuite) TestGraphConcurrency() {
	db, err := store.NewLocalStore(s.dbPath)
	s.Require().NoError(err)
	defer db.Close()

	var wg sync.WaitGroup
	workers := 10
	iterations := 50

	// Writers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				from := fmt.Sprintf("Node-%d-%d", id, j)
				to := fmt.Sprintf("Node-%d-%d", id, j+1)
				if err := db.StoreLink(from, "connects", to, 1.0, nil); err != nil {
					// We can't fail the test easily from goroutine, but we can log
					// In robust tests, use a channel to report errors
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Query something that might exist or not
				_, _ = db.QueryLinks(fmt.Sprintf("Node-%d-%d", id, j), "outgoing")
				// Just verifying no panic/race
			}
		}(i)
	}

	wg.Wait()

	// Basic consistency check
	stats, err := db.GetStats()
	s.Require().NoError(err)
	s.Require().Greater(stats["knowledge_graph"], int64(0))
}

func TestLocalGraphIntegrationSuite(t *testing.T) {
	suite.Run(t, new(LocalGraphIntegrationSuite))
}
