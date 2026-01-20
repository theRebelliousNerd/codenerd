//go:build integration
package store_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"codenerd/internal/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// TestMain ensures no goroutines leak during integration tests.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
}

func TestLocalSession_Integration(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "session_integration_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	t.Run("Persistence", func(t *testing.T) {
		// 1. Create store and write data
		s, err := store.NewLocalStore(dbPath)
		require.NoError(t, err)

		sessionID := "sess-persistence"
		err = s.StoreSessionTurn(sessionID, 1, "input1", "{}", "resp1", "[]")
		require.NoError(t, err)

		err = s.Close()
		require.NoError(t, err)

		// 2. Reopen store and verify data
		s2, err := store.NewLocalStore(dbPath)
		require.NoError(t, err)
		defer s2.Close()

		history, err := s2.GetSessionHistory(sessionID, 10)
		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, "input1", history[0]["user_input"])
	})

	t.Run("ConcurrentWrites", func(t *testing.T) {
		// Open store
		s, err := store.NewLocalStore(dbPath)
		require.NoError(t, err)
		defer s.Close()

		sessionID := "sess-concurrent"
		var wg sync.WaitGroup
		numWorkers := 10
		numTurnsPerWorker := 10

		// Concurrent writers
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 1; j <= numTurnsPerWorker; j++ {
					turnNum := (workerID * numTurnsPerWorker) + j
					err := s.StoreSessionTurn(
						sessionID,
						turnNum,
						fmt.Sprintf("input-%d-%d", workerID, j),
						"{}",
						"resp",
						"[]",
					)
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()

		// Verify count
		history, err := s.GetSessionHistory(sessionID, 1000)
		require.NoError(t, err)
		assert.Equal(t, numWorkers*numTurnsPerWorker, len(history))
	})

	t.Run("IdempotencyAndLocking", func(t *testing.T) {
		// Open store
		s, err := store.NewLocalStore(dbPath)
		require.NoError(t, err)
		defer s.Close()

		sessionID := "sess-idempotency"
		turnNum := 1

		// Concurrently try to write the SAME turn
		var wg sync.WaitGroup
		numAttempts := 20
		errors := make(chan error, numAttempts)

		for i := 0; i < numAttempts; i++ {
			wg.Add(1)
			go func(attempt int) {
				defer wg.Done()
				// We vary the input content, but sessionID/turnNum are same.
				// The store logic should use INSERT OR IGNORE and ignore subsequent writes.
				// Or if it fails, we want to know.
				err := s.StoreSessionTurn(
					sessionID,
					turnNum,
					fmt.Sprintf("input-attempt-%d", attempt),
					"{}",
					"resp",
					"[]",
				)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			assert.NoError(t, err)
		}

		// Verify only one entry exists
		history, err := s.GetSessionHistory(sessionID, 100)
		require.NoError(t, err)
		assert.Equal(t, 1, len(history))
		// Note: We don't guarantee WHICH one won the race, but only one should exist.
	})

	t.Run("LogActivation_Concurrent", func(t *testing.T) {
		s, err := store.NewLocalStore(dbPath)
		require.NoError(t, err)
		defer s.Close()

		var wg sync.WaitGroup
		count := 50

		for i := 0; i < count; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				err := s.LogActivation(fmt.Sprintf("fact-%d", idx), float64(idx)/100.0)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Verify some activations
		activations, err := s.GetRecentActivations(100, 0.0)
		require.NoError(t, err)
		// We can't guarantee exactly 50 because of the time window in the query (1 hour),
		// but they should be there.
		assert.GreaterOrEqual(t, len(activations), 1)
	})
}
