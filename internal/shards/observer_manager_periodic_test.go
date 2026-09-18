package shards

import (
	"context"
	"sync"
	"testing"
	"time"
)

// countingNorthstarHandler stands in for the northstar handler and counts what
// reaches it, by event type.
type countingNorthstarHandler struct {
	mu     sync.Mutex
	checks int
	other  int
}

func (h *countingNorthstarHandler) HandleEvent(_ context.Context, event ObserverEvent) (*ObserverAssessment, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if event.Type == EventAlignmentCheck {
		h.checks++
	} else {
		h.other++
	}
	return nil, nil
}

func (h *countingNorthstarHandler) counts() (checks, other int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.checks, h.other
}

// A periodic alignment check with nothing to assess is an LLM call spent
// scoring an empty tick. Measured 2026-09-17 in an idle chat session: one every
// five minutes from boot, 13-43 s each, scored 10-50/100 for showing no work.
// The interval must only produce a check when some other event arrived since
// the last one -- and then exactly one, not one per tick until the next event.
func TestPeriodicCheck_FiresOnlyWhenSomethingHappenedSinceTheLastOne(t *testing.T) {
	handler := &countingNorthstarHandler{}
	mgr := NewBackgroundObserverManager(&mockObserverSpawner{})
	mgr.checkInterval = 20 * time.Millisecond
	mgr.SetNorthstarHandler(handler)
	if err := mgr.RegisterObserver("northstar"); err != nil {
		t.Fatalf("RegisterObserver: %v", err)
	}
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()

	// Idle: many intervals pass, nothing happened.
	time.Sleep(200 * time.Millisecond)
	if checks, _ := handler.counts(); checks != 0 {
		t.Fatalf("idle session received %d periodic checks in 200ms at a 20ms interval; want 0", checks)
	}

	// One event, then many more intervals.
	mgr.SendEvent(ObserverEvent{Type: EventTaskCompleted, Source: "coder", Target: "fix boot guard"})
	time.Sleep(200 * time.Millisecond)
	checks, other := handler.counts()
	if other != 1 {
		t.Errorf("the task event itself reached the handler %d times, want 1", other)
	}
	if checks != 1 {
		t.Errorf("one event produced %d periodic checks over 200ms; want exactly 1", checks)
	}
}
