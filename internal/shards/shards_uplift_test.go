package shards

import (
	"testing"
)

// The advisor list feeds user-visible delegation output: map order would
// shuffle the consultation phase from run to run.
func TestGetStrategicAdvisorsFor_Sorted(t *testing.T) {
	for i := 0; i < 5; i++ {
		advisors := GetStrategicAdvisorsFor("coder")
		for j := 1; j < len(advisors); j++ {
			if advisors[j-1] > advisors[j] {
				t.Fatalf("advisors not sorted: %v", advisors)
			}
		}
	}
}

// The assessment Score documents 0-100: a model writing SCORE: 150 or
// SCORE: -20 must be clamped onto the scale, not stored off it.
func TestParseAssessment_ScoreClamped(t *testing.T) {
	m := NewBackgroundObserverManager(nil)
	event := ObserverEvent{Type: EventTaskCompleted, Target: "x"}
	if got := m.parseAssessment("obs", event, "SCORE: 150").Score; got != 100 {
		t.Errorf("150 -> %d, want 100", got)
	}
	if got := m.parseAssessment("obs", event, "SCORE: -20").Score; got != 0 {
		t.Errorf("-20 -> %d, want 0", got)
	}
	if got := m.parseAssessment("obs", event, "SCORE: 75").Score; got != 75 {
		t.Errorf("75 -> %d, want 75", got)
	}
}

// GetActiveObservers reads a map registry: the list must come out sorted,
// not in iteration order.
func TestGetActiveObservers_Sorted(t *testing.T) {
	m := NewBackgroundObserverManager(nil)
	for _, name := range []string{"zebra", "alpha", "northstar"} {
		m.observers[name] = &ObserverState{Name: name, Active: true}
	}
	m.observers["sleepy"] = &ObserverState{Name: "sleepy", Active: false}
	for i := 0; i < 5; i++ {
		got := m.GetActiveObservers()
		if len(got) != 3 || got[0] != "alpha" || got[1] != "northstar" || got[2] != "zebra" {
			t.Fatalf("got %v, want [alpha northstar zebra]", got)
		}
	}
}
