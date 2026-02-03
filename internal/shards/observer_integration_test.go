//go:build integration
package shards_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/shards"
	"github.com/stretchr/testify/suite"
)

type SlowObserverSpawner struct {
	Delay time.Duration
}

func (s *SlowObserverSpawner) SpawnObserver(ctx context.Context, observerName, task string) (string, error) {
	select {
	case <-time.After(s.Delay):
		return "SCORE: 90\nVISION: Integration Test\nDEVIATIONS: none\nRECOMMENDATIONS: none", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

type MockNorthstarHandler struct {
	Called bool
	mu     sync.Mutex
}

func (m *MockNorthstarHandler) HandleEvent(ctx context.Context, event shards.ObserverEvent) (*shards.ObserverAssessment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Called = true
	return &shards.ObserverAssessment{
		ObserverName: "Northstar",
		Score:        95,
		Level:        shards.LevelProceed,
		VisionMatch:  "Direct Handler Works",
	}, nil
}

type ObserverManagerIntegrationSuite struct {
	suite.Suite
	mgr     *shards.BackgroundObserverManager
	spawner *SlowObserverSpawner
}

func (s *ObserverManagerIntegrationSuite) SetupTest() {
	s.spawner = &SlowObserverSpawner{Delay: 10 * time.Millisecond}
	s.mgr = shards.NewBackgroundObserverManager(s.spawner)
}

func (s *ObserverManagerIntegrationSuite) TearDownTest() {
	s.mgr.Stop()
}

func (s *ObserverManagerIntegrationSuite) TestConcurrentEventProcessing() {
	err := s.mgr.RegisterObserver("northstar")
	s.Require().NoError(err)

	err = s.mgr.Start()
	s.Require().NoError(err)

	// Use a number smaller than the channel buffer (100) to ensure no drops
	const numEvents = 50
	var wg sync.WaitGroup
	wg.Add(numEvents)

	// Callback to track completion
	s.mgr.AddCallback(func(assessment shards.ObserverAssessment) {
		wg.Done()
	})

	// Send events concurrently
	for i := 0; i < numEvents; i++ {
		go func(id int) {
			s.mgr.SendEvent(shards.ObserverEvent{
				Type:   shards.EventTaskStarted,
				Source: fmt.Sprintf("worker-%d", id),
				Target: "integration-test",
			})
		}(i)
	}

	// Wait for all assessments with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		s.Fail("Timed out waiting for concurrent events")
	}

	// Verify assessments
	// Note: We might get more assessments if other events fired (e.g. periodic),
	// but strictly we expect at least numEvents.
	assessments := s.mgr.GetRecentAssessments(numEvents)
	s.Require().GreaterOrEqual(len(assessments), numEvents)

	// Check the last few match our expectation
	for _, a := range assessments {
		if a.VisionMatch == "Integration Test" {
			s.Equal(90, a.Score)
		}
	}
}

func (s *ObserverManagerIntegrationSuite) TestNorthstarDirectHandler() {
	handler := &MockNorthstarHandler{}
	s.mgr.SetNorthstarHandler(handler)

	err := s.mgr.RegisterObserver("northstar")
	s.Require().NoError(err)

	err = s.mgr.Start()
	s.Require().NoError(err)

	var wg sync.WaitGroup
	wg.Add(1)
	s.mgr.AddCallback(func(a shards.ObserverAssessment) {
		wg.Done()
	})

	s.mgr.SendEvent(shards.ObserverEvent{
		Type:   shards.EventUserIntent,
		Source: "user",
	})

	// Wait
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		s.Fail("Timed out waiting for direct handler")
	}

	handler.mu.Lock()
	s.True(handler.Called)
	handler.mu.Unlock()

	last := s.mgr.GetLastAssessment("northstar")
	s.Require().NotNil(last)
	s.Equal(95, last.Score)
	s.Equal("Direct Handler Works", last.VisionMatch)
}

func (s *ObserverManagerIntegrationSuite) TestGracefulShutdown() {
	err := s.mgr.RegisterObserver("northstar")
	s.Require().NoError(err)
	s.mgr.Start()

	// Send one event to make sure it's running
	s.mgr.SendEvent(shards.ObserverEvent{Type: shards.EventTaskStarted})

	s.mgr.Stop()

	// Should not panic and should be safe to call
	s.mgr.SendEvent(shards.ObserverEvent{Type: shards.EventTaskCompleted})
}

func TestObserverManagerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ObserverManagerIntegrationSuite))
}
