package shards

import "testing"

func TestObserverManager_CallbacksAndHandler(t *testing.T) {
	m := NewBackgroundObserverManager(nil)

	var called bool
	m.AddCallback(func(ObserverAssessment) { called = true })
	if len(m.callbacks) != 1 {
		t.Fatalf("AddCallback should register one callback, got %d", len(m.callbacks))
	}
	// Invoke the stored callback to confirm it is the one we registered.
	m.callbacks[0](ObserverAssessment{})
	if !called {
		t.Error("registered callback was not the one invoked")
	}

	m.SetNorthstarHandler(nil) // exercises the setter path without a real handler
}

func TestObserverManager_Assessments(t *testing.T) {
	m := NewBackgroundObserverManager(nil)

	// Seed the assessment buffer directly (same-package white-box access).
	m.assessmentBuffer = []ObserverAssessment{
		{ObserverName: "a", Score: 90},
		{ObserverName: "b", Score: 70},
		{ObserverName: "c", Score: 30},
	}
	recent := m.GetRecentAssessments(2)
	if len(recent) != 2 || recent[0].ObserverName != "b" || recent[1].ObserverName != "c" {
		t.Errorf("GetRecentAssessments(2)=%+v, want the last two (b,c)", recent)
	}
	// limit <= 0 returns the whole buffer.
	if all := m.GetRecentAssessments(0); len(all) != 3 {
		t.Errorf("GetRecentAssessments(0)=%d, want all 3", len(all))
	}

	// GetLastAssessment resolves by lowercased observer name.
	m.observers["northstar"] = &ObserverState{
		Name:           "northstar",
		LastAssessment: &ObserverAssessment{ObserverName: "northstar", Score: 88},
	}
	if got := m.GetLastAssessment("NorthStar"); got == nil || got.Score != 88 {
		t.Errorf("GetLastAssessment(NorthStar)=%+v, want score 88", got)
	}
	if got := m.GetLastAssessment("missing"); got != nil {
		t.Errorf("GetLastAssessment(missing)=%+v, want nil", got)
	}
}
