//go:build integration

package shards_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/shards"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
	)
}

// ControllableSpawner implements shards.ObserverSpawner for testing.
// It allows synchronization of the "background" process.
type ControllableSpawner struct {
	mu           sync.Mutex
	spawnCalled  chan struct{}
	shouldReturn string
	delay        time.Duration
}

func NewControllableSpawner() *ControllableSpawner {
	return &ControllableSpawner{
		spawnCalled:  make(chan struct{}, 1000), // Buffered to prevent blocking
		shouldReturn: "SCORE: 90\nVISION: Great\nDEVIATIONS: none\nRECOMMENDATIONS: none",
	}
}

func (s *ControllableSpawner) SpawnObserver(ctx context.Context, observerName, task string) (string, error) {
	// Signal that we were called
	select {
	case s.spawnCalled <- struct{}{}:
	default:
		// Channel full, ignore
	}

	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return s.shouldReturn, nil
}

func (s *ControllableSpawner) WaitForCalls(t *testing.T, count int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for i := 0; i < count; i++ {
		select {
		case <-s.spawnCalled:
			// Good
		case <-time.After(time.Until(deadline)):
			t.Fatalf("Timeout waiting for call %d/%d", i+1, count)
		}
	}
}

func TestBackgroundObserverManager_Integration_EventFlow(t *testing.T) {
	spawner := NewControllableSpawner()
	mgr := shards.NewBackgroundObserverManager(spawner)

	// Register a valid observer (Northstar)
	err := mgr.RegisterObserver("northstar")
	require.NoError(t, err)

	// Verify initial state
	active := mgr.GetActiveObservers()
	require.Contains(t, active, "northstar")

	// Setup a callback to verify assessment receipt
	var wgCallback sync.WaitGroup
	wgCallback.Add(1)
	var receivedAssessment shards.ObserverAssessment

	mgr.AddCallback(func(a shards.ObserverAssessment) {
		receivedAssessment = a
		wgCallback.Done()
	})

	// Start the manager
	err = mgr.Start()
	require.NoError(t, err)
	defer mgr.Stop()

	// Send an event
	event := shards.ObserverEvent{
		Type:      shards.EventTaskStarted,
		Source:    "integration_test",
		Target:    "test_task",
		Timestamp: time.Now(),
	}
	mgr.SendEvent(event)

	// Wait for spawner to be called (verifies dispatch to background worker)
	spawner.WaitForCalls(t, 1, 2*time.Second)

	// Wait for callback to be fired (verifies assessment processing)
	done := make(chan struct{})
	go func() {
		wgCallback.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for assessment callback")
	}

	// Verify assessment details
	assert.Equal(t, "northstar", receivedAssessment.ObserverName)
	assert.Equal(t, 90, receivedAssessment.Score)
	assert.Equal(t, "Great", receivedAssessment.VisionMatch)
}

func TestBackgroundObserverManager_Integration_Concurrency(t *testing.T) {
	spawner := NewControllableSpawner()
	// Add a small delay to simulate work and increase chance of overlap
	spawner.delay = 10 * time.Millisecond

	mgr := shards.NewBackgroundObserverManager(spawner)
	require.NoError(t, mgr.RegisterObserver("northstar"))

	require.NoError(t, mgr.Start())
	defer mgr.Stop()

	// Send multiple events concurrently
	const eventCount = 50
	var wgSend sync.WaitGroup

	for i := 0; i < eventCount; i++ {
		wgSend.Add(1)
		go func(id int) {
			defer wgSend.Done()
			mgr.SendEvent(shards.ObserverEvent{
				Type:   shards.EventFileModified,
				Source: "concurrent_test",
				Target: fmt.Sprintf("file_%d", id),
			})
		}(i)
	}

	wgSend.Wait()

	// Wait for all assessments to be processed
	// Since spawner is called for each event, we expect eventCount calls
	spawner.WaitForCalls(t, eventCount, 10*time.Second)

	// Check internal buffer
	// Use Eventually because WaitForCalls signals *start* of execution, but assessments are recorded after *completion*
	// and there is a delay in the spawner.
	require.Eventually(t, func() bool {
		return len(mgr.GetRecentAssessments(1000)) == eventCount
	}, 5*time.Second, 100*time.Millisecond, "Should have recorded all assessments")
}
