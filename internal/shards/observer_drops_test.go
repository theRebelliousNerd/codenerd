package shards

import (
	"testing"
)

// An event the observers' queue cannot take is counted rather than lost
// without a trace. SendEvent used to drop it with only a comment ("could log
// this"): nothing could tell a quiet session from one whose events never
// reached an observer.
func TestSendEvent_CountsWhatAFullQueueDrops(t *testing.T) {
	m := NewBackgroundObserverManager(nil)
	// Enabled with no loop draining the queue: exactly the saturated state.
	m.enabled = true
	capacity := cap(m.eventChan)
	for range capacity + 3 {
		m.SendEvent(ObserverEvent{Type: EventTaskCompleted, Source: "test"})
	}
	if got := m.DroppedEvents(); got != 3 {
		t.Fatalf("DroppedEvents() = %d after %d sends into a queue of %d, want 3", got, capacity+3, capacity)
	}
	if got := len(m.eventChan); got != capacity {
		t.Fatalf("queued %d events, want the queue's capacity %d", got, capacity)
	}
}

// GetLastAssessment hands back a snapshot. The manager's own record was
// returned by pointer, so a caller could rewrite what the observer had said.
func TestGetLastAssessment_ReturnsASnapshot(t *testing.T) {
	m := NewBackgroundObserverManager(nil)
	if err := m.RegisterObserver("northstar"); err != nil {
		t.Skipf("northstar is not a registered observer here: %v", err)
	}
	m.recordAssessment("northstar", ObserverAssessment{
		ObserverName: "northstar", Score: 40, Level: LevelClarify,
		Suggestions: []string{"ask first"}, Metadata: map[string]string{"k": "v"},
	})
	got := m.GetLastAssessment("northstar")
	if got == nil {
		t.Fatal("no assessment recorded")
	}
	got.Score = 100
	got.Suggestions[0] = "rewritten"
	got.Metadata["k"] = "rewritten"

	again := m.GetLastAssessment("northstar")
	if again.Score != 40 || again.Suggestions[0] != "ask first" || again.Metadata["k"] != "v" {
		t.Fatalf("a caller's edit reached the manager's record: %+v", again)
	}
}
